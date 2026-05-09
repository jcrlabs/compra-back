package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthHandler struct {
	db      *pgxpool.Pool
	version string
}

func NewHealthHandler(db *pgxpool.Pool, version string) *HealthHandler {
	return &HealthHandler{db: db, version: version}
}

func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dbOK := h.db.Ping(ctx) == nil
	status := "ok"
	code := http.StatusOK
	if !dbOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{"status": status, "version": h.version, "db": dbOK})
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.Status(http.StatusOK)
}
