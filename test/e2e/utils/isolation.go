package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TestIsolation provides isolation mechanisms for E2E tests
type TestIsolation struct {
	namespaces map[string]*TestNamespace
	mu         sync.RWMutex
}

// TestNamespace represents an isolated test namespace
type TestNamespace struct {
	ID          string
	Name        string
	TenantID    string
	Resources   map[string]interface{}
	CreatedAt   time.Time
	CleanupFunc func() error
}

// NewTestIsolation creates a new test isolation instance
func NewTestIsolation() *TestIsolation {
	return &TestIsolation{
		namespaces: make(map[string]*TestNamespace),
	}
}

// CreateNamespace creates an isolated namespace for testing
func (ti *TestIsolation) CreateNamespace(name string) (*TestNamespace, error) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	
	ns := &TestNamespace{
		ID:        uuid.New().String(),
		Name:      name,
		TenantID:  fmt.Sprintf("test-tenant-%s-%d", name, time.Now().Unix()),
		Resources: make(map[string]interface{}),
		CreatedAt: time.Now(),
	}
	
	ti.namespaces[ns.ID] = ns
	return ns, nil
}

// GetNamespace retrieves a namespace by ID
func (ti *TestIsolation) GetNamespace(id string) (*TestNamespace, bool) {
	ti.mu.RLock()
	defer ti.mu.RUnlock()
	
	ns, exists := ti.namespaces[id]
	return ns, exists
}

// DeleteNamespace removes a namespace and cleans up resources
func (ti *TestIsolation) DeleteNamespace(id string) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	
	ns, exists := ti.namespaces[id]
	if !exists {
		return fmt.Errorf("namespace %s not found", id)
	}
	
	// Execute cleanup function if provided
	if ns.CleanupFunc != nil {
		if err := ns.CleanupFunc(); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}
	}
	
	delete(ti.namespaces, id)
	return nil
}

// CleanupAll cleans up all namespaces
func (ti *TestIsolation) CleanupAll() error {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	
	var errors []error
	
	for id, ns := range ti.namespaces {
		if ns.CleanupFunc != nil {
			if err := ns.CleanupFunc(); err != nil {
				errors = append(errors, fmt.Errorf("cleanup %s failed: %w", id, err))
			}
		}
		delete(ti.namespaces, id)
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	
	return nil
}

// ResourceManager manages isolated resources
type ResourceManager struct {
	resources map[string]*Resource
	mu        sync.RWMutex
}

// Resource represents an isolated resource
type Resource struct {
	ID          string
	Type        string
	NamespaceID string
	Data        interface{}
	CreatedAt   time.Time
	TTL         time.Duration
}

// NewResourceManager creates a new resource manager
func NewResourceManager() *ResourceManager {
	rm := &ResourceManager{
		resources: make(map[string]*Resource),
	}
	
	// Start cleanup goroutine for TTL resources
	go rm.cleanupExpiredResources()
	
	return rm
}

// CreateResource creates a new isolated resource
func (rm *ResourceManager) CreateResource(namespaceID, resourceType string, data interface{}, ttl time.Duration) *Resource {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	resource := &Resource{
		ID:          uuid.New().String(),
		Type:        resourceType,
		NamespaceID: namespaceID,
		Data:        data,
		CreatedAt:   time.Now(),
		TTL:         ttl,
	}
	
	rm.resources[resource.ID] = resource
	return resource
}

// GetResource retrieves a resource by ID
func (rm *ResourceManager) GetResource(id string) (*Resource, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	resource, exists := rm.resources[id]
	return resource, exists
}

// GetResourcesByNamespace retrieves all resources for a namespace
func (rm *ResourceManager) GetResourcesByNamespace(namespaceID string) []*Resource {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	var resources []*Resource
	for _, r := range rm.resources {
		if r.NamespaceID == namespaceID {
			resources = append(resources, r)
		}
	}
	
	return resources
}

// DeleteResource removes a resource
func (rm *ResourceManager) DeleteResource(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if _, exists := rm.resources[id]; !exists {
		return fmt.Errorf("resource %s not found", id)
	}
	
	delete(rm.resources, id)
	return nil
}

// DeleteNamespaceResources removes all resources for a namespace
func (rm *ResourceManager) DeleteNamespaceResources(namespaceID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	for id, r := range rm.resources {
		if r.NamespaceID == namespaceID {
			delete(rm.resources, id)
		}
	}
	
	return nil
}

// cleanupExpiredResources periodically removes expired resources
func (rm *ResourceManager) cleanupExpiredResources() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		rm.mu.Lock()
		now := time.Now()
		
		for id, r := range rm.resources {
			if r.TTL > 0 && now.Sub(r.CreatedAt) > r.TTL {
				delete(rm.resources, id)
			}
		}
		
		rm.mu.Unlock()
	}
}

// TestContext provides an isolated context for test execution
type TestContext struct {
	ctx        context.Context
	cancel     context.CancelFunc
	namespace  *TestNamespace
	resources  *ResourceManager
	values     map[string]interface{}
	mu         sync.RWMutex
}

// NewTestContext creates a new test context
func NewTestContext(parent context.Context, namespace *TestNamespace) *TestContext {
	ctx, cancel := context.WithCancel(parent)
	
	return &TestContext{
		ctx:       ctx,
		cancel:    cancel,
		namespace: namespace,
		resources: NewResourceManager(),
		values:    make(map[string]interface{}),
	}
}

// Context returns the underlying context
func (tc *TestContext) Context() context.Context {
	return tc.ctx
}

// Namespace returns the test namespace
func (tc *TestContext) Namespace() *TestNamespace {
	return tc.namespace
}

// SetValue stores a value in the test context
func (tc *TestContext) SetValue(key string, value interface{}) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.values[key] = value
}

// GetValue retrieves a value from the test context
func (tc *TestContext) GetValue(key string) (interface{}, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	value, exists := tc.values[key]
	return value, exists
}

// CreateResource creates an isolated resource in this context
func (tc *TestContext) CreateResource(resourceType string, data interface{}, ttl time.Duration) *Resource {
	return tc.resources.CreateResource(tc.namespace.ID, resourceType, data, ttl)
}

// Cleanup cleans up the test context
func (tc *TestContext) Cleanup() error {
	tc.cancel()
	return tc.resources.DeleteNamespaceResources(tc.namespace.ID)
}

// IsolatedTestRunner runs tests in isolation
type IsolatedTestRunner struct {
	isolation *TestIsolation
	options   IsolationOptions
}

// IsolationOptions configures test isolation
type IsolationOptions struct {
	NamespacePrefix string
	ResourceTTL     time.Duration
	CleanupDelay    time.Duration
	MaxParallel     int
}

// DefaultIsolationOptions returns default isolation options
func DefaultIsolationOptions() IsolationOptions {
	return IsolationOptions{
		NamespacePrefix: "e2e-test",
		ResourceTTL:     30 * time.Minute,
		CleanupDelay:    5 * time.Second,
		MaxParallel:     10,
	}
}

// NewIsolatedTestRunner creates a new isolated test runner
func NewIsolatedTestRunner(options IsolationOptions) *IsolatedTestRunner {
	return &IsolatedTestRunner{
		isolation: NewTestIsolation(),
		options:   options,
	}
}

// RunTest runs a test in isolation
func (itr *IsolatedTestRunner) RunTest(ctx context.Context, name string, testFunc func(*TestContext) error) error {
	// Create isolated namespace
	namespace, err := itr.isolation.CreateNamespace(fmt.Sprintf("%s-%s", itr.options.NamespacePrefix, name))
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	
	// Set cleanup function
	namespace.CleanupFunc = func() error {
		// Add cleanup delay to ensure resources are released
		time.Sleep(itr.options.CleanupDelay)
		return nil
	}
	
	// Create test context
	testCtx := NewTestContext(ctx, namespace)
	
	// Ensure cleanup happens
	defer func() {
		_ = testCtx.Cleanup()
		_ = itr.isolation.DeleteNamespace(namespace.ID)
	}()
	
	// Run the test
	return testFunc(testCtx)
}

// RunParallelTests runs multiple tests in parallel with isolation
func (itr *IsolatedTestRunner) RunParallelTests(ctx context.Context, tests map[string]func(*TestContext) error) map[string]error {
	results := make(map[string]error)
	resultsMu := sync.Mutex{}
	
	semaphore := make(chan struct{}, itr.options.MaxParallel)
	wg := sync.WaitGroup{}
	
	for name, testFunc := range tests {
		wg.Add(1)
		go func(testName string, fn func(*TestContext) error) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Run test in isolation
			err := itr.RunTest(ctx, testName, fn)
			
			// Store result
			resultsMu.Lock()
			results[testName] = err
			resultsMu.Unlock()
		}(name, testFunc)
	}
	
	wg.Wait()
	return results
}

// Cleanup cleans up all resources
func (itr *IsolatedTestRunner) Cleanup() error {
	return itr.isolation.CleanupAll()
}