# MCP Server Testing Guide

This guide describes how to test the MCP (Managing Contexts Platform) server, including unit testing, integration testing, and functional testing approaches.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Unit Testing](#unit-testing)
   - [Running Unit Tests](#running-unit-tests)
   - [Test Coverage](#test-coverage)
   - [Table-Driven Tests](#table-driven-tests)
   - [Mocking Dependencies](#mocking-dependencies)
   - [Property-Based Testing](#property-based-testing)
3. [Integration Testing](#integration-testing)
   - [Setting Up the Test Environment](#setting-up-the-test-environment)
   - [Running Integration Tests](#running-integration-tests)
   - [AI Agent Simulation Tests](#ai-agent-simulation-tests)
4. [Functional Testing](#functional-testing)
   - [Test Environment Overview](#test-environment-overview)
   - [Running Functional Tests](#running-functional-tests)
   - [Test Structure](#test-structure)
   - [Extending Functional Tests](#extending-functional-tests)
   - [Troubleshooting Functional Tests](#troubleshooting-functional-tests)
5. [Key Components Being Tested](#key-components-being-tested)
6. [Troubleshooting](#troubleshooting)
7. [Further Testing](#further-testing)

## Prerequisites

- Go 1.24+
- Docker and Docker Compose (for integration and functional tests)
- Python 3.8+ (for agent simulation tests)
- Basic knowledge of Go testing and HTTP APIs
- Ginkgo and Gomega (for functional tests)

## Unit Testing

The MCP Server uses Go's built-in testing package along with testify for assertions and mocking. Our unit testing approach follows these key principles:

### Running Unit Tests

To run all unit tests:

```bash
go test ./...
```

To run tests in a specific package:

```bash
go test ./internal/api
```

To run a specific test:

```bash
go test ./internal/api -run TestCreateContext
```

### Test Coverage

We aim for a minimum of 80% test coverage across the codebase. To check test coverage:

```bash
# Get overall coverage
go test -cover ./...

# Generate a coverage report
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out
```

The HTML report highlights covered and uncovered code sections, making it easy to identify areas that need additional testing.

### Table-Driven Tests

We use table-driven tests for testing multiple scenarios with the same test logic. This approach makes tests more maintainable and easier to extend. Example:

```go
func TestSummarizeContext(t *testing.T) {
    testCases := []struct {
        name           string
        contextID      string
        mockSetup      func()
        expectedResult string
        expectedError  bool
    }{
        {
            name:           "valid context",
            contextID:      "context-123",
            mockSetup:      func() { /* Set up mocks */ },
            expectedResult: "2 messages, 100 tokens",
            expectedError:  false,
        },
        {
            name:           "context not found",
            contextID:      "nonexistent",
            mockSetup:      func() { /* Set up mocks */ },
            expectedResult: "",
            expectedError:  true,
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

Benefits of this approach:
- Eliminates code duplication
- Makes adding new test cases easier
- Improves test readability and maintenance
- Clearly documents the expected behavior for each scenario

### Mocking Dependencies

We use the `testify/mock` package to create mocks for dependencies. For consistent mocking:

1. Define interfaces for all dependencies to enable mocking
2. Create mock implementations of these interfaces using testify's `mock.Mock`
3. Set up expectations using `On()` and `Return()` methods

Example:

```go
// Mock setup
mockDB := new(mocks.DatabaseInterface)
mockDB.On("GetContext", mock.Anything, "context-123").Return(expectedContext, nil)

// Call the method being tested
sut := NewContextManager(mockDB)
result, err := sut.GetContext(context.Background(), "context-123")

// Verify expectations
mockDB.AssertExpectations(t)
```

For database interactions, we use `DATA-DOG/go-sqlmock` to mock SQL queries and results.

### Property-Based Testing

For complex functions with many possible inputs, we use property-based testing to discover edge cases automatically:

```go
func FuzzProcessContext(f *testing.F) {
    // Seed corpus
    f.Add("sample text", 10)
    
    // Define the fuzz test
    f.Fuzz(func(t *testing.T, input string, maxTokens int) {
        // Test that invariants hold for all inputs
    })
}
```

To run fuzz tests:

```bash
go test -fuzz=FuzzProcessContext -fuzztime=30s ./...
```

## Integration Testing

Integration tests verify that components work together correctly. These tests interact with actual dependencies and external services.

### Setting Up the Test Environment

1. Start the integration test environment:

   ```bash
   docker-compose up -d
   ```

   This starts PostgreSQL, Redis, and other required services.

2. Verify that services are running:

   ```bash
   docker-compose ps
   ```

### Running Integration Tests

Integration tests are tagged with `//go:build integration` and can be run separately:

```bash
go test -tags=integration ./...
```

For specific integration tests:

```bash
go test -tags=integration ./test/integration
```

### AI Agent Simulation Tests

We provide Python-based tests that simulate AI Agent interactions with the MCP server:

1. Navigate to the test directory:

   ```bash
   cd test
   ```

2. Run the agent simulation tests:

   ```bash
   ./run_agent_tests.sh
   ```

   This script tests the end-to-end workflow including context management and tool integration.

## Functional Testing

Functional tests verify that the system works correctly from an external perspective, testing entire features and workflows as a user would experience them. We use Ginkgo and Gomega for our functional testing framework.

### Test Environment Overview

The functional testing environment is defined in `docker-compose.test.yml` and includes:

- **PostgreSQL with pgvector extension**: For database operations and vector search
- **Redis**: For caching and temporary data storage
- **Mockserver**: For simulating external services
- **MCP Server**: Configured to use these dependencies

### Running Functional Tests

To run the functional tests:

```bash
# Make the script executable (if needed)
chmod +x run_functional_tests.sh

# Run all functional tests
./run_functional_tests.sh

# Run with verbose output
./run_functional_tests.sh --verbose

# Run specific tests by focus
./run_functional_tests.sh --focus "API"

# Keep containers running after tests for debugging
./run_functional_tests.sh --keep-up
```

The script handles:
1. Starting the test environment with Docker Compose
2. Waiting for all services to be healthy
3. Running the Ginkgo tests
4. Cleaning up resources when done

### Test Structure

The functional tests are organized in the `test/functional` directory:

1. **API Tests** (`api/api_test.go`): 
   - Verify API contracts (endpoints, responses)
   - Test authentication and authorization
   - Check error handling

2. **Tool Integration Tests** (`integrations/tool_integrations_test.go`):
   - Test GitHub integration
   - Test MockServer integration
   - Verify tool actions work correctly

3. **End-to-End Workflow Tests** (`workflows/workflows_test.go`):
   - Context management workflows
   - Tool integration workflows
   - Vector search workflows

4. **Client** (`client/client.go`):
   - Provides a client for interacting with the MCP server
   - Handles HTTP requests and response parsing

### Extending Functional Tests

To add new functional tests:

1. Identify which category your test belongs to (API, Integration, Workflow)
2. Add a new test function using Ginkgo's BDD-style syntax
3. Run the tests to verify they work correctly

Example of adding a new test:

```go
Describe("New Feature", func() {
    It("should work correctly", func() {
        // Test code here
        result, err := someOperation()
        Expect(err).NotTo(HaveOccurred())
        Expect(result).To(Equal(expectedValue))
    })
})
```

### Troubleshooting Functional Tests

If you encounter issues with functional tests:

1. **Verify all services are running**:
   ```bash
   docker-compose -f docker-compose.test.yml ps
   ```

2. **Check service logs**:
   ```bash
   docker-compose -f docker-compose.test.yml logs mcp-server
   ```

3. **Verify PostgreSQL pgvector extension**:
   ```bash
   docker-compose -f docker-compose.test.yml exec postgres psql -U postgres -c "SELECT * FROM pg_extension WHERE extname = 'vector';"
   ```

4. **Inspect container health**:
   ```bash
   docker-compose -f docker-compose.test.yml ps
   ```

5. **Run tests in verbose mode**:
   ```bash
   ./run_functional_tests.sh --verbose
   ```

6. **Focus on specific tests**:
   ```bash
   ./run_functional_tests.sh --focus "specific test description"
   ```

## Key Components Being Tested

### 1. Core Components

- **Context Manager**: Handles creation, retrieval, and updating of conversation contexts
- **Embedding Repository**: Manages vector embeddings for semantic search
- **Tool Integration**: Handles interactions with external tools like GitHub, AWS, etc.

### 2. API Layer

- **RESTful API Endpoints**: Expose context management and tool integration capabilities
- **Middleware**: Handling authentication, logging, error handling, etc.

### 3. External Integrations

- **PostgreSQL Database**: For structured data and vector storage
- **Redis Cache**: For temporary data and performance optimization
- **S3 Storage**: For large context data storage

## Troubleshooting

### Common Issues

1. **Test Failures Due to Changed Dependencies**

   If mocked dependencies change, update the mock expectations accordingly.

2. **Flaky Tests**

   Use `-count=N` to run tests multiple times and detect flakiness:
   
   ```bash
   go test -count=10 ./...
   ```

3. **Database Connection Issues in Integration Tests**

   Check Docker Compose logs and ensure database migrations are applied:
   
   ```bash
   docker-compose logs postgres
   ```

### Debugging Tips

1. Use `t.Logf()` for debugging output (only visible when tests fail or with `-v` flag)
2. For mock debugging, add `mockObj.On(...).Run(func(args mock.Arguments) { fmt.Printf("%+v\n", args) }).Return(...)`
3. Use the `-v` flag for verbose test output: `go test -v ./...`
4. Enable race detection with `-race` to find concurrency issues

## Further Testing

Beyond unit, integration, and functional tests, consider:

1. **Load Testing**: Evaluate system performance under high load using tools like `wrk` or `k6`
2. **Chaos Testing**: Introduce failures to test resilience and recovery
3. **Security Testing**: Scan for vulnerabilities and test authentication/authorization
4. **Continuous Integration**: Automate testing in CI/CD pipelines

## Conclusion

This testing guide provides comprehensive practices for testing the MCP server. By implementing a combination of unit, integration, and functional testing, we ensure that the platform remains reliable and maintainable as it evolves.
