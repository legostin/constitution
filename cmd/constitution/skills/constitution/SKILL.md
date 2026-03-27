---
name: constitution
description: Manage AI agent constitutional rules via the constitution CLI. Use when the user asks to validate config, list rules, diagnose hook issues, set up constitution, or check rule status.
allowed-tools: Bash(constitution *), Read, Write, Edit, Glob, Grep
argument-hint: "[action: validate|rules|setup|diagnose]"
---

# Constitution — AI Agent Rule Management

You have access to the `constitution` CLI tool for managing AI agent rules (hooks).

## Available Commands

### Viewing & Validation
```bash
constitution rules list --json      # List all rules as JSON
constitution validate               # Validate config, show merge conflicts
constitution version                # Show version
```

### Adding Rules (non-interactive)
```bash
constitution rules add \
  --id=RULE_ID \
  --name="Rule Name" \
  --severity=block \
  --priority=1 \
  --events=PreToolUse \
  --tools=Bash \
  --check-type=cmd_validate \
  --params='{"deny_patterns":[{"name":"Pattern Name","regex":"REGEX"}]}' \
  --message="Block message"

# Or via JSON stdin:
echo '{"id":"rule-id","name":"Name","enabled":true,"priority":1,"severity":"block","hook_events":["PreToolUse"],"tool_match":["Bash"],"check":{"type":"cmd_validate","params":{"deny_patterns":[{"name":"test","regex":"test"}]}}}' | constitution rules add --json
```

### Modifying Rules
```bash
constitution rules toggle RULE_ID --yes    # Enable/disable
constitution rules delete RULE_ID --yes    # Remove rule
```

## Check Types Reference

| Type | Use Case | Key Params |
|------|----------|-----------|
| `secret_regex` | Block secrets in file writes | `scan_field`, `patterns: [{name, regex}]`, `allow_patterns` |
| `dir_acl` | Block file access by path | `mode` (deny/allow), `patterns`, `allow_within_project` |
| `cmd_validate` | Block dangerous bash commands | `deny_patterns: [{name, regex}]`, `allow_patterns` |
| `repo_access` | Restrict repositories | `mode`, `patterns`, `detect_from` |
| `cel` | Custom CEL expressions | `expression` (returns bool) |
| `linter` | Run external linter | `command` (with {file}), `file_extensions`, `timeout` |
| `secret_yelp` | Yelp detect-secrets | `plugins: [{name}]`, `exclude_secrets` |
| `prompt_modify` | Inject prompt context | `system_context`, `prepend`, `append` |
| `skill_inject` | Session start context | `context`, `context_file` |
| `cmd_check` | Run command, check exit code | `command`, `working_dir`, `timeout` |

## Hook Events

| Event | When | Tool Match |
|-------|------|-----------|
| `SessionStart` | Session begins | No |
| `UserPromptSubmit` | User sends prompt | No |
| `PreToolUse` | Before tool call | Yes: Bash, Read, Write, Edit, Glob, Grep |
| `PostToolUse` | After tool call | Yes |
| `Stop` | Agent stopping | No |

## Severity Levels

- `block` — Prevent the action entirely
- `warn` — Allow but show warning
- `audit` — Allow silently, log only

## Workflow

Based on `$ARGUMENTS`:
- **validate** → Run `constitution validate` and report results
- **rules** → Run `constitution rules list --json`, show formatted table, ask what to do
- **setup** → Check if hooks are installed, offer `constitution setup --scope project --all`
- **diagnose** → Check: binary available? config exists? hooks in settings.json? run validate
- **No args** → Show status summary (version, rule count, hook status)

## Config File

The config file is `.constitution.yaml` in the project root. Read it to understand current rules before making changes.
