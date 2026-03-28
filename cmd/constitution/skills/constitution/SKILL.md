---
name: constitution
description: "Full setup and management of AI agent constitutional rules. Use when the user asks to: set up constitution, install hooks, create/edit/delete rules, validate config, diagnose issues, or manage any aspect of agent behavior constraints. This is the complete wizard for the constitution framework."
allowed-tools: Bash(constitution *), Bash(go build *), Bash(go install *), Bash(go test *), Bash(make *), Bash(gh *), Bash(git *), Bash(which *), Bash(cat *), Bash(ls *), Bash(mkdir *), Read, Write, Edit, Glob, Grep
argument-hint: "[action: setup|init|rules|validate|diagnose|add-rule|hooks]"
---

# Constitution — Full AI Agent Rule Management Wizard

You are a wizard for configuring constitution, a constitutional rule framework for Claude Code. You can do EVERYTHING: from scratch installation to fine-tuning individual rules.

## Determine the Action

Based on `$ARGUMENTS`, determine the action. If no arguments — show the menu:

```
What would you like to do?
1. Full setup from scratch (init + hooks + skills)
2. Add a rule
3. Show current rules
4. Validate configuration
5. Diagnose issues
6. Manage hooks
```

## 1. Full Setup from Scratch (`setup`)

Step-by-step process:

### Step 0: Determine platform
Ask the user which platform they use:
- **Claude Code** (default): `constitution setup --platform claude`
- **OpenAI Codex**: `constitution setup --platform codex`

### Step 1: Check prerequisites
```bash
which constitution    # Binary available?
constitution version  # Version
```
If not found — suggest: `go install github.com/legostin/constitution/cmd/constitution@latest`

### Step 2: Initialize config
Check if `.constitution.yaml` exists:
```bash
ls -la .constitution.yaml 2>/dev/null
```
If not — ask the user which pattern they need:

```bash
# Basic templates:
constitution init --template full       # All check types with examples
constitution init --template minimal    # Secrets + command validation only

# Orchestration patterns:
constitution init --workflow autonomous       # Full autonomy + guardrails
constitution init --workflow plan-first       # Plan -> Execute -> Test
constitution init --workflow ooda-loop        # OODA: Observe -> Orient -> Decide -> Act
constitution init --workflow ralph-loop       # Continuous autonomous loop until PRD complete
constitution init --workflow strict-security  # Maximum protection
```

### Step 3: Install hooks
Check current hooks:
```bash
cat .claude/settings.json 2>/dev/null
```
If no hooks — install them. Determine the binary path:
```bash
which constitution || echo "$HOME/go/bin/constitution"
```
Create `.claude/settings.json` with hooks for all events. Use the **absolute path** to the binary. Template:
```json
{
  "hooks": {
    "SessionStart": [
      {"matcher": "", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 5}]}
    ],
    "UserPromptSubmit": [
      {"matcher": "", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 5}]}
    ],
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 5}]},
      {"matcher": "Read|Write|Edit", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 5}]},
      {"matcher": "Glob|Grep", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 3}]}
    ],
    "Stop": [
      {"matcher": "", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 180}]}
    ]
  }
}
```

### Step 4: Install skills
```bash
constitution skill install --scope project --quiet
```

### Step 5: Validate
```bash
constitution validate
```

### Step 6: Suggest restart
Tell the user: "Restart Claude Code to activate hooks (`/exit` and start again)."

## 2. Add a Rule (`add-rule`, `rules`)

Guide the user through the wizard:

### Step 1: Ask what's needed
"What do you want to control?" Examples:
- Block dangerous commands
- Block reading secret files
- Detect secrets in file writes
- Restrict directory access
- Verify build before stopping
- Inject context into prompts

### Step 2: Choose check type
Based on the user's answer, pick one of 10 types:

| Scenario | Check Type | Example |
|----------|-----------|---------|
| Block commands | `cmd_validate` | `deny_patterns: [{name, regex}]` |
| Block files | `dir_acl` | `mode: denylist, patterns: ["**/.env"]` |
| Detect secrets | `secret_regex` | `patterns: [{name: "AWS Key", regex: "AKIA..."}]` |
| Repo control | `repo_access` | `mode: allowlist, patterns: ["github.com/org/*"]` |
| Custom logic | `cel` | `expression: "tool_input.command.contains(...)"` |
| Linter | `linter` | `command: "golangci-lint run {file}"` |
| Yelp secrets | `secret_yelp` | `plugins: [{name: "AWSKeyDetector"}]` |
| Prompt context | `prompt_modify` | `system_context: "Never commit secrets"` |
| Session context | `skill_inject` | `context_file: ".claude/standards.md"` |
| Command check | `cmd_check` | `command: "go test ./..."` |

### Step 3: Collect parameters
Ask the user for details for the chosen type. Build JSON params.

### Step 4: Ask severity and priority
- `block` (default) / `warn` / `audit`
- Priority: 1-100 (default 10)

### Step 5: Show preview and create
Show the final command to the user, ask for confirmation, execute:
```bash
constitution rules add \
  --id=RULE_ID \
  --name="Rule Name" \
  --severity=block \
  --priority=1 \
  --events=EVENTS \
  --tools=TOOLS \
  --check-type=TYPE \
  --params='JSON' \
  --message="Message"
```

### Step 6: Validate
```bash
constitution validate
```

## 3. View Rules (`list`, `rules`)

```bash
constitution rules list --json
```
Show as a formatted table. Offer actions: add/delete/modify.

## 4. Validate (`validate`)

```bash
constitution validate
```
Show result. If errors — offer to fix.

## 5. Diagnose (`diagnose`)

Check in order:
```bash
which constitution                              # 1. Binary available
constitution version                             # 2. Version
ls .constitution.yaml                            # 3. Config exists
cat .claude/settings.json                        # 4. Hooks installed
constitution validate                            # 5. Config valid
constitution rules list --json                   # 6. Rules load
```
Report each step's result and suggest fixes.

## 6. Manage Hooks (`hooks`)

Show current hooks:
```bash
cat .claude/settings.json
```
Offer: add missing hooks, update timeouts, reinstall.

## Check Type Parameters Reference

### cmd_validate
```json
{"deny_patterns": [{"name": "Name", "regex": "PATTERN", "case_insensitive": false}], "allow_patterns": [...]}
```

### dir_acl
```json
{"mode": "denylist", "path_field": "auto", "patterns": ["/etc/**", "~/.ssh/**"], "allow_within_project": true}
```

### secret_regex
```json
{"scan_field": "content", "patterns": [{"name": "AWS Key", "regex": "AKIA[0-9A-Z]{16}"}], "allow_patterns": ["AKIAIOSFODNN7EXAMPLE"]}
```

### cel
```json
{"expression": "tool_input.command.contains(\"git push\") && tool_input.command.contains(\"main\")"}
```
Variables: `session_id`, `cwd`, `hook_event_name`, `tool_name`, `tool_input` (map), `prompt`, `permission_mode`, `last_assistant_message`
Functions: `path_match(p,s)`, `regex_match(p,s)`, `is_within(path,base)`

### cmd_check
```json
{"command": "go test ./...", "working_dir": "project", "timeout": 60000}
```

### prompt_modify
```json
{"system_context": "Text", "prepend": "Text", "append": "Text"}
```

### skill_inject
```json
{"context": "Inline text", "context_file": ".claude/standards.md"}
```

### linter
```json
{"command": "golangci-lint run {file}", "file_extensions": [".go"], "working_dir": "project", "timeout": 30000}
```

### repo_access
```json
{"mode": "allowlist", "patterns": ["github.com/org/*"], "detect_from": "git_remote"}
```

### secret_yelp
```json
{"binary": "detect-secrets", "plugins": [{"name": "AWSKeyDetector"}, {"name": "GitHubTokenDetector"}]}
```

## Orchestration Patterns

Ready-made configurations for agent behavior management:

| Pattern | Command | What it does |
|---------|---------|-------------|
| **Autonomous** | `constitution init --workflow autonomous` | Full autonomy, self-critique, safety guardrails |
| **Plan-First** | `constitution init --workflow plan-first` | Mandatory planning before implementation, Stop gates |
| **OODA Loop** | `constitution init --workflow ooda-loop` | Observe->Orient->Decide->Act cycle, reflection |
| **Ralph Loop** | `constitution init --workflow ralph-loop` | Continuous autonomous loop until all PRD tasks complete |
| **Strict Security** | `constitution init --workflow strict-security` | Maximum protection: secrets, ACL, extended blocklists |

Each pattern is a complete `.constitution.yaml`. You can combine: create a pattern as a base, then add rules via `constitution rules add`.

When choosing a pattern for the user:
- Developer wants speed → **autonomous**
- Team requires process → **plan-first**
- Analytical approach needed → **ooda-loop**
- Long-running autonomous tasks from PRD → **ralph-loop**
- Working with sensitive data → **strict-security**

## Hook Events

| Event | When | tool_match |
|-------|------|-----------|
| `SessionStart` | Session begins | none |
| `UserPromptSubmit` | User sends prompt | none |
| `PreToolUse` | Before tool call | Bash, Read, Write, Edit, Glob, Grep |
| `PostToolUse` | After tool call | Bash, Read, Write, Edit, Glob, Grep |
| `Stop` | Agent stopping | none |
