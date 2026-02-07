#!/bin/bash
#
# Bash proxy script for Claude Code agent
# This script intercepts bash commands and proxies them to the orchestrator
# which executes them in a devcontainer environment.

set -euo pipefail

# Check if required environment variables are set
if [ -z "${LAFORGE_BASH_PROXY_URL:-}" ]; then
    echo "Error: LAFORGE_BASH_PROXY_URL not set, falling back to direct execution" >&2
    exec bash -c "$@"
fi

if [ -z "${LAFORGE_BASH_PROXY_TOKEN:-}" ]; then
    echo "Error: LAFORGE_BASH_PROXY_TOKEN not set, falling back to direct execution" >&2
    exec bash -c "$@"
fi

# Get the command to execute
COMMAND="$@"

# Create JSON payload
PAYLOAD=$(jq -n --arg cmd "$COMMAND" '{command: $cmd}')

# Call the orchestrator API
RESPONSE=$(curl -s -w "\n%{http_code}" \
    -X POST \
    -H "Authorization: Bearer ${LAFORGE_BASH_PROXY_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "${LAFORGE_BASH_PROXY_URL}/api/v1/bash")

# Split response and HTTP code
HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')

# Check HTTP status
if [ "$HTTP_CODE" -ne 200 ]; then
    echo "Error: Bash proxy request failed with HTTP $HTTP_CODE" >&2
    echo "$BODY" >&2
    exit 1
fi

# Parse the JSON response
STDOUT=$(echo "$BODY" | jq -r '.stdout // ""')
STDERR=$(echo "$BODY" | jq -r '.stderr // ""')
EXIT_CODE=$(echo "$BODY" | jq -r '.exit_code // 1')

# Output stdout and stderr
if [ -n "$STDOUT" ]; then
    echo "$STDOUT"
fi

if [ -n "$STDERR" ]; then
    echo "$STDERR" >&2
fi

# Exit with the command's exit code
exit "$EXIT_CODE"
