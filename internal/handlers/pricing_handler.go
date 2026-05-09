package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jcrlabs/compra-back/internal/middleware"
	"github.com/jcrlabs/compra-back/internal/models"
	"github.com/jcrlabs/compra-back/internal/repository"
)

type PricingHandler struct {
	priceRepo *repository.PricingRepository
	userRepo  *repository.UserRepository
}

func NewPricingHandler(priceRepo *repository.PricingRepository, userRepo *repository.UserRepository) *PricingHandler {
	return &PricingHandler{priceRepo: priceRepo, userRepo: userRepo}
}

func (h *PricingHandler) GetPrices(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	supers := h.getEnabledSupermarkets(c)
	prices, err := h.priceRepo.GetCurrentPricesFiltered(productID, supers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get prices"})
		return
	}
	c.JSON(http.StatusOK, prices)
}

func (h *PricingHandler) GetPriceHistory(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	superStr := c.Query("supermarket")
	if superStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "supermarket query param required"})
		return
	}
	super := models.Supermarket(superStr)
	if !super.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid supermarket"})
		return
	}
	prices, err := h.priceRepo.GetPriceHistory(productID, super, 30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get price history"})
		return
	}
	c.JSON(http.StatusOK, prices)
}

func (h *PricingHandler) GetCheapest(c *gin.Context) {
	idsStr := c.Query("product_ids")
	if idsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "product_ids required"})
		return
	}

	var productIDs []uuid.UUID
	for _, s := range strings.Split(idsStr, ",") {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err == nil {
			productIDs = append(productIDs, id)
		}
	}

	supers := h.getEnabledSupermarkets(c)
	bestPrices, err := h.priceRepo.GetBestPrices(productIDs, supers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute best prices"})
		return
	}
	c.JSON(http.StatusOK, bestPrices)
}

func (h *PricingHandler) GetUserPrefs(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	prefs, err := h.userRepo.GetSupermarketPrefs(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get preferences"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"supermarkets": prefs})
}

func (h *PricingHandler) SetUserPrefs(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		Supermarkets map[string]bool `json:"supermarkets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	prefs := make(map[models.Supermarket]bool)
	for k, v := range req.Supermarkets {
		s := models.Supermarket(k)
		if s.Valid() {
			prefs[s] = v
		}
	}
	if err := h.userRepo.SetSupermarketPrefs(userID, prefs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save preferences"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"supermarkets": prefs})
}

// getEnabledSupermarkets reads user prefs or query param override
func (h *PricingHandler) getEnabledSupermarkets(c *gin.Context) []models.Supermarket {
	if superStr := c.Query("supermarkets"); superStr != "" {
		var supers []models.Supermarket
		for _, s := range strings.Split(superStr, ",") {
			sm := models.Supermarket(strings.TrimSpace(s))
			if sm.Valid() {
				supers = append(supers, sm)
			}
		}
		return supers
	}

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

// ListOptimizer wires through services.ListOptimizer
type ListOptimizeRequest struct {
	ProductIDs []string `json:"product_ids"`
}

func OptimizeHandler(priceRepo *repository.PricingRepository, userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ph := &PricingHandler{priceRepo: priceRepo, userRepo: userRepo}
		idsStr := c.Query("product_ids")
		if idsStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "product_ids required"})
			return
		}

		var productIDs []uuid.UUID
		for _, s := range strings.Split(idsStr, ",") {
			id, err := uuid.Parse(strings.TrimSpace(s))
			if err == nil {
				productIDs = append(productIDs, id)
			}
		}

		supers := ph.getEnabledSupermarkets(c)
		bestPrices, err := priceRepo.GetBestPrices(productIDs, supers)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "optimization failed"})
			return
		}

		// Group by supermarket
		groups := make(map[models.Supermarket][]models.BestPrice)
		totalBest := 0.0
		for _, bp := range bestPrices {
			groups[bp.Supermarket] = append(groups[bp.Supermarket], bp)
			totalBest += bp.Price
		}

		c.JSON(http.StatusOK, gin.H{
			"best_prices":   bestPrices,
			"groups":        groups,
			"total_minimum": totalBest,
		})
	}
}
