#!/bin/bash

set -euo pipefail

export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

BIN=/home/laforge/.opencode/bin/opencode
$BIN -m $MODELNAME run "Work on the next task."

# Check if COMMIT.md file exists. If it doesn't, create it.
if [ ! -f COMMIT.md ]; then
    $BIN -m $MODELNAME run --continue "Write a commit message to COMMIT.md"
fi
