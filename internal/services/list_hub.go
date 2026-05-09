package services

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

// ListHub manages SSE subscribers for shared shopping lists.
// When any list mutation occurs, Broadcast notifies all open connections.

type ListEvent struct {
	Type    string `json:"type"`    // "item_added", "item_updated", "item_deleted", "list_updated"
	ListID  string `json:"list_id"`
	Payload any    `json:"payload,omitempty"`
}

type subscriber struct {
	ch chan string
}

type ListHub struct {
	mu   sync.RWMutex
	subs map[uuid.UUID][]*subscriber
}

var GlobalHub = &ListHub{
	subs: make(map[uuid.UUID][]*subscriber),
}

func (h *ListHub) Subscribe(listID uuid.UUID) (chan string, func()) {
	sub := &subscriber{ch: make(chan string, 8)}
	h.mu.Lock()
	h.subs[listID] = append(h.subs[listID], sub)
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.subs[listID]
		for i, s := range list {
			if s == sub {
				h.subs[listID] = append(list[:i], list[i+1:]...)
				close(sub.ch)
				break
			}
		}
		if len(h.subs[listID]) == 0 {
			delete(h.subs, listID)
		}
	}

	return sub.ch, unsubscribe
}

func (h *ListHub) Broadcast(listID uuid.UUID, event ListEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Warn("hub marshal error", slog.String("error", err.Error()))
		return
	}
	msg := "data: " + string(data) + "\n\n"

	h.mu.RLock()
	subs := make([]*subscriber, len(h.subs[listID]))
	copy(subs, h.subs[listID])
	h.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- msg:
		default:
			// client too slow — drop
		}
	}
}
