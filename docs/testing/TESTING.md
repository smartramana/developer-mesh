# Testing Guide

## Overview
This guide covers testing strategies, tools, and best practices for DevOps MCP.

## Testing Pyramid

```
         /\
        /  \    E2E Tests (5%)
       /    \   - User journeys
      /      \  - Critical paths
     /________\ 
    /          \ Integration Tests (20%)
   /            \ - API contracts
  /              \ - Database interactions
 /________________\ Unit Tests (75%)
                    - Business logic
                    - Pure functions
```

## Unit Testing

### Go Unit Tests

```go
// pkg/adapters/github/adapter_test.go
package github_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestGitHubAdapter_CreateIssue(t *testing.T) {
    // Arrange
    mockClient := new(MockGitHubClient)
    adapter := &GitHubAdapter{client: mockClient}
    
    expectedIssue := &Issue{
        ID:     123,
        Number: 42,
        Title:  "Test Issue",
    }
    
    mockClient.On("CreateIssue", mock.Anything, "owner/repo", mock.Anything).
        Return(expectedIssue, nil)
    
    // Act
    issue, err := adapter.CreateIssue(context.Background(), "owner/repo", "Test Issue", "Test body")
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expectedIssue, issue)
    mockClient.AssertExpectations(t)
}

// Table-driven tests
func TestValidateWebhookSignature(t *testing.T) {
    tests := []struct {
        name      string
        payload   string
        signature string
        secret    string
        want      bool
    }{
        {
            name:      "valid signature",
            payload:   `{"action":"opened"}`,
            signature: "sha256=valid_signature_here",
            secret:    "webhook_secret",
            want:      true,
        },
        {
            name:      "invalid signature",
            payload:   `{"action":"opened"}`,
            signature: "sha256=invalid",
            secret:    "webhook_secret",
            want:      false,
        },
        {
            name:      "empty signature",
            payload:   `{"action":"opened"}`,
            signature: "",
            secret:    "webhook_secret",
            want:      false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ValidateWebhookSignature([]byte(tt.payload), tt.signature, tt.secret)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Test Coverage

```bash
# Run tests with coverage
make test-coverage

# Generate HTML coverage report
make test-coverage-html

# View coverage in browser
open coverage.html
```

### Mocking Best Practices

```go
// Use interfaces for mockability
type GitHubClient interface {
    CreateIssue(ctx context.Context, repo string, issue *IssueRequest) (*Issue, error)
    GetIssue(ctx context.Context, repo string, number int) (*Issue, error)
}

// Generate mocks
//go:generate mockery --name=GitHubClient --output=mocks --filename=github_client.go

// Use testify/mock for assertions
func TestWithMock(t *testing.T) {
    mockClient := new(mocks.GitHubClient)
    
    // Set expectations
    mockClient.On("GetIssue", mock.Anything, "owner/repo", 123).
        Return(&Issue{ID: 123}, nil).
        Once() // Ensure called exactly once
    
    // Your test code here
    
    // Verify all expectations met
    mockClient.AssertExpectations(t)
}
```

## Integration Testing

### Database Integration Tests

```go
// tests/integration/repository_test.go
// +build integration

package integration_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/suite"
)

type RepositoryTestSuite struct {
    suite.Suite
    db   *sql.DB
    repo repository.Repository
}

func (s *RepositoryTestSuite) SetupSuite() {
    // Setup test database
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        s.T().Skip("TEST_DATABASE_URL not set")
    }
    
    db, err := sql.Open("postgres", dsn)
    s.Require().NoError(err)
    
    // Run migrations
    s.Require().NoError(migrate.Up(db))
    
    s.db = db
    s.repo = repository.New(db)
}

func (s *RepositoryTestSuite) TearDownSuite() {
    s.db.Close()
}

func (s *RepositoryTestSuite) SetupTest() {
    // Clean database before each test
    _, err := s.db.Exec("TRUNCATE contexts, users CASCADE")
    s.Require().NoError(err)
}

func (s *RepositoryTestSuite) TestCreateContext() {
    // Test context creation with database
    ctx := context.Background()
    
    context := &models.Context{
        Name:     "Test Context",
        Content:  "Test content",
        TenantID: "test-tenant",
    }
    
    created, err := s.repo.CreateContext(ctx, context)
    s.Require().NoError(err)
    s.NotEmpty(created.ID)
    s.Equal(context.Name, created.Name)
    
    // Verify in database
    var count int
    err = s.db.QueryRow("SELECT COUNT(*) FROM contexts WHERE id = $1", created.ID).Scan(&count)
    s.Require().NoError(err)
    s.Equal(1, count)
}

func TestRepositoryTestSuite(t *testing.T) {
    suite.Run(t, new(RepositoryTestSuite))
}
```

### API Integration Tests

```go
// tests/integration/api_test.go
// +build integration

func TestAPIIntegration(t *testing.T) {
    // Start test server
    app := api.NewServer(testConfig)
    server := httptest.NewServer(app.Handler())
    defer server.Close()
    
    client := &http.Client{Timeout: 10 * time.Second}
    
    t.Run("Create and retrieve context", func(t *testing.T) {
        // Create context
        payload := `{"name":"Test","content":"Test content"}`
        resp, err := client.Post(
            server.URL+"/api/v1/contexts",
            "application/json",
            strings.NewReader(payload),
        )
        require.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusCreated, resp.StatusCode)
        
        var created map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&created)
        require.NoError(t, err)
        
        contextID := created["id"].(string)
        
        // Retrieve context
        resp, err = client.Get(server.URL + "/api/v1/contexts/" + contextID)
        require.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusOK, resp.StatusCode)
    })
}
```

## E2E Testing

### Cypress Tests

```javascript
// cypress/integration/user_journey.spec.js
describe('User Journey: Create and Search Context', () => {
  before(() => {
    cy.task('db:seed'); // Seed test data
  });

  it('should create a new context and search for it', () => {
    // Login
    cy.visit('/login');
    cy.get('[data-cy=email]').type('test@example.com');
    cy.get('[data-cy=password]').type('password123');
    cy.get('[data-cy=submit]').click();
    
    // Create context
    cy.get('[data-cy=create-context]').click();
    cy.get('[data-cy=context-name]').type('E2E Test Context');
    cy.get('[data-cy=context-content]').type('This is an E2E test');
    cy.get('[data-cy=save-context]').click();
    
    // Verify creation
    cy.get('[data-cy=success-message]').should('contain', 'Context created');
    
    // Search for context
    cy.get('[data-cy=search-input]').type('E2E Test');
    cy.get('[data-cy=search-button]').click();
    
    // Verify search results
    cy.get('[data-cy=search-results]').should('contain', 'E2E Test Context');
  });
});
```

### Playwright Tests

```typescript
// tests/e2e/api.spec.ts
import { test, expect } from '@playwright/test';

test.describe('API E2E Tests', () => {
  let apiContext;
  let authToken;

  test.beforeAll(async ({ playwright }) => {
    apiContext = await playwright.request.newContext({
      baseURL: process.env.API_URL || 'http://localhost:8080',
    });
    
    // Authenticate
    const loginResponse = await apiContext.post('/api/v1/auth/login', {
      data: {
        email: 'test@example.com',
        password: 'password123'
      }
    });
    
    const { token } = await loginResponse.json();
    authToken = token;
  });

  test('Full context lifecycle', async () => {
    // Create context
    const createResponse = await apiContext.post('/api/v1/contexts', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      },
      data: {
        name: 'Playwright Test',
        content: 'E2E test content'
      }
    });
    
    expect(createResponse.ok()).toBeTruthy();
    const context = await createResponse.json();
    
    // Update context
    const updateResponse = await apiContext.put(`/api/v1/contexts/${context.id}`, {
      headers: {
        'Authorization': `Bearer ${authToken}`
      },
      data: {
        content: 'Updated content'
      }
    });
    
    expect(updateResponse.ok()).toBeTruthy();
    
    // Search for context
    const searchResponse = await apiContext.post('/api/v1/contexts/search', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      },
      data: {
        query: 'Playwright'
      }
    });
    
    const results = await searchResponse.json();
    expect(results).toHaveLength(1);
    expect(results[0].id).toBe(context.id);
    
    // Delete context
    const deleteResponse = await apiContext.delete(`/api/v1/contexts/${context.id}`, {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    });
    
    expect(deleteResponse.ok()).toBeTruthy();
  });
});
```

## Load Testing

### K6 Load Tests

```javascript
// tests/load/spike-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export let options = {
  stages: [
    { duration: '2m', target: 100 }, // Ramp up
    { duration: '5m', target: 100 }, // Stay at 100
    { duration: '2m', target: 1000 }, // Spike to 1000
    { duration: '5m', target: 1000 }, // Stay at 1000
    { duration: '2m', target: 100 }, // Scale down
    { duration: '5m', target: 0 }, // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    errors: ['rate<0.1'], // Error rate under 10%
  },
};

const API_BASE = __ENV.API_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY;

export default function () {
  // Create context
  let createRes = http.post(
    `${API_BASE}/api/v1/contexts`,
    JSON.stringify({
      name: `Load Test ${__VU}-${__ITER}`,
      content: 'Load test context content',
    }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': API_KEY,
      },
    }
  );
  
  check(createRes, {
    'create status is 201': (r) => r.status === 201,
    'create time < 500ms': (r) => r.timings.duration < 500,
  }) || errorRate.add(1);
  
  sleep(1);
  
  // Search contexts
  let searchRes = http.post(
    `${API_BASE}/api/v1/contexts/search`,
    JSON.stringify({ query: 'Load Test' }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': API_KEY,
      },
    }
  );
  
  check(searchRes, {
    'search status is 200': (r) => r.status === 200,
    'search time < 1000ms': (r) => r.timings.duration < 1000,
  }) || errorRate.add(1);
  
  sleep(Math.random() * 3);
}
```

### Gatling Load Tests

```scala
// tests/load/BasicSimulation.scala
import io.gatling.core.Predef._
import io.gatling.http.Predef._
import scala.concurrent.duration._

class BasicSimulation extends Simulation {
  val httpProtocol = http
    .baseUrl("http://localhost:8080")
    .acceptHeader("application/json")
    .header("X-API-Key", System.getenv("API_KEY"))

  val createContext = exec(http("Create Context")
    .post("/api/v1/contexts")
    .body(StringBody("""{"name":"Gatling Test","content":"Load test"}"""))
    .check(status.is(201))
    .check(jsonPath("$.id").saveAs("contextId")))

  val getContext = exec(http("Get Context")
    .get("/api/v1/contexts/${contextId}")
    .check(status.is(200)))

  val userJourney = scenario("User Journey")
    .exec(createContext)
    .pause(1)
    .exec(getContext)

  setUp(
    userJourney.inject(
      rampUsersPerSec(1).to(100).during(5.minutes),
      constantUsersPerSec(100).during(10.minutes),
      rampUsersPerSec(100).to(0).during(5.minutes)
    )
  ).protocols(httpProtocol)
   .assertions(
     global.responseTime.max.lt(2000),
     global.successfulRequests.percent.gt(95)
   )
}
```

## Chaos Engineering

### Litmus Chaos Tests

```yaml
# chaos/network-delay.yaml
apiVersion: litmuschaos.io/v1alpha1
kind: ChaosEngine
metadata:
  name: network-chaos
spec:
  appinfo:
    appns: mcp-prod
    applabel: app=mcp-server
  chaosServiceAccount: litmus-admin
  experiments:
    - name: pod-network-latency
      spec:
        components:
          env:
            - name: NETWORK_INTERFACE
              value: eth0
            - name: NETWORK_LATENCY
              value: "2000" # 2 second delay
            - name: TOTAL_CHAOS_DURATION
              value: "600" # 10 minutes
            - name: PODS_AFFECTED_PERC
              value: "50" # Affect 50% of pods
```

### Chaos Toolkit Tests

```json
{
  "title": "Verify system handles database failure",
  "description": "Kill database and ensure graceful degradation",
  "steady-state-hypothesis": {
    "title": "System is healthy",
    "probes": [
      {
        "type": "probe",
        "name": "api-health-check",
        "provider": {
          "type": "http",
          "url": "http://localhost:8080/health",
          "timeout": 5
        }
      }
    ]
  },
  "method": [
    {
      "type": "action",
      "name": "kill-database",
      "provider": {
        "type": "process",
        "path": "kubectl",
        "arguments": ["delete", "pod", "-l", "app=postgres", "-n", "mcp-prod"]
      }
    }
  ],
  "rollbacks": [
    {
      "type": "action",
      "name": "restart-database",
      "provider": {
        "type": "process",
        "path": "kubectl",
        "arguments": ["rollout", "restart", "deployment/postgres", "-n", "mcp-prod"]
      }
    }
  ]
}
```

## Contract Testing

### Pact Consumer Tests

```javascript
// tests/contract/consumer.pact.test.js
const { Pact } = require('@pact-foundation/pact');
const { getContext } = require('../src/api-client');

describe('API Client Pact Tests', () => {
  const provider = new Pact({
    consumer: 'Frontend',
    provider: 'MCP-API',
    port: 8989,
  });

  beforeAll(() => provider.setup());
  afterAll(() => provider.finalize());
  afterEach(() => provider.verify());

  describe('get context', () => {
    test('should return a context', async () => {
      const expectedContext = {
        id: '123',
        name: 'Test Context',
        content: 'Test content',
      };

      await provider.addInteraction({
        state: 'a context with ID 123 exists',
        uponReceiving: 'a request to get context',
        withRequest: {
          method: 'GET',
          path: '/api/v1/contexts/123',
          headers: {
            'Authorization': 'Bearer token',
          },
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedContext,
        },
      });

      const context = await getContext('123', 'token');
      expect(context).toEqual(expectedContext);
    });
  });
});
```

### Pact Provider Verification

```go
// tests/contract/provider_test.go
func TestPactProvider(t *testing.T) {
    pact := &dsl.Pact{
        Provider: "MCP-API",
        LogLevel: "INFO",
    }

    // Start test server
    server := httptest.NewServer(api.NewServer(testConfig).Handler())
    defer server.Close()

    _, err := pact.VerifyProvider(t, types.VerifyRequest{
        ProviderBaseURL:    server.URL,
        PactURLs:           []string{"./pacts/frontend-mcp-api.json"},
        StateHandlers: types.StateHandlers{
            "a context with ID 123 exists": func() error {
                return seedContext("123", "Test Context", "Test content")
            },
        },
    })

    assert.NoError(t, err)
}
```

## Test Data Management

### Test Fixtures

```go
// tests/fixtures/contexts.go
package fixtures

import (
    "github.com/S-Corkum/devops-mcp/pkg/models"
)

func ValidContext() *models.Context {
    return &models.Context{
        Name:     "Test Context",
        Content:  "Test content",
        TenantID: "test-tenant",
        Metadata: map[string]interface{}{
            "source": "test",
        },
    }
}

func ContextWithLongContent() *models.Context {
    ctx := ValidContext()
    ctx.Content = strings.Repeat("a", 10000)
    return ctx
}
```

### Test Database Seeding

```go
// tests/helpers/seed.go
func SeedTestData(db *sql.DB) error {
    // Use transactions for isolation
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Seed users
    _, err = tx.Exec(`
        INSERT INTO users (id, email, name, tenant_id)
        VALUES 
            ('user-1', 'test1@example.com', 'Test User 1', 'tenant-1'),
            ('user-2', 'test2@example.com', 'Test User 2', 'tenant-2')
    `)
    if err != nil {
        return err
    }

    // Seed contexts
    _, err = tx.Exec(`
        INSERT INTO contexts (id, name, content, tenant_id, created_by)
        VALUES 
            ('ctx-1', 'Test Context 1', 'Content 1', 'tenant-1', 'user-1'),
            ('ctx-2', 'Test Context 2', 'Content 2', 'tenant-1', 'user-1')
    `)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

## CI/CD Test Pipeline

### GitHub Actions

```yaml
# .github/workflows/test.yml
name: Test Suite

on:
  pull_request:
  push:
    branches: [main]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run unit tests
        run: make test
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:14
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      
      redis:
        image: redis:7
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      
      - name: Run integration tests
        env:
          TEST_DATABASE_URL: postgres://postgres:test@localhost:5432/test?sslmode=disable
          TEST_REDIS_URL: redis://localhost:6379
        run: make test-integration

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Start services
        run: docker-compose up -d
      
      - name: Wait for services
        run: |
          timeout 300 bash -c 'until curl -f http://localhost:8080/health; do sleep 5; done'
      
      - name: Run E2E tests
        run: |
          npm install
          npm run test:e2e
      
      - name: Upload artifacts
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: e2e-artifacts
          path: |
            cypress/screenshots
            cypress/videos

  load-tests:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3
      
      - name: Run load tests
        run: |
          docker run --rm \
            -v $PWD:/scripts \
            loadimpact/k6 run /scripts/tests/load/spike-test.js
```

## Test Best Practices

### 1. Test Naming
```go
// Good: Descriptive test names
func TestCreateContext_WithValidData_ReturnsCreatedContext(t *testing.T) {}
func TestCreateContext_WithMissingName_ReturnsValidationError(t *testing.T) {}

// Bad: Unclear names
func TestCreate(t *testing.T) {}
func TestError(t *testing.T) {}
```

### 2. Test Isolation
```go
// Each test should be independent
func TestExample(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    defer cleanupDB(db)
    
    // Test logic
    
    // No dependency on other tests
}
```

### 3. Use Test Helpers
```go
func assertContextEqual(t *testing.T, expected, actual *Context) {
    t.Helper() // Mark as test helper
    
    assert.Equal(t, expected.Name, actual.Name)
    assert.Equal(t, expected.Content, actual.Content)
    assert.Equal(t, expected.TenantID, actual.TenantID)
}
```

### 4. Parallel Testing
```go
func TestParallel(t *testing.T) {
    t.Parallel() // Run tests in parallel
    
    // Test must be thread-safe
}
```

### 5. Cleanup
```go
func TestWithCleanup(t *testing.T) {
    resource := createResource()
    t.Cleanup(func() {
        resource.Close()
    })
    
    // Test logic
}
```

## Testing Checklist

- [ ] Unit tests cover >80% of business logic
- [ ] Integration tests verify database operations
- [ ] E2E tests cover critical user journeys
- [ ] Load tests validate performance requirements
- [ ] Contract tests ensure API compatibility
- [ ] Security tests check for vulnerabilities
- [ ] All tests run in CI/CD pipeline
- [ ] Test data is isolated and repeatable
- [ ] Flaky tests are identified and fixed
- [ ] Test documentation is up to date

Last Updated: $(date)
Version: 1.0.0