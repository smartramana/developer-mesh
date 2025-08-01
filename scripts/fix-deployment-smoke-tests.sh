#!/bin/bash

# Script to fix deployment smoke tests that are failing silently
# This script updates the smoke test logic to properly fail the pipeline

set -e

echo "🔧 Fixing deployment smoke tests to properly fail on errors..."

# Create a temporary file with the new smoke test logic
cat > /tmp/smoke-test-fix.yml << 'EOF'
      - name: Smoke Test
        run: |
          echo "Running smoke tests..."
          sleep 10  # Give services time to stabilize
          
          # Track overall test status
          SMOKE_TEST_PASSED=true
          
          # Test REST API health endpoint
          echo "Testing REST API health endpoint..."
          API_HEALTHY=false
          for i in {1..5}; do
            if curl -f -s "https://api.dev-mesh.io/health"; then
              echo "✓ REST API is healthy"
              API_HEALTHY=true
              break
            fi
            echo "Attempt $i failed, retrying..."
            sleep 5
          done
          
          if [ "$API_HEALTHY" = "false" ]; then
            echo "❌ REST API health check failed after 5 attempts"
            SMOKE_TEST_PASSED=false
          fi
          
          # Test MCP health endpoint
          echo "Testing MCP health endpoint..."
          MCP_HEALTHY=false
          for i in {1..5}; do
            if curl -f -s "https://mcp.dev-mesh.io/health"; then
              echo "✓ MCP Server is healthy"
              MCP_HEALTHY=true
              break
            fi
            echo "Attempt $i failed, retrying..."
            sleep 5
          done
          
          if [ "$MCP_HEALTHY" = "false" ]; then
            echo "❌ MCP Server health check failed after 5 attempts"
            SMOKE_TEST_PASSED=false
          fi
          
          # Test WebSocket endpoint
          echo "Testing WebSocket endpoint..."
          ws_key=$(openssl rand -base64 16)
          ws_response=$(curl -s -o /dev/null -w "%{http_code}" \
            -H "Connection: Upgrade" \
            -H "Upgrade: websocket" \
            -H "Sec-WebSocket-Version: 13" \
            -H "Sec-WebSocket-Key: $ws_key" \
            "https://mcp.dev-mesh.io/ws")
          
          if [ "$ws_response" = "101" ] || [ "$ws_response" = "400" ] || [ "$ws_response" = "401" ]; then
            echo "✓ WebSocket endpoint accessible (HTTP $ws_response)"
          else
            echo "❌ WebSocket endpoint returned HTTP $ws_response (expected 101, 400, or 401)"
            SMOKE_TEST_PASSED=false
          fi
          
          # Check container status on failure
          if [ "$SMOKE_TEST_PASSED" = "false" ]; then
            echo ""
            echo "🔍 Checking container status on EC2..."
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
              -i ~/.ssh/ec2-nat-instance.pem ec2-user@\${{ secrets.EC2_INSTANCE_IP }} \
              "cd ~/developer-mesh && docker-compose ps" || echo "Failed to check container status"
            
            echo ""
            echo "📋 Recent container logs:"
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
              -i ~/.ssh/ec2-nat-instance.pem ec2-user@\${{ secrets.EC2_INSTANCE_IP }} \
              "cd ~/developer-mesh && docker-compose logs --tail=50" || echo "Failed to fetch logs"
          fi
          
          # Exit with failure if any smoke test failed
          if [ "$SMOKE_TEST_PASSED" = "false" ]; then
            echo ""
            echo "❌ SMOKE TESTS FAILED - Deployment is not healthy"
            exit 1
          else
            echo ""
            echo "✅ All smoke tests passed"
          fi
EOF

echo "✅ Created smoke test fix template"

# Also create a docker login fix for the deployment
cat > /tmp/docker-login-fix.yml << 'EOF'
      - name: Configure Docker Authentication
        run: |
          # Create docker login script for EC2
          cat > docker_login.sh << 'SCRIPT'
          #!/bin/bash
          echo "Configuring Docker authentication for GitHub Container Registry..."
          echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin
          if [ $? -eq 0 ]; then
            echo "✓ Docker login successful"
          else
            echo "❌ Docker login failed"
            exit 1
          fi
          SCRIPT
          
          # Copy and execute on EC2
          scp -o StrictHostKeyChecking=no -i ~/.ssh/ec2-nat-instance.pem \
            docker_login.sh ec2-user@${{ secrets.EC2_INSTANCE_IP }}:~/
          
          ssh -o StrictHostKeyChecking=no -i ~/.ssh/ec2-nat-instance.pem \
            ec2-user@${{ secrets.EC2_INSTANCE_IP }} \
            "chmod +x ~/docker_login.sh && ~/docker_login.sh"
EOF

echo "✅ Created docker authentication fix template"

echo ""
echo "📝 Summary of required changes:"
echo ""
echo "1. Update the Smoke Test step in deploy-production-v2.yml to:"
echo "   - Track the overall test status with SMOKE_TEST_PASSED variable"
echo "   - Check if each service health check actually succeeded"
echo "   - Exit with code 1 if any smoke test failed"
echo "   - Show container status and logs on failure for debugging"
echo ""
echo "2. Ensure Docker authentication is configured before pulling images:"
echo "   - Add a step to login to GitHub Container Registry"
echo "   - Use the GITHUB_TOKEN secret for authentication"
echo ""
echo "3. The key changes are:"
echo "   - API_HEALTHY and MCP_HEALTHY flags to track actual success"
echo "   - SMOKE_TEST_PASSED to track overall status"
echo "   - 'exit 1' at the end if any test failed"
echo ""
echo "These changes will ensure the deployment pipeline properly fails when services are not healthy."
echo ""
echo "The fix templates have been created in:"
echo "  - /tmp/smoke-test-fix.yml"
echo "  - /tmp/docker-login-fix.yml"