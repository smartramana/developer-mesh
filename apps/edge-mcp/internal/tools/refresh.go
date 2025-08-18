package tools

import (
	"context"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RefreshConfig configures tool refresh behavior
type RefreshConfig struct {
	// AutoRefresh enables automatic periodic refresh
	AutoRefresh bool
	// RefreshInterval is the interval between automatic refreshes
	RefreshInterval time.Duration
	// RefreshOnReconnect refreshes tools when connection is restored
	RefreshOnReconnect bool
	// NotifyOnChange sends notifications when tools change
	NotifyOnChange bool
}

// DefaultRefreshConfig returns default refresh configuration
func DefaultRefreshConfig() RefreshConfig {
	return RefreshConfig{
		AutoRefresh:        true,
		RefreshInterval:    5 * time.Minute,
		RefreshOnReconnect: true,
		NotifyOnChange:     true,
	}
}

// RefreshManager manages tool refresh operations
type RefreshManager struct {
	registry  *Registry
	config    RefreshConfig
	logger    observability.Logger
	refresher func(context.Context) ([]ToolDefinition, error)
	listeners []func([]ToolDefinition)
	lastHash  string
	mu        sync.RWMutex
	stopChan  chan struct{}
	running   bool
}

// NewRefreshManager creates a new refresh manager
func NewRefreshManager(
	registry *Registry,
	config RefreshConfig,
	logger observability.Logger,
	refresher func(context.Context) ([]ToolDefinition, error),
) *RefreshManager {
	return &RefreshManager{
		registry:  registry,
		config:    config,
		logger:    logger,
		refresher: refresher,
		listeners: make([]func([]ToolDefinition), 0),
		stopChan:  make(chan struct{}),
	}
}

// Start begins automatic refresh if configured
func (rm *RefreshManager) Start(ctx context.Context) {
	rm.mu.Lock()
	if rm.running || !rm.config.AutoRefresh {
		rm.mu.Unlock()
		return
	}
	rm.running = true
	rm.mu.Unlock()

	// Initial refresh
	if err := rm.Refresh(ctx); err != nil {
		rm.logger.Warn("Initial tool refresh failed", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Start periodic refresh
	go rm.periodicRefresh(ctx)
}

// Stop stops automatic refresh
func (rm *RefreshManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.running {
		close(rm.stopChan)
		rm.running = false
	}
}

// Refresh manually triggers a tool refresh
func (rm *RefreshManager) Refresh(ctx context.Context) error {
	if rm.refresher == nil {
		return nil
	}

	// Fetch new tools
	newTools, err := rm.refresher(ctx)
	if err != nil {
		return err
	}

	// Calculate hash to detect changes
	newHash := rm.calculateHash(newTools)

	rm.mu.Lock()
	hasChanged := newHash != rm.lastHash
	rm.lastHash = newHash
	rm.mu.Unlock()

	if hasChanged {
		// Clear and re-register tools
		rm.clearAndRegister(newTools)

		// Notify listeners if configured
		if rm.config.NotifyOnChange {
			rm.notifyListeners(newTools)
		}

		rm.logger.Info("Tools refreshed with changes", map[string]interface{}{
			"count":      len(newTools),
			"hash":       newHash,
			"hasChanged": true,
		})
	} else {
		rm.logger.Debug("Tools refreshed without changes", map[string]interface{}{
			"count": len(newTools),
			"hash":  newHash,
		})
	}

	return nil
}

// OnReconnect handles tool refresh on reconnection
func (rm *RefreshManager) OnReconnect(ctx context.Context) {
	if !rm.config.RefreshOnReconnect {
		return
	}

	rm.logger.Info("Refreshing tools after reconnection", nil)
	if err := rm.Refresh(ctx); err != nil {
		rm.logger.Error("Failed to refresh tools on reconnect", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// AddListener adds a listener for tool changes
func (rm *RefreshManager) AddListener(listener func([]ToolDefinition)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.listeners = append(rm.listeners, listener)
}

// periodicRefresh runs periodic refresh loop
func (rm *RefreshManager) periodicRefresh(ctx context.Context) {
	ticker := time.NewTicker(rm.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := rm.Refresh(ctx); err != nil {
				rm.logger.Warn("Periodic tool refresh failed", map[string]interface{}{
					"error": err.Error(),
				})
			}
		case <-rm.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// clearAndRegister clears the registry and registers new tools
func (rm *RefreshManager) clearAndRegister(tools []ToolDefinition) {
	// Note: This is a simplified version. In production, you'd want
	// to handle this more gracefully to avoid race conditions
	rm.registry.mu.Lock()
	defer rm.registry.mu.Unlock()

	// Clear existing remote tools (keep local tools if any)
	newTools := make(map[string]ToolDefinition)
	for name, tool := range rm.registry.tools {
		// Keep tools that aren't remote (e.g., have handlers)
		if tool.Handler != nil {
			newTools[name] = tool
		}
	}
	rm.registry.tools = newTools

	// Register new remote tools
	for _, tool := range tools {
		rm.registry.tools[tool.Name] = tool
	}
}

// notifyListeners notifies all registered listeners of tool changes
func (rm *RefreshManager) notifyListeners(tools []ToolDefinition) {
	rm.mu.RLock()
	listeners := make([]func([]ToolDefinition), len(rm.listeners))
	copy(listeners, rm.listeners)
	rm.mu.RUnlock()

	for _, listener := range listeners {
		go listener(tools)
	}
}

// calculateHash calculates a hash of the tool list for change detection
func (rm *RefreshManager) calculateHash(tools []ToolDefinition) string {
	// Simple hash based on tool names and descriptions
	// In production, use a proper hash function
	var hash string
	for _, tool := range tools {
		hash += tool.Name + ":" + tool.Description + ";"
	}
	return hash
}

// GetLastRefreshTime returns the last refresh time
func (rm *RefreshManager) GetLastRefreshTime() time.Time {
	// This would need to be tracked in the struct
	// For now, return current time as placeholder
	return time.Now()
}

// GetRefreshStats returns refresh statistics
func (rm *RefreshManager) GetRefreshStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return map[string]interface{}{
		"running":          rm.running,
		"auto_refresh":     rm.config.AutoRefresh,
		"refresh_interval": rm.config.RefreshInterval.String(),
		"last_hash":        rm.lastHash,
		"listener_count":   len(rm.listeners),
	}
}
