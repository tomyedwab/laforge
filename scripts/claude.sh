#!/bin/bash

set -euo pipefail

export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

# Copy config files with fixed permissions
mkdir -p ~/.claude
cp ~/.claude-laforge/.claude.json ~/
chmod 600 ~/.claude.json
cp ~/.claude-laforge/.credentials.json ~/.claude/
chmod 600 ~/.claude/.credentials.json

mkdir -p /src/.claude
cp /bin/.claude/settings.local.json /src/.claude/

claude --model $MODELNAME -p "Work on the next task."

# Check if COMMIT.md file exists. If it doesn't, create it.
if [ ! -f COMMIT.md ]; then
    claude --model $MODELNAME -c -p "Write a commit message to COMMIT.md"
fi

rm /src/.claude/settings.local.json
