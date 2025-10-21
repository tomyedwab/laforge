.PHONY:

build: .PHONY
	mkdir -p build
	go build -o build/laforge cmd/laforge/main.go
	go build -o build/latools cmd/latools/main.go
	docker build -t laforge-agent .
	docker build -t laforge-self-agent -f Dockerfile.laforge .

install: build
	cp build/laforge ~/.local/bin/
	cp build/latools ~/.local/bin/

run-opencode: .PHONY
	mkdir -p /tmp/laforge/log
	docker run -it --rm \
		-v $(HOME)/.config/opencode:/home/laforge/.config/opencode \
		-v $(HOME)/.local/share/opencode/auth.json:/home/laforge/.local/share/opencode/auth.json \
		-v /tmp/laforge/log:/home/laforge/.local/share/opencode/log \
		-v $(HOME)/.laforge/projects/laforge:/state \
		-v $(PWD):/src \
		laforge-self-agent

run-claudecode-login: .PHONY
	mkdir -p ~/.claude-laforge
	docker run -it --rm \
        -v $(HOME)/.claude-laforge:/home/laforge/.claude-laforge \
		-v $(HOME)/.laforge/projects/laforge:/state \
		-v $(PWD):/src \
		laforge-self-agent /bin/claude-login.sh

run-claudecode: .PHONY
	mkdir -p ~/.claude-laforge
	docker run -it --rm \
        -v $(HOME)/.claude-laforge:/home/laforge/.claude-laforge \
		-v $(HOME)/.laforge/projects/laforge:/state \
		-v $(PWD):/src \
		laforge-self-agent

# Run all tests
test: .PHONY
	@echo "Running all tests..."
	go test ./... -v

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
	@echo "  test               - Run all tests"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  test-coverage-html - Generate HTML coverage report"
	@echo "  test-verbose       - Run tests in verbose mode"
	@echo "  test-integration   - Run integration tests"
	@echo "  test-bench         - Run benchmark tests"
	@echo "  test-race          - Run race condition tests"
	@echo "  ci-test            - Run CI-friendly tests with coverage"
	@echo "  clean              - Clean test artifacts"
	@echo "  help               - Show this help message"
