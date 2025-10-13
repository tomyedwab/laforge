#!/bin/bash

# Integration test script for latasks
# This script demonstrates the basic functionality of the latasks CLI tool

echo "=== latasks Integration Test ==="
echo

# Set up test database
export TASKS_DB_PATH="./integration-test.db"

# Build the latasks binary
echo "Building latasks..."
go build -o latasks-test ./cmd/latasks
if [ $? -ne 0 ]; then
    echo "Failed to build latasks"
    exit 1
fi

echo "Build successful!"
echo

# Test 1: Add tasks
echo "Test 1: Adding tasks..."
./latasks-test add "Implement user authentication"
./latasks-test add "Create login page" T1
./latasks-test add "Add password validation" T2
./latasks-test add "Setup session management" T2
echo

# Test 2: List tasks
echo "Test 2: Listing all tasks..."
./latasks-test list
echo

# Test 3: View task details
echo "Test 3: Viewing task T1 details..."
./latasks-test view T1
echo

# Test 4: Get next task ready for work
echo "Test 4: Getting next task ready for work..."
./latasks-test next
echo

# Test 5: Get next task after updating some statuses
echo "Test 5: Getting next task after updating some statuses..."
./latasks-test update T3 "in-progress"
./latasks-test next
echo

# Test 6: Update more task statuses
echo "Test 6: Updating task status..."
./latasks-test update T4 "completed"
echo

# Test 7: Add task logs
echo "Test 7: Adding task logs..."
./latasks-test log T3 "Started working on password validation"
./latasks-test log T3 "Implemented basic validation rules"
echo

# Test 8: Create review request
echo "Test 8: Creating review request..."
./latasks-test review T4 "Ready for code review" "docs/review.md"
echo

# Test 9: Try to complete parent task (should fail)
echo "Test 9: Attempting to complete parent task with incomplete children..."
./latasks-test update T1 "completed"
echo

# Test 10: Complete remaining child tasks
echo "Test 10: Completing remaining child tasks..."
./latasks-test update T2 "completed"
./latasks-test update T3 "completed"
echo

# Test 11: Now complete parent task
echo "Test 11: Completing parent task after all children are done..."
./latasks-test update T1 "completed"
echo

# Test 12: Final task list
echo "Test 12: Final task list..."
./latasks-test list
echo

# Test 13: Delete a task
echo "Test 13: Deleting a task..."
./latasks-test delete T4
echo

# Test 14: Final task list after deletion
echo "Test 14: Final task list after deletion..."
./latasks-test list
echo

# Cleanup
rm -f latasks-test

echo "=== Integration test completed ==="