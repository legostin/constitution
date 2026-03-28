# Constitution

A rule enforcement framework for AI coding agents. Immutable rules -- the agent's "constitution" -- are defined via a YAML config and enforced through a hooks system. The agent cannot bypass or modify them.

## Architecture

```
AI agent hooks ──► constitution (Go binary)
                       │
                       ├── Local checks (< 50ms)
                       │     ├── Secret detection
                       │     ├── Directory ACL
                       │     ├── Command validation
                       │     ├── CEL expressions
                       │     └── Repository control
                       │
                       └── POST ──► constitutiond (remote service)
                                     ├── Stateful checks
                                     ├── Audit log (slog → stdout)
                                     └── Centralized config
```

A single binary serves all hook events. It reads JSON from stdin, determines the event type from the `hook_event_name` field, applies rules from the YAML config, and returns JSON to stdout. The protocol is platform-agnostic -- any AI agent that supports JSON stdin/stdout hooks can use Constitution.

## Quick Start

```bash
go install github.com/legostin/constitution/cmd/constitution@latest
constitution setup
```

`constitution setup` is a guided wizard that walks you through platform selection, security rules, orchestration patterns, stop validation, skills, and hook installation -- all in one command.

### Non-interactive Mode

```bash
constitution setup --yes --platform claude --security all
constitution setup --yes --platform codex --scope project
constitution setup --yes --platform both --workflow ooda-loop --security minimal
```

### Connecting to a Company Server

```bash
constitution setup
# Step 2 of the wizard asks for the remote server URL
```

## CLI

```
constitution                        Hook handler mode (reads JSON from stdin)
constitution setup                  Guided setup wizard (config + hooks + skills)
constitution validate               Validate configuration
constitution uninstall              Remove hooks and skills
constitution rules                  Interactive rule manager
constitution rules list             List all rules (--json for machine output)
constitution rules add              Add a rule (interactive or --id/--json flags)
constitution rules edit <id>        Edit a rule (by ID or number)
constitution rules delete <id>      Delete a rule
constitution rules toggle <id>      Enable/disable a rule
constitution version                Show version
```

Six commands total: `setup`, `validate`, `uninstall`, `rules`, `version`, plus the implicit hook handler mode (no subcommand, stdin is a pipe).

### Setup Wizard Steps

The `constitution setup` wizard runs through 7 steps:

| Step | What It Does |
|------|-------------|
| 1. Platform | Choose Claude Code, OpenAI Codex, or both |
| 2. Remote | Optionally connect to a centralized rules server |
| 3. Security Rules | Pick security protections (secrets, commands, ACL, etc.) |
| 4. Orchestration | Apply an orchestration pattern (autonomous, plan-first, etc.) |
| 5. Stop Validation | Define what the agent must verify before stopping |
| 6. Skills | Install /constitution slash command |
| 7. Install | Write `.constitution.yaml`, install hooks, install skills |

### Non-interactive Flags

```bash
constitution setup \
  --yes                          # Accept all defaults
  --platform claude|codex|both   # Target platform
  --scope user|project           # Installation scope
  --workflow autonomous          # Orchestration pattern
  --security all|minimal|none    # Security preset
```

## Platform Support

Constitution supports multiple AI agent platforms through a unified protocol:

| Platform | Setup Command | Config Location |
|----------|-------------|-----------------|
| **Claude Code** | `constitution setup --platform claude` (default) | `.claude/settings.json` |
| **OpenAI Codex** | `constitution setup --platform codex` | `.codex/hooks.json` |

### OpenAI Codex

```bash
constitution setup --platform codex --scope project
```

Codex hooks use the same JSON stdin/stdout protocol as Claude Code. Limitations:
- Only `Bash` tool is currently supported (no Read/Write/Edit/Glob/Grep matchers)
- Requires `codex_hooks = true` in `config.toml`
- Config is standalone `.codex/hooks.json` (not embedded in settings.json)

All constitution rules, check types, and orchestration patterns work identically on both platforms.

## Orchestration Patterns

Ready-made configurations for agent behavior management. Selected in step 4 of the setup wizard or via the `--workflow` flag:

```bash
constitution setup --workflow autonomous
constitution setup --workflow plan-first
constitution setup --workflow ooda-loop
constitution setup --workflow ralph-loop
constitution setup --workflow autoproduct
constitution setup --workflow strict-security
```

| Pattern | Description | Key Rules |
|---------|-------------|-----------|
| **Autonomous** | Agent makes decisions on its own, safety guardrails | skill_inject (self-critique), cmd_validate, secret_regex, Stop gates |
| **Plan-First** | Plan first, then code, then tests | skill_inject (workflow), prompt_modify (reminder), Stop: build+tests+commit |
| **OODA Loop** | Military framework: observe, orient, decide, act | skill_inject (OODA cycle), prompt_modify (cycle reminder) |
| **Ralph Loop** | Continuous autonomous loop until all PRD tasks complete | skill_inject (loop behavior), Stop: build+tests+committed |
| **Autoproduct** | Spec-driven autonomous dev (inspired by Karpathy's autoresearch) | SPEC.md as source of truth, iteration logging to PROGRESS.md, commit/revert cycle |
| **Strict Security** | Maximum protection | Extended secrets, Yelp detect-secrets, strict ACL, expanded cmd blocklist, repo control |

Each pattern generates a complete `.constitution.yaml` with pre-configured rules. The wizard merges orchestration rules (skill_inject, prompt_modify) with the security rules you selected in step 3, so there are no duplicates. You can further customize rules via `constitution rules add` after setup.

## Configuration

### Config Hierarchy (4 Levels)

Constitution uses a multi-level configuration system based on the principle of constitutional hierarchy: **a more global level has greater authority** and cannot be weakened by a lower level.

| Level | Authority | Source | Managed By |
|-------|-----------|--------|------------|
| **Global** | Highest | Defined by the platform/model | Model / platform developers (outside constitution's control) |
| **Enterprise** | High | Defined by the LLM provider | LLM provider / platform (outside constitution's control) |
| **User** | Medium | `~/.config/constitution/constitution.yaml` | User |
| **Project** | Lowest | `{cwd}/.constitution.yaml` or `{cwd}/.claude/constitution.yaml` | Project developer |

> **Note**: The Global and Enterprise levels are reserved for rules set by model developers or the platform. Constitution does not create, search for, or manage configs at these levels -- they exist in the type system for compatibility with future platform rule injection. Users work with the **User** and **Project** levels.

All found configs are **loaded and merged**. The `--config` flag and `$CONSTITUTION_CONFIG` have the User level.

**Merge Rules:**

- A lower level **can** add new rules
- A lower level **can** increase severity (warn → block)
- A lower level **CANNOT** decrease severity (block → warn)
- A lower level **CANNOT** disable a rule from a higher level
- Settings: the first non-empty value from the highest level wins
- Remote: the highest level with `enabled: true` wins entirely
- Plugins: merged by name; in case of conflict, the highest level wins

```
~/.config/constitution/constitution.yaml   ← user rules (all projects)
~/work/project-a/.constitution.yaml        ← additional project rules (cannot weaken user rules)
~/work/project-b/                          ← no config of its own, user rules apply
```

Running `constitution validate` shows all discovered sources and merge conflicts.

### Config Format

```yaml
version: "1"
name: "my-constitution"

settings:
  log_level: "info"          # debug | info | warn | error
  log_file: "/tmp/constitution.log"

remote:
  enabled: false
  url: "http://localhost:8081"
  auth_token_env: "CONSTITUTION_TOKEN"
  timeout: 5000              # ms
  fallback: "local-only"     # allow | deny | local-only

plugins:
  - name: "my-plugin"
    type: "exec"             # exec | http
    path: "/usr/local/bin/my-check"
    timeout: 3000

rules:
  - id: unique-rule-id
    name: "Human-readable name"
    description: "Optional description"
    enabled: true
    priority: 1              # Lower = runs first
    severity: block          # block | warn | audit
    hook_events: [PreToolUse]
    tool_match: [Bash]       # Optional, regex-compatible
    remote: false            # Delegate to remote service
    message: "Custom message"
    check:
      type: cmd_validate     # Check type
      params:                # Parameters depend on the type
        deny_patterns:
          - { name: "Root rm", regex: "rm\\s+-rf\\s+/" }
```

### Severity

| Value | Action |
|-------|--------|
| `block` | Blocks the agent's action. Returns `deny` for PreToolUse or `exit 2` for SessionStart. |
| `warn` | Allows the action but adds a warning to `systemMessage`. |
| `audit` | Allows without intervention, only logs to a file. |

### Hook Events

| Event | When It Fires | Typical Checks |
|-------|---------------|----------------|
| `SessionStart` | Session start | `repo_access`, `skill_inject` |
| `UserPromptSubmit` | Before processing a prompt | `prompt_modify` |
| `PreToolUse` | Before a tool call | `cmd_validate`, `secret_regex`, `dir_acl`, `cel` |
| `PostToolUse` | After a tool call | `linter` |
| `Stop` | Agent is finishing | `cmd_check` (tests, build), `cel` |

## Check Types

### `cmd_validate` -- Bash Command Validation

Blocks dangerous commands by regex patterns.

```yaml
check:
  type: cmd_validate
  params:
    deny_patterns:
      - name: "Root deletion"
        regex: "rm\\s+-rf\\s+/"
      - name: "Drop database"
        regex: "\\bdrop\\s+database\\b"
        case_insensitive: true
    allow_patterns:           # Exceptions (checked first)
      - name: "Apt exception"
        regex: "sudo\\s+apt"
```

**How it works**: extracts the `command` field from `tool_input`, first checks `allow_patterns` (if matched -- skips), then `deny_patterns` (if matched -- blocks).

### `secret_regex` -- Secret Detection

Scans file contents for secrets before writing.

```yaml
check:
  type: secret_regex
  params:
    scan_field: content       # tool_input field to scan
    patterns:
      - name: "AWS Access Key"
        regex: "AKIA[0-9A-Z]{16}"
      - name: "GitHub Token"
        regex: "gh[ps]_[A-Za-z0-9_]{36,}"
      - name: "Private Key"
        regex: "-----BEGIN .* PRIVATE KEY-----"
    allow_patterns:           # Exceptions (false positives)
      - "AKIAIOSFODNN7EXAMPLE"
      - "(?i)test|example|dummy"
```

**How it works**: for `Write` it scans the `content` field, for `Edit` -- the `new_string` field. If a pattern matches and the match is not covered by `allow_patterns` -- it blocks.

### `dir_acl` -- Directory Access Control

Restricts which files and directories the agent can access.

```yaml
check:
  type: dir_acl
  params:
    mode: denylist            # denylist | allowlist
    path_field: auto          # auto | file_path | path | pattern
    patterns:
      - "/etc/**"
      - "~/.ssh/**"
      - "~/.aws/**"
      - "**/.env"
      - "**/*.pem"
    allow_within_project: true  # Allow everything within CWD
```

**Supported glob patterns**:
- `**` -- any depth of directory nesting
- `*` -- any file name
- `~` -- user's home directory

**`path_field: auto`** -- automatically tries the fields `file_path` -> `path` -> `pattern` and uses the first one found.

### `repo_access` -- Repository Control

Allows or denies running the agent in specific repositories.

```yaml
check:
  type: repo_access
  params:
    mode: allowlist           # allowlist | denylist
    patterns:
      - "github.com/my-org/*"
      - "github.com/my-org-internal/*"
    detect_from: git_remote   # git_remote | directory
```

**How it works**: on `SessionStart`, it determines the current repository via `git remote get-url origin`, normalizes the URL (SSH and HTTPS -> `github.com/org/repo`), and compares it with patterns. If the repo is not in the allowlist -- the session is blocked.

### `cel` -- CEL Expressions

For complex logic that cannot be expressed with simple regex patterns. Uses the [Common Expression Language](https://github.com/google/cel-go).

```yaml
check:
  type: cel
  params:
    expression: >
      tool_input.command.contains("git push") &&
      (tool_input.command.contains("main") || tool_input.command.contains("master"))
```

**Available variables**:

| Variable | Type | Description |
|----------|------|-------------|
| `session_id` | `string` | Session ID |
| `cwd` | `string` | Current working directory |
| `hook_event_name` | `string` | Event type |
| `tool_name` | `string` | Tool name |
| `tool_input` | `map(string, dyn)` | Tool input data |
| `prompt` | `string` | User prompt text |
| `permission_mode` | `string` | Permission mode |
| `last_assistant_message` | `string` | Last agent message (Stop events) |

**Built-in functions**:

| Function | Signature | Description |
|----------|-----------|-------------|
| `path_match` | `(pattern, path) -> bool` | Glob matching for paths |
| `regex_match` | `(pattern, str) -> bool` | Regex matching for strings |
| `is_within` | `(path, base) -> bool` | Checks that the path is inside the base directory |

**CEL expression examples**:

```yaml
# Block writing to prod directories unless in bypass mode
expression: >
  is_within(tool_input.file_path, "/prod") &&
  permission_mode != "bypassPermissions"

# Block curl with suspicious domains
expression: >
  tool_name == "Bash" &&
  tool_input.command.contains("curl") &&
  regex_match("https?://(pastebin|hastebin|0x0)", tool_input.command)

# Block writing SQL files with DROP statements
expression: >
  tool_name == "Write" &&
  tool_input.file_path.endsWith(".sql") &&
  tool_input.content.contains("DROP")
```

### `secret_yelp` -- Yelp detect-secrets

Integration with [Yelp detect-secrets](https://github.com/Yelp/detect-secrets) -- 28+ secret detectors (AWS, GitHub, GitLab, Slack, Stripe, JWT, entropy-based, and more).

**Requirements**: `pip install detect-secrets`

```yaml
check:
  type: secret_yelp
  params:
    plugins:
      - name: AWSKeyDetector
      - name: GitHubTokenDetector
      - name: PrivateKeyDetector
      - name: Base64HighEntropyString
        limit: 4.5
      - name: HexHighEntropyString
        limit: 3.0
      - name: KeywordDetector
      - name: SlackDetector
      - name: StripeDetector
    filters:
      - path: secret_yelp.filters.gibberish.should_exclude_secret
      - path: secret_yelp.filters.allowlist.is_line_allowlisted
    exclude_secrets: ["(?i)example|test|dummy"]
    exclude_lines: ["pragma: allowlist"]
    binary: "detect-secrets"
```

**How it works**: extracts content from `tool_input`, scans each line via `detect-secrets scan --string` (line-by-line scanning is more reliable than file-based). The plugins/filters config from YAML is dynamically generated into a JSON baseline file. If `detect-secrets` is not installed -- `Init()` will return an error; with `severity: block`, the action will be blocked (fail-closed).

**Available plugins** (28+): `AWSKeyDetector`, `ArtifactoryDetector`, `AzureStorageKeyDetector`, `Base64HighEntropyString`, `BasicAuthDetector`, `CloudantDetector`, `DiscordBotTokenDetector`, `GitHubTokenDetector`, `GitLabTokenDetector`, `HexHighEntropyString`, `IbmCloudIamDetector`, `JwtTokenDetector`, `KeywordDetector`, `MailchimpDetector`, `NpmDetector`, `OpenAIDetector`, `PrivateKeyDetector`, `SendGridDetector`, `SlackDetector`, `StripeDetector`, `TelegramBotTokenDetector`, `TwilioKeyDetector`, and more.

**Compatibility**: can be used simultaneously with `secret_regex` (regex) -- they work independently.

### `linter` -- Running Linters

Runs an external linter after writing/editing files.

```yaml
check:
  type: linter
  params:
    file_extensions: [".go"]  # Filter by extensions
    command: "golangci-lint run --timeout=30s {file}"
    working_dir: project      # project | file
    timeout: 30000            # ms
```

**Substitutions**: `{file}` is replaced with the file path.

**`working_dir`**: `project` -- runs from the project CWD, `file` -- from the file's directory.

### `prompt_modify` -- Prompt Modification

Adds context to user prompts.

```yaml
check:
  type: prompt_modify
  params:
    system_context: |
      IMPORTANT: Never commit secrets.
      Always run tests after changes.
    prepend: "Security reminder: "
    append: ""
```

The context is added via `additionalContext` in the hook response -- the agent sees it as a system message.

### `skill_inject` -- Skill Injection

Loads context from a file or inline text at session start.

```yaml
check:
  type: skill_inject
  params:
    context: |
      You follow ACME Corp coding standards.
    context_file: ".claude/company-context.md"
```

If both are specified -- `context_file` takes priority. If the file is not found -- falls back to `context`.

### `cmd_check` -- Running Arbitrary Commands

Runs a shell command and checks the exit code. Unlike `linter`, it is not tied to a file -- suitable for Stop validation (checking builds, tests).

```yaml
check:
  type: cmd_check
  params:
    command: "go test ./... -count=1"   # Shell command
    working_dir: project                # project (CWD) | absolute path
    timeout: 120000                     # ms, default 30s
```

**Substitutions**: `{cwd}` is replaced with the current project working directory.

**How it works**: runs `sh -c "command"`, exit 0 means the check passed, otherwise it failed. Command output is returned in `Message` (on error) and `AdditionalContext`.

**Typical usage** -- Stop validation:

```yaml
- id: stop-tests
  name: "Tests Must Pass"
  enabled: true
  priority: 1
  severity: block
  hook_events: [Stop]
  message: "Tests are failing. Fix test failures before stopping."
  check:
    type: cmd_check
    params:
      command: "go test ./internal/... ./pkg/... -count=1"
      working_dir: project
      timeout: 120000
```

### `plugin` -- External Plugins (planned)

> **Note**: the plugin system is under development. The infrastructure (exec/http) is implemented, but the `plugin` check type is not yet registered in the engine. The `plugins` section in the config is parsed, but rules with `type: plugin` are not yet supported.

## Remote Service (constitutiond)

For centralized rule management and auditing.

### Running

```bash
constitutiond \
  --config constitution.yaml \
  --addr :8081 \
  --token "your-secret-token"
```

### Docker Compose

```yaml
# docker-compose.yaml
services:
  constitutiond:
    image: ghcr.io/legostin/constitutiond:latest
    ports:
      - "8081:8081"
    volumes:
      - ./company-rules.yaml:/etc/constitution/config.yaml:ro
    environment:
      - CONSTITUTION_TOKEN=${CONSTITUTION_TOKEN}
```

```bash
docker compose up -d
```

### From Source

```bash
go install github.com/legostin/constitution/cmd/constitutiond@latest
constitutiond --config rules.yaml --addr :8081
```

### API

```
POST /api/v1/evaluate    — Execute rules for hook input
POST /api/v1/audit       — Write audit log (→ slog structured logging)
GET  /api/v1/config      — Get current config
GET  /api/v1/health      — Health check
```

### Client Configuration

```yaml
remote:
  enabled: true
  url: "http://localhost:8081"
  auth_token_env: "CONSTITUTION_TOKEN"
  timeout: 5000
  fallback: "local-only"   # What to do if remote is unavailable
```

**Fallback strategies**:

| Value | Behavior |
|-------|----------|
| `local-only` | Run only local rules, skip remote |
| `allow` | Skip all remote rules, allow the action |
| `deny` | Block everything if remote is unavailable |

### Server Deployment (for Companies)

The Platform team runs `constitutiond` with the company's rules. Developers connect via the setup wizard (step 2).

```
company-constitution/              <- Platform team's Git repo
├── company-rules.yaml             <- rules
├── docker-compose.yaml            <- deployment
└── .github/workflows/deploy.yaml  <- CI: push → redeploy
```

The Platform team edits the YAML, pushes it, and CI updates the container. Developers do nothing.

## Testing

### Unit Tests

```bash
make test           # All tests with race detector
make test-v         # With verbose output
```

### E2E Tests

E2E tests verify the **compiled binary** against a real `.constitution.yaml`. Each test feeds JSON to stdin and checks exit code + JSON output.

```bash
make e2e            # Build binary + run E2E tests
```

35 test cases across all active rules:

| Group | Tests | What It Checks |
|-------|-------|----------------|
| `secret-read` | 7 | Block `.env`, `.env.*`, `credentials.json`, `.pem`, `.key` + allow regular files |
| `secret-write` | 6 | Block AWS key, GitHub token, RSA key, JWT + allow example keys |
| `cmd-validate` | 9 | Block `rm -rf /`, `chmod 777`, `curl\|bash`, force push, hard reset, DROP DATABASE |
| `CEL` | 3 | Block push to main/master + allow feature branches |
| `dir-acl` | 5 | Block `/etc/`, `/var/`, `~/.ssh/`, `~/.aws/` + allow project files |
| `prompt-safety` | 1 | Safety context injection into prompts |
| `stop` | 4 | Block on failing build/tests, without `VERIFIED_PRODUCTION_READY` + allow on success |

E2E tests are located in `e2e/e2e_test.go`. To add a new case, create a `testCase` and call `run(t, tc)`.

### Smoke Test

```bash
make smoke-test     # Quick check: rm -rf / should be blocked
```

## Development

```bash
make build          # Build binaries into bin/
make install        # Install globally (go install)
make test           # Unit tests with race detector
make e2e            # E2E tests (binary + real config)
make lint           # go vet
make fmt            # Format code
make tidy           # Tidy modules
make smoke-test     # Verify rm -rf / is blocked
make run-server     # Run constitutiond locally
make docker-build   # Build Docker image
make docker-run     # Run via docker compose
```

### Project Structure

```
cmd/
  constitution/       CLI + hook handler (setup, validate, rules, ...)
    configs/          Embedded config templates (go:embed)
    skills/           Embedded skill definitions (go:embed)
  constitutiond/      Remote service
internal/
  celenv/             CEL environment (variables + functions)
  check/              10 check types
  config/             YAML loading and validation
  engine/             Rule orchestration
  handler/            Event handlers (PreToolUse, Stop, ...)
  hook/               JSON I/O + dispatcher
  plugin/             Exec + HTTP plugins
  remote/             HTTP client for constitutiond
  server/             HTTP server + middleware (stateless)
pkg/types/            Shared types (HookInput, HookOutput, Rule, ...)
e2e/                  E2E tests (binary + real .constitution.yaml)
configs/              Example configurations (standalone)
Dockerfile            Multi-stage build
docker-compose.yaml   Server deployment
```

## Protocol

Constitution communicates with AI agents via JSON on stdin/stdout. This protocol is platform-agnostic -- any agent that supports command-based hooks can use it.

### Input (stdin)

The agent passes JSON to the hook's stdin:

```json
{
  "session_id": "sess-abc123",
  "cwd": "/home/user/project",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "rm -rf /" },
  "permission_mode": "default"
}
```

### Output (stdout)

#### Allow (empty output or exit 0 without stdout)

No output -- action is allowed.

#### Block (PreToolUse)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "Command blocked: Root deletion"
  }
}
```

#### Warn

```json
{
  "systemMessage": "[Command Validation] Potentially dangerous command detected"
}
```

#### Inject Context (SessionStart / UserPromptSubmit)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "Follow ACME coding standards..."
  }
}
```

#### Block Stop (Stop)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "Stop",
    "decision": "block",
    "reason": "Tests not executed after code changes"
  }
}
```

---

[Dokumentация на русском](docs/ru/README.md)
