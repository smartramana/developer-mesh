# MCP Server Integration Testing Guide

This guide provides best practices, strategies, and procedures for implementing and running integration tests for the MCP (Model Context Protocol) Server.

## Table of Contents

1. [Introduction to Integration Testing](#introduction-to-integration-testing)
2. [Integration Testing Principles](#integration-testing-principles)
3. [Test Environment Setup](#test-environment-setup)
4. [Test Structure and Organization](#test-structure-and-organization)
5. [Test Data Management](#test-data-management)
6. [Mocking External Services](#mocking-external-services)
7. [Test Cases Coverage](#test-cases-coverage)
8. [Running Integration Tests](#running-integration-tests)
9. [Handling Failures and Troubleshooting](#handling-failures-and-troubleshooting)
10. [Continuous Integration](#continuous-integration)

---

## Introduction to Integration Testing

Integration testing verifies that different modules, components, or services within the MCP Server function correctly when combined. Unlike unit tests that focus on individual functions or methods in isolation, integration tests validate the interactions between integrated components, external dependencies, and the application as a whole.

For MCP Server, integration testing is critical because:

1. **Complex Dependencies**: MCP Server integrates with multiple external services (GitHub, databases, caching systems)
2. **API Correctness**: We need to ensure API endpoints behave as expected
3. **Database Interactions**: Tests validate that data is properly stored, retrieved, and managed
4. **Tool Integration Flow**: Verifies that tool operations work end-to-end

Integration tests provide confidence that components work together correctly in real-world scenarios, catching issues that isolated unit tests might miss.

---

## Integration Testing Principles

### Core Principles

1. **Test Real Interactions**: Integration tests should exercise actual interactions between components rather than mocking them entirely.

2. **Minimize Test Scope**: Each test should focus on a specific integration point rather than testing everything at once.

3. **Independent and Idempotent**: Tests should be able to run independently and repeatedly without interference.

4. **Clean State**: Tests should start with a known state and clean up after themselves.

5. **Realistic Scenarios**: Tests should mimic real-world usage patterns and edge cases.

6. **Fast Feedback**: While integration tests are slower than unit tests, they should still be optimized for speed where possible.

### Test Isolation Strategies

1. **Database Isolation**: Use separate schemas, transactions, or containerized databases
   
2. **Service Isolation**: Run test instances of services with separate configurations
   
3. **Cleanup Procedures**: Implement proper cleanup to avoid test pollution

---

## Test Environment Setup

### Local Development Environment

For running integration tests locally, ensure:

1. **Docker and Docker Compose**: Used to run dependent services

2. **PostgreSQL**: Database for storing structured data 

3. **Redis**: For caching and ephemeral data

4. **Mock Server**: For simulating external API responses

### Setup Script

Use the `scripts/test_integration.sh` script to automatically:

1. Build the MCP Server
2. Start required Docker containers
3. Set up the test environment
4. Run the integration tests
5. Clean up resources

Example:
```bash
# Run the setup script
./scripts/test_integration.sh
```

### Manual Setup

If you prefer a manual setup:

1. Start dependencies:
   ```bash
   docker-compose up -d postgres redis mockserver
   ```

2. Build the server:
   ```bash
   make build
   ```

3. Run integration tests:
   ```bash
   go test -tags=integration ./test/integration/...
   ```

---

## Test Structure and Organization

### Directory Structure

Integration tests should be organized as follows:

```
test/
  ├── integration/                 # All integration tests
  │   ├── api/                     # Tests for API endpoints
  │   │   ├── health_test.go       # Health endpoint tests
  │   │   ├── tools_test.go        # Tool operations tests
  │   │   └── webhook_test.go      # Webhook processing tests
  │   ├── adapters/                # Tests for external integrations 
  │   │   ├── github_test.go       # GitHub integration tests
  │   │   └── ...
  │   ├── database/                # Database interaction tests
  │   │   ├── postgres_test.go     # PostgreSQL integration tests
  │   │   └── ...
  │   ├── cache/                   # Cache integration tests
  │   │   ├── redis_test.go        # Redis integration tests
  │   │   └── ...
  │   ├── testutils/               # Shared testing utilities
  │   │   ├── docker.go            # Docker container management
  │   │   ├── http.go              # HTTP test helpers
  │   │   └── cleanup.go           # Cleanup utilities
  │   ├── fixtures/                # Test data fixtures
  │   │   ├── github_events.json   # Sample GitHub webhook payloads
  │   │   └── ...
  │   └── setup_test.go            # Common test setup and teardown
```

### Test File Structure

Each test file should follow this structure:

```go
// Use build tag to mark integration tests
//go:build integration

package api_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/S-Corkum/mcp-server/internal/api"
    "github.com/S-Corkum/mcp-server/test/integration/testutils"
)

// TestMain for setup/teardown of test suite
func TestMain(m *testing.M) {
    // Setup code
    
    // Run tests
    code := m.Run()
    
    // Teardown code
    
    os.Exit(code)
}

// Individual test functions
func TestHealthEndpoint(t *testing.T) {
    // Test implementation
}
```

---

## Test Data Management

### Test Fixtures

1. **Static Fixtures**: JSON, YAML, or other files containing pre-defined test data

2. **Generated Fixtures**: Programmatically generated test data for specific scenarios

3. **Database Fixtures**: SQL scripts or Go code to populate the database with test data

### Test Data Cleanup

Strategies for ensuring test data doesn't pollute the test environment:

1. **Transaction Rollback**: Use database transactions and roll back after tests
   
2. **Isolated Schemas**: Use separate database schemas for testing
   
3. **Container Recreation**: Destroy and recreate containers between test runs
   
4. **Explicit Cleanup**: Execute cleanup code to remove test data

Example cleanup in TestMain:

```go
func TestMain(m *testing.M) {
    // Setup test database
    db, cleanup := setupTestDB()
    defer cleanup()
    
    // Run tests
    code := m.Run()
    
    os.Exit(code)
}

func setupTestDB() (*sql.DB, func()) {
    // Connect to the database
    db, err := sql.Open("postgres", testDBConnString)
    if err != nil {
        log.Fatalf("Failed to connect to test database: %v", err)
    }
    
    // Execute setup SQL scripts
    setupSQL, err := ioutil.ReadFile("testdata/setup.sql")
    if err != nil {
        log.Fatalf("Failed to read setup SQL: %v", err)
    }
    
    _, err = db.Exec(string(setupSQL))
    if err != nil {
        log.Fatalf("Failed to execute setup SQL: %v", err)
    }
    
    // Return the database connection and a cleanup function
    return db, func() {
        // Execute cleanup SQL
        cleanupSQL, err := ioutil.ReadFile("testdata/cleanup.sql")
        if err != nil {
            log.Printf("Failed to read cleanup SQL: %v", err)
        } else {
            _, err = db.Exec(string(cleanupSQL))
            if err != nil {
                log.Printf("Failed to execute cleanup SQL: %v", err)
            }
        }
        
        // Close the database connection
        db.Close()
    }
}
```

---

## Mocking External Services

### Approaches to Mocking

1. **Mock Server**: Use a dedicated mock server to simulate external APIs
   
2. **Service Virtualization**: Record and replay API responses
   
3. **Test Containers**: Use lightweight containers that mimic external services
   
4. **Network Interceptors**: Intercept HTTP requests and return predefined responses

### Mock Server for DevOps Tools

The MCP Server includes a mockserver (`/cmd/mockserver`) specifically designed to simulate responses from various DevOps tools like GitHub.

To use the mock server:

1. Start the mock server:
   ```bash
   ./mockserver
   ```

2. Configure adapters to use mock responses:
   ```yaml
   adapters:
     github:
       mock_responses: true
       mock_url: "http://localhost:8081/mock-github"
   ```

3. The mock server provides canned responses for common API requests, such as:
   - Repository listing
   - Issue creation
   - Pull request operations
   - Commit information

4. Custom responses can be added to the mock server as needed for specific test cases.

---

## Test Cases Coverage

Integration tests should cover key integration points and scenarios, including:

### API Endpoints

1. **Health Endpoint**: Verify health reporting is accurate
2. **Tool Operations**: Test tool discovery, execution, and querying
3. **Webhooks**: Test webhook processing and event handling

### Database Operations

1. **CRUD Operations**: Create, Read, Update, Delete operations
2. **Transaction Handling**: Verify transaction behavior
3. **Resilience**: Test handling of database failures

### Cache Operations

1. **Caching Logic**: Verify data is correctly cached and retrieved
2. **Expiration**: Test cache key expiration
3. **Cache Invalidation**: Verify cache is properly invalidated when data changes

### External Integrations (GitHub)

1. **Authentication**: Test authentication with external services
2. **API Operations**: Verify operations against external APIs
3. **Webhook Processing**: Test handling of webhook events

### Resilience and Error Handling

1. **Service Unavailability**: Test behavior when external services are down
2. **Retry Logic**: Verify retry mechanisms work as expected
3. **Circuit Breaking**: Test circuit breaker patterns
4. **Timeout Handling**: Verify timeouts are properly enforced

---

## Running Integration Tests

### Command-Line Execution

Run all integration tests:

```bash
go test -tags=integration ./test/integration/...
```

Run tests in a specific package:

```bash
go test -tags=integration ./test/integration/api
```

Run a specific test:

```bash
go test -tags=integration ./test/integration/api -run TestHealthEndpoint
```

### Automation Script

Use the provided script for running integration tests:

```bash
./scripts/test_integration.sh
```

### Options and Flags

Common flags for integration tests:

- `-v`: Verbose output
- `-count=N`: Run tests N times to check for flakiness
- `-timeout=5m`: Set a timeout for the entire test run
- `-parallel=4`: Run tests in parallel (adjust based on system resources)

Example:

```bash
go test -tags=integration -v -timeout=5m ./test/integration/...
```

---

## Handling Failures and Troubleshooting

### Common Integration Test Failures

1. **Environment Issues**: Docker services not running, wrong ports, etc.
2. **Database Connectivity**: Cannot connect to the database
3. **Redis Connectivity**: Cannot connect to Redis
4. **Mock Server Issues**: Mock server not running or not responding correctly
5. **Configuration Issues**: Incorrect configurations or environment variables
6. **Timing Issues**: Race conditions, timeouts, etc.

### Troubleshooting Strategies

1. **Verbose Logging**: Enable verbose test output with `-v`
2. **Container Logs**: Check Docker container logs
   ```bash
   docker-compose logs postgres
   ```

3. **Database Inspection**: Connect to the test database to inspect data
   ```bash
   docker exec -it devops-mcp-postgres-1 psql -U postgres -d mcp
   ```

4. **Mock Server Logs**: Check the mock server logs
   ```bash
   docker-compose logs mockserver
   ```

5. **HTTP Tracing**: Enable HTTP request/response tracing in tests
   ```go
   client := &http.Client{Transport: &loggingTransport{http.DefaultTransport}}
   ```

6. **Single Test Run**: Run a single failing test for focused debugging
   ```bash
   go test -tags=integration -v ./test/integration/api -run TestSpecificFeature
   ```

---

## Continuous Integration

### CI Pipeline Integration

Integration tests should be part of your CI/CD pipeline:

1. **Separate Stage**: Run integration tests in a separate CI stage
2. **Dependencies**: Ensure Docker and other dependencies are available
3. **Parallelization**: Split tests for faster execution
4. **Caching**: Cache dependencies when possible
5. **Timeout Setting**: Set appropriate timeouts for integration tests

### Example GitHub Actions Workflow

```yaml
name: Integration Tests

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:17-alpine
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: mcp
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.20'
    
    - name: Build
      run: |
        make build
        make mockserver-build
    
    - name: Start mockserver
      run: |
        ./mockserver &
        sleep 5
    
    - name: Run integration tests
      run: |
        go test -tags=integration -v -timeout=10m ./test/integration/...
```

---

## Conclusion

Integration testing is a critical part of ensuring the MCP Server functions correctly as a whole. By following the practices outlined in this guide, you'll create robust integration tests that provide confidence in the system's behavior and quickly identify issues when they arise.

Remember that while integration tests are more complex and slower than unit tests, they provide valuable validation of system behavior that unit tests cannot. A balanced approach using both unit and integration tests will provide the best coverage and confidence in your codebase.
