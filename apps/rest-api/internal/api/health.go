package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// HealthChecker provides health check functionality
type HealthChecker struct {
	db     *sqlx.DB
	mu     sync.RWMutex
	ready  bool
	checks map[string]HealthCheck
}

// HealthCheck represents a single health check
type HealthCheck struct {
	Name      string
	CheckFunc func(ctx context.Context) error
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *sqlx.DB) *HealthChecker {
	hc := &HealthChecker{
		db:     db,
		ready:  false,
		checks: make(map[string]HealthCheck),
	}

	// Register default checks
	hc.RegisterCheck("database", hc.checkDatabase)
	
	return hc
}

// RegisterCheck registers a new health check
func (h *HealthChecker) RegisterCheck(name string, checkFunc func(ctx context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = HealthCheck{
		Name:      name,
		CheckFunc: checkFunc,
	}
}

// SetReady sets the ready state
func (h *HealthChecker) SetReady(ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ready = ready
}

// IsReady returns the ready state
func (h *HealthChecker) IsReady() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.ready
}

// checkDatabase checks database connectivity
func (h *HealthChecker) checkDatabase(ctx context.Context) error {
	if h.db == nil {
		return nil // Database is optional
	}
	return h.db.PingContext(ctx)
}

// LivenessHandler handles liveness probe requests
// Returns 200 if the service is alive
func (h *HealthChecker) LivenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// ReadinessHandler handles readiness probe requests
// Returns 200 if the service is ready to accept traffic
func (h *HealthChecker) ReadinessHandler(c *gin.Context) {
	if !h.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"error":  "Service is starting up",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Run all health checks
	errors := make(map[string]string)
	h.mu.RLock()
	checks := make(map[string]HealthCheck, len(h.checks))
	for k, v := range h.checks {
		checks[k] = v
	}
	h.mu.RUnlock()

	for name, check := range checks {
		if err := check.CheckFunc(ctx); err != nil {
			errors[name] = err.Error()
		}
	}

	if len(errors) > 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"errors": errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// HealthHandler provides combined health status
func (h *HealthChecker) HealthHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	health := gin.H{
		"status": "healthy",
		"ready":  h.IsReady(),
		"time":   time.Now().UTC().Format(time.RFC3339),
		"checks": make(map[string]string),
	}

	// Run all health checks
	h.mu.RLock()
	checks := make(map[string]HealthCheck, len(h.checks))
	for k, v := range h.checks {
		checks[k] = v
	}
	h.mu.RUnlock()

	hasErrors := false
	for name, check := range checks {
		if err := check.CheckFunc(ctx); err != nil {
			health["checks"].(map[string]string)[name] = "unhealthy: " + err.Error()
			hasErrors = true
		} else {
			health["checks"].(map[string]string)[name] = "healthy"
		}
	}

	if hasErrors || !h.IsReady() {
		health["status"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}