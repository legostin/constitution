# How To — Constitution

A practical guide to usage scenarios.

---

## For developers

### Installation and first run

```bash
# 1. Install the binary
go install github.com/legostin/constitution/cmd/constitution@latest

# 2. Create a config
constitution init
# Choose a template: Full or Minimal

# 3. Install hooks in Claude Code
constitution setup
# Select hooks via checklist, specify scope (user/project)

# 4. Verify
constitution validate
# ✓ .constitution.yaml
#   10 rules (7 enabled)

# 5. Restart Claude Code
```

### Setup for OpenAI Codex

Constitution also supports OpenAI Codex hooks (same rule engine, same config format):

```bash
# 1. Install the binary (same as Claude Code)
go install github.com/legostin/constitution/cmd/constitution@latest

# 2. Create a config (same .constitution.yaml)
constitution init

# 3. Install hooks for Codex
constitution setup --platform codex --scope project
# Hooks written to .codex/hooks.json

# 4. Enable hooks in Codex config.toml
# codex_hooks = true

# 5. Restart Codex
```

**Codex limitations:**
- Only `Bash` tool is supported (no Read/Write/Edit/Glob/Grep matchers)
- Config is standalone `.codex/hooks.json` (not embedded in settings)
- Feature must be enabled via `codex_hooks = true` in `config.toml`

All rules and check types work identically on both platforms.

### Installing for all projects (globally)

To make constitution work for any project on the machine, not just the current one:

```bash
# 1. Hooks — in user-level settings (apply to all projects)
constitution setup --scope user

# 2. Config — in the home directory
constitution init --output ~/.config/constitution/constitution.yaml
```

Configs follow the principle of constitutional hierarchy — **the more global the level, the higher its authority**:

| Level | Source | Authority | Managed by |
|-------|--------|-----------|------------|
| Global | Defined by the platform/model | Highest | Model / platform developers |
| Enterprise | Defined by the LLM provider | High | LLM provider / platform |
| User | `~/.config/constitution/constitution.yaml` | Medium | User |
| Project | `{cwd}/.constitution.yaml` | Lowest | Project developer |

> Global and Enterprise are reserved levels. Constitution does not create or look for configs at these levels. They exist for compatibility with future platform-level rule injection. You work with **User** and **Project**.

All levels are loaded and merged. A lower level **can add** its own rules and **strengthen** existing ones (warn->block), but **cannot weaken** or **disable** rules from a higher level.

```
~/.config/constitution/constitution.yaml   <- user rules (all projects)
~/work/project-a/.constitution.yaml        <- additional project rules (cannot weaken user)
~/work/project-b/                          <- no own config, user config is used
```

To check all sources and conflicts:
```bash
cd ~/work/project-a
constitution validate
#   Config sources:
#     [user] /Users/you/.config/constitution/constitution.yaml
#     [project] /Users/you/work/project-a/.constitution.yaml
#   ✓ Merged: 12 rules (9 enabled) from 2 sources
```

### Connecting to a company server

If the Platform team has already set up a server:

```bash
# One command — creates config + installs hooks
constitution setup --remote https://constitution.company.com

# Or step by step
constitution init --remote https://constitution.company.com
constitution setup
```

If authorization is required — set the token:

```bash
export CONSTITUTION_TOKEN="your-token"
```

### Testing rules

#### E2E tests (recommended)

E2E tests verify the compiled binary against a real `.constitution.yaml`. 31 test cases across all active rules:

```bash
make e2e
```

```
=== RUN   TestSecretRead_BlockEnvFile
--- PASS: TestSecretRead_BlockEnvFile (0.18s)
=== RUN   TestSecretWrite_BlockAWSKey
--- PASS: TestSecretWrite_BlockAWSKey (0.00s)
=== RUN   TestCmdValidate_BlockRmRf
--- PASS: TestCmdValidate_BlockRmRf (0.00s)
...
PASSok  	github.com/legostin/constitution/e2e	0.802s
```

To add your own test case — open `e2e/e2e_test.go` and create a function:

```go
func TestMyRule_BlocksSomething(t *testing.T) {
	run(t, testCase{
		name:            "description",
		hookEvent:       "PreToolUse",
		toolName:        "Bash",
		toolInput:       map[string]interface{}{"command": "dangerous command"},
		wantDeny:        true,
		wantReasonMatch: "substring from deny reason",
	})
}
```

Available `testCase` fields:

| Field | Type | Description |
|-------|------|-------------|
| `hookEvent` | string | `PreToolUse`, `PostToolUse`, `SessionStart`, `UserPromptSubmit`, `Stop` |
| `toolName` | string | `Bash`, `Read`, `Write`, `Edit`, `Glob`, `Grep` |
| `toolInput` | map | Tool input data (`command`, `file_path`, `content`, ...) |
| `prompt` | string | Prompt text (for `UserPromptSubmit`) |
| `wantDeny` | bool | Expect `permissionDecision: "deny"` |
| `wantExitCode` | int | Expected exit code (for `SessionStart`/`Stop`) |
| `wantSystemMsg` | bool | Expect non-empty `systemMessage` |
| `wantContext` | bool | Expect non-empty `additionalContext` |
| `wantReasonMatch` | string | Substring that must be in deny reason |

#### Manual testing (ad-hoc)

No need to run Claude Code — you can pipe JSON directly:

```bash
# Test: dangerous command
echo '{
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {"command": "rm -rf /"},
  "cwd": "'$(pwd)'"
}' | constitution

# Expected output: {"hookSpecificOutput":{"hookEventName":"PreToolUse",
#   "permissionDecision":"deny","permissionDecisionReason":"Command blocked: Root deletion"}}
```

```bash
# Test: reading .env file
echo '{
  "hook_event_name": "PreToolUse",
  "tool_name": "Read",
  "tool_input": {"file_path": "'$(pwd)'/.env"},
  "cwd": "'$(pwd)'"
}' | constitution

# Expected output: deny, "matches deny pattern **/.env"
```

```bash
# Test: secret in a file
echo '{
  "hook_event_name": "PreToolUse",
  "tool_name": "Write",
  "tool_input": {"file_path": "config.go", "content": "key = AKIAIOSFODNN7ABCDEFG"},
  "cwd": "'$(pwd)'"
}' | constitution

# Expected output: deny, "Secret detected: AWS Access Key pattern matched"
```

```bash
# Test: safe command (should pass — empty output)
echo '{
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {"command": "ls -la"},
  "cwd": "'$(pwd)'"
}' | constitution

# Empty output = allowed
```

#### Live testing with Claude Code

To verify real integration, install hooks at the project level:

```bash
constitution setup --scope project
# Or manually: create .claude/settings.json (see below)
```

Example `.claude/settings.json` with a full set of hooks:

```json
{
  "hooks": {
    "SessionStart": [
      { "matcher": "", "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }] }
    ],
    "UserPromptSubmit": [
      { "matcher": "", "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }] }
    ],
    "PreToolUse": [
      { "matcher": "Bash", "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }] },
      { "matcher": "Read|Write|Edit", "hooks": [{ "type": "command", "command": "constitution", "timeout": 5 }] },
      { "matcher": "Glob|Grep", "hooks": [{ "type": "command", "command": "constitution", "timeout": 3 }] }
    ]
  }
}
```

After creating the file, restart Claude Code and try blocked actions (reading `.env`, dangerous commands, etc.).

### What to do when a hook blocks

1. **Read the reason** — the message in `permissionDecisionReason` explains which rule was triggered
2. **Find the rule** — open `.constitution.yaml`, search by rule name
3. **Verify manually** — pipe the same JSON input (see above)
4. **Logs** — if `log_file` is set in the config, check there:
   ```yaml
   settings:
     log_level: debug
     log_file: /tmp/constitution.log
   ```

### Temporarily disabling constitution

**Option 1**: Disable a specific rule in the config:
```yaml
rules:
  - id: cmd-validate
    enabled: false    # <- was true
```

**Option 2**: Remove hooks from Claude Code:
```bash
constitution uninstall
# Then restore: constitution setup
```

**Option 3**: Rename the config:
```bash
mv .constitution.yaml .constitution.yaml.disabled
# Without a config, constitution allows everything (exit 0)
```

### Troubleshooting

**Config not found:**
```bash
constitution validate
# ✗ No config file found

# Check:
ls -la .constitution.yaml .claude/constitution.yaml
# Or specify explicitly:
constitution validate --config path/to/config.yaml
```

**Hooks not firing:**
1. Verify that hooks are installed:
   ```bash
   cat ~/.claude/settings.json | grep constitution
   ```
2. Verify that the binary is accessible:
   ```bash
   which constitution
   ```
3. Restart Claude Code (hooks are loaded at session start)

**detect-secrets not installed (for secret_yelp):**
```bash
# macOS
brew install detect-secrets
# or
pip3 install detect-secrets

# Verify
detect-secrets --version
```
Without detect-secrets, `secret_yelp` rules with severity `block` will block all actions (fail-closed). Install the utility or disable the rule. Built-in `secret_regex` rules always work.

---

## For platform engineers

### Deploying the server

#### Step 1: Create a rules file

```yaml
# company-rules.yaml
version: "1"
name: "acme-corp"

rules:
  - id: secret-scan
    name: "Secret Detection"
    enabled: true
    priority: 1
    severity: block
    hook_events: [PreToolUse]
    tool_match: [Write, Edit]
    check:
      type: secret_regex
      params:
        scan_field: content
        patterns:
          - { name: "AWS Key", regex: "AKIA[0-9A-Z]{16}" }
          - { name: "GitHub Token", regex: "gh[ps]_[A-Za-z0-9_]{36,}" }
          - { name: "Private Key", regex: "-----BEGIN .* PRIVATE KEY-----" }

  - id: cmd-block
    name: "Dangerous Commands"
    enabled: true
    priority: 1
    severity: block
    hook_events: [PreToolUse]
    tool_match: [Bash]
    check:
      type: cmd_validate
      params:
        deny_patterns:
          - { name: "Root deletion", regex: "rm\\s+-rf\\s+/" }
          - { name: "Force push", regex: "\\bgit\\s+push\\s+.*--force" }
          - { name: "Drop database", regex: "\\bdrop\\s+database\\b", case_insensitive: true }

  - id: company-standards
    name: "Inject Standards"
    enabled: true
    priority: 10
    severity: audit
    hook_events: [SessionStart]
    check:
      type: skill_inject
      params:
        context: |
          You follow ACME Corp coding standards:
          - Use structured logging (slog)
          - Write table-driven tests
          - No TODO/FIXME in production code
```

#### Step 2: docker-compose.yaml

```yaml
services:
  constitutiond:
    image: ghcr.io/legostin/constitutiond:latest
    # or build: . if building yourself
    ports:
      - "8081:8081"
    volumes:
      - ./company-rules.yaml:/etc/constitution/config.yaml:ro
    environment:
      - CONSTITUTION_TOKEN=${CONSTITUTION_TOKEN}
    restart: unless-stopped
```

#### Step 3: Launch

```bash
# Set the token
export CONSTITUTION_TOKEN="$(openssl rand -hex 32)"
echo "Token: $CONSTITUTION_TOKEN"

# Start
docker compose up -d

# Verify
curl http://localhost:8081/api/v1/health
# {"status":"ok","version":"1.0.0"}
```

#### Step 4: Distribute to developers

Send to the team:
```
constitution setup --remote https://constitution.company.com
export CONSTITUTION_TOKEN="..."
```

### Writing rules

#### Block secrets

```yaml
- id: secret-scan
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Write, Edit]
  check:
    type: secret_regex
    params:
      scan_field: content
      patterns:
        - { name: "AWS Key", regex: "AKIA[0-9A-Z]{16}" }
        - { name: "GitHub Token", regex: "gh[ps]_[A-Za-z0-9_]{36,}" }
      allow_patterns:
        - "AKIAIOSFODNN7EXAMPLE"          # AWS example key
        - "(?i)test|example|dummy"         # Test values
```

#### Restrict directories

```yaml
- id: dir-guard
  enabled: true
  priority: 2
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Read, Write, Edit, Glob, Grep]
  check:
    type: dir_acl
    params:
      mode: denylist
      path_field: auto
      patterns:
        - "~/.ssh/**"
        - "~/.aws/**"
        - "/etc/**"
        - "**/.env"
        - "**/*.pem"
      allow_within_project: true        # Allow within CWD
```

#### Block dangerous commands

```yaml
- id: cmd-block
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Bash]
  check:
    type: cmd_validate
    params:
      deny_patterns:
        - { name: "Root deletion", regex: "rm\\s+-rf\\s+/" }
        - { name: "Force push", regex: "\\bgit\\s+push\\s+.*--force" }
        - { name: "Hard reset", regex: "\\bgit\\s+reset\\s+--hard" }
        - { name: "Chmod 777", regex: "chmod\\s+777" }
        - { name: "Pipe to shell", regex: "curl.*\\|\\s*(bash|sh)" }
      allow_patterns:
        - { name: "Apt exception", regex: "sudo\\s+(apt|brew)" }
```

#### CEL for complex logic

```yaml
# Block git push to main/master
- id: no-main-push
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Bash]
  check:
    type: cel
    params:
      expression: >
        tool_input.command.contains("git push") &&
        (tool_input.command.contains("main") || tool_input.command.contains("master"))

# Block SQL DROP in .sql files
- id: no-drop
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Write]
  check:
    type: cel
    params:
      expression: >
        tool_input.file_path.endsWith(".sql") &&
        tool_input.content.contains("DROP")
```

CEL variables: `session_id`, `cwd`, `hook_event_name`, `tool_name`, `tool_input` (map), `prompt`, `permission_mode`, `last_assistant_message`.

CEL functions: `path_match(pattern, path)`, `regex_match(pattern, str)`, `is_within(path, base)`.

#### Inject company standards

```yaml
- id: standards
  enabled: true
  priority: 10
  severity: audit
  hook_events: [SessionStart]
  check:
    type: skill_inject
    params:
      context: |
        Follow ACME Corp standards:
        - Structured logging with slog
        - Table-driven tests
      # Or load from a file:
      context_file: ".claude/company-standards.md"
```

### Rolling out rules to a team

**Option A — Remote server** (recommended):
- Rules live on the server, developers connect via `constitution setup --remote URL`
- Updating: modify the YAML -> redeploy the container
- Developers receive new rules on the next hook invocation

**Option B — Config in the repository**:
- Place `.constitution.yaml` in the repo root, commit it
- Developers run `constitution setup` — hooks are installed, config is already in place
- Updating: PR with config changes

### Updating rules

Claude Code hooks read the config on every invocation (no caching). Therefore:

- **Local config**: edit the file -> the next hook invocation already uses the new rules. No Claude Code restart needed.
- **Remote server**: update the container -> clients receive new rules on the next request to `/api/v1/evaluate`.

### Monitoring

The server writes structured JSON logs via slog to stdout:

```json
{"level":"INFO","msg":"audit","session_id":"sess-123","event":"PreToolUse","rule_id":"cmd-block","passed":false,"message":"Command blocked: Force push","severity":"block"}
{"level":"INFO","msg":"request","method":"POST","path":"/api/v1/evaluate","status":200,"duration":"12ms"}
```

Connect to your logging system (DataDog, Splunk, ELK):
```yaml
# docker-compose.yaml
services:
  constitutiond:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

### Token rotation

```bash
# 1. Generate a new token
NEW_TOKEN="$(openssl rand -hex 32)"

# 2. Update on the server
export CONSTITUTION_TOKEN="$NEW_TOKEN"
docker compose up -d  # restart with the new token

# 3. Distribute the new token to developers
# They update CONSTITUTION_TOKEN in their environment
```

---

## For plugin authors

### Exec plugin (bash)

A plugin is any executable file. It receives JSON on stdin and returns JSON on stdout.

```bash
#!/bin/bash
# no-todos.sh — blocks TODO/FIXME/HACK in code

INPUT=$(cat)
CONTENT=$(echo "$INPUT" | jq -r '.input.tool_input.content // empty')

if [ -z "$CONTENT" ]; then
  echo '{"passed": true}'
  exit 0
fi

if echo "$CONTENT" | grep -qE 'TODO|FIXME|HACK'; then
  echo '{"passed": false, "message": "Code contains TODO/FIXME/HACK markers"}'
  exit 2
fi

echo '{"passed": true, "message": "OK"}'
```

```bash
chmod +x no-todos.sh
```

### Exec plugin (Go)

```go
package main

import (
    "encoding/json"
    "os"
    "strings"
)

type Input struct {
    Input struct {
        ToolInput map[string]interface{} `json:"tool_input"`
    } `json:"input"`
    Params map[string]interface{} `json:"params"`
}

type Result struct {
    Passed  bool   `json:"passed"`
    Message string `json:"message"`
}

func main() {
    var input Input
    json.NewDecoder(os.Stdin).Decode(&input)

    content, _ := input.Input.ToolInput["content"].(string)
    if strings.Contains(content, "TODO") {
        json.NewEncoder(os.Stdout).Encode(Result{Passed: false, Message: "Contains TODO"})
        os.Exit(2)
    }
    json.NewEncoder(os.Stdout).Encode(Result{Passed: true, Message: "OK"})
}
```

### Protocol

**Stdin** (JSON):
```json
{
  "input": {
    "session_id": "sess-123",
    "cwd": "/home/user/project",
    "hook_event_name": "PreToolUse",
    "tool_name": "Write",
    "tool_input": {
      "file_path": "/project/main.go",
      "content": "package main..."
    }
  },
  "params": {
    "custom_param": "value"
  }
}
```

**Stdout** (JSON):
```json
{
  "passed": true,
  "message": "OK",
  "details": {"key": "value"},
  "additional_context": "Optional context for the agent"
}
```

**Exit codes:**
| Code | Meaning |
|------|---------|
| `0`  | Check passed (`passed` from stdout) |
| `2`  | Check failed, block the action |
| Other | Plugin error, action is allowed (not blocked) |

### Manual testing

```bash
echo '{
  "input": {
    "hook_event_name": "PreToolUse",
    "tool_name": "Write",
    "tool_input": {"content": "// TODO: fix this"}
  },
  "params": {}
}' | ./no-todos.sh

# {"passed": false, "message": "Code contains TODO/FIXME/HACK markers"}
# Exit code: 2
```

### Registration in the config

> **Note**: check type `plugin` is under development. The plugin infrastructure (exec/http) is implemented, but integration with the rules engine is not yet complete. The `plugins` section in the config is parsed, but rules with `type: plugin` are not yet supported.

```yaml
plugins:
  - name: "no-todos"
    type: exec
    path: "/usr/local/bin/no-todos.sh"
    timeout: 3000

  - name: "compliance-api"
    type: http
    url: "https://compliance.internal/api/check"
    timeout: 5000
```

---

## Recipes

### Block push to main

```yaml
- id: no-main-push
  name: "Block main push"
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Bash]
  check:
    type: cel
    params:
      expression: >
        tool_input.command.contains("git push") &&
        (tool_input.command.contains("main") || tool_input.command.contains("master"))
```

### Allow sudo only for apt

```yaml
- id: sudo-control
  name: "Sudo control"
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Bash]
  check:
    type: cmd_validate
    params:
      deny_patterns:
        - { name: "Sudo", regex: "\\bsudo\\b" }
      allow_patterns:
        - { name: "Apt", regex: "sudo\\s+(apt|apt-get)" }
```

### Scan with Yelp detect-secrets

```bash
# Install
pip3 install detect-secrets
```

```yaml
- id: yelp-scan
  name: "Yelp Secret Scan"
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Write, Edit]
  check:
    type: secret_yelp
    params:
      plugins:
        - name: AWSKeyDetector
        - name: GitHubTokenDetector
        - name: PrivateKeyDetector
        - name: Base64HighEntropyString
          limit: 4.5
        - name: KeywordDetector
        - name: SlackDetector
        - name: StripeDetector
      exclude_lines: ["pragma: allowlist"]
```

### Run golangci-lint after writing Go files

```yaml
- id: lint-go
  name: "Go Linter"
  enabled: true
  priority: 10
  severity: warn      # warn = don't block, but inform the agent
  hook_events: [PostToolUse]
  tool_match: [Write, Edit]
  check:
    type: linter
    params:
      file_extensions: [".go"]
      command: "golangci-lint run --timeout=30s {file}"
      working_dir: project
      timeout: 30000
```

### Restrict the agent to a single repository

```yaml
- id: repo-lock
  name: "Repo Allowlist"
  enabled: true
  priority: 1
  severity: block
  hook_events: [SessionStart]
  check:
    type: repo_access
    params:
      mode: allowlist
      patterns:
        - "github.com/acme-corp/*"
        - "github.com/acme-corp-internal/*"
      detect_from: git_remote
```

The agent will receive an error at session start if the repository is not in the list.

### Add safety context to every prompt

```yaml
- id: safety-context
  name: "Safety Reminder"
  enabled: true
  priority: 5
  severity: audit
  hook_events: [UserPromptSubmit]
  check:
    type: prompt_modify
    params:
      system_context: |
        IMPORTANT: Never commit secrets.
        Always run tests after changes.
        Never run destructive commands without confirmation.
```

### Completeness check before stopping the agent

Use `cmd_check` for Stop events so the agent cannot stop until the project is in good shape:

```yaml
# Build must pass
- id: stop-build
  name: "Build Must Succeed"
  enabled: true
  priority: 1
  severity: block
  hook_events: [Stop]
  message: "Build is broken. Fix compilation errors before stopping."
  check:
    type: cmd_check
    params:
      command: "go build ./..."
      working_dir: project
      timeout: 60000

# Unit tests must pass
- id: stop-tests
  name: "Tests Must Pass"
  enabled: true
  priority: 2
  severity: block
  hook_events: [Stop]
  message: "Tests are failing. Fix test failures before stopping."
  check:
    type: cmd_check
    params:
      command: "go test ./internal/... ./pkg/... -count=1"
      working_dir: project
      timeout: 120000

# E2E tests must pass
- id: stop-e2e
  name: "E2E Tests Must Pass"
  enabled: true
  priority: 3
  severity: block
  hook_events: [Stop]
  message: "E2E tests are failing. Fix them before stopping."
  check:
    type: cmd_check
    params:
      command: "go test ./e2e/ -count=1"
      working_dir: project
      timeout: 120000
```

**Important**: increase the timeout for the Stop hook in `.claude/settings.json` — tests may take longer than 5 seconds:

```json
{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "constitution", "timeout": 180 }]
      }
    ]
  }
}
```

`cmd_check` is also available for other events. You can use `{cwd}` as a substitution in the command.

The CEL variable `last_assistant_message` allows you to analyze the agent's last message:

```yaml
# Warn if the agent did not mention tests in the final message
- id: stop-mention-tests
  name: "Mention Tests"
  enabled: true
  priority: 10
  severity: warn
  hook_events: [Stop]
  check:
    type: cel
    params:
      expression: >
        !(last_assistant_message.contains("test") ||
          last_assistant_message.contains("test"))
```
