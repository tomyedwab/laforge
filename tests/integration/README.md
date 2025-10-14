# LaForge Integration Tests

This directory contains comprehensive integration tests for the LaForge project management system.

## Test Structure

### Test Files

- **`laforge_init_test.go`** - Tests for the `laforge init` command
  - Basic project initialization
  - Flag validation and handling
  - Duplicate project prevention
  - Project metadata validation

- **`laforge_step_test.go`** - Tests for the `laforge step` command
  - Command validation and error handling
  - Flag parsing and validation
  - Project existence verification

- **`project_validation_test.go`** - Tests for project structure validation
  - Project configuration file validation
  - Task database integrity checks
  - Git repository initialization verification
  - Repository state validation

### Supporting Files

- **`mock-agent/`** - Mock Docker agent for testing step execution
  - `mock_agent.go` - Simple Go program that simulates agent behavior
  - `Dockerfile` - Docker image definition for the mock agent

- **`run_integration_tests.sh`** - Test runner script that:
  - Builds test binaries
  - Creates mock Docker images (if Docker is available)
  - Runs all integration tests
  - Cleans up test artifacts

## Running Tests

### Quick Test Run
```bash
cd tests/integration
go test -v ./...
```

### Full Integration Test Suite
```bash
cd tests/integration
./run_integration_tests.sh
```

### Individual Test Files
```bash
go test -v laforge_init_test.go
go test -v laforge_step_test.go
go test -v project_validation_test.go
```

## Test Coverage

### Init Command Tests
- ✅ Basic project creation with minimal arguments
- ✅ Project creation with name and description flags
- ✅ Error handling for empty project IDs
- ✅ Duplicate project prevention
- ✅ Flag validation and processing

### Step Command Tests
- ✅ Validation of non-existent projects
- ✅ Empty project ID handling
- ✅ Flag parsing (timeout, agent-image)
- ✅ Invalid input handling

### Project Validation Tests
- ✅ Project configuration file structure
- ✅ Task database creation and integrity
- ✅ Database table existence verification
- ✅ Git repository initialization (when available)
- ✅ Repository state verification

## Environment Requirements

### Minimum Requirements
- Go 1.21 or later
- SQLite3 support

### Full Feature Testing
- Docker (for step command testing with mock agent)
- Git (for git repository validation tests)

## Test Behavior

### Git Availability
The tests gracefully handle environments without Git:
- Git repository tests are skipped when Git is not available
- Project creation succeeds without Git (as designed)
- Tests continue to validate other project components

### Docker Availability
Step command tests validate flag parsing and error handling without requiring Docker:
- Command-line validation works without Docker
- Mock agent image is built when Docker is available
- Tests can be extended for full step execution when Docker is present

## Mock Agent

The mock agent simulates a LaForge agent for testing step execution:
- Creates a test file to simulate agent work
- Can be extended to simulate various agent behaviors
- Provides a controlled environment for step testing

To build and use the mock agent:
```bash
cd tests/integration/mock-agent
docker build -t laforge-mock-agent:latest .
```

## Adding New Tests

When adding new integration tests:

1. **Follow existing patterns** - Use the same test structure and naming conventions
2. **Handle missing dependencies** - Check for Git, Docker availability and skip tests appropriately
3. **Clean up resources** - Always clean up test projects and temporary files
4. **Use helper functions** - Leverage existing validation and cleanup functions
5. **Test error cases** - Include both success and failure scenarios

## CI/CD Integration

The integration tests are designed for CI/CD environments:
- No external dependencies required for basic tests
- Graceful handling of missing optional tools
- Comprehensive error reporting and logging
- Fast execution for quick feedback loops

## Troubleshooting

### Common Issues

1. **"Cannot build laforge binary"**
   - Ensure you're in the correct directory (`tests/integration`)
   - Check that Go is properly installed

2. **Git repository tests failing**
   - Git is optional - tests will skip if not available
   - Check that Git is installed if you need repository tests

3. **Docker-related test failures**
   - Docker is optional for basic tests
   - Install Docker for full step command testing

### Debug Mode
Run tests with verbose output to see detailed execution:
```bash
go test -v -run TestName
```