# Phase 4.1: Integration Testing Plan

## Overview
This document outlines the approach for verifying cross-package functionality as part of the Go Workspace migration from `internal` to `pkg` directories. The focus is on ensuring that packages interact correctly following their migration and interface standardization, while also cleaning up architectural issues and removing unnecessary files.

## Key Objectives

1. Verify cross-package interactions function correctly
2. Identify and relocate misplaced application logic
3. Remove unnecessary files and legacy code
4. Ensure clean architectural boundaries between packages

## Critical Package Interactions

Based on the completed migration work, we've identified the following critical package interactions that require integration testing:

### 1. Core Event System Interactions
- **Packages**: `pkg/events`, `pkg/core`, `pkg/adapters`
- **Critical Paths**: 
  - Event emission from adapters through the event bus
  - Event subscription and handling across different components
  - Event type conversion and processing
- **Test Approach**: Create integration tests that verify events flow correctly from producers to consumers

### 2. Observability Stack
- **Packages**: `pkg/observability`, `pkg/metrics`, various consumers
- **Critical Paths**:
  - Logger implementation usage across components
  - Metrics collection and reporting
  - Span creation and propagation
- **Test Approach**: Verify logs, metrics, and spans are properly created and collected

### 3. Database and Repository Layer
- **Packages**: `pkg/database`, `pkg/repository`, `pkg/models`
- **Critical Paths**:
  - Data persistence through repository interfaces
  - Model conversion between layers
  - Transaction handling
- **Test Approach**: End-to-end tests that verify data travels correctly from API to storage and back

### 4. AWS Integration
- **Packages**: `pkg/aws`, `pkg/common/aws`, dependent services
- **Critical Paths**:
  - Authentication and configuration
  - Service client usage (S3, RDS, etc.)
- **Test Approach**: Verify AWS service interactions using mock implementations

### 5. Embedding and Chunking Pipeline
- **Packages**: `pkg/embedding`, `pkg/chunking`, `pkg/repository`
- **Critical Paths**:
  - Content processing pipeline
  - Vector storage and retrieval
  - Search functionality
- **Test Approach**: End-to-end tests of the embedding and search functionality

## Architectural Cleanup

In addition to integration testing, we need to perform targeted cleanup of architectural issues discovered during verification:

### 1. Misplaced Application Logic

We've identified application-specific code that should be moved from shared libraries to application directories:

- **pkg/mcp/server/** - Contains Gin web server implementation and HTTP handlers that should be in apps/mcp-server
- **Other potential issues** - Need to systematically review all pkg/ directories for similar issues

### 2. Unnecessary Files

Identify and remove files that are no longer needed due to the migration:

- Compatibility layers that are no longer needed (as confirmed by previous analysis)
- Duplicate implementations that have been consolidated
- Legacy files that have been replaced by new implementations

### 3. Cleanup Approach

- Create inventory of files to relocate or remove
- For each file to be relocated:
  - Create proper target location in apps/ directory
  - Update imports in all dependents
  - Move file with appropriate updates
  - Verify compilation after move
- For each file to be removed:
  - Verify no remaining references
  - Document reason for removal
  - Remove file
  - Verify compilation after removal

## Implementation Strategy

### Test Creation Approach

1. **Test Location**: 
   - Create a new `integration` directory under `pkg/tests` to house integration tests
   - Organize tests by critical path rather than by package

2. **Test Implementation**:
   - Use table-driven tests to cover multiple scenarios
   - Implement mock services for external dependencies
   - Focus on verifying the complete flow rather than individual components

3. **Continuous Integration**:
   - Ensure tests can run in CI environment
   - Add integration test stage to existing CI pipeline

### Test Prioritization

Tests will be prioritized based on:
1. Critical path for business functionality
2. Complexity of package interactions
3. History of issues in related areas

## Success Criteria

Phase 4.1 will be considered successful when:

1. All critical package interactions have passing tests
2. Tests verify both happy paths and error handling
3. Test coverage includes all major cross-package boundaries
4. Tests are integrated into the CI/CD pipeline
5. Misplaced application logic has been relocated to appropriate directories
6. Unnecessary files have been removed without breaking functionality
7. Clean architectural boundaries are established between pkg/ and apps/

## Schedule

| Category | Task | Estimated Completion | Priority |
|----------|------|----------------------|----------|
| Testing | Core Event System Integration | 2025-05-23 | High |
| Testing | Observability Stack Integration | 2025-05-24 | High |
| Testing | Database & Repository Integration | 2025-05-25 | High |
| Testing | AWS Integration | 2025-05-26 | Medium |
| Testing | Embedding & Chunking Integration | 2025-05-27 | High |
| Cleanup | Move pkg/mcp/server to apps/mcp-server | 2025-05-23 | High |
| Cleanup | Identify and inventory misplaced code | 2025-05-24 | High |
| Cleanup | Remove unnecessary compatibility layers | 2025-05-25 | Medium |
| Cleanup | Remove duplicate implementations | 2025-05-26 | Medium |

## Next Steps

1. Create the base test infrastructure in `pkg/tests/integration`
2. Implement the Core Event System integration tests
3. Set up test helpers and mock implementations for remaining tests
4. Create an inventory of misplaced application logic and unnecessary files
5. Implement relocation plan for pkg/mcp/server
6. Execute tests and document results
7. Update the Migration Status Tracker with progress
