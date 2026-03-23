#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Building constitution..."
go build -o /usr/local/bin/constitution "$PROJECT_DIR/cmd/constitution"
echo "Installed: /usr/local/bin/constitution"

echo "Building constitutiond..."
go build -o /usr/local/bin/constitutiond "$PROJECT_DIR/cmd/constitutiond"
echo "Installed: /usr/local/bin/constitutiond"

# Generate Claude Code hooks snippet
echo ""
echo "Add the following to your .claude/settings.json (or ~/.claude/settings.json):"
echo ""
cat << 'HOOKS_JSON'
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }]
      },
      {
        "matcher": "Read|Write|Edit",
        "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }]
      },
      {
        "matcher": "Glob|Grep",
        "hooks": [{ "type": "command", "command": "constitution", "timeout": 3 }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [{ "type": "command", "command": "constitution", "timeout": 60 }]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }]
      }
    ]
  }
}
HOOKS_JSON

echo ""
echo "Then create .constitution.yaml in your project root (or copy from configs/)."
echo "Done!"
