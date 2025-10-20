#!/bin/bash

# Test script for project switching functionality in laserve

set -e

echo "Testing project switching functionality..."

# Start the server in background
JWT_SECRET="test-secret-$(date +%s)"
./laserve -jwt-secret "$JWT_SECRET" -port 8082 &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Function to make authenticated requests
make_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    
    # Get JWT token first
    TOKEN=$(curl -s -X POST http://localhost:8082/api/v1/public/login | jq -r '.data.token')
    
    if [ -n "$data" ]; then
        curl -s -X "$method" \
             -H "Authorization: Bearer $TOKEN" \
             -H "Content-Type: application/json" \
             -d "$data" \
             "http://localhost:8082/api/v1$endpoint"
    else
        curl -s -X "$method" \
             -H "Authorization: Bearer $TOKEN" \
             "http://localhost:8082/api/v1$endpoint"
    fi
}

echo "1. Testing project listing endpoint..."
PROJECTS_RESPONSE=$(make_request GET "/projects")
echo "Projects response: $PROJECTS_RESPONSE"

echo "2. Testing health endpoint..."
HEALTH_RESPONSE=$(curl -s http://localhost:8082/api/v1/public/health)
echo "Health response: $HEALTH_RESPONSE"

echo "3. Testing task endpoint with project parameter..."
# This should work even if no projects exist yet
TASKS_RESPONSE=$(make_request GET "/projects/test-project/tasks")
echo "Tasks response: $TASKS_RESPONSE"

echo "4. Testing steps endpoint with project parameter..."
STEPS_RESPONSE=$(make_request GET "/projects/test-project/steps")
echo "Steps response: $STEPS_RESPONSE"

# Clean up
echo "Stopping server..."
kill $SERVER_PID || true
wait $SERVER_PID 2>/dev/null || true

echo "Test completed successfully!"