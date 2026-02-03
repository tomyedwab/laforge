#!/bin/bash

set -euo pipefail

echo "Invoking Claude Code..."

# Copy config files and fix permissions
mkdir -p ~/.claude
cp /credentials/.claude/.claude.json ~/
cp /credentials/.claude/.credentials.json ~/.claude/
chmod 600 ~/.claude.json
chmod 600 ~/.claude/.credentials.json

mkdir -p .claude
cp /config/settings.local.json .claude/

if [ -f CLAUDE.md ]; then
    mv CLAUDE.md .pr/CLAUDE-PROJECT.md
fi
mv /config/CLAUDE.md ./CLAUDE.md

PROMPT=$(cat /config/prompts/$PROMPTNAME.md)

claude --model $MODELNAME --output-format stream-json --verbose -p "$PROMPT" | node /bin/format-claude-output.js

# Check if there are changes outside of the .pr directory
if git diff --quiet HEAD -- ':!.pr'; then
    echo "No changes outside .pr directory, skipping commit message generation"
else
    # Check if .pr/commit.md file exists. If it doesn't, create it.
    if [ ! -f .pr/commit.md ]; then
        claude --model $MODELNAME --output-format stream-json --verbose -c -p "Write a commit message to .pr/commit.md" | node /bin/format-claude-output.js
    fi
fi

rm .claude/settings.local.json || echo "Claude settings nowhere to be found"
rm CLAUDE.md || echo "CLAUDE.md nowhere to be found"

# Restore moved CLAUDE.md file
if [ -f .pr/CLAUDE-PROJECT.md ]; then
    mv .pr/CLAUDE-PROJECT.md CLAUDE.md
fi
