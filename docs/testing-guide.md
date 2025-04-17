# MCP Server Testing Guide

This guide describes how to test the MCP (Managing Contexts Platform) server, including simulating AI Agent interactions with the platform.

## Prerequisites

- Docker and Docker Compose
- Python 3.8+
- Basic knowledge of HTTP requests and RESTful APIs

## Setting Up the Application Locally

1. **Clone the Repository**

   ```bash
   git clone https://github.com/yourusername/devops-mcp.git
   cd devops-mcp
   ```

2. **Start the Services**

   ```bash
   docker-compose up -d
   ```

   This will start the following services:
   - PostgreSQL with pg_vector extension (for vector embeddings)
   - Redis (for caching)
   - MCP Server (the main application)
   - Mock Server (for testing integrations)
   - Prometheus (for metrics)
   - Grafana (for dashboards)

3. **Verify Services are Running**

   ```bash
   docker-compose ps
   ```

   All services should show as "Up".

4. **Verify the Server is Healthy**

   ```bash
   curl http://localhost:8080/health
   ```

   You should see a response indicating that the server is healthy:
   ```json
   {"components":{"engine":"healthy","github":"healthy (mock)"},"status":"healthy"}
   ```

## AI Agent Simulation Tests

We've created a Python script to simulate an AI Agent interacting with the MCP server. This script tests:

1. Health check endpoint
2. Context creation
3. Context retrieval
4. Context updates
5. Vector embedding operations

### Running the Tests

1. Navigate to the test directory:

   ```bash
   cd test
   ```

2. Run the test script:

   ```bash
   ./run_agent_tests.sh
   ```

   This script will:
   - Set up a Python virtual environment
   - Install the required dependencies (requests, numpy)
   - Run the AI Agent simulation tests

3. Check the test results:

   The script will output the results of each test, indicating whether it passed or failed.

### Understanding Test Output

The test output provides detailed information about each HTTP request made to the server, including:

- Request URL
- Request headers
- Request body
- Response status code
- Response body

This information is valuable for debugging and understanding how the server works.

### Adapting the Tests

If the server API changes or you need to test different functionality, you can modify the `ai_agent_simulation.py` script. The main classes and functions are:

- `MCPClient`: A client for interacting with the MCP server
- `test_health_check`: Tests the health endpoint
- `test_context_creation`: Tests creating contexts
- `test_context_retrieval`: Tests retrieving contexts
- `test_context_update`: Tests updating contexts
- `test_embedding_operations`: Tests vector embedding operations

## Key Components Being Tested

### 1. PostgreSQL with pg_vector

The PostgreSQL database with the pg_vector extension is used for:

- Storing context references and metadata
- Storing vector embeddings for semantic search
- Enabling efficient similarity searches using the vector cosine distance

### 2. S3 Storage (or Local Storage in Development)

The storage component is used for:

- Storing large context data efficiently
- Providing scalable storage that can handle growing context volumes
- Maintaining separation between metadata (in PostgreSQL) and the actual context data

### 3. API Endpoints

The API endpoints being tested include:

- `/health`: Checks if the server is healthy
- `/api/v1/mcp/context`: Creates and manages contexts
- `/api/v1/embeddings`: Stores and retrieves vector embeddings (simulated in our tests)
- `/api/v1/embeddings/search`: Searches for similar embeddings (simulated in our tests)

## Troubleshooting

### Common Issues

1. **Services Not Starting**

   Check the Docker logs:
   ```bash
   docker-compose logs mcp-server
   ```

2. **Database Connection Issues**

   Ensure PostgreSQL is running and that the MCP server has the correct connection details:
   ```bash
   docker-compose logs postgres
   ```

3. **API Endpoint Errors**

   Check that you're using the correct API endpoints and request formats. The server logs may provide more information:
   ```bash
   docker-compose logs mcp-server
   ```

### Debugging Tips

1. Add `print()` statements to the test script to see more details about what's happening
2. Use the `-v` flag with curl to see detailed request and response information
3. Check the server logs for error messages
4. Use the Prometheus and Grafana dashboards to monitor server performance

## Further Testing

Beyond the basic API tests, consider testing:

1. **Load Testing**: Use tools like `wrk` or `ab` to test how the server performs under load
2. **Concurrency Testing**: Test multiple agents interacting with the server simultaneously
3. **Failure Recovery**: Test how the system recovers from component failures
4. **Persistence**: Test if contexts are properly persisted and can be recovered after a restart

## Conclusion

This testing guide provides a starting point for validating the MCP server functionality. As the platform evolves, the tests can be expanded to cover new features and integration points.
