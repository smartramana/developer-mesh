# MCP Server Troubleshooting Guide

This guide provides solutions for common issues encountered when running and using the MCP Server.

## Health Check Issues

### Problem: Health Check Failing

If the `/health` endpoint returns an unhealthy status:

1. **Check which component is unhealthy in the response**
   ```json
   {
     "status": "unhealthy",
     "components": {
       "engine": "healthy",
       "github": "unhealthy",
       "harness": "healthy"
     }
   }
   ```

2. **For unhealthy adapters**:
   - Verify the adapter's API credentials in your configuration
   - Check if the external service is available
   - Check for rate limiting or IP blocking
   - Verify network connectivity to the external service

3. **For unhealthy engine**:
   - Check database connectivity
   - Check cache connectivity
   - Inspect logs for engine initialization errors

4. **For database issues**:
   - Verify PostgreSQL is running: `docker-compose ps postgres`
   - Check connection settings in `config.yaml`
   - Try connecting manually: `psql -h localhost -U postgres -d mcp`

5. **For cache issues**:
   - Verify Redis is running: `docker-compose ps redis`
   - Check connection settings in `config.yaml`
   - Try connecting manually: `redis-cli ping`

## Startup Issues

### Problem: Server Fails to Start

If the MCP Server fails to start:

1. **Check logs for error messages**:
   ```bash
   docker-compose logs mcp-server
   # Or if running locally
   ./mcp-server 2>&1 | tee mcp-server.log
   ```

2. **Common startup errors**:

   - **Database connection failure**:
     ```
     Failed to initialize database: dial tcp 127.0.0.1:5432: connect: connection refused
     ```
     Solution: Ensure PostgreSQL is running and accessible.

   - **Redis connection failure**:
     ```
     Failed to initialize cache: dial tcp 127.0.0.1:6379: connect: connection refused
     ```
     Solution: Ensure Redis is running and accessible.

   - **Port already in use**:
     ```
     listen tcp :8080: bind: address already in use
     ```
     Solution: Change the `listen_address` in configuration or stop the process using port 8080.

   - **Configuration errors**:
     ```
     Failed to load configuration: invalid field ...
     ```
     Solution: Check your `config.yaml` against the template for syntax errors.

3. **Verify environment variables**:
   - If using environment variables, check they are properly set and formatted
   - Verify no conflicting configuration between file and environment

4. **Check file permissions**:
   - Ensure the MCP Server has read access to configuration files
   - Ensure the MCP Server has write access to log directories

## Webhook Issues

### Problem: Webhooks Not Processed

If webhooks from external systems are not being processed:

1. **Verify webhook endpoint configuration**:
   - Check if the webhook URL is correct and accessible from the external system
   - Verify that your server is accessible from the public internet if needed
   - Ensure ports are open on firewalls

2. **Check webhook signature verification**:
   - Ensure the webhook secret in your configuration matches the one in the external system
   - Look for webhook signature errors in logs:
     ```
     Failed to validate webhook signature: signature mismatch
     ```

3. **Verify webhook payload**:
   - Check logs for payload parsing errors
   - Verify the webhook payload format matches what the adapter expects
   - Test with a simplified payload

4. **Test webhook manually**:
   ```bash
   # GitHub webhook example
   curl -X POST \
     -H "Content-Type: application/json" \
     -H "X-GitHub-Event: push" \
     -H "X-Hub-Signature-256: sha256=<signature>" \
     -d '{"repository":{"full_name":"owner/repo"},"ref":"refs/heads/main"}' \
     http://your-server/webhook/github
   ```

5. **Check adapter configuration**:
   - Ensure the adapter for the webhook source is properly configured
   - Verify the adapter is initialized in the health check

## API Authentication Issues

### Problem: API Authentication Fails

If API authentication is failing:

1. **Check authentication headers**:
   - For JWT authentication, ensure the header format is: `Authorization: Bearer <token>`
   - For API key authentication, ensure the header format is: `Authorization: ApiKey <api-key>`

2. **Verify API keys**:
   - Check if the API key is in the list of authorized keys in your configuration
   - Verify the API key has not expired if you're using expiry logic

3. **Check JWT token**:
   - Ensure the JWT token is not expired
   - Verify the token signature with your JWT secret
   - Check if all required claims are present

4. **Test with curl**:
   ```bash
   # API key authentication
   curl -X GET -H "Authorization: ApiKey your-api-key" http://your-server/api/v1/github/repos
   
   # JWT authentication
   curl -X GET -H "Authorization: Bearer your-jwt-token" http://your-server/api/v1/github/repos
   ```

5. **Enable debug logging**:
   - Set logging level to DEBUG in your configuration
   - Look for authentication-related error messages

## Event Processing Issues

### Problem: Events Not Processed

If events are not being processed correctly:

1. **Check event processing logs**:
   - Look for error messages during event processing
   - Verify that events are being received by the engine

2. **Verify adapter subscriptions**:
   - Check if event handlers are registered for the event type
   - Ensure the adapter is properly subscribing to events

3. **Check for event queue backlog**:
   - If the event queue is full, events might be dropped
   - Increase the `event_buffer_size` in the engine configuration

4. **Verify external system responses**:
   - If event processing requires API calls to external systems, check their status
   - Look for timeout or rate limiting errors

5. **Check database and cache operations**:
   - If event processing requires database or cache operations, verify connectivity
   - Look for error messages related to database or cache operations

## Performance Issues

### Problem: High Latency

If the MCP Server is experiencing high latency:

1. **Check system resources**:
   - Monitor CPU and memory usage
   - Verify disk I/O is not a bottleneck
   - Check network latency to external systems

2. **Database query performance**:
   - Look for slow queries in PostgreSQL logs
   - Add indexes for frequently queried fields
   - Optimize complex queries

3. **Cache effectiveness**:
   - Check cache hit/miss ratio in metrics
   - Ensure critical data is cached appropriately
   - Verify cache size is adequate for your workload

4. **Concurrency settings**:
   - Adjust `concurrency_limit` in engine configuration
   - Tune database connection pool settings
   - Adjust worker counts based on available CPU cores

5. **External API latency**:
   - Monitor response times from external systems
   - Implement more aggressive caching for slow APIs
   - Consider using circuit breakers for unstable APIs

### Problem: Memory Leaks

If the MCP Server's memory usage keeps increasing:

1. **Monitor resource usage over time**:
   - Use Prometheus metrics to track memory usage
   - Look for patterns in memory growth

2. **Check for goroutine leaks**:
   - Monitor the number of goroutines over time
   - Look for consistently increasing goroutine count
   - Use `pprof` to profile the application

3. **Restart the server**:
   - As a temporary solution, restart the server
   - Consider implementing automatic restarts if memory exceeds threshold

4. **Upgrade to latest version**:
   - Memory leaks are often fixed in newer versions
   - Check the changelog for memory-related fixes

## Adapter-Specific Issues

### GitHub Adapter Issues

1. **Authentication errors**:
   - Ensure your GitHub token has the required scopes
   - Verify the token has not expired
   - Check for rate limiting headers in GitHub responses

2. **Webhook verification failures**:
   - Verify the webhook secret is correctly configured
   - Check that GitHub is sending the `X-Hub-Signature-256` header
   - Ensure payload is not modified in transit (e.g., by proxies)

3. **API rate limiting**:
   - Implement caching for frequently accessed data
   - Use conditional requests with If-None-Match
   - Adjust polling frequency if applicable

### Harness Adapter Issues

1. **Authentication errors**:
   - Verify Harness API token and account ID
   - Check API token permissions
   - Ensure the Harness instance is accessible

2. **Webhook processing errors**:
   - Check Harness webhook configuration
   - Verify webhook payload format
   - Ensure the webhook endpoint is accessible by Harness

3. **API response parsing errors**:
   - Check if the Harness API has changed
   - Update adapter to handle API changes
   - Look for specific parsing error messages

### SonarQube Adapter Issues

1. **Authentication errors**:
   - Verify SonarQube token is valid
   - Check token permissions
   - Ensure SonarQube instance is accessible

2. **Webhook processing errors**:
   - Check SonarQube webhook configuration
   - Verify webhook payload format
   - Test webhook delivery with SonarQube's test button

### Artifactory Adapter Issues

1. **Authentication errors**:
   - Try API key instead of username/password
   - Verify credentials and permissions
   - Check Artifactory access logs for failed attempts

2. **Repository access issues**:
   - Verify permissions for accessing repositories
   - Check for repository name changes
   - Ensure repositories exist

### Xray Adapter Issues

1. **Authentication errors**:
   - Verify Xray credentials
   - Check if Xray is licensed and active
   - Ensure Xray API endpoints are accessible

2. **Scan result issues**:
   - Check if scans are completing successfully
   - Verify webhook configuration for scan completion events
   - Ensure correct component paths are being monitored

## Mock Server Issues

### Problem: Mock Server Not Working

If the mock server is not working correctly:

1. **Verify mock server is running**:
   ```bash
   docker-compose ps mockserver
   # Or check process
   ps aux | grep mockserver
   ```

2. **Check mock server logs**:
   ```bash
   docker-compose logs mockserver
   # Or if running locally
   ./mockserver 2>&1 | tee mockserver.log
   ```

3. **Test mock server endpoints directly**:
   ```bash
   curl http://localhost:8081/mock-github/
   curl http://localhost:8081/health
   ```

4. **Verify mock configuration**:
   - Check that adapters are configured to use mock mode
   - Verify mock URLs are correct
   - Ensure `mock_responses` is set to `true` in configuration

## Docker Issues

### Problem: Docker Container Not Starting

If the Docker container fails to start:

1. **Check container logs**:
   ```bash
   docker-compose logs mcp-server
   ```

2. **Verify Docker resources**:
   - Ensure Docker has enough resources (CPU, memory)
   - Check disk space for Docker volumes

3. **Check Docker network**:
   - Verify containers can communicate with each other
   - Check Docker network configuration
   ```bash
   docker network inspect mcp-network
   ```

4. **Rebuild containers**:
   ```bash
   docker-compose build --no-cache
   docker-compose up -d
   ```

5. **Check Docker Compose file**:
   - Verify service dependencies are correct
   - Check volume mappings and port bindings

## Configuration Issues

### Problem: Configuration Not Applied

If configuration changes are not being applied:

1. **Verify configuration file location**:
   - Check if the server is using the correct configuration file
   - Look for file path in logs at startup

2. **Check environment variables**:
   - Environment variables may override file configuration
   - Verify no conflicting environment variables are set

3. **Restart the server**:
   - Some configuration changes require a server restart
   ```bash
   docker-compose restart mcp-server
   # Or if running locally
   ./mcp-server
   ```

4. **Validate configuration format**:
   - Check YAML syntax is correct
   - Verify all required fields are present
   - Use a YAML validator to check for syntax errors

## Database Issues

### Problem: Database Connection Errors

If you're experiencing database connection issues:

1. **Check database status**:
   ```bash
   docker-compose ps postgres
   # Or if running externally
   pg_isready -h localhost -p 5432
   ```

2. **Verify connection settings**:
   - Check database host, port, username, password, and database name
   - Verify SSL settings if applicable

3. **Test manual connection**:
   ```bash
   psql -h localhost -U postgres -d mcp
   ```

4. **Check connection pool settings**:
   - Adjust `max_open_conns` and `max_idle_conns` in configuration
   - Monitor active connections with `SELECT count(*) FROM pg_stat_activity`

5. **Check database logs**:
   ```bash
   docker-compose logs postgres
   ```

## Cache Issues

### Problem: Cache Connection Errors

If you're experiencing cache connection issues:

1. **Check Redis status**:
   ```bash
   docker-compose ps redis
   # Or if running externally
   redis-cli ping
   ```

2. **Verify connection settings**:
   - Check Redis host, port, password, and database number
   - Test connection with `redis-cli -h localhost -p 6379`

3. **Monitor Redis usage**:
   ```bash
   redis-cli info memory
   redis-cli info stats
   ```

4. **Check cache configuration**:
   - Adjust pool size and connection settings
   - Verify TTL settings for cached items

5. **Flush cache if corrupted**:
   ```bash
   redis-cli flushdb
   ```

## Getting Support

If you continue to experience issues after trying the solutions in this guide:

1. **Check logs for detailed error messages**
2. **Review the documentation for configuration guidance**
3. **Search for similar issues in the project repository**
4. **Create a detailed bug report including**:
   - MCP Server version
   - Configuration (with sensitive information removed)
   - Error messages and logs
   - Steps to reproduce the issue
   - Environment information (OS, Docker version, etc.)