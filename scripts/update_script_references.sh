#!/bin/bash
# Script to update references to moved script files

set -e  # Exit on error

# Set current directory to project root
cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

echo "Updating references to moved script files..."

# Test scripts that have been moved to test/scripts
TEST_SCRIPTS=(
  "run_functional_tests.sh"
  "run_functional_tests_fixed.sh"
  "run_functional_tests_host.sh"
  "run_api_tests.sh"
  "run_core_tests.sh"
  "run_github_tests.sh"
  "run_inside_docker_tests.sh"
  "fixed_github_test.sh"
  "github_ginkgo_test.sh"
)

# Utility scripts that have been moved to scripts
UTILITY_SCRIPTS=(
  "verify_github_integration.sh"
  "verify_github_integration_complete.sh"
  "restart_servers.sh"
)

# Update references in files
find . -type f -not -path "*/\.*" -not -path "*/node_modules/*" -not -path "*/vendor/*" | while read -r file; do
  # Skip binary files
  if [ -f "$file" ] && file "$file" | grep -q text; then
    # Update references to test scripts
    for script in "${TEST_SCRIPTS[@]}"; do
      # Update references like ./script.sh or script.sh to ./test/scripts/script.sh
      sed -i.bak "s|\./\?$script|./test/scripts/$script|g" "$file"
    done
    
    # Update references to utility scripts
    for script in "${UTILITY_SCRIPTS[@]}"; do
      # Update references like ./script.sh or script.sh to ./scripts/script.sh
      sed -i.bak "s|\./\?$script|./scripts/$script|g" "$file"
    done
    
    # Remove backup files
    if [ -f "$file.bak" ]; then
      rm "$file.bak"
    fi
  fi
done

echo "References updated successfully!"
echo ""
echo "You may need to manually verify and update some references,"
echo "especially in more complex files like Dockerfiles or CI configurations."
