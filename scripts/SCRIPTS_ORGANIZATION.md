# Scripts Organization

This document describes the organization of scripts in the Developer Mesh project.

## Directory Structure

The project scripts have been organized into the following structure:

```
project-root/
├── scripts/            # General utility scripts and verification tools
├── test/
│   ├── scripts/        # Test-specific scripts
│   ├── functional/     # Functional test code
│   └── integration/    # Integration test code
```

## Scripts Directory (/scripts)

Contains general utility scripts and verification tools:

- Utility scripts for deployment, server management, etc.
- Verification scripts that check the integrity of components
- CI/CD scripts

See the [scripts README](./scripts/README.md) for details.

## Test Scripts Directory (/test/scripts)

Contains test-specific scripts:

- Functional test runners
- Component test scripts
- Specialized test scripts

See the [test scripts README](./test/scripts/README.md) for details.

## Running Tests

To run tests, use the appropriate script from the `/test/scripts` directory. For example:

```bash
# Run functional tests on the host
./test/scripts/run_functional_tests_host.sh

# Run API tests
./test/scripts/run_api_tests.sh
```

## Verification

To verify components, use the appropriate script from the `/scripts` directory:

```bash
# Verify GitHub integration
./scripts/verify_github_integration_complete.sh
```
