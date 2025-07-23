#!/bin/bash
# Script to help find GitHub App installation ID

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}GitHub App Installation Finder${NC}"
echo "================================"

# Method 1: Direct browser link
echo -e "\n${YELLOW}Method 1: Via Browser${NC}"
echo "1. Open this link in your browser:"
echo -e "   ${BLUE}https://github.com/settings/installations${NC}"
echo ""
echo "2. You should see your installations listed"
echo "3. Click on the installation (e.g., 'S-Corkum' or your org name)"
echo "4. Look at the URL - it will contain a number like:"
echo "   https://github.com/settings/installations/12345678"
echo "   The number (12345678) is your installation ID"

# Method 2: Using GitHub CLI if available
if command -v gh &> /dev/null; then
    echo -e "\n${YELLOW}Method 2: Using GitHub CLI${NC}"
    echo "Checking if you're logged in to GitHub CLI..."
    
    if gh auth status &> /dev/null; then
        echo -e "${GREEN}✓ GitHub CLI is authenticated${NC}"
        echo ""
        echo "Your GitHub App installations:"
        
        # Try to list app installations
        if gh api /user/installations --jq '.installations[] | {id: .id, account: .account.login}' 2>/dev/null; then
            echo ""
            echo -e "${GREEN}Found installations above!${NC}"
            echo "Use the 'id' value for your test organization/account"
        else
            echo -e "${YELLOW}Note: This requires GitHub App user-to-server token permissions${NC}"
        fi
    else
        echo -e "${RED}GitHub CLI not authenticated. Run: gh auth login${NC}"
    fi
else
    echo -e "\n${YELLOW}GitHub CLI not installed${NC}"
    echo "Install with: brew install gh"
fi

# Method 3: Test with curl
echo -e "\n${YELLOW}Method 3: Manual Testing${NC}"
echo "Once you have a potential installation ID, test it by updating .env.test:"
echo ""
echo "GITHUB_APP_INSTALLATION_ID=<your-id-here>"
echo ""
echo "Then run: ./scripts/test-github-integration.sh"

# Additional help
echo -e "\n${YELLOW}Common Installation Locations:${NC}"
echo "- Personal account: https://github.com/settings/apps/developer-mesh/installations"
echo "- Organization: https://github.com/organizations/YOUR_ORG/settings/installations"

echo -e "\n${YELLOW}Still can't find it?${NC}"
echo "The GitHub adapter might be able to auto-discover the installation ID"
echo "if there's only one installation for your app."