#!/bin/bash
# Run security scans on all modules in the Go workspace
# This script runs multiple security tools and aggregates results

set -euo pipefail

echo "=== Running comprehensive security scan ==="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track overall status
overall_status=0

# Check if go.work exists, if not create it
if [ ! -f "go.work" ]; then
    echo "No go.work file found, creating one..."
    go work init
    # Add all modules
    for dir in apps/mcp-server apps/rest-api apps/worker pkg; do
        if [ -f "$dir/go.mod" ]; then
            go work use "$dir"
        fi
    done
fi

# Ensure workspace is synchronized
echo "Synchronizing Go workspace..."
go work sync

# Get all module directories from go.work
modules=$(go work edit -json | jq -r '.Use[].DiskPath // empty' 2>/dev/null || echo "")

if [ -z "$modules" ]; then
    echo "No modules found in go.work, falling back to directory scan"
    modules="apps/mcp-server apps/rest-api apps/worker pkg"
fi

# 1. Run gosec
echo ""
echo "=== Running gosec security scanner ==="
if command -v gosec &> /dev/null; then
    gosec -fmt json -out gosec-report.json ./... 2>/dev/null || {
        echo -e "${YELLOW}⚠ Gosec found security issues${NC}"
        overall_status=1
    }
    echo -e "${GREEN}✓ Gosec scan completed${NC}"
else
    echo -e "${RED}✗ gosec not installed${NC}"
fi

# 2. Run govulncheck
echo ""
echo "=== Running govulncheck for vulnerability detection ==="
if command -v govulncheck &> /dev/null; then
    vuln_found=0
    for module in $modules; do
        if [ -d "$module" ]; then
            echo "Scanning $module for vulnerabilities..."
            (cd "$module" && govulncheck ./...) || vuln_found=1
        fi
    done
    
    if [ $vuln_found -eq 0 ]; then
        echo -e "${GREEN}✓ No known vulnerabilities found${NC}"
    else
        echo -e "${RED}✗ Vulnerabilities detected!${NC}"
        overall_status=1
    fi
else
    echo -e "${RED}✗ govulncheck not installed${NC}"
fi

# 3. Run staticcheck
echo ""
echo "=== Running staticcheck ==="
if command -v staticcheck &> /dev/null; then
    staticcheck ./... || {
        echo -e "${YELLOW}⚠ Staticcheck found issues${NC}"
        overall_status=1
    }
    echo -e "${GREEN}✓ Staticcheck completed${NC}"
else
    echo -e "${RED}✗ staticcheck not installed${NC}"
fi

# 4. Check for hardcoded secrets
echo ""
echo "=== Checking for hardcoded secrets ==="
# Simple regex patterns for common secrets
patterns=(
    'password\s*=\s*"[^"]+"'
    'api_key\s*=\s*"[^"]+"'
    'secret\s*=\s*"[^"]+"'
    'token\s*=\s*"[^"]+"'
    'AWS_SECRET_ACCESS_KEY'
    'PRIVATE_KEY'
)

secret_found=0
for pattern in "${patterns[@]}"; do
    if grep -r -i -E "$pattern" --include="*.go" --exclude-dir=vendor --exclude-dir=.git . 2>/dev/null | grep -v -E "(test|mock|example|_test\.go)"; then
        secret_found=1
    fi
done

if [ $secret_found -eq 0 ]; then
    echo -e "${GREEN}✓ No hardcoded secrets found${NC}"
else
    echo -e "${RED}✗ Potential hardcoded secrets detected!${NC}"
    overall_status=1
fi

# 5. Check dependencies for known issues
echo ""
echo "=== Checking dependency health ==="
for module in $modules; do
    if [ -d "$module" ]; then
        echo "Checking $module dependencies..."
        (cd "$module" && go mod verify) || {
            echo -e "${RED}✗ Module verification failed for $module${NC}"
            overall_status=1
        }
    fi
done
echo -e "${GREEN}✓ Dependency verification completed${NC}"

# Summary
echo ""
echo "=== Security Scan Summary ==="
if [ $overall_status -eq 0 ]; then
    echo -e "${GREEN}✓ All security checks passed!${NC}"
else
    echo -e "${RED}✗ Security issues found. Please review the output above.${NC}"
fi

exit $overall_status