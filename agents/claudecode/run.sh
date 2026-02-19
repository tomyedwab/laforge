#!/bin/bash

set -euo pipefail

echo "Invoking Claude Code: ${ANTHROPIC_BASE_URL}"

mkdir -p .claude
cp /config/settings.local.json .claude/

# Set up bash proxy if configured
if [ -n "${LAFORGE_BASH_PROXY_URL:-}" ] && [ -n "${LAFORGE_BASH_PROXY_TOKEN:-}" ]; then
    echo "Bash proxy enabled: ${LAFORGE_BASH_PROXY_URL}"
    export CLAUDE_CODE_SHELL_PREFIX="/bin/bash-proxy.sh"
fi

if [ -f CLAUDE.md ]; then
    mv CLAUDE.md .pr/CLAUDE-PROJECT.md
fi
cp /config/CLAUDE.md ./CLAUDE.md

PROMPT=$(cat /config/prompts/$PROMPTNAME.md)

# Tee Claude output to log file if LAFORGE_LOG_FILE is set
if [ -n "${LAFORGE_LOG_FILE:-}" ]; then
    # Ensure log directory exists and is writable
    LOG_DIR=$(dirname "$LAFORGE_LOG_FILE")
    if [ -d "$LOG_DIR" ] && [ -w "$LOG_DIR" ]; then
        echo "Logging Claude output to: ${LAFORGE_LOG_FILE}"
        claude --model $MODELNAME --output-format stream-json --verbose -p "$PROMPT" | tee -a "$LAFORGE_LOG_FILE" | node /bin/format-claude-output.js
    else
        echo "Warning: Log directory $LOG_DIR not writable, logging disabled"
        claude --model $MODELNAME --output-format stream-json --verbose -p "$PROMPT" | node /bin/format-claude-output.js
    fi
else
    claude --model $MODELNAME --output-format stream-json --verbose -p "$PROMPT" | node /bin/format-claude-output.js
fi

# Check if there are changes outside of the .pr directory
if git diff --quiet HEAD -- ':!.pr'; then
    echo "No changes outside .pr directory, skipping commit message generation"
else
    # Check if .pr/commit.md file exists. If it doesn't, create it.
    if [ ! -f .pr/commit.md ]; then
        if [ -n "${LAFORGE_LOG_FILE:-}" ] && [ -d "$(dirname "$LAFORGE_LOG_FILE")" ] && [ -w "$(dirname "$LAFORGE_LOG_FILE")" ]; then
            claude --model $MODELNAME --output-format stream-json --verbose -c -p "Write a commit message to .pr/commit.md" | tee -a "$LAFORGE_LOG_FILE" | node /bin/format-claude-output.js
        else
            claude --model $MODELNAME --output-format stream-json --verbose -c -p "Write a commit message to .pr/commit.md" | node /bin/format-claude-output.js
        fi
    fi
fi

rm .claude/settings.local.json || echo "Claude settings nowhere to be found"
rm CLAUDE.md || echo "CLAUDE.md nowhere to be found"

# Restore moved CLAUDE.md file
if [ -f .pr/CLAUDE-PROJECT.md ]; then
    mv .pr/CLAUDE-PROJECT.md CLAUDE.md
fi
