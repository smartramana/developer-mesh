package data

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TestData manages test data for E2E tests
type TestData struct {
	TenantID     string
	APIKeys      map[string]string
	TestAccounts map[string]*TestAccount
	Repositories map[string]*TestRepository
	Contexts     map[string]*TestContext
	Workflows    map[string]*TestWorkflow
}

// TestAccount represents a test account
type TestAccount struct {
	ID          string
	Name        string
	Email       string
	APIKey      string
	Permissions []string
	CreatedAt   time.Time
}

// TestRepository represents a test repository
type TestRepository struct {
	Name        string
	URL         string
	Branch      string
	CommitHash  string
	PRNumbers   []int
	Description string
}

// TestContext represents a test context
type TestContext struct {
	ID       string
	Name     string
	Content  string
	Metadata map[string]interface{}
	Size     int
}

// TestWorkflow represents a test workflow
type TestWorkflow struct {
	ID          string
	Name        string
	Steps       []WorkflowStep
	Strategy    string
	CreatedAt   time.Time
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	Name       string
	AgentType  string
	Capability string
	Input      map[string]interface{}
	Timeout    time.Duration
}

// NewTestData creates a new test data instance
func NewTestData(tenantID string) *TestData {
	if tenantID == "" {
		tenantID = "test-tenant-" + uuid.New().String()[:8]
	}
	
	return &TestData{
		TenantID:     tenantID,
		APIKeys:      make(map[string]string),
		TestAccounts: make(map[string]*TestAccount),
		Repositories: make(map[string]*TestRepository),
		Contexts:     make(map[string]*TestContext),
		Workflows:    make(map[string]*TestWorkflow),
	}
}

// GenerateAPIKey generates a test API key
func (td *TestData) GenerateAPIKey(name string) string {
	key := fmt.Sprintf("test-key-%s-%s", name, uuid.New().String())
	td.APIKeys[name] = key
	return key
}

// CreateTestAccount creates a test account
func (td *TestData) CreateTestAccount(name string, permissions []string) *TestAccount {
	account := &TestAccount{
		ID:          uuid.New().String(),
		Name:        name,
		Email:       fmt.Sprintf("%s@test.example.com", strings.ToLower(name)),
		APIKey:      td.GenerateAPIKey(name),
		Permissions: permissions,
		CreatedAt:   time.Now(),
	}
	
	td.TestAccounts[name] = account
	return account
}

// CreateTestRepository creates a test repository
func (td *TestData) CreateTestRepository(name string) *TestRepository {
	repo := &TestRepository{
		Name:        name,
		URL:         fmt.Sprintf("https://github.com/test-org/%s", name),
		Branch:      "main",
		CommitHash:  generateCommitHash(),
		PRNumbers:   []int{rand.Intn(1000) + 1, rand.Intn(1000) + 1},
		Description: fmt.Sprintf("Test repository for %s", name),
	}
	
	td.Repositories[name] = repo
	return repo
}

// CreateTestContext creates a test context
func (td *TestData) CreateTestContext(name string, size int) *TestContext {
	content := GenerateLargeContext(size)
	
	ctx := &TestContext{
		ID:      uuid.New().String(),
		Name:    name,
		Content: content,
		Metadata: map[string]interface{}{
			"type":        "test",
			"importance": rand.Intn(100),
			"tokens":     size * 4, // Approximate tokens
		},
		Size: len(content),
	}
	
	td.Contexts[name] = ctx
	return ctx
}

// CreateTestWorkflow creates a test workflow
func (td *TestData) CreateTestWorkflow(name string, steps []WorkflowStep, strategy string) *TestWorkflow {
	workflow := &TestWorkflow{
		ID:        uuid.New().String(),
		Name:      name,
		Steps:     steps,
		Strategy:  strategy,
		CreatedAt: time.Now(),
	}
	
	td.Workflows[name] = workflow
	return workflow
}

// LoadFromFile loads test data from a JSON file
func (td *TestData) LoadFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	if err := json.Unmarshal(data, td); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}
	
	return nil
}

// SaveToFile saves test data to a JSON file
func (td *TestData) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
}

// GenerateLargeContext generates a large text context for testing
func GenerateLargeContext(lines int) string {
	var sb strings.Builder
	
	// Sample text patterns
	patterns := []string{
		"The quick brown fox jumps over the lazy dog.",
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		"In the realm of software engineering, testing is paramount.",
		"DevOps practices enable continuous integration and deployment.",
		"Machine learning models require extensive training data.",
		"Cloud computing has revolutionized infrastructure management.",
		"Microservices architecture provides scalability and flexibility.",
		"Security is a critical aspect of modern application development.",
	}
	
	for i := 0; i < lines; i++ {
		pattern := patterns[i%len(patterns)]
		sb.WriteString(fmt.Sprintf("Line %d: %s\n", i+1, pattern))
		
		// Add some variation
		if i%10 == 0 {
			sb.WriteString(fmt.Sprintf("Timestamp: %s\n", time.Now().Format(time.RFC3339)))
		}
		
		if i%20 == 0 {
			sb.WriteString(fmt.Sprintf("Random UUID: %s\n", uuid.New().String()))
		}
	}
	
	return sb.String()
}

// generateCommitHash generates a mock git commit hash
func generateCommitHash() string {
	const chars = "0123456789abcdef"
	b := make([]byte, 40)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// TestDataGenerator generates various test data
type TestDataGenerator struct {
	rand *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateTaskPayload generates a test task payload
func (g *TestDataGenerator) GenerateTaskPayload(taskType string) map[string]interface{} {
	switch taskType {
	case "code_analysis":
		return map[string]interface{}{
			"repository": fmt.Sprintf("https://github.com/test-org/repo-%d", g.rand.Intn(100)),
			"branch":     "main",
			"checks":     []string{"style", "bugs", "security"},
		}
	
	case "deployment":
		return map[string]interface{}{
			"application": fmt.Sprintf("app-%d", g.rand.Intn(10)),
			"environment": []string{"dev", "staging", "prod"}[g.rand.Intn(3)],
			"version":     fmt.Sprintf("v1.%d.%d", g.rand.Intn(10), g.rand.Intn(100)),
		}
	
	case "security_scan":
		return map[string]interface{}{
			"target":     fmt.Sprintf("service-%d", g.rand.Intn(20)),
			"scan_types": []string{"vulnerabilities", "compliance", "secrets"},
			"severity":   []string{"low", "medium", "high", "critical"}[g.rand.Intn(4)],
		}
	
	default:
		return map[string]interface{}{
			"type":      taskType,
			"timestamp": time.Now().Unix(),
			"data":      uuid.New().String(),
		}
	}
}

// GenerateMetrics generates test metrics
func (g *TestDataGenerator) GenerateMetrics() map[string]interface{} {
	return map[string]interface{}{
		"cpu_usage":     g.rand.Float64() * 100,
		"memory_usage":  g.rand.Float64() * 100,
		"request_count": g.rand.Intn(10000),
		"error_rate":    g.rand.Float64() * 10,
		"latency_p99":   g.rand.Intn(1000),
		"timestamp":     time.Now().Unix(),
	}
}

// DefaultTestData creates default test data for common scenarios
func DefaultTestData() *TestData {
	td := NewTestData("e2e-test-tenant")
	
	// Create test accounts
	td.CreateTestAccount("admin", []string{"*"})
	td.CreateTestAccount("developer", []string{"read", "write", "execute"})
	td.CreateTestAccount("viewer", []string{"read"})
	
	// Create test repositories
	td.CreateTestRepository("frontend-app")
	td.CreateTestRepository("backend-api")
	td.CreateTestRepository("infrastructure")
	
	// Create test contexts
	td.CreateTestContext("small-context", 100)
	td.CreateTestContext("medium-context", 1000)
	td.CreateTestContext("large-context", 5000)
	
	// Create test workflows
	td.CreateTestWorkflow("code-review", []WorkflowStep{
		{
			Name:       "analyze_code",
			AgentType:  "code_analysis",
			Capability: "code_analysis",
			Timeout:    5 * time.Minute,
		},
		{
			Name:       "security_scan",
			AgentType:  "security_scanner",
			Capability: "security_scanning",
			Timeout:    10 * time.Minute,
		},
	}, "sequential")
	
	return td
}