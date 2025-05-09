package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/observability"
)

// HealthStatus represents an adapter's health status
type HealthStatus struct {
	Status      string
	LastChecked time.Time
	Message     string
	Details     map[string]interface{}
}

// AdapterRegistry manages adapter registration and discovery
type AdapterRegistry struct {
	adapters       map[string]Adapter
	factory        AdapterFactory
	healthStatuses map[string]HealthStatus
	callbacks      map[string][]func(Adapter, HealthStatus)
	mu             sync.RWMutex
	logger         *observability.Logger
}

// NewAdapterRegistry creates a new adapter registry
func NewAdapterRegistry(factory AdapterFactory, logger *observability.Logger) *AdapterRegistry {
	registry := &AdapterRegistry{
		adapters:       make(map[string]Adapter),
		factory:        factory,
		healthStatuses: make(map[string]HealthStatus),
		callbacks:      make(map[string][]func(Adapter, HealthStatus)),
		logger:         logger,
	}
	
	// Start health check routine
	go registry.healthCheckLoop()
	
	return registry
}

// RegisterAdapter registers an adapter
func (r *AdapterRegistry) RegisterAdapter(adapterType string, adapter Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.adapters[adapterType] = adapter
	r.healthStatuses[adapterType] = HealthStatus{
		Status:      "initializing",
		LastChecked: time.Now(),
		Message:     "Adapter registered, awaiting first health check",
	}
	
	r.logger.Info("Adapter registered", map[string]interface{}{
		"adapterType": adapterType,
		"version":     adapter.Version(),
	})
}

// DeregisterAdapter removes an adapter from the registry
func (r *AdapterRegistry) DeregisterAdapter(adapterType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	adapter, exists := r.adapters[adapterType]
	if !exists {
		return fmt.Errorf("adapter not found: %s", adapterType)
	}
	
	// Close the adapter
	if err := adapter.Close(); err != nil {
		r.logger.Warn("Failed to close adapter gracefully", map[string]interface{}{
			"adapterType": adapterType,
			"error":       err.Error(),
		})
	}
	
	// Remove from registry
	delete(r.adapters, adapterType)
	delete(r.healthStatuses, adapterType)
	delete(r.callbacks, adapterType)
	
	r.logger.Info("Adapter deregistered", map[string]interface{}{
		"adapterType": adapterType,
	})
	
	return nil
}

// GetAdapter gets an adapter by type, creating it if it doesn't exist
func (r *AdapterRegistry) GetAdapter(ctx context.Context, adapterType string) (Adapter, error) {
	r.mu.RLock()
	adapter, exists := r.adapters[adapterType]
	r.mu.RUnlock()
	
	if exists {
		return adapter, nil
	}
	
	// Create adapter if it doesn't exist
	adapter, err := r.factory.CreateAdapter(ctx, adapterType)
	if err != nil {
		return nil, err
	}
	
	// Register the new adapter
	r.RegisterAdapter(adapterType, adapter)
	
	return adapter, nil
}

// ListAdapters returns all registered adapters
func (r *AdapterRegistry) ListAdapters() map[string]Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to prevent race conditions
	adapters := make(map[string]Adapter)
	for k, v := range r.adapters {
		adapters[k] = v
	}
	
	return adapters
}

// GetAdapterHealth returns an adapter's health status
func (r *AdapterRegistry) GetAdapterHealth(adapterType string) (HealthStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	status, exists := r.healthStatuses[adapterType]
	if !exists {
		return HealthStatus{}, fmt.Errorf("adapter not found: %s", adapterType)
	}
	
	return status, nil
}

// RegisterHealthCallback registers a callback for health status changes
func (r *AdapterRegistry) RegisterHealthCallback(adapterType string, callback func(Adapter, HealthStatus)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	callbacks, exists := r.callbacks[adapterType]
	if !exists {
		callbacks = []func(Adapter, HealthStatus){}
	}
	
	r.callbacks[adapterType] = append(callbacks, callback)
}

// healthCheckLoop periodically checks adapter health
func (r *AdapterRegistry) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		r.checkAllAdaptersHealth()
	}
}

// checkAllAdaptersHealth checks the health of all registered adapters
func (r *AdapterRegistry) checkAllAdaptersHealth() {
	r.mu.RLock()
	adaptersCopy := make(map[string]Adapter)
	for k, v := range r.adapters {
		adaptersCopy[k] = v
	}
	r.mu.RUnlock()
	
	for adapterType, adapter := range adaptersCopy {
		healthStatus := adapter.Health()
		
		r.mu.Lock()
		
		// Update health status
		oldStatus := r.healthStatuses[adapterType]
		newStatus := HealthStatus{
			Status:      healthStatus,
			LastChecked: time.Now(),
			Message:     "",
		}
		r.healthStatuses[adapterType] = newStatus
		
		// Notify if status changed
		if oldStatus.Status != newStatus.Status {
			r.logger.Info("Adapter health status changed", map[string]interface{}{
				"adapterType": adapterType,
				"oldStatus":   oldStatus.Status,
				"newStatus":   newStatus.Status,
			})
			
			// Call registered callbacks
			callbacks := r.callbacks[adapterType]
			for _, callback := range callbacks {
				go callback(adapter, newStatus)
			}
		}
		
		r.mu.Unlock()
	}
}

// recoverAdapter attempts to recover a failed adapter
func (r *AdapterRegistry) recoverAdapter(ctx context.Context, adapterType string) {
	r.logger.Info("Attempting to recover adapter", map[string]interface{}{
		"adapterType": adapterType,
	})
	
	// Deregister the old adapter
	if err := r.DeregisterAdapter(adapterType); err != nil {
		r.logger.Warn("Failed to deregister adapter during recovery", map[string]interface{}{
			"adapterType": adapterType,
			"error":       err.Error(),
		})
	}
	
	// Create a new adapter
	adapter, err := r.factory.CreateAdapter(ctx, adapterType)
	if err != nil {
		r.logger.Error("Failed to recover adapter", map[string]interface{}{
			"adapterType": adapterType,
			"error":       err.Error(),
		})
		return
	}
	
	// Register the new adapter
	r.RegisterAdapter(adapterType, adapter)
	
	r.logger.Info("Successfully recovered adapter", map[string]interface{}{
		"adapterType": adapterType,
	})
}
