package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jcrlabs/compra-back/internal/repository"
)

type CatalogHandler struct {
	repo *repository.CatalogRepository
}

func NewCatalogHandler(repo *repository.CatalogRepository) *CatalogHandler {
	return &CatalogHandler{repo: repo}
}

func (h *CatalogHandler) ListCategories(c *gin.Context) {
	cats, err := h.repo.ListCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list categories"})
		return
	}
	c.JSON(http.StatusOK, cats)
}

func (h *CatalogHandler) ListProducts(c *gin.Context) {
	filter := repository.ProductFilter{
		Search: c.Query("search"),
	}

	if catIDStr := c.Query("category_id"); catIDStr != "" {
		if id, err := uuid.Parse(catIDStr); err == nil {
			filter.CategoryID = &id
		}
	}

	if p := c.DefaultQuery("page", "1"); p != "" {
		if _, err := parseInt(p, &filter.Page); err != nil || filter.Page < 1 {
			filter.Page = 1
		}
	}
	if l := c.DefaultQuery("limit", "40"); l != "" {
		if _, err := parseInt(l, &filter.Limit); err != nil || filter.Limit < 1 || filter.Limit > 100 {
			filter.Limit = 40
		}
	}

	result, err := h.repo.ListProducts(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list products"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *CatalogHandler) GetProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	product, err := h.repo.GetProduct(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	c.JSON(http.StatusOK, product)
}

func parseInt(s string, dest *int) (int, error) {
	n, e := parseIntRaw(s)
	if e == nil {
		*dest = n
	}
	return n, e
}

func parseIntRaw(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &parseError{}
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

type parseError struct{}

func (e *parseError) Error() string { return "not a number" }
