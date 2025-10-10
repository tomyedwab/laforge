PHONY: build test test-all test-tasks test-cli test-integration test-coverage test-coverage-html test-verbose clean test-pattern test-bench test-race test-quick help ci-test

build:
	docker build -t opencode-agent -f agents/Dockerfile.opencode .

run:
	mkdir -p /tmp/opencode-agent/log /tmp/opencode-agent/state
	docker run -it --rm \
		-v $(HOME)/.config/opencode:/home/opencode/.config/opencode \
		-v $(HOME)/.local/share/opencode/auth.json:/home/opencode/.local/share/opencode/auth.json \
		-v /tmp/opencode-agent/log:/home/opencode/.local/share/opencode/log \
		-v /tmp/opencode-agent/state:/state \
		-v $(PWD):/src \
		opencode-agent

# Test targets
test: test-all

# Run all tests
test-all:
	@echo "Running all tests..."
	go test ./... -v

# Run only tasks package tests
test-tasks:
	@echo "Running tasks package tests..."
	go test ./tasks -v

# Run only CLI command tests
test-cli:
	@echo "Running CLI command tests..."
	go test ./cmd/latasks -v

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -coverprofile=coverage.out
	@echo "\nCoverage report:"
	go tool cover -func=coverage.out

# Generate HTML coverage report
test-coverage-html: test-coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report generated: coverage.html"

# Run tests in verbose mode
test-verbose:
	go test ./... -v -count=1

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	chmod +x test_integration.sh
	./test_integration.sh

# Run specific test patterns
test-pattern:
	@echo "Usage: make test-pattern PATTERN=TestAddTask"
	go test ./... -v -run $(PATTERN)

# Benchmark tests
test-bench:
	go test ./... -bench=. -benchmem

# Race condition tests
test-race:
	go test ./... -race

# Clean test artifacts
clean:
	rm -f coverage.out coverage.html
	rm -f latasks-test
	rm -rf test_state
	@echo "Test artifacts cleaned up"

# Quick test (no verbose output)
test-quick:
	go test ./...

# CI-friendly test target (no verbose, with coverage)
ci-test:
	@echo "Running CI tests..."
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

# Help target
help:
	@echo "Available targets:"
	@echo "  build              - Build the Docker image"
	@echo "  run                - Run the Docker container"
	@echo "  test               - Run all tests (alias for test-all)"
	@echo "  test-all           - Run all tests with verbose output"
	@echo "  test-tasks         - Run only tasks package tests"
	@echo "  test-cli           - Run only CLI command tests"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  test-coverage-html - Generate HTML coverage report"
	@echo "  test-verbose       - Run tests in verbose mode"
	@echo "  test-integration   - Run integration tests"
	@echo "  test-pattern       - Run tests matching a pattern (use PATTERN=TestName)"
	@echo "  test-bench         - Run benchmark tests"
	@echo "  test-race          - Run race condition tests"
	@echo "  test-quick         - Run tests without verbose output"
	@echo "  ci-test            - Run CI-friendly tests with coverage"
	@echo "  clean              - Clean test artifacts"
	@echo "  help               - Show this help message"

# -m moonshot/kimi-k2-0905-preview run "What can you do?"
