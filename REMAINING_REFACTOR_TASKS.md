# Remaining Refactor Tasks

## Current Status: 85% Complete

### ✅ Completed
1. **Import Cycle Resolution** - Database package is now self-contained
2. **Context Handler Links** - Fixed using response DTOs
3. **Cache Package** - All components added
4. **pkg/mcp References** - All cleaned up
5. **Adapters Package** - Refactored to follow Go best practices

### ❌ Remaining Issues

## 1. Module Import Structure Issues (Critical - Blocking Builds)

### Problem
Many packages have replace directives pointing to non-existent modules:
- `pkg/adapters/events` (referenced as separate module but it's part of pkg/adapters)
- Various go.mod files have incorrect replace directives

### Solution
```bash
# Find all go.mod files with adapter/events references
find . -name "go.mod" -exec grep -l "pkg/adapters/events" {} \;

# Update each to remove the separate module reference
# Change from:
#   github.com/S-Corkum/devops-mcp/pkg/adapters/events => ../adapters/events
# To:
#   (remove this line - events is part of adapters package)
```

## 2. GitHub Integration Test Issues

### Location
`pkg/tests/integration/github_integration_test.go`

### Problems
1. Imports from `apps/mcp-server/internal/adapters/github` (violates Go's internal package rules)
2. Test expects different API than what the refactored adapters provide

### Solution
Update the test to use the new clean adapter interface from `pkg/adapters`

## 3. Clean Up go.work File

Remove references to non-existent modules:
- `./pkg/interfaces` (removed)
- `./pkg/relationship` (removed)
- `./pkg/config` (removed)

## 4. Documentation Updates (Phase 6.3 - 40% Complete)

### Required Updates
1. **Architecture Documentation**
   - Update system-overview.md with new adapter structure
   - Update adapter-pattern.md with new implementation
   
2. **API Documentation**
   - Document the new adapter interfaces
   - Update any API endpoints that use adapters

3. **Migration Guide**
   - Create guide for migrating from old adapter structure to new
   - Document breaking changes

4. **Package READMEs**
   - Add/update README files in each package
   - Ensure godoc comments are complete

## 5. Final Build and Test Verification

### Steps
1. Fix all module import issues
2. Run `go mod tidy` in each module
3. Execute full build: `make build`
4. Run all tests: `make test`
5. Run integration tests
6. Performance benchmarks

## Quick Fix Script

```bash
#!/bin/bash
# fix-remaining-imports.sh

echo "Fixing remaining import issues..."

# Remove adapter/events module references
for gomod in $(find . -name "go.mod" -type f); do
    if grep -q "pkg/adapters/events =>" "$gomod"; then
        echo "Fixing $gomod"
        sed -i '' '/pkg\/adapters\/events =>/d' "$gomod"
    fi
done

# Clean up go.work
sed -i '' '/\.\/pkg\/interfaces/d' go.work
sed -i '' '/\.\/pkg\/relationship/d' go.work
sed -i '' '/\.\/pkg\/config/d' go.work

# Remove empty lines in go.work
sed -i '' '/^$/N;/^\n$/d' go.work

# Tidy all modules
for dir in apps/mcp-server apps/rest-api apps/worker pkg/*; do
    if [ -f "$dir/go.mod" ]; then
        echo "Tidying $dir"
        (cd "$dir" && go mod tidy)
    fi
done

echo "Done!"
```

## Priority Order

1. **Fix Module Imports** (Blocking - Do First)
   - Remove incorrect replace directives
   - Clean up go.work
   
2. **Fix GitHub Integration Test** (High Priority)
   - Update to use new adapter interfaces
   
3. **Documentation** (Medium Priority)
   - Can be done incrementally
   
4. **Performance/Security Review** (Low Priority)
   - After everything builds and tests pass

## Estimated Time to Complete

- Module Import Fixes: 1-2 hours
- GitHub Integration Test: 1 hour
- Documentation Updates: 2-3 hours
- Final Verification: 1 hour

**Total: 5-7 hours**

## Success Criteria

```bash
# All of these should pass:
./validate-refactor.sh  # All checks green
make build             # Builds successfully
make test              # All tests pass
make test-integration  # Integration tests pass
```

The refactor has achieved its architectural goals. The remaining work is primarily fixing module dependencies and updating documentation.