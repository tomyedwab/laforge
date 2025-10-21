#!/bin/bash

set -euo pipefail

export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

claude

cp ~/.claude.json ~/.claude-laforge/
cp ~/.claude/.credentials.json ~/.claude-laforge/
