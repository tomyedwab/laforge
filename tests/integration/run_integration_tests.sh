#!/bin/bash

# Integration test runner for LaForge
set -e

echo "=== LaForge Integration Tests ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    if [ "$status" = "success" ]; then
        echo -e "${GREEN}✓${NC} $message"
    elif [ "$status" = "error" ]; then
        echo -e "${RED}✗${NC} $message"
    else
        echo -e "${YELLOW}→${NC} $message"
    fi
}

# Change to the integration test directory
cd "$(dirname "$0")"

# Clean up any previous test artifacts
print_status "info" "Cleaning up previous test artifacts..."
rm -f laforge-test
rm -f latasks-test

# Build the laforge binary
print_status "info" "Building laforge binary..."
if go build -o laforge-test ../../cmd/laforge; then
    print_status "success" "LaForge binary built successfully"
else
    print_status "error" "Failed to build LaForge binary"
    exit 1
fi

# Build the latasks binary (for testing task operations)
print_status "info" "Building latasks binary..."
if go build -o latasks-test ../../cmd/latasks; then
    print_status "success" "Latasks binary built successfully"
else
    print_status "error" "Failed to build latasks binary"
    exit 1
fi

# Build mock agent Docker image if Docker is available
if command -v docker &> /dev/null && docker --version &> /dev/null; then
    print_status "info" "Building mock agent Docker image..."
    if docker build -t laforge-mock-agent:latest mock-agent/. > /dev/null 2>&1; then
        print_status "success" "Mock agent Docker image built successfully"
    else
        print_status "error" "Failed to build mock agent Docker image"
        exit 1
    fi
else
    print_status "warning" "Docker not available, skipping Docker-dependent tests"
fi

# Run the integration tests
print_status "info" "Running integration tests..."
if go test -v .; then
    print_status "success" "All integration tests passed"
    TEST_RESULT="success"
else
    print_status "error" "Some integration tests failed"
    TEST_RESULT="failure"
fi

# Clean up test binaries
print_status "info" "Cleaning up test binaries..."
rm -f laforge-test
rm -f latasks-test

# Clean up Docker image if it was built
if command -v docker &> /dev/null; then
    docker rmi laforge-mock-agent:latest > /dev/null 2>&1 || true
fi

echo
echo "=== Integration Tests Complete ==="

if [ "$TEST_RESULT" = "success" ]; then
    exit 0
else
    exit 1
fi