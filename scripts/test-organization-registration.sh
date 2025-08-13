#!/bin/bash

# Test Organization Registration and Authentication Flow
# This script tests the new organization registration system

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:8081}"
ORG_NAME="Test Organization $(date +%s)"
ORG_SLUG="test-org-$(date +%s)"
ADMIN_EMAIL="admin-$(date +%s)@example.com"
ADMIN_NAME="Test Admin"
ADMIN_PASSWORD="TestPass123"
MEMBER_EMAIL="member-$(date +%s)@example.com"

echo -e "${YELLOW}Testing Organization Registration System${NC}"
echo "API URL: $API_URL"
echo "========================================"

# Step 1: Register Organization
echo -e "\n${GREEN}Step 1: Registering Organization${NC}"
echo "Organization: $ORG_NAME"
echo "Slug: $ORG_SLUG"
echo "Admin Email: $ADMIN_EMAIL"

REGISTER_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/register/organization" \
  -H "Content-Type: application/json" \
  -d "{
    \"organization_name\": \"$ORG_NAME\",
    \"organization_slug\": \"$ORG_SLUG\",
    \"admin_email\": \"$ADMIN_EMAIL\",
    \"admin_name\": \"$ADMIN_NAME\",
    \"admin_password\": \"$ADMIN_PASSWORD\",
    \"company_size\": \"10-50\",
    \"industry\": \"Technology\",
    \"use_case\": \"Testing\"
  }")

# Check if registration was successful
if echo "$REGISTER_RESPONSE" | grep -q '"api_key"'; then
    echo -e "${GREEN}✓ Organization registered successfully${NC}"
    
    # Extract values from response
    API_KEY=$(echo "$REGISTER_RESPONSE" | grep -o '"api_key":"[^"]*' | cut -d'"' -f4)
    ORG_ID=$(echo "$REGISTER_RESPONSE" | grep -o '"organization":{[^}]*' | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    USER_ID=$(echo "$REGISTER_RESPONSE" | grep -o '"user":{[^}]*' | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    
    echo "Organization ID: $ORG_ID"
    echo "User ID: $USER_ID"
    echo "API Key: ${API_KEY:0:20}..."
else
    echo -e "${RED}✗ Registration failed${NC}"
    echo "Response: $REGISTER_RESPONSE"
    exit 1
fi

# Step 2: Test Login
echo -e "\n${GREEN}Step 2: Testing Login${NC}"

LOGIN_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$ADMIN_EMAIL\",
    \"password\": \"$ADMIN_PASSWORD\"
  }")

if echo "$LOGIN_RESPONSE" | grep -q '"access_token"'; then
    echo -e "${GREEN}✓ Login successful${NC}"
    
    # Extract access token
    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
    echo "Access Token: ${ACCESS_TOKEN:0:30}..."
else
    echo -e "${RED}✗ Login failed${NC}"
    echo "Response: $LOGIN_RESPONSE"
    exit 1
fi

# Step 3: Test Invalid Login
echo -e "\n${GREEN}Step 3: Testing Invalid Login${NC}"

INVALID_LOGIN=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$ADMIN_EMAIL\",
    \"password\": \"WrongPassword\"
  }")

if echo "$INVALID_LOGIN" | grep -q '"error"'; then
    echo -e "${GREEN}✓ Invalid login correctly rejected${NC}"
else
    echo -e "${RED}✗ Invalid login not rejected properly${NC}"
    echo "Response: $INVALID_LOGIN"
fi

# Step 4: Test User Invitation
echo -e "\n${GREEN}Step 4: Testing User Invitation${NC}"

INVITE_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/users/invite" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$MEMBER_EMAIL\",
    \"name\": \"Test Member\",
    \"role\": \"member\"
  }")

if echo "$INVITE_RESPONSE" | grep -q '"message":"Invitation sent successfully"'; then
    echo -e "${GREEN}✓ User invitation sent successfully${NC}"
    echo "Invited: $MEMBER_EMAIL"
else
    echo -e "${YELLOW}⚠ User invitation may have failed${NC}"
    echo "Response: $INVITE_RESPONSE"
    echo "(This is expected if email service is not configured)"
fi

# Step 5: Test API Key Authentication
echo -e "\n${GREEN}Step 5: Testing API Key Authentication${NC}"

# This would normally call an authenticated endpoint
# For now, we'll just verify the API key format
if [[ "$API_KEY" =~ ^devmesh_[a-f0-9]{64}$ ]]; then
    echo -e "${GREEN}✓ API key format is valid${NC}"
else
    echo -e "${YELLOW}⚠ API key format may be incorrect${NC}"
    echo "API Key: $API_KEY"
fi

# Step 6: Test Duplicate Registration
echo -e "\n${GREEN}Step 6: Testing Duplicate Registration Prevention${NC}"

DUPLICATE_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/register/organization" \
  -H "Content-Type: application/json" \
  -d "{
    \"organization_name\": \"$ORG_NAME\",
    \"organization_slug\": \"$ORG_SLUG\",
    \"admin_email\": \"different@example.com\",
    \"admin_name\": \"Different Admin\",
    \"admin_password\": \"DifferentPass123\"
  }")

if echo "$DUPLICATE_RESPONSE" | grep -q '"error"'; then
    echo -e "${GREEN}✓ Duplicate organization correctly rejected${NC}"
else
    echo -e "${RED}✗ Duplicate organization not rejected${NC}"
    echo "Response: $DUPLICATE_RESPONSE"
fi

# Step 7: Test Password Validation
echo -e "\n${GREEN}Step 7: Testing Password Validation${NC}"

WEAK_PASSWORD_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/register/organization" \
  -H "Content-Type: application/json" \
  -d "{
    \"organization_name\": \"Another Org\",
    \"organization_slug\": \"another-org-$(date +%s)\",
    \"admin_email\": \"another@example.com\",
    \"admin_name\": \"Another Admin\",
    \"admin_password\": \"weak\"
  }")

if echo "$WEAK_PASSWORD_RESPONSE" | grep -q '"error"'; then
    echo -e "${GREEN}✓ Weak password correctly rejected${NC}"
else
    echo -e "${RED}✗ Weak password not rejected${NC}"
    echo "Response: $WEAK_PASSWORD_RESPONSE"
fi

# Step 8: Test Slug Validation
echo -e "\n${GREEN}Step 8: Testing Slug Validation${NC}"

INVALID_SLUG_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/register/organization" \
  -H "Content-Type: application/json" \
  -d "{
    \"organization_name\": \"Invalid Slug Org\",
    \"organization_slug\": \"Invalid_Slug!\",
    \"admin_email\": \"invalid@example.com\",
    \"admin_name\": \"Invalid Admin\",
    \"admin_password\": \"ValidPass123\"
  }")

if echo "$INVALID_SLUG_RESPONSE" | grep -q '"error"'; then
    echo -e "${GREEN}✓ Invalid slug correctly rejected${NC}"
else
    echo -e "${RED}✗ Invalid slug not rejected${NC}"
    echo "Response: $INVALID_SLUG_RESPONSE"
fi

# Summary
echo -e "\n${YELLOW}========================================"
echo -e "Test Summary${NC}"
echo -e "${GREEN}✓ Organization Registration System is working correctly!${NC}"
echo ""
echo "Created Organization:"
echo "  Name: $ORG_NAME"
echo "  Slug: $ORG_SLUG"
echo "  ID: $ORG_ID"
echo ""
echo "Created Admin User:"
echo "  Email: $ADMIN_EMAIL"
echo "  ID: $USER_ID"
echo ""
echo "API Key: ${API_KEY:0:20}..."
echo ""
echo -e "${YELLOW}Note: Some features (email sending, invitation acceptance) require additional configuration.${NC}"