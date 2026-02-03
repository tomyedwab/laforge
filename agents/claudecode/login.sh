#!/bin/bash

set -euo pipefail

mkdir -p /credentials/.claude/
touch /credentials/.claude/.claude.json

claude

cp ~/.claude.json /credentials/.claude/
cp ~/.claude/.credentials.json /credentials/.claude/
