package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type slidingWindow struct {
	mu        sync.Mutex
	attempts  []time.Time
	maxHits   int
	windowDur time.Duration
}

func (sw *slidingWindow) allow() bool {
	now := time.Now()
	sw.mu.Lock()
	defer sw.mu.Unlock()
	cutoff := now.Add(-sw.windowDur)
	valid := sw.attempts[:0]
	for _, t := range sw.attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	sw.attempts = valid
	if len(sw.attempts) >= sw.maxHits {
		return false
	}
	sw.attempts = append(sw.attempts, now)
	return true
}

type ipLimiter struct {
	mu      sync.Mutex
	entries map[string]*slidingWindow
	maxHits int
	window  time.Duration
}

func newIPLimiter(maxHits int, window time.Duration) *ipLimiter {
	l := &ipLimiter{
		entries: make(map[string]*slidingWindow),
		maxHits: maxHits,
		window:  window,
	}
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			l.mu.Lock()
			for ip, sw := range l.entries {
				sw.mu.Lock()
				if len(sw.attempts) == 0 {
					delete(l.entries, ip)
				}
				sw.mu.Unlock()
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func (l *ipLimiter) get(ip string) *slidingWindow {
	l.mu.Lock()
	defer l.mu.Unlock()
	sw, ok := l.entries[ip]
	if !ok {
		sw = &slidingWindow{maxHits: l.maxHits, windowDur: l.window}
		l.entries[ip] = sw
	}
	return sw
}

func GlobalRateLimiter(maxHits int, window time.Duration) gin.HandlerFunc {
	limiter := newIPLimiter(maxHits, window)
	return func(c *gin.Context) {
		if !limiter.get(c.ClientIP()).allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}

func StrictRateLimiter(maxHits int, window time.Duration) gin.HandlerFunc {
	limiter := newIPLimiter(maxHits, window)
	return func(c *gin.Context) {
		if !limiter.get(c.ClientIP()).allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, please try again later"})
			return
		}
		c.Next()
	}
}
