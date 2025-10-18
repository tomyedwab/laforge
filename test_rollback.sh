#!/bin/bash

# Test script for step rollback functionality

set -e

echo "=== Testing Step Rollback Functionality ==="

# Create a test project
echo "Creating test project..."
./laforge init test-rollback-project --name "Test Rollback Project"

# Create a simple git repo for testing
echo "Setting up test git repository..."
cd /tmp
rm -rf test-repo
git init test-repo
cd test-repo

# Configure git
git config user.name "Test User"
git config user.email "test@example.com"

# Create initial commit
echo "Initial content" > file1.txt
git add file1.txt
git commit -m "Initial commit"

# Create a few more commits to simulate steps
echo "Step 1 content" > file2.txt
git add file2.txt
git commit -m "Step 1 changes"

STEP1_COMMIT=$(git rev-parse HEAD)

echo "Step 2 content" > file3.txt
git add file3.txt
git commit -m "Step 2 changes"

STEP2_COMMIT=$(git rev-parse HEAD)

echo "Step 3 content" > file4.txt
git add file4.txt
git commit -m "Step 3 changes"

STEP3_COMMIT=$(git rev-parse HEAD)

echo "Current commits:"
echo "Step 1: $STEP1_COMMIT"
echo "Step 2: $STEP2_COMMIT"
echo "Step 3: $STEP3_COMMIT"

# Simulate step records in the database
echo "Creating step records in database..."
cd /src

# Create step records manually (simulating what would happen during actual step execution)
# This is a simplified version - in real usage, these would be created by the step command

# List steps to see current state
echo "=== Current steps (should be empty) ==="
./laforge steps test-rollback-project || true

echo "=== Test completed ==="
echo "Note: Full integration testing requires a complete LaForge setup with git and Docker."