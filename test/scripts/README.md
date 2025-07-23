# Test Scripts Directory

This directory contains scripts specifically for testing the Developer Mesh project.

## Functional Testing Scripts

- `run_functional_tests.sh` - Original script to run functional tests in Docker containers
- `run_functional_tests_fixed.sh` - Fixed version of the functional test script with explicit Go path
- `run_functional_tests_host.sh` - Enhanced script that runs functional tests on the host machine

## Component Testing Scripts

- `run_api_tests.sh` - Script to run API tests
- `run_core_tests.sh` - Script to run core component tests
- `run_github_tests.sh` - Script for GitHub integration tests
- `run_inside_docker_tests.sh` - Script to run tests inside Docker containers

## Specialized Test Scripts

- `fixed_github_test.sh` - Focused test script for GitHub integration
- `github_ginkgo_test.sh` - Script to run GitHub tests using Ginkgo framework

## Usage Examples

```bash
# To run functional tests on the host
./run_functional_tests_host.sh

# To run just the GitHub integration tests
./github_ginkgo_test.sh

# To run API tests
./run_api_tests.sh
```

For scripts with additional options, run them with the `-h` or `--help` flag or check the script's header comments.
