# Go Workspace Migration Verification Plan

## Executive Summary

This document outlines a comprehensive, step-by-step approach to verify the successful migration from internal packages to the new pkg directory structure. Designed specifically for execution with AI assistance (Anthropic's Claude 3.7 Sonnet), this plan leverages extended thinking capabilities to systematically analyze, reason through, and resolve complex migration issues across multiple chat sessions.

The verification strategy follows Go ecosystem best practices while incorporating deliberate analysis points for AI reasoning. Each phase is structured to maintain continuity across sessions using Windsurf IDE's memory capabilities, with systematic error handling approaches for the expected compilation and linting errors.

## Windsurf IDE Integration Strategy

This plan is optimized to leverage the full capabilities of Windsurf IDE alongside Claude 3.7 Sonnet's extended thinking. Each phase utilizes specific Windsurf tools for maximum efficiency.

### Windsurf IDE Tool Utilization

| Tool Category | Specific Tools | Use Cases in Migration |
|--------------|----------------|------------------------|
| **Code Search** | `grep_search`, `find_by_name`, `codebase_search` | - Find interface declarations<br>- Locate implementations<br>- Identify import patterns |
| **Code Analysis** | `view_file_outline`, `view_code_item`, `search_in_file` | - Analyze interface hierarchies<br>- Examine method implementations<br>- Understand dependency chains |
| **Code Editing** | `replace_file_content`, `write_to_file` | - Fix interface mismatches<br>- Standardize method signatures<br>- Create adapter implementations |
| **Execution** | `run_command`, `command_status` | - Run compilation tests<br>- Execute targeted test suites<br>- Check implementation status |
| **Integration** | `browser_preview` | - Test web service functionality<br>- Verify API endpoints<br>- Validate UI components |

### Memory Management Strategy

#### Package State Memory Template
```markdown
Package: [package name]
Status: [In Progress/Complete/Blocked]
Key Interfaces: [List of critical interfaces]
Pending Issues: [List of unresolved errors]
Next Steps: [Specific actions for next session]
Implementation Decisions: [Record of key architectural decisions]
Windsurf Tool History: [Record of effective tool sequences for this package]
```

### Error Tracking Memory Template
```markdown
Error Category: [Interface Mismatch/Redeclaration/Import Error/Other]
Affected Files: [List of files with this error pattern]
Root Cause: [Analysis of the underlying issue]
Resolution Strategy: [Approach to fix systematically]
Status: [Fixed/In Progress/Blocked]
```

### Test Failure Memory Template
```markdown
Test Category: [Unit/Integration/Functional/Performance]
Test Pattern: [Common pattern across multiple failing tests]
Affected Components: [List of components with failing tests]
Failure Analysis: [Reasoning about why tests are failing]
Core Issue: [Fundamental cause requiring resolution]
Resolution Approach: [Systematic strategy to resolve these tests]
Priority: [High/Medium/Low]
```

### Implementation Decision Memory Template
```markdown
Decision Point: [What needed to be decided]
Alternatives Considered: [List of alternative approaches]
Selected Approach: [The chosen implementation strategy]
Rationale: [Extended reasoning about why this approach was chosen]
Trade-offs: [What was gained and what was sacrificed]
Implementation Notes: [Details for implementing this decision]
```

These structured memories will be created and updated throughout the migration process to maintain context across sessions and track progress systematically.

## Phase 1: Initial Compilation and Dependency Check

### Step 1.1: Linting and Compilation Error Catalog with Windsurf Tools

**Windsurf Tool Sequence:**
```markdown
1. Use run_command to execute compilation and linting checks
2. Use grep_search to identify error patterns across the codebase
3. Use view_code_item to examine specific errors in context
4. Use codebase_search for semantic understanding of affected code
```

**Example Tool Execution:**
```markdown
# Run compilation to identify build errors
run_command("go build -v ./pkg/...", "/Users/seancorkum/projects/devops-mcp")

# Find all redeclaration errors with grep_search
grep_search("redeclared", "/Users/seancorkum/projects/devops-mcp", includes=["*.go"])

# View specific interface declarations with view_code_item
view_code_item("/path/to/file.go", "Package.InterfaceName")
```

- **Objective**: Create a comprehensive error catalog organized by pattern and root cause
- **Expected Outcome**: Structured error database with prioritization
- **Success Criteria**: Every error categorized and assigned a resolution strategy

#### AI Analysis Tasks:

1. **Structured Error Classification**:
   - Group errors by type: Interface mismatch, redeclaration, missing import, type mismatch, etc.
   - For each error type, identify common patterns and create resolution templates
   - Create memory entries for major error categories using the Error Tracking Memory Template

2. **Multi-dimensional Error Analysis**:
   - **Severity Analysis**: Categorize errors by severity (blocking, non-blocking)
   - **Dependency Analysis**: Map errors to dependency chains to identify upstream causes
   - **Pattern Analysis**: Identify recurring patterns that can be fixed with similar approaches
   - **Root Cause Analysis**: Distinguish between migration-induced vs. pre-existing issues

3. **Error Resolution Planning**:
   - **Fix Templates**: Create reusable fix patterns for common error types
   - **Fix Sequencing**: Determine optimal order to minimize cascading fixes
   - **Batch Processing Strategy**: Group similar errors for batch resolution
   - **Test-Driven Resolution**: Define test cases to validate each fix

### Step 1.2: Dependency Graph Analysis and Import Path Resolution

```bash
# Find all files still importing from internal packages
find ./pkg -name "*.go" -type f -exec grep -l "github.com/S-Corkum/devops-mcp/internal" {} \;

# Find all internal packages still being imported
grep -r "github.com/S-Corkum/devops-mcp/internal" --include="*.go" ./pkg | awk -F'internal/' '{print $2}' | awk -F'"' '{print $1}' | sort | uniq

# Validate module dependencies
go mod graph | grep github.com/S-Corkum/devops-mcp/internal
```

- **Objective**: Create comprehensive mapping of import dependencies to systematically resolve them
- **Expected Outcome**: Actionable plan for updating import paths with minimal cascading effects
- **Success Criteria**: All import paths properly mapped and categorized by fix complexity

#### AI Reasoning and Memory Construction:

1. **Import Dependency Graph Construction**:
   - Build a directed graph of package imports (who imports whom)
   - Identify strongly connected components (circular dependencies)
   - Create memory entries for critical dependency chains
   - Generate visualization of dependency relationships

2. **Import Resolution Strategy**:
   - **Direct Replacements**: Map straightforward internal → pkg path replacements
   - **Interface Adaptation**: Identify where interfaces don't match between packages
   - **Type Conversion**: Locate instances requiring type conversion between packages
   - **Breaking Changes**: Flag imports that require API changes

3. **Multi-Session Planning**:
   - Create package-specific memories for each major component
   - Define clear entry and exit criteria for each package resolution
   - Structure fix batches that can be completed within single sessions
   - Establish checkpoints for validating progress

## Phase 2: Critical Interface Resolution

### Step 2.1: Fix Observability Package
This package has significant structural issues with interface redeclarations across multiple files, mismatched method signatures, and circular dependencies that require systematic resolution.

#### Pre-Session Preparation:
```bash
# Catalog all interface declarations in observability package
grep -r "type.*interface" --include="*.go" ./pkg/observability

# Find all implementations of these interfaces
grep -r "func (.*) .*Logger" --include="*.go" ./pkg
grep -r "func (.*) .*MetricsClient" --include="*.go" ./pkg
grep -r "func (.*) .*Span" --include="*.go" ./pkg
```

#### Memory Creation for Observability:
```markdown
Package: pkg/observability
Status: In Progress
Key Interfaces: Logger, MetricsClient, Span
Pending Issues: 
- Multiple interface redeclarations
- Inconsistent method signatures
- Circular dependencies
Next Steps: 
- Create canonical interfaces.go file
- Standardize method signatures
Implementation Decisions: 
- [To be populated during analysis]
```

#### AI Deep Analysis with Windsurf-Assisted Extended Thinking:

1. **Interface Variance Analysis**:
   - **Tool Application**: Use `grep_search` to find all interface declarations, then `view_code_item` to extract detailed signatures
   ```markdown
   grep_search("type.*interface", "/Users/seancorkum/projects/devops-mcp/pkg/observability", includes=["*.go"])
   view_code_item("/Users/seancorkum/projects/devops-mcp/pkg/observability/logger.go", "observability.Logger")
   ```
   - Create comparison matrix of interface methods across files
   - Identify semantic equivalences despite syntactic differences
   - Map the evolution pattern of interfaces to understand why variations exist

2. **Usage Pattern Analysis**:
   - **Tool Application**: Use `grep_search` to find implementations and consumers
   ```markdown
   grep_search("func \\(.*\\) (Info|Error|Debug)", "/Users/seancorkum/projects/devops-mcp", includes=["*.go"])
   codebase_search("Logger interface usage patterns", ["/Users/seancorkum/projects/devops-mcp/pkg"])
   ```
   - Analyze how each method is used in practice (parameter usage, error handling)
   - Create frequency analysis of method invocation patterns
   - Determine which signature variants are most widely used

3. **Dependency Cycle Breaking Strategy**:
   - **Tool Application**: Use file outlines to understand dependencies
   ```markdown
   view_file_outline("/Users/seancorkum/projects/devops-mcp/pkg/observability/logger.go")
   view_file_outline("/Users/seancorkum/projects/devops-mcp/pkg/observability/tracing.go")
   ```
   - Create directed graph of file-level dependencies
   - Identify minimal set of changes to break cycles
   - Evaluate interface extraction vs. package reorganization approaches
   - Design clean layering structure that prevents future cycles

#### Implementation Steps with Windsurf Tools:

1. **Create/validate interfaces.go with all interface definitions**
   - **Tool Application**: Use `write_to_file` or `replace_file_content` to create/update interfaces
   ```markdown
   # Check if file exists first
   find_by_name("/Users/seancorkum/projects/devops-mcp/pkg/observability", "interfaces.go")
   
   # Create or update the file with standardized interfaces
   write_to_file("/Users/seancorkum/projects/devops-mcp/pkg/observability/interfaces.go",
     "// pkg/observability/interfaces.go\npackage observability\n\n// Standard interface definitions\ntype Logger interface {\n  // Methods with standardized signatures\n}\n...")
   ```

2. **Update implementations for consistent interface usage**
   - **Tool Application**: Use `replace_file_content` to update implementations
   ```markdown
   # View current implementation
   view_code_item("/Users/seancorkum/projects/devops-mcp/pkg/observability/logger.go", "DefaultLogger")
   
   # Update implementation to match interface
   replace_file_content("/Users/seancorkum/projects/devops-mcp/pkg/observability/logger.go",
     "Update Logger implementation to match standard interface",
     [{ /*ReplacementChunks*/ }])
   ```

3. **Remove redundant declarations from individual files**
   - **Tool Application**: Use `grep_search` to find redundancies, then `replace_file_content` to remove
   ```markdown
   # Find redundant declarations
   grep_search("type (Logger|MetricsClient|Span) interface", "/Users/seancorkum/projects/devops-mcp/pkg/observability", includes=["*.go"])
   
   # Remove each redundant declaration
   replace_file_content("/path/to/file.go", "Remove redundant interface declaration", [{ /*ReplacementChunks*/ }])
   ```

4. **Apply adapter pattern for implementation misalignments**
   - **Tool Application**: Combine `view_code_item` to understand implementations with `write_to_file` for adapters
   ```markdown
   # Create adapter implementations
   write_to_file("/Users/seancorkum/projects/devops-mcp/pkg/observability/adapters.go",
     "// Adapter implementations to ensure interface compatibility\npackage observability\n...")
   
   # Verify implementation with compilation check
   run_command("go build ./pkg/observability/...", "/Users/seancorkum/projects/devops-mcp")
   ```

### Step 2.2: Fix Database Package
Resolve interface mismatches between internal/database.VectorDatabase and pkg/database.VectorDatabase implementations while addressing AWS authentication and connection pooling issues.

#### Memory Creation for Database:
```markdown
Package: pkg/database
Status: In Progress
Key Interfaces: VectorDatabase, GeneralDatabase
Pending Issues: 
- Interface mismatch with internal implementation
- Type conversion between models.Vector and repository.Embedding
- AWS IAM authentication handling
Next Steps: 
- Compare interface definitions
- Create standardized interfaces
Implementation Decisions: 
- [To be populated during analysis]
```

#### IDE-Assisted Interface Comparison:

1. **Structured Interface Comparison**:
   - Create side-by-side comparison of internal and pkg interfaces
   - For each method, document:
     - Parameter differences (name, type, order)
     - Return value differences
     - Semantic meaning differences
     - Error handling variations
   - Mark compatibility-breaking vs. fixable differences

2. **Type Conversion Mapping**:
   - Create exhaustive field mapping between equivalent types:
     ```go
     // Example mapping to be analyzed
     // models.Vector ↔ repository.Embedding
     // - ID: direct match
     // - TenantID ↔ ContextID: name difference
     // - Content ↔ Text: name difference
     // - Metadata: needs field extraction/merging
     ```
   - Document serialization/deserialization differences
   - Analyze performance impact of conversions

#### Multi-Session Error Resolution Strategy:

1. **Approach per Error Type**:
   - **Method Signature Mismatches**: Create adapter functions with precise type conversion
   - **Parameter Name Differences**: Update to standardized naming convention
   - **Missing Methods**: Implement missing functionality
   - **Authentication Differences**: Standardize on single auth pattern

#### Implementation Steps:
1. Create standardized interface definition
   ```go
   // Define canonical database interfaces in pkg/database/interfaces.go
   type VectorDatabase interface {
     // Standard methods with proper signatures
   }
   ```

2. Implement direct type conversions (no adapter pattern)
   - Convert between `models.Vector` and `repository.Embedding` types
   - Ensure field mappings are correct (`tenant_id` ↔ `context_id`, `content` ↔ `text`)
   - Handle metadata extraction and merging properly

3. Update database implementation to match interface
   - Fix AWS authentication handling
   - Ensure proper error wrapping
   - Verify connection management (refresh, pooling)

### Step 2.3: Fix Repository Package
Standardize repository interfaces to resolve the pattern mismatch between API-specific method names and generic repository interface methods.

#### AI Design Decision Process:
1. **Repository Pattern Analysis**:
   - Analyze current separation between generic (Create, List, Get, Update) and specific (CreateAgent, ListAgents) method names
   - Evaluate pros/cons of generic vs. specific naming conventions
   - Consider interface composition as an alternative to adapter pattern

2. **Implementation Strategy Reasoning**:
   - Since backward compatibility is not required (app not in production), design direct implementation
   - Consider impacts on testing strategy and mock generation
   - Analyze query method similarities for potential consolidation

#### Implementation Steps:
1. Standardize repository interface definitions
   ```go
   // Define base repository interface with generic methods
   type Repository[T any] interface {
     Create(ctx context.Context, entity *T) error
     Get(ctx context.Context, id string) (*T, error)
     List(ctx context.Context, filter Filter) ([]*T, error)
     Update(ctx context.Context, entity *T) error
     Delete(ctx context.Context, id string) error
   }
   ```

2. Implement entity-specific repositories without adapters
   - Convert direct entity implementations (AgentRepository, ModelRepository)
   - Remove adapter layers between API and repository
   - Ensure transaction handling is consistent

## Phase 3: Package-by-Package Verification

### Step 3.1: Verify Core Packages
These packages have minimal dependencies and should be verified first, with systematic error handling approaches for each.

#### Linting Error Resolution Framework:

1. **Error Categorization Matrix**:
   Create a standardized approach for each error type encountered:

   | Error Type | Detection Approach | Resolution Strategy | Verification Method |
   |------------|-------------------|---------------------|--------------------|
   | Multiple declarations | `grep -r "type X"` | Consolidate to interfaces.go | Go build without errors |
   | Import conflicts | `go list -f '{{ .Imports }}'` | Update import paths | Successful compilation |
   | Unused code | `golangci-lint run --disable-all --enable deadcode` | Safe removal or documentation | No new errors introduced |
   | Interface mismatch | Compare method signatures | Update implementation to match interface | Interface satisfaction test |
   | Circular dependency | Build dependency graph | Extract shared code to common package | `go mod graph` verification |

2. **Session Continuity Strategy**:
   - Create package-specific memory at the start of each package verification
   - Document all errors found during verification
   - Record resolution approaches applied
   - Track pending issues for continued work
   - Create checkpoint at the end of each session

3. **Advanced Error Analysis with Extended Thinking**:
   - Apply root cause analysis to each error pattern
   - Consider architectural implications beyond simple fixes
   - Evaluate consistency with Go best practices
   - Document reasoning about each fix approach
   - Consider alternative approaches with trade-offs

| Package | Verification Steps | AI Analysis Focus |
|---------|-------------------|-------------------|
| pkg/common/util | 1. Verify identical functionality<br>2. Run package tests<br>3. Check for redundant code | - Detect subtle behavioral differences<br>- Analyze error handling patterns<br>- Identify optimization opportunities |
| pkg/common/aws | 1. Verify AWS client configurations<br>2. Test connection methods<br>3. Validate error handling | - Analyze AWS auth pattern consistency<br>- Evaluate retry logic<br>- Reason about timeout configurations |
| pkg/config | 1. Verify config loading<br>2. Test environment overrides<br>3. Check defaults | - Analyze configuration precedence logic<br>- Evaluate validation patterns<br>- Consider cross-cutting configuration concerns |
| pkg/models | 1. Verify all model definitions<br>2. Check validation methods<br>3. Test serialization/deserialization | - Analyze model integrity constraints<br>- Evaluate JSON tag consistency<br>- Reason about model evolution patterns |

### Step 3.2: Verify Infrastructure Packages

| Package | Verification Steps |
|---------|-------------------|
| pkg/database | 1. Verify connection methods<br>2. Test CRUD operations<br>3. Validate transaction handling<br>4. Check IAM authentication |
| pkg/storage | 1. Verify file operations<br>2. Test S3 integrations<br>3. Validate content handling |
| pkg/embedding | 1. Test vector operations<br>2. Verify model loading<br>3. Check content handling |
| pkg/observability | 1. Test logging functionality<br>2. Verify metrics reporting<br>3. Check trace propagation |

### Step 3.3: Verify Domain Packages

| Package | Verification Steps |
|---------|-------------------|
| pkg/repository | 1. Verify entity CRUD operations<br>2. Test query methods<br>3. Validate transaction handling |
| pkg/api | 1. Verify API routes<br>2. Test handler methods<br>3. Check validation logic<br>4. Verify error responses |
| pkg/events | 1. Test event publishing<br>2. Verify event handling<br>3. Check subscription logic |
| pkg/adapters | 1. Verify correct delegation<br>2. Test interface compatibility<br>3. Validate type conversions |

## Phase 4: Full Integration Testing

### Step 4.1: Run Application-level Tests
```bash
cd /Users/seancorkum/projects/devops-mcp
go test ./apps/... -v
```
- **Objective**: Verify applications work with the migrated packages
- **Success Criteria**: All tests pass

### Step 4.2: Run Docker Services
```bash
cd /Users/seancorkum/projects/devops-mcp
docker-compose -f docker-compose.local.yml up -d
```
- **Objective**: Verify all services start correctly with migrated code
- **Success Criteria**: All containers healthy

### Step 4.3: Integration Test with API Endpoints
Test critical API endpoints to verify end-to-end functionality:
1. MCP Server endpoints
2. REST API service endpoints
3. Worker service endpoints

## Phase 5: Comprehensive Test Suite Resolution

### Step 5.1: Unit Test Resolution Framework

#### Test Failure Cataloging and Analysis
```bash
# Run all unit tests in verbose mode to catalog failures
cd /Users/seancorkum/projects/devops-mcp
go test -v ./pkg/... 2>&1 | tee test-failures.log

# Group test failures by error pattern
grep "FAIL" test-failures.log | sort | uniq -c | sort -nr

# Extract specific error messages
grep -A 3 "--- FAIL" test-failures.log > test-error-messages.log
```

#### Memory Template for Test Failures
```markdown
Test Failure Category: [Interface Mismatch/Mock Expectation/Import Error/Behavior Change]
Affected Tests: [List of failing tests with this pattern]
Error Pattern: [Common error message or pattern]
Root Cause Analysis: [Reasoning about why tests are failing]
Resolution Strategy: [Systematic approach for fixing this class of test failures]
Example Fix: [Code snippet showing resolution pattern]
```

#### AI-Optimized Test Resolution Strategy

1. **Test Failure Pattern Recognition**:
   - Apply extended thinking to categorize test failures by common patterns
   - Create visual dependency graph of failing tests to identify upstream causes
   - Distinguish between interface changes, behavior changes, and mock failures

2. **Test Fix Templating**:
   - Create reusable fix templates for common test failure patterns:
     - Mock update templates for interface changes
     - Test input updates for type changes
     - Assertion updates for behavior changes
     - Test skip patterns for temporarily disabling problematic tests

3. **Progressive Test Resolution Approach**:
   - Prioritize fixing foundational tests first (core packages, utilities)
   - Implement fixes in dependency order to minimize rework
   - Create test helper functions to encapsulate common test setup changes
   - Document systematic changes in test utilities and mocks

4. **AI-Assisted Test Analysis**:
   - Apply code comprehension to understand test intent beyond implementation
   - Identify cases where tests validate incorrect behavior from old implementation
   - Reason about whether test expectations or implementations should change
   - Create clear documentation explaining test modifications

### Step 5.2: Integration Test Resolution Framework

#### Integration Test Failure Analysis
```bash
# Run integration tests specifically
cd /Users/seancorkum/projects/devops-mcp
go test -tags=integration ./pkg/... -v 2>&1 | tee integration-failures.log

# Identify slow or flaky tests
go test -tags=integration ./pkg/... -v -count=5 2>&1 | grep -A 1 "FAIL"
```

#### Memory Template for Integration Test Issues
```markdown
Integration Point: [Database/API/External Service]
Affected Components: [List of components involved]
Integration Failure: [Description of the failure]
Environment Factors: [Relevant environment configuration]
Resolution Strategy: [Approach to fix integration issues]
Verification Method: [How to confirm the integration is working]
```

#### AI-Optimized Integration Resolution

1. **Integration Boundary Analysis**:
   - Map all integration points between components
   - Identify contract changes across integration boundaries
   - Create interaction diagrams showing data flow and transformation
   - Analyze configuration dependencies across components

2. **Environmental Factor Analysis**:
   - Create matrix of environment configuration requirements
   - Identify discrepancies between test and production configurations
   - Document connection string and credential requirements
   - Analyze timing and retry patterns at integration points

3. **Integration Stub and Mock Strategy**:
   - Design selective stubbing approach for external dependencies
   - Create consistent patterns for integration mocks
   - Document integration contract expectations
   - Develop verification tests for integration boundaries

### Step 5.3: Functional Test and End-to-End Resolution

#### Functional Test Analysis
```bash
# Run end-to-end tests
cd /Users/seancorkum/projects/devops-mcp
./scripts/run_e2e_tests.sh 2>&1 | tee e2e-failures.log

# Test specific critical user flows
./scripts/run_e2e_tests.sh --flow=auth 2>&1 | tee auth-flow-test.log
```

#### Memory Template for Functional Test Issues
```markdown
User Flow: [Authentication/Data Processing/API Interaction]
Failing Scenario: [Specific scenario that fails]
Expected Behavior: [What should happen]
Actual Behavior: [What actually happens]
Component Chain: [Sequence of components involved]
Root Cause Analysis: [Reasoning about the failure]
Resolution Strategy: [Approach to fix the user flow]
Verification Steps: [How to verify the fix works end-to-end]
```

#### AI-Optimized Functional Test Resolution

1. **User Flow Analysis with Extended Thinking**:
   - Create sequence diagrams of complete user flows
   - Map each step in the flow to affected components
   - Identify data transformation points between steps
   - Analyze authentication and authorization boundaries

2. **Systematic Debugging Approach**:
   - Implement progressive tracing through the flow
   - Create data snapshot comparisons at each stage
   - Design targeted logging for problematic flows
   - Implement step-by-step verification methods

3. **Cross-Component Analysis**:
   - Identify subtle interactions between components
   - Create end-to-end transaction tracking
   - Analyze timing and race conditions
   - Evaluate consistency guarantees across components

4. **End-to-End Resolution Strategy**:
   - Document complete fix strategy across multiple components
   - Create verification plan for end-to-end scenarios
   - Design regression test suite for critical flows
   - Develop monitoring approaches for key integration points

### Step 5.4: Performance Validation
Run benchmarks to ensure no performance regressions:
```bash
cd /Users/seancorkum/projects/devops-mcp
go test -bench=. ./pkg/database/...
go test -bench=. ./pkg/repository/...
```

### Step 5.5: Load Test APIs
Use a load testing tool to verify performance under load:
```bash
# Example with hey tool
hey -n 1000 -c 50 http://localhost:8080/api/v1/health
```

## Phase 6: Cleanup and Documentation

### Step 6.1: Remove Internal Package
Once all tests pass and functionality is verified:
```bash
# Add deprecation notice to internal directory
echo "This directory is deprecated and will be removed. Use pkg/ directory instead." > /Users/seancorkum/projects/devops-mcp/internal/README.md

# Optionally, rename for safety before final removal
mv /Users/seancorkum/projects/devops-mcp/internal /Users/seancorkum/projects/devops-mcp/internal.deprecated
```

### Step 6.2: Update Migration Tracker
Mark all packages as completed in the migration tracker:
```markdown
| ✅ | `github.com/S-Corkum/devops-mcp/internal/adapters` | `github.com/S-Corkum/devops-mcp/pkg/adapters` | ✅ Migration Complete | - |
```

### Step 6.3: Documentation Update
Create or update documentation about the new package structure:
1. Package responsibilities
2. Interface definitions
3. Configuration requirements
4. Integration patterns

## AI-Assisted Execution Framework with Memory Management

This plan leverages Claude 3.7 Sonnet's extended thinking capabilities with Windsurf IDE's memory system to maintain context across multiple sessions while systematically addressing compilation and linting errors.

### Resolution Priority Order:

1. **Blocking Interface Issues**: Observability, Database, Repository interfaces
2. **Core Type Definitions**: Models, Config, Common utilities
3. **Implementation Mismatches**: Method signatures, parameter types
4. **Critical Unit Test Failures**: Tests for core functionality and utilities
5. **Import Path Resolutions**: Update remaining internal imports
6. **Integration Test Failures**: Component interaction tests
7. **Functional End-to-End Tests**: Complete user flow tests
8. **Linting and Style Issues**: After functionality is restored

### Windsurf-Optimized Multi-Session Workflow Pattern:

Each session will leverage Windsurf IDE's specialized tools alongside Claude 3.7's extended thinking:

#### 1. **Session Initialization with Memory Integration**:
   - **Tool Application**: Begin by reviewing memories and initializing workspace
   ```markdown
   # Pull up relevant package memory before starting work
   # Review previous session's tool commands and results
   # Set up working directory for current session
   list_dir("/Users/seancorkum/projects/devops-mcp/pkg/[target_package]")
   ```
   - Summarize current state and progress from memories
   - Establish specific goals for current session
   - Define exit criteria and checkpoints

#### 2. **Systematic Error and Test Analysis with Windsurf Tools**:
   - **Tool Application**: Use IDE's diagnostic capabilities
   ```markdown
   # Run compilation check to identify current errors
   run_command("go build ./pkg/[target_package]/...", "/Users/seancorkum/projects/devops-mcp")
   
   # Execute specific test suite to find test failures
   run_command("go test ./pkg/[target_package]/...", "/Users/seancorkum/projects/devops-mcp")
   
   # Use semantic search to understand code context
   codebase_search("[key concept]" ["/Users/seancorkum/projects/devops-mcp/pkg/[target_package]"])
   ```
   - Apply extended thinking to analyze error and failure patterns
   - Group related issues for batch processing
   - Create detailed reasoning about root causes

#### 3. **Reasoned Implementation with Windsurf Editing Tools**:
   - **Tool Application**: Use IDE's code modification capabilities
   ```markdown
   # View existing code before modifying
   view_file_outline("/Users/seancorkum/projects/devops-mcp/pkg/[target_package]/[file].go")
   view_code_item("/Users/seancorkum/projects/devops-mcp/pkg/[target_package]/[file].go", "[Symbol]")
   
   # Make systematic changes to implementation
   replace_file_content("/Users/seancorkum/projects/devops-mcp/pkg/[target_package]/[file].go", "[change description]", [{ /*ReplacementChunks*/ }])
   
   # Create new files if needed
   write_to_file("/Users/seancorkum/projects/devops-mcp/pkg/[target_package]/[new_file].go", "[file content]")
   ```
   - Document architectural decisions in memory and code comments
   - Create fix templates for recurring error patterns
   - Update tests to align with implementation changes

#### 4. **Progressive Verification with Windsurf Execution Tools**:
   - **Tool Application**: Use IDE's execution and testing capabilities
   ```markdown
   # Verify compilation after changes
   run_command("go build ./pkg/[target_package]/...", "/Users/seancorkum/projects/devops-mcp")
   
   # Run specific tests for the modified component
   run_command("go test ./pkg/[target_package] -run=[TestName]", "/Users/seancorkum/projects/devops-mcp")
   ```
   - Execute incremental compilation checks
   - Run unit tests for modified components
   - Validate that fixes don't introduce new errors
   - Document unexpected behaviors or edge cases

#### 5. **Cross-Component Testing with Windsurf Integration Tools**:
   - **Tool Application**: Use IDE's cross-package analysis capabilities
   ```markdown
   # Find all imports of the modified package
   grep_search("import.*[target_package]", "/Users/seancorkum/projects/devops-mcp", includes=["*.go"])
   
   # Test integration with dependent packages
   run_command("go test ./pkg/[dependent_package] -run=[TestName]", "/Users/seancorkum/projects/devops-mcp")
   
   # For web services, use browser preview to test API endpoints
   run_command("go run ./apps/[app_name]/main.go", "/Users/seancorkum/projects/devops-mcp")
   browser_preview("API Test", "http://localhost:8080")
   ```
   - Test integration points between modified components
   - Verify end-to-end functionality across component chains
   - Update integration test expectations

#### 6. **Memory Creation and Session Closure**:
   - **Tool Application**: Document session outcomes for future reference
   ```markdown
   # Create or update package memory with the current state
   create_memory("Action": "update", "Title": "[target_package] Migration Status", "Content": "[detailed status]", "Tags": ["migration", "[target_package]"])
   ```
   - Document error patterns and their resolutions
   - Create test failure pattern memories with examples
   - Record tool sequences that were most effective
   - Define clear starting point for next session

### Memory Utilization Strategy:

At key points in the migration process, Claude will:

1. **Create Package State Memories**:
   - One per major package being migrated
   - Updated at the end of each session working on that package
   - Includes current status, pending issues, and next steps

2. **Create Error Pattern Memories**:
   - For recurring error types that require consistent handling
   - Documents the pattern, root cause, and resolution approach
   - Serves as reference for handling similar errors

3. **Create Test Failure Pattern Memories**:
   - For common test failure patterns across multiple components
   - Documents test intent, failure mode, and resolution approach
   - Provides templates for fixing similar test failures
   - Tracks related tests requiring the same fix pattern

4. **Create Implementation Decision Memories**:
   - For significant architectural or design decisions
   - Documents alternatives considered, trade-offs, and rationale
   - Records implementation notes and verification approach
   - Ensures consistent application of design principles

5. **Create Integration Point Memories**:
   - For critical component boundaries and integration points
   - Documents contract expectations and interaction patterns
   - Records environment and configuration dependencies
   - Serves as reference for debugging cross-component issues

6. **Reference Memories Explicitly and Deliberately**:
   - Begin each session by loading relevant memories
   - Reference specific memories when making related decisions
   - Cross-reference memories to build comprehensive understanding
   - Update memories with new insights or changes
   - Use memories to validate implementation consistency

This structured approach ensures continuity across sessions while systematically resolving the complex web of compilation, linting, and testing issues that emerge during the migration process. The deliberate memory creation and referencing strategy allows Claude to build a comprehensive understanding of the system architecture and maintain consistency across multiple migration sessions.
