package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ReadinessChecker interface {
	Ready(context.Context) error
}

type HealthHandler struct {
	checker ReadinessChecker
	timeout time.Duration
}

func NewHealthHandler(checker ReadinessChecker, timeout time.Duration) *HealthHandler {
	return &HealthHandler{checker: checker, timeout: timeout}
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()
	if err := h.checker.Ready(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
