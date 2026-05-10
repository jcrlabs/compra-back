package handlers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jcrlabs/compra-back/internal/middleware"
	"github.com/jcrlabs/compra-back/internal/models"
	"github.com/jcrlabs/compra-back/internal/repository"
	"github.com/jcrlabs/compra-back/internal/services"
)

type ListHandler struct {
	listRepo  *repository.ListRepository
	priceRepo *repository.PricingRepository
	userRepo  *repository.UserRepository
}

func NewListHandler(listRepo *repository.ListRepository, priceRepo *repository.PricingRepository, userRepo *repository.UserRepository) *ListHandler {
	return &ListHandler{listRepo: listRepo, priceRepo: priceRepo, userRepo: userRepo}
}

func (h *ListHandler) GetLists(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	lists, err := h.listRepo.GetListsForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get lists"})
		return
	}
	if lists == nil {
		lists = []models.ShoppingList{}
	}
	c.JSON(http.StatusOK, lists)
}

func (h *ListHandler) CreateList(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	list := &models.ShoppingList{OwnerID: userID, Name: req.Name}
	if err := h.listRepo.CreateList(list); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create list"})
		return
	}
	c.JSON(http.StatusCreated, list)
}

func (h *ListHandler) GetList(c *gin.Context) {
	listID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid list id"})
		return
	}
	userID, _ := middleware.GetUserID(c)
	if ok, _ := h.listRepo.IsMember(listID, userID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	list, err := h.listRepo.GetList(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	items, err := h.listRepo.GetItems(listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get items"})
		return
	}
	list.Items = items
	c.JSON(http.StatusOK, list)
}

func (h *ListHandler) UpdateList(c *gin.Context) {
	listID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid list id"})
		return
	}
	userID, _ := middleware.GetUserID(c)
	list, err := h.listRepo.GetList(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	if list.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner can rename"})
		return
	}
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	list.Name = req.Name
	if err := h.listRepo.UpdateList(list); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update list"})
		return
	}
	services.GlobalHub.Broadcast(listID, services.ListEvent{Type: "list_updated", ListID: listID.String(), Payload: list})
	c.JSON(http.StatusOK, list)
}

func (h *ListHandler) DeleteList(c *gin.Context) {
	listID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid list id"})
		return
	}
	userID, _ := middleware.GetUserID(c)
	list, err := h.listRepo.GetList(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	if list.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner can delete"})
		return
	}
	if err := h.listRepo.DeleteList(listID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete list"})
		return
	}
	c.Status(http.StatusNoContent)
}

// JoinByToken — join a shared list via share_token
func (h *ListHandler) JoinByToken(c *gin.Context) {
	token, err := uuid.Parse(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	list, err := h.listRepo.GetListByShareToken(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	if err := h.listRepo.AddMember(list.ID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join list"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list_id": list.ID, "message": "joined"})
}

// Items

func (h *ListHandler) AddItem(c *gin.Context) {
	listID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid list id"})
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if ok, _ := h.listRepo.IsMember(listID, userID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req struct {
		ProductID *uuid.UUID `json:"product_id"`
		Name      string     `json:"name" binding:"required"`
		Quantity  float64    `json:"quantity"`
		Unit      string     `json:"unit"`
		Notes     string     `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Quantity == 0 {
		req.Quantity = 1
	}

	item := &models.ListItem{
		ListID:    listID,
		ProductID: req.ProductID,
		Name:      req.Name,
		Quantity:  req.Quantity,
		Unit:      req.Unit,
		Notes:     req.Notes,
		AddedBy:   &userID,
	}
	if err := h.listRepo.AddItem(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add item"})
		return
	}
	_ = h.listRepo.TouchList(listID)
	services.GlobalHub.Broadcast(listID, services.ListEvent{Type: "item_added", ListID: listID.String(), Payload: item})
	c.JSON(http.StatusCreated, item)
}

func (h *ListHandler) UpdateItem(c *gin.Context) {
	listID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid list id"})
		return
	}
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	userID, _ := middleware.GetUserID(c)
	if ok, _ := h.listRepo.IsMember(listID, userID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req struct {
		Name     string  `json:"name"`
		Quantity float64 `json:"quantity"`
		Unit     string  `json:"unit"`
		Checked  bool    `json:"checked"`
		Notes    string  `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item := &models.ListItem{
		ID:       itemID,
		ListID:   listID,
		Name:     req.Name,
		Quantity: req.Quantity,
		Unit:     req.Unit,
		Checked:  req.Checked,
		Notes:    req.Notes,
	}
	if err := h.listRepo.UpdateItem(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update item"})
		return
	}
	_ = h.listRepo.TouchList(listID)
	services.GlobalHub.Broadcast(listID, services.ListEvent{Type: "item_updated", ListID: listID.String(), Payload: item})
	c.JSON(http.StatusOK, item)
}

func (h *ListHandler) DeleteItem(c *gin.Context) {
	listID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid list id"})
		return
	}
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	userID, _ := middleware.GetUserID(c)
	if ok, _ := h.listRepo.IsMember(listID, userID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.listRepo.DeleteItem(itemID, listID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete item"})
		return
	}
	_ = h.listRepo.TouchList(listID)
	services.GlobalHub.Broadcast(listID, services.ListEvent{Type: "item_deleted", ListID: listID.String(), Payload: gin.H{"item_id": itemID}})
	c.Status(http.StatusNoContent)
}

// OptimizeList — best prices for all items with linked products
func (h *ListHandler) OptimizeList(c *gin.Context) {
	listID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid list id"})
		return
	}
	userID, _ := middleware.GetUserID(c)
	if ok, _ := h.listRepo.IsMember(listID, userID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	items, err := h.listRepo.GetItems(listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get items"})
		return
	}

	var productIDs []uuid.UUID
	productItems := map[uuid.UUID]models.ListItem{}
	var uncovered []models.ListItem
	for _, it := range items {
		if it.ProductID != nil {
			productIDs = append(productIDs, *it.ProductID)
			productItems[*it.ProductID] = it
		} else {
			uncovered = append(uncovered, it)
		}
	}

	supers := h.getEnabledSupermarkets(c)
	bestPrices, err := h.priceRepo.GetBestPrices(productIDs, supers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "optimization failed"})
		return
	}

	groups := make(map[models.Supermarket][]models.OptimizedItem)
	totalBest := 0.0
	coveredIDs := map[uuid.UUID]bool{}
	for _, bp := range bestPrices {
		it := productItems[bp.ProductID]
		oi := models.OptimizedItem{
			Item:        it,
			Price:       bp.Price,
			Supermarket: bp.Supermarket,
			ExternalURL: bp.ExternalURL,
		}
		groups[bp.Supermarket] = append(groups[bp.Supermarket], oi)
		totalBest += bp.Price * it.Quantity
		coveredIDs[bp.ProductID] = true
	}

	for pid, it := range productItems {
		if !coveredIDs[pid] {
			uncovered = append(uncovered, it)
		}
	}

	result := models.OptimizedList{
		TotalBestPrice: totalBest,
		Uncovered:      uncovered,
	}

	groupMap := make(map[models.Supermarket]models.SuperGroup)
	for s, ois := range groups {
		sub := 0.0
		for _, oi := range ois {
			sub += oi.Price * oi.Item.Quantity
		}
		groupMap[s] = models.SuperGroup{Supermarket: s, Items: ois, Subtotal: sub}
	}
	result.Groups = groupMap

	c.JSON(http.StatusOK, result)
}

// SSE — real-time list events
func (h *ListHandler) StreamEvents(c *gin.Context) {
	listID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid list id"})
		return
	}
	userID, _ := middleware.GetUserID(c)
	if ok, _ := h.listRepo.IsMember(listID, userID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch, unsubscribe := services.GlobalHub.Subscribe(listID)
	defer unsubscribe()

	// Send initial ping
	_, _ = fmt.Fprintf(c.Writer, "data: {\"type\":\"connected\",\"list_id\":\"%s\"}\n\n", listID)
	c.Writer.Flush()

	notify := c.Request.Context().Done()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, err := io.WriteString(c.Writer, msg)
			if err != nil {
				return
			}
			c.Writer.Flush()
		case <-notify:
			return
		}
	}
}

func (h *ListHandler) getEnabledSupermarkets(c *gin.Context) []models.Supermarket {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return nil
	}
	prefs, err := h.userRepo.GetSupermarketPrefs(userID)
	if err != nil {
		return nil
	}
	var enabled []models.Supermarket
	for s, on := range prefs {
		if on {
			enabled = append(enabled, s)
		}
	}
	return enabled
}
