# How To — Constitution

A practical guide to using Constitution, the rule-enforcement layer for AI coding agents.

---

## For developers

### Installation and first run

```bash
# 1. Install the binary
go install github.com/legostin/constitution/cmd/constitution@latest

# 2. Run the setup wizard (config + hooks + skills — all in one step)
constitution setup

# 3. Verify
constitution validate
# ✓ .constitution.yaml
#   10 rules (7 enabled)

# 4. Restart your agent
```

### Updating

```bash
# Pull the latest version
go install github.com/legostin/constitution/cmd/constitution@latest

# Re-run setup to update hooks and skills (rules are preserved)
constitution setup
```

Your `.constitution.yaml` rules are never overwritten by setup — only hooks and skills are updated. If the hook format changed between versions, `constitution setup` will regenerate them.

`constitution setup` is a 7-step interactive wizard that:
1. Asks which platform you use (Claude Code, OpenAI Codex, or both)
2. Optionally connects to a remote rule server
3. Selects security rules (secret detection, command blocking, directory ACLs)
4. Picks an orchestration pattern (autonomous, plan-first, OODA loop, etc.)
5. Configures stop validation (what the agent must verify before finishing)
6. Installs the `/constitution` skill into your agent
7. Writes everything: `.constitution.yaml`, platform hooks, skill files

### Non-interactive setup

For CI, scripts, or when you know what you want:

```bash
# Accept all defaults — project scope, default security rules, no orchestration
constitution setup --yes --scope project

# Specify everything explicitly
constitution setup --platform claude --scope project --workflow ooda-loop --security all --yes
```

Available flags:

| Flag | Values | Default |
|------|--------|---------|
| `--platform` | `claude`, `codex`, `both` | interactive |
| `--scope` | `user`, `project` | interactive |
| `--workflow` | `autonomous`, `plan-first`, `ooda-loop`, `ralph-loop`, `strict-security` | none |
| `--security` | `all`, `minimal`, `none` | interactive |
| `--yes` | (boolean) | `false` |

### Orchestration patterns

The `--workflow` flag injects orchestration rules that shape how the agent works:

| Pattern | Description |
|---------|-------------|
| `autonomous` | Full autonomy with self-critique and guardrails |
| `plan-first` | Plan, then execute, then test |
| `ooda-loop` | Observe, orient, decide, act cycle |
| `ralph-loop` | Continuous autonomous loop until PRD is complete |
| `strict-security` | Maximum protection with extended blocklists |

### Installing for all projects (globally)

To make Constitution work for any project on the machine:

```bash
constitution setup --scope user
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

No need to run your agent — you can pipe JSON directly:

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
# Test: secret in a file (use a real-looking key, not the example key)
echo '{
  "hook_event_name": "PreToolUse",
  "tool_name": "Write",
  "tool_input": {"file_path": "config.go", "content": "key = <YOUR_AWS_ACCESS_KEY>"},
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

### Troubleshooting

**Config not found:**
```bash
constitution validate
# ✗ No config file found

# Check:
ls -la .constitution.yaml
# Or specify explicitly:
constitution validate --config path/to/config.yaml
```

**Hooks not firing:**
1. Verify that hooks are installed:
   ```bash
   # Claude Code
   cat .claude/settings.json | grep constitution
   # Codex
   cat .codex/hooks.json | grep constitution
   ```
2. Verify that the binary is accessible:
   ```bash
   which constitution
   ```
3. Restart your agent (hooks are loaded at session start)

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

**What to do when a hook blocks:**

1. **Read the reason** — the message in `permissionDecisionReason` explains which rule was triggered
2. **Find the rule** — open `.constitution.yaml`, search by rule name
3. **Verify manually** — pipe the same JSON input (see manual testing above)
4. **Logs** — if `log_file` is set in the config, check there:
   ```yaml
   settings:
     log_level: debug
     log_file: /tmp/constitution.log
   ```

### Temporarily disabling Constitution

**Option 1**: Disable a specific rule in the config:
```yaml
rules:
  - id: cmd-validate
    enabled: false    # <- was true
```

**Option 2**: Remove hooks:
```bash
constitution uninstall
# Then restore: constitution setup
```

**Option 3**: Rename the config:
```bash
mv .constitution.yaml .constitution.yaml.disabled
# Without a config, constitution allows everything (exit 0)
```

---

## For Codex users

Constitution supports OpenAI Codex with the same rule engine and the same config format.

### Setup

```bash
# 1. Install the binary
go install github.com/legostin/constitution/cmd/constitution@latest

# 2. Run setup with --platform codex
constitution setup --platform codex

# 3. Enable hooks in Codex config.toml
# codex_hooks = true

# 4. Restart Codex
```

Non-interactive:
```bash
constitution setup --platform codex --scope project --yes
```

### Codex limitations

- Only `Bash` tool is supported (no Read/Write/Edit/Glob/Grep matchers)
- Config is standalone `.codex/hooks.json` (not embedded in settings)
- Feature must be enabled via `codex_hooks = true` in `config.toml`

All rules and check types work identically on both platforms.

### Dual platform setup

To install hooks for both Claude Code and Codex at once:

```bash
constitution setup --platform both
```

This writes hooks to both `.claude/settings.json` and `.codex/hooks.json` and shares a single `.constitution.yaml`.

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
```bash
constitution setup
# When prompted for remote server URL, enter: https://constitution.company.com
export CONSTITUTION_TOKEN="..."
```

### Writing rules

See the Recipes section below for rule patterns covering secrets, commands, directories, CEL expressions, stop checks, and prompt injection defense.

### Rolling out rules to a team

**Option A — Remote server** (recommended):
- Rules live on the server, developers connect during `constitution setup`
- Updating: modify the YAML, redeploy the container
- Developers receive new rules on the next hook invocation

**Option B — Config in the repository**:
- Place `.constitution.yaml` in the repo root, commit it
- Developers run `constitution setup` — hooks are installed, config is already in place
- Updating: PR with config changes

### Updating rules

The agent hooks read the config on every invocation (no caching). Therefore:

- **Local config**: edit the file and the next hook invocation already uses the new rules. No agent restart needed.
- **Remote server**: update the container and clients receive new rules on the next request to `/api/v1/evaluate`.

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

### HTTP plugin

```yaml
plugins:
  - name: "compliance-api"
    type: http
    url: "https://compliance.internal/api/check"
    timeout: 5000
```

The HTTP plugin receives the same JSON payload as a POST body and must return the same response format.

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

### Block dangerous commands

```yaml
- id: cmd-block
  name: "Command Validation"
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
        - { name: "Drop database", regex: "\\bdrop\\s+database\\b", case_insensitive: true }
      allow_patterns:
        - { name: "Apt exception", regex: "sudo\\s+(apt|brew)" }
```

### Block secret files and detect secrets in writes

```yaml
# Block reading secret files
- id: secret-read
  name: "Block Secret Files"
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Read]
  check:
    type: dir_acl
    params:
      mode: denylist
      path_field: file_path
      patterns:
        - "**/.env"
        - "**/.env.*"
        - "**/credentials.json"
        - "**/service-account*.json"
        - "**/*.pem"
        - "**/*.key"

# Detect secrets in file writes
- id: secret-write
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
      allow_patterns:
        - "<known-example-key-placeholder>"   # AWS example key
        - "(?i)test|example|dummy"            # Test values
```

### Restrict directories

```yaml
- id: dir-guard
  name: "Directory ACL"
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
        - "/var/**"
        - "~/.config/gcloud/**"
        - "../**"
      allow_within_project: true        # Allow within CWD
```

### CEL for complex logic

```yaml
# Block git push to main/master
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

# Block SQL DROP in .sql files
- id: no-drop
  name: "Block DROP in SQL"
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

### Stop validation (prompt-based)

The recommended approach is prompt-based: tell the agent what to verify, and it figures out the right commands for your stack. The setup wizard generates this automatically.

```yaml
- id: stop-validation
  name: "Stop Validation"
  enabled: true
  priority: 10
  severity: block
  hook_events: [Stop]
  message: "Verify the project builds and all tests pass. Commit all changes. Include VERIFIED_COMPLETE in your final message."
  check:
    type: cel
    params:
      expression: >
        hook_event_name == "Stop" &&
        !last_assistant_message.contains("VERIFIED_COMPLETE")
```

The agent will not be allowed to stop until it includes `VERIFIED_COMPLETE` in its final message, which it should only do after genuinely verifying the build, tests, and committing changes.

### Stop validation (git checks)

For hard requirements that should not depend on agent compliance, use `cmd_check`:

```yaml
# No uncommitted changes
- id: stop-committed
  name: "No Uncommitted Changes"
  enabled: true
  priority: 3
  severity: block
  hook_events: [Stop]
  message: "Uncommitted changes. Commit before stopping."
  check:
    type: cmd_check
    params:
      command: "git diff --quiet && git diff --cached --quiet"
      working_dir: project
      timeout: 5000

# Branch pushed to remote
- id: stop-pushed
  name: "Branch Must Be Pushed"
  enabled: true
  priority: 4
  severity: block
  hook_events: [Stop]
  message: "Branch not pushed."
  check:
    type: cmd_check
    params:
      command: >
        BRANCH=$(git rev-parse --abbrev-ref HEAD);
        git fetch origin $BRANCH --quiet 2>/dev/null;
        git diff --quiet $BRANCH..origin/$BRANCH 2>/dev/null
      working_dir: project
      timeout: 10000

# PR exists for current branch
- id: stop-pr-exists
  name: "PR Must Exist"
  enabled: true
  priority: 5
  severity: block
  hook_events: [Stop]
  message: "No PR found. Create a PR before stopping."
  check:
    type: cmd_check
    params:
      command: >
        BRANCH=$(git rev-parse --abbrev-ref HEAD);
        if [ "$BRANCH" = 'main' ] || [ "$BRANCH" = 'master' ]; then exit 0; fi;
        gh pr view --json state -q '.state' 2>/dev/null | grep -qE 'OPEN|MERGED'
      working_dir: project
      timeout: 10000
```

**Important**: increase the timeout for the Stop hook in your agent's hook config — tests and git checks may take longer than the default:

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

### Inject company standards at session start

```yaml
- id: standards
  name: "Inject Standards"
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

### Defend against prompt injection

```yaml
- id: prompt-guard
  name: "Prompt Injection Detection"
  enabled: true
  priority: 1
  severity: block
  hook_events: [UserPromptSubmit]
  check:
    type: cel
    params:
      expression: >
        prompt.contains("ignore previous instructions") ||
        prompt.contains("disregard all rules") ||
        prompt.contains("you are now")
```
