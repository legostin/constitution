# Constitution

A Go framework for governing Claude Code behavior through a hooks system. Immutable rules -- the agent's "constitution" -- are defined via a YAML config and cannot be bypassed by the agent.

## Architecture

```
Claude Code hooks ──► constitution (Go binary)
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

A single binary serves all Claude Code hooks. It reads JSON from stdin, determines the event type from the `hook_event_name` field, applies rules from the YAML config, and returns JSON to stdout.

## Quick Start

### Installation

```bash
go install github.com/legostin/constitution/cmd/constitution@latest
```

### Scenario 1: Local Rules

```bash
constitution init                 # Create .constitution.yaml from a template
constitution setup                # Interactively install hooks into Claude Code
```

### Scenario 2: Connecting to a Company Server

```bash
constitution setup --remote https://constitution.company.com
# → Creates .constitution.yaml with remote URL + installs hooks
```

### Scenario 3: Config Already in the Repository

If `.constitution.yaml` is already in the repo (added by the Platform team):

```bash
constitution setup                # Finds the config, installs hooks
```

## CLI

```
constitution                      # Hook handler (stdin/stdout) — called by Claude Code
constitution init                 # Create .constitution.yaml
constitution init --template minimal
constitution init --remote URL    # Create a remote-only config
constitution setup                # Interactive hook installation
constitution setup --remote URL   # Quick remote setup + hooks
constitution setup --scope user   # Install into ~/.claude/settings.json
constitution validate             # Validate config
constitution uninstall            # Remove hooks from settings.json
constitution rules                # Interactive rules manager
constitution rules add            # Step-by-step rule creation wizard
constitution rules list           # Show all rules
constitution rules edit <id>      # Edit a rule
constitution rules delete <id>    # Delete a rule
constitution rules toggle <id>    # Enable/disable a rule
constitution rules add --id=X --check-type=Y --events=Z --params='{...}'  # Non-interactive mode
constitution rules list --json    # JSON output for scripts
constitution skill install        # Install Claude Code skills
constitution skill uninstall      # Remove skills
constitution skill list           # Show installed skills
constitution version
```

### Claude Code Skills

Constitution ships with two Claude Code skills:

| Skill | Description |
|-------|-------------|
| `/constitution` | Rule management, validation, diagnostics -- Claude calls the CLI |
| `/constitution-rules` | Quick rule creation through a dialog with the user |

```bash
constitution skill install --scope project   # Install for this project
constitution skill install --scope user      # Install for all projects
```

Skills use the non-interactive CLI mode (`--json`, `--yes` flags) so that Claude can programmatically invoke commands.

### Orchestration Patterns

Ready-made configurations for popular agent management patterns:

```bash
constitution init --workflow autonomous       # Full autonomy + guardrails
constitution init --workflow plan-first       # Plan → Execute → Test
constitution init --workflow ooda-loop        # OODA: Observe → Orient → Decide → Act
constitution init --workflow ralph-loop       # Continuous autonomous loop until PRD complete
constitution init --workflow strict-security  # Maximum security
```

| Pattern | Description | Key Rules |
|---------|-------------|-----------|
| **Autonomous** | Agent makes decisions on its own, safety guardrails | skill_inject (self-critique), cmd_validate, secret_regex, Stop gates |
| **Plan-First** | Plan first, then code, then tests | skill_inject (workflow), prompt_modify (reminder), Stop: build+tests+commit |
| **OODA Loop** | Military framework: observe → orient → decide → act | skill_inject (OODA cycle), prompt_modify (cycle reminder) |
| **Ralph Loop** | Continuous autonomous loop until all PRD tasks complete | skill_inject (loop behavior), Stop: build+tests+committed |
| **Strict Security** | Maximum protection | Extended secrets, Yelp detect-secrets, strict ACL, expanded cmd blocklist, repo control |

Each pattern is a complete `.constitution.yaml` with pre-configured rules. You can combine them: create a pattern as a base, then add rules via `constitution rules add`.

## Server Deployment (for Companies)

The Platform team runs `constitutiond` with the company's rules. Developers connect via `constitution setup --remote URL`.

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

### Rule Management

```
company-constitution/              ← Platform team's Git repo
├── company-rules.yaml             ← rules
├── docker-compose.yaml            ← deployment
└── .github/workflows/deploy.yaml  ← CI: push → redeploy
```

The Platform team edits the YAML, pushes it, and CI updates the container. Developers do nothing.

## Configuration

### Config Hierarchy (4 Levels)

Constitution uses a multi-level configuration system based on the principle of constitutional hierarchy: **a more global level has greater authority** and cannot be weakened by a lower level.

| Level | Authority | Source | Managed By |
|-------|-----------|--------|------------|
| **Global** | Highest | Defined by the platform/model | Model / platform developers (outside constitution's control) |
| **Enterprise** | High | Defined by the LLM provider | LLM provider / platform (outside constitution's control) |
| **User** | Medium | `~/.config/constitution/constitution.yaml` | User |
| **Project** | Lowest | `{cwd}/.constitution.yaml` or `{cwd}/.claude/constitution.yaml` | Project developer |

> **Note**: The Global and Enterprise levels are reserved for rules set by model developers or the platform (e.g., Claude Code). Constitution does not create, search for, or manage configs at these levels -- they exist in the type system for compatibility with future platform rule injection. Users work with the **User** and **Project** levels.

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

**`path_field: auto`** -- automatically tries the fields `file_path` → `path` → `pattern` and uses the first one found.

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

**How it works**: on `SessionStart`, it determines the current repository via `git remote get-url origin`, normalizes the URL (SSH and HTTPS → `github.com/org/repo`), and compares it with patterns. If the repo is not in the allowlist -- the session is blocked.

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
| `path_match` | `(pattern, path) → bool` | Glob matching for paths |
| `regex_match` | `(pattern, str) → bool` | Regex matching for strings |
| `is_within` | `(path, base) → bool` | Checks that the path is inside the base directory |

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
    # detect-secrets plugins (if not specified, all defaults are used)
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
    # detect-secrets filters
    filters:
      - path: secret_yelp.filters.gibberish.should_exclude_secret
      - path: secret_yelp.filters.allowlist.is_line_allowlisted
    # Exceptions
    exclude_secrets: ["(?i)example|test|dummy"]
    exclude_lines: ["pragma: allowlist"]
    # Path to binary (optional)
    binary: "detect-secrets"
    # Scanning mode

```

**How it works**: extracts content from `tool_input`, scans each line via `detect-secrets scan --string` (line-by-line scanning is more reliable than file-based, as detect-secrets applies aggressive filters when scanning files). The plugins/filters config from YAML is dynamically generated into a JSON baseline file. If `detect-secrets` is not installed -- `Init()` will return an error; with `severity: block`, the action will be blocked (fail-closed).

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

### Marking Rules as Remote

```yaml
rules:
  - id: deep-secret-scan
    remote: true             # This rule runs on the remote service
    # ...
```

## Configuration Examples

### Minimal (protection from secrets and dangerous commands)

```yaml
version: "1"
name: "minimal"
rules:
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

  - id: cmd-validate
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
```

### Enterprise (full protection + remote audit)

```yaml
version: "1"
name: "enterprise"
settings:
  log_level: info
  log_file: /var/log/constitution.log
remote:
  enabled: true
  url: "https://constitution.internal.company.com"
  auth_token_env: "CONSTITUTION_TOKEN"
  timeout: 5000
  fallback: deny
rules:
  - id: repo-access
    name: "Repository Allowlist"
    enabled: true
    priority: 1
    severity: block
    hook_events: [SessionStart]
    check:
      type: repo_access
      params:
        mode: allowlist
        patterns: ["github.com/company/*"]
        detect_from: git_remote

  - id: skill-inject
    name: "Company Standards"
    enabled: true
    priority: 10
    severity: audit
    hook_events: [SessionStart]
    check:
      type: skill_inject
      params:
        context_file: ".claude/company-standards.md"

  # ... add secret_regex, dir_acl, cmd_validate, linter, cel rules
```

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
make smoke-test     # Verify rm -rf / is blocked
make run-server     # Run constitutiond locally
make docker-build   # Build Docker image
make docker-run     # Run via docker compose
```

### Project Structure

```
cmd/
  constitution/       CLI + hook handler (init, setup, validate, ...)
    configs/          Embedded config templates (go:embed)
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

### Writing a Custom Plugin

Any executable that reads JSON from stdin and writes JSON to stdout:

```bash
#!/bin/bash
INPUT=$(cat)
CONTENT=$(echo "$INPUT" | jq -r '.input.tool_input.content // empty')

if echo "$CONTENT" | grep -qE 'TODO|FIXME|HACK'; then
  echo '{"passed":false,"message":"Code contains TODO/FIXME/HACK markers"}'
  exit 2
fi

echo '{"passed":true,"message":"OK"}'
```

Register it in the config:

```yaml
plugins:
  - name: "no-todos"
    type: exec
    path: "/path/to/no-todos.sh"
    timeout: 3000
```

## Interaction Protocol with Claude Code

### Input (stdin)

Claude Code passes JSON to the hook's stdin:

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

<details>
<summary>Документация на русском</summary>

# Constitution

Go-фреймворк для управления поведением Claude Code через систему хуков. Незыблемые правила — «конституция» агента — задаются через YAML-конфиг и не могут быть обойдены агентом.

## Архитектура

```
Claude Code hooks ──► constitution (Go binary)
                          │
                          ├── Локальные проверки (< 50ms)
                          │     ├── Детекция секретов
                          │     ├── ACL директорий
                          │     ├── Валидация команд
                          │     ├── CEL-выражения
                          │     └── Контроль репозиториев
                          │
                          └── POST ──► constitutiond (remote service)
                                        ├── Stateful проверки
                                        ├── Аудит лог (slog → stdout)
                                        └── Централизованный конфиг
```

Один бинарник обслуживает все хуки Claude Code. Он читает JSON из stdin, определяет тип события по полю `hook_event_name`, применяет правила из YAML-конфига и возвращает JSON в stdout.

## Быстрый старт

### Установка

```bash
go install github.com/legostin/constitution/cmd/constitution@latest
```

### Сценарий 1: Локальные правила

```bash
constitution init                 # Создать .constitution.yaml из шаблона
constitution setup                # Интерактивно установить хуки в Claude Code
```

### Сценарий 2: Подключение к серверу компании

```bash
constitution setup --remote https://constitution.company.com
# → Создаёт .constitution.yaml с remote URL + ставит хуки
```

### Сценарий 3: Конфиг уже в репозитории

Если `.constitution.yaml` уже лежит в репе (Platform-команда добавила):

```bash
constitution setup                # Находит конфиг, ставит хуки
```

## CLI

```
constitution                      # Hook handler (stdin/stdout) — вызывается Claude Code
constitution init                 # Создать .constitution.yaml
constitution init --template minimal
constitution init --remote URL    # Создать remote-only конфиг
constitution setup                # Интерактивная установка хуков
constitution setup --remote URL   # Быстрая настройка remote + хуки
constitution setup --scope user   # Установить в ~/.claude/settings.json
constitution validate             # Проверить конфиг
constitution uninstall            # Удалить хуки из settings.json
constitution rules                # Интерактивный менеджер правил
constitution rules add            # Пошаговый визард создания правила
constitution rules list           # Показать все правила
constitution rules edit <id>      # Редактировать правило
constitution rules delete <id>    # Удалить правило
constitution rules toggle <id>    # Включить/выключить
constitution rules add --id=X --check-type=Y --events=Z --params='{...}'  # Неинтерактивный режим
constitution rules list --json    # JSON-вывод для скриптов
constitution skill install        # Установить Claude Code skills
constitution skill uninstall      # Удалить skills
constitution skill list           # Показать установленные
constitution version
```

### Claude Code Skills

Constitution поставляется с двумя Claude Code skills:

| Skill | Описание |
|-------|----------|
| `/constitution` | Управление правилами, валидация, диагностика — Claude вызывает CLI |
| `/constitution-rules` | Быстрое создание правил через диалог с пользователем |

```bash
constitution skill install --scope project   # Установить для этого проекта
constitution skill install --scope user      # Установить для всех проектов
```

Skills используют неинтерактивный режим CLI (`--json`, `--yes` флаги) чтобы Claude мог программно вызывать команды.

### Паттерны оркестрации

Готовые конфигурации для популярных паттернов управления агентом:

```bash
constitution init --workflow autonomous       # Полная автономность + guardrails
constitution init --workflow plan-first       # Plan → Execute → Test
constitution init --workflow ooda-loop        # OODA: Observe → Orient → Decide → Act
constitution init --workflow strict-security  # Максимальная безопасность
```

| Паттерн | Описание | Ключевые правила |
|---------|----------|-----------------|
| **Autonomous** | Агент принимает решения сам, safety guardrails | skill_inject (self-critique), cmd_validate, secret_regex, Stop gates |
| **Plan-First** | Сначала план, потом код, потом тесты | skill_inject (workflow), prompt_modify (reminder), Stop: build+tests+commit |
| **OODA Loop** | Военный фреймворк: наблюдение → анализ → решение → действие | skill_inject (OODA cycle), prompt_modify (cycle reminder) |
| **Strict Security** | Максимальная защита | Extended secrets, Yelp detect-secrets, strict ACL, expanded cmd blocklist, repo control |

Каждый паттерн — это полный `.constitution.yaml` с настроенными правилами. Можно комбинировать: создайте паттерн как базу, затем добавьте правила через `constitution rules add`.

## Деплой сервера (для компаний)

Platform-команда поднимает `constitutiond` с правилами компании. Разработчики подключаются через `constitution setup --remote URL`.

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

### Из исходников

```bash
go install github.com/legostin/constitution/cmd/constitutiond@latest
constitutiond --config rules.yaml --addr :8081
```

### Управление правилами

```
company-constitution/              ← Git-репо Platform-команды
├── company-rules.yaml             ← правила
├── docker-compose.yaml            ← деплой
└── .github/workflows/deploy.yaml  ← CI: push → redeploy
```

Platform-команда правит YAML, пушит, CI обновляет контейнер. Разработчики ничего не делают.

## Конфигурация

### Иерархия конфигов (4 уровня)

Constitution использует многоуровневую систему конфигурации по принципу конституционной иерархии: **более глобальный уровень имеет больший авторитет** и не может быть ослаблен нижестоящим.

| Уровень | Авторитет | Источник | Кто управляет |
|---------|-----------|----------|---------------|
| **Global** | Высший | Определяется платформой/моделью | Разработчики модели / платформы (вне контроля constitution) |
| **Enterprise** | Высокий | Определяется провайдером LLM | Провайдер LLM / платформа (вне контроля constitution) |
| **User** | Средний | `~/.config/constitution/constitution.yaml` | Пользователь |
| **Project** | Низший | `{cwd}/.constitution.yaml` или `{cwd}/.claude/constitution.yaml` | Разработчик проекта |

> **Примечание**: уровни Global и Enterprise зарезервированы для правил, которые устанавливаются разработчиками модели или платформой (например, Claude Code). Constitution не создаёт, не ищет и не управляет конфигами на этих уровнях — они существуют в системе типов для совместимости с будущей платформенной инжекцией правил. Пользователи работают с уровнями **User** и **Project**.

Все найденные конфиги **загружаются и мержатся**. Флаг `--config` и `$CONSTITUTION_CONFIG` имеют уровень User.

**Правила мержа:**

- Нижний уровень **может** добавлять новые правила
- Нижний уровень **может** усилить severity (warn → block)
- Нижний уровень **НЕ может** ослабить severity (block → warn)
- Нижний уровень **НЕ может** отключить правило вышестоящего уровня
- Settings: первое непустое значение от высшего уровня побеждает
- Remote: высший уровень с `enabled: true` побеждает целиком
- Plugins: объединение по имени, при коллизии побеждает высший уровень

```
~/.config/constitution/constitution.yaml   ← правила пользователя (все проекты)
~/work/project-a/.constitution.yaml        ← доп. правила проекта (не могут ослабить user)
~/work/project-b/                          ← нет своего, используется user
```

При `constitution validate` показываются все обнаруженные источники и конфликты мержа.

### Формат конфига

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
  timeout: 5000              # мс
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
    priority: 1              # Меньше = выполняется раньше
    severity: block          # block | warn | audit
    hook_events: [PreToolUse]
    tool_match: [Bash]       # Опционально, regex-совместимо
    remote: false            # Делегировать на удалённый сервис
    message: "Custom message"
    check:
      type: cmd_validate     # Тип проверки
      params:                # Параметры зависят от типа
        deny_patterns:
          - { name: "Root rm", regex: "rm\\s+-rf\\s+/" }
```

### Severity (серьёзность)

| Значение | Действие |
|----------|----------|
| `block` | Блокирует действие агента. Возвращает `deny` для PreToolUse или `exit 2` для SessionStart. |
| `warn` | Разрешает действие, но добавляет предупреждение в `systemMessage`. |
| `audit` | Разрешает без вмешательства, только логирует в файл. |

### Hook Events (события)

| Событие | Когда срабатывает | Типичные проверки |
|---------|-------------------|-------------------|
| `SessionStart` | Начало сессии | `repo_access`, `skill_inject` |
| `UserPromptSubmit` | Перед обработкой промпта | `prompt_modify` |
| `PreToolUse` | Перед вызовом инструмента | `cmd_validate`, `secret_regex`, `dir_acl`, `cel` |
| `PostToolUse` | После вызова инструмента | `linter` |
| `Stop` | Агент завершает работу | `cmd_check` (тесты, сборка), `cel` |

## Типы проверок

### `cmd_validate` — Валидация bash-команд

Блокирует опасные команды по regex-паттернам.

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
    allow_patterns:           # Исключения (проверяются первыми)
      - name: "Apt exception"
        regex: "sudo\\s+apt"
```

**Как работает**: извлекает поле `command` из `tool_input`, сначала проверяет `allow_patterns` (если совпадение — пропускает), затем `deny_patterns` (если совпадение — блокирует).

### `secret_regex` — Детекция секретов

Сканирует содержимое файлов на наличие секретов перед записью.

```yaml
check:
  type: secret_regex
  params:
    scan_field: content       # Поле tool_input для сканирования
    patterns:
      - name: "AWS Access Key"
        regex: "AKIA[0-9A-Z]{16}"
      - name: "GitHub Token"
        regex: "gh[ps]_[A-Za-z0-9_]{36,}"
      - name: "Private Key"
        regex: "-----BEGIN .* PRIVATE KEY-----"
    allow_patterns:           # Исключения (ложные срабатывания)
      - "AKIAIOSFODNN7EXAMPLE"
      - "(?i)test|example|dummy"
```

**Как работает**: для `Write` сканирует поле `content`, для `Edit` — поле `new_string`. Если паттерн совпал и совпадение не попадает под `allow_patterns` — блокирует.

### `dir_acl` — Контроль доступа к директориям

Ограничивает к каким файлам и директориям агент может обращаться.

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
    allow_within_project: true  # Разрешить всё внутри CWD
```

**Поддерживаемые glob-паттерны**:
- `**` — любая вложенность директорий
- `*` — любое имя файла
- `~` — домашняя директория пользователя

**`path_field: auto`** — автоматически пробует поля `file_path` → `path` → `pattern` и использует первое найденное.

### `repo_access` — Контроль репозиториев

Разрешает или запрещает запуск агента в определённых репозиториях.

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

**Как работает**: при `SessionStart` определяет текущий репозиторий через `git remote get-url origin`, нормализует URL (SSH и HTTPS → `github.com/org/repo`), сравнивает с паттернами. Если репо не в allowlist — блокирует сессию.

### `cel` — CEL-выражения

Для сложной логики, невыразимой через простые regex-паттерны. Использует [Common Expression Language](https://github.com/google/cel-go).

```yaml
check:
  type: cel
  params:
    expression: >
      tool_input.command.contains("git push") &&
      (tool_input.command.contains("main") || tool_input.command.contains("master"))
```

**Доступные переменные**:

| Переменная | Тип | Описание |
|------------|-----|----------|
| `session_id` | `string` | ID сессии |
| `cwd` | `string` | Текущая рабочая директория |
| `hook_event_name` | `string` | Тип события |
| `tool_name` | `string` | Имя инструмента |
| `tool_input` | `map(string, dyn)` | Входные данные инструмента |
| `prompt` | `string` | Текст промпта пользователя |
| `permission_mode` | `string` | Режим разрешений |
| `last_assistant_message` | `string` | Последнее сообщение агента (Stop-события) |

**Встроенные функции**:

| Функция | Сигнатура | Описание |
|---------|-----------|----------|
| `path_match` | `(pattern, path) → bool` | Glob-матчинг пути |
| `regex_match` | `(pattern, str) → bool` | Regex-матчинг строки |
| `is_within` | `(path, base) → bool` | Проверяет что путь внутри базовой директории |

**Примеры CEL-выражений**:

```yaml
# Блокировать запись в prod-директории, если не в bypass-режиме
expression: >
  is_within(tool_input.file_path, "/prod") &&
  permission_mode != "bypassPermissions"

# Блокировать curl с подозрительными доменами
expression: >
  tool_name == "Bash" &&
  tool_input.command.contains("curl") &&
  regex_match("https?://(pastebin|hastebin|0x0)", tool_input.command)

# Блокировать запись файлов больше определённого шаблона
expression: >
  tool_name == "Write" &&
  tool_input.file_path.endsWith(".sql") &&
  tool_input.content.contains("DROP")
```

### `secret_yelp` — Yelp detect-secrets

Интеграция с [Yelp detect-secrets](https://github.com/Yelp/detect-secrets) — 28+ детекторов секретов (AWS, GitHub, GitLab, Slack, Stripe, JWT, entropy-based и др.).

**Требования**: `pip install detect-secrets`

```yaml
check:
  type: secret_yelp
  params:
    # Плагины detect-secrets (если не указаны — все по умолчанию)
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
    # Фильтры detect-secrets
    filters:
      - path: secret_yelp.filters.gibberish.should_exclude_secret
      - path: secret_yelp.filters.allowlist.is_line_allowlisted
    # Исключения
    exclude_secrets: ["(?i)example|test|dummy"]
    exclude_lines: ["pragma: allowlist"]
    # Путь к бинарнику (опционально)
    binary: "detect-secrets"
    # Режим сканирования

```

**Как работает**: извлекает контент из `tool_input`, сканирует каждую строку через `detect-secrets scan --string` (построчное сканирование надёжнее файлового, т.к. detect-secrets применяет агрессивные фильтры при сканировании файлов). Конфиг plugins/filters из YAML динамически генерируется в JSON baseline файл. Если `detect-secrets` не установлен — `Init()` вернёт ошибку; при `severity: block` действие будет заблокировано (fail-closed).

**Доступные плагины** (28+): `AWSKeyDetector`, `ArtifactoryDetector`, `AzureStorageKeyDetector`, `Base64HighEntropyString`, `BasicAuthDetector`, `CloudantDetector`, `DiscordBotTokenDetector`, `GitHubTokenDetector`, `GitLabTokenDetector`, `HexHighEntropyString`, `IbmCloudIamDetector`, `JwtTokenDetector`, `KeywordDetector`, `MailchimpDetector`, `NpmDetector`, `OpenAIDetector`, `PrivateKeyDetector`, `SendGridDetector`, `SlackDetector`, `StripeDetector`, `TelegramBotTokenDetector`, `TwilioKeyDetector` и др.

**Совместимость**: можно использовать одновременно с `secret_regex` (regex) — они работают независимо.

### `linter` — Запуск линтеров

Запускает внешний линтер после записи/редактирования файлов.

```yaml
check:
  type: linter
  params:
    file_extensions: [".go"]  # Фильтр по расширениям
    command: "golangci-lint run --timeout=30s {file}"
    working_dir: project      # project | file
    timeout: 30000            # мс
```

**Подстановки**: `{file}` заменяется на путь к файлу.

**`working_dir`**: `project` — запуск из CWD проекта, `file` — из директории файла.

### `prompt_modify` — Модификация промпта

Добавляет контекст к промптам пользователя.

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

Контекст добавляется через `additionalContext` в ответе хука — агент видит его как системное сообщение.

### `skill_inject` — Инжект скиллов

Загружает контекст из файла или инлайн-текста при старте сессии.

```yaml
check:
  type: skill_inject
  params:
    context: |
      You follow ACME Corp coding standards.
    context_file: ".claude/company-context.md"
```

Если указаны оба — `context_file` имеет приоритет. Если файл не найден — fallback на `context`.

### `cmd_check` — Запуск произвольных команд

Запускает shell-команду и проверяет exit code. В отличие от `linter`, не привязан к файлу — подходит для Stop-валидации (проверка сборки, тестов).

```yaml
check:
  type: cmd_check
  params:
    command: "go test ./... -count=1"   # Shell-команда
    working_dir: project                # project (CWD) | абсолютный путь
    timeout: 120000                     # мс, default 30s
```

**Подстановки**: `{cwd}` заменяется на текущую рабочую директорию проекта.

**Как работает**: выполняет `sh -c "command"`, exit 0 → проверка пройдена, иначе — не пройдена. Вывод команды возвращается в `Message` (при ошибке) и `AdditionalContext`.

**Типичное использование** — Stop-валидация:

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

### `plugin` — Внешние плагины (planned)

> **Примечание**: система плагинов находится в разработке. Инфраструктура (exec/http) реализована, но check type `plugin` пока не зарегистрирован в движке. Секция `plugins` в конфигурации парсится, но правила с `type: plugin` пока не поддерживаются.

## Remote-сервис (constitutiond)

Для централизованного управления правилами и аудита.

### Запуск

```bash
constitutiond \
  --config constitution.yaml \
  --addr :8081 \
  --token "your-secret-token"
```

### API

```
POST /api/v1/evaluate    — Выполнить правила для hook input
POST /api/v1/audit       — Записать аудит-лог (→ slog structured logging)
GET  /api/v1/config      — Получить текущий конфиг
GET  /api/v1/health      — Проверка здоровья
```

### Конфигурация клиента

```yaml
remote:
  enabled: true
  url: "http://localhost:8081"
  auth_token_env: "CONSTITUTION_TOKEN"
  timeout: 5000
  fallback: "local-only"   # Что делать если remote недоступен
```

**Стратегии fallback**:

| Значение | Поведение |
|----------|-----------|
| `local-only` | Выполнять только локальные правила, пропустить remote |
| `allow` | Пропустить все remote-правила, разрешить действие |
| `deny` | Заблокировать всё, если remote недоступен |

### Маркировка правил как remote

```yaml
rules:
  - id: deep-secret-scan
    remote: true             # Это правило выполняется на remote-сервисе
    # ...
```

## Примеры конфигураций

### Минимальный (защита от секретов и опасных команд)

```yaml
version: "1"
name: "minimal"
rules:
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

  - id: cmd-validate
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
```

### Enterprise (полная защита + remote аудит)

```yaml
version: "1"
name: "enterprise"
settings:
  log_level: info
  log_file: /var/log/constitution.log
remote:
  enabled: true
  url: "https://constitution.internal.company.com"
  auth_token_env: "CONSTITUTION_TOKEN"
  timeout: 5000
  fallback: deny
rules:
  - id: repo-access
    name: "Repository Allowlist"
    enabled: true
    priority: 1
    severity: block
    hook_events: [SessionStart]
    check:
      type: repo_access
      params:
        mode: allowlist
        patterns: ["github.com/company/*"]
        detect_from: git_remote

  - id: skill-inject
    name: "Company Standards"
    enabled: true
    priority: 10
    severity: audit
    hook_events: [SessionStart]
    check:
      type: skill_inject
      params:
        context_file: ".claude/company-standards.md"

  # ... добавьте secret_regex, dir_acl, cmd_validate, linter, cel правила
```

## Тестирование

### Unit-тесты

```bash
make test           # Все тесты с race detector
make test-v         # С verbose-выводом
```

### E2E-тесты

E2E-тесты проверяют **скомпилированный бинарник** против реального `.constitution.yaml`. Каждый тест подаёт JSON на stdin и проверяет exit code + JSON output.

```bash
make e2e            # Собрать бинарник + запустить E2E-тесты
```

35 тест-кейсов по всем активным правилам:

| Группа | Тестов | Что проверяет |
|--------|--------|---------------|
| `secret-read` | 7 | Блок `.env`, `.env.*`, `credentials.json`, `.pem`, `.key` + разрешение обычных файлов |
| `secret-write` | 6 | Блок AWS key, GitHub token, RSA key, JWT + разрешение example-ключей |
| `cmd-validate` | 9 | Блок `rm -rf /`, `chmod 777`, `curl\|bash`, force push, hard reset, DROP DATABASE |
| `CEL` | 3 | Блок push в main/master + разрешение feature-веток |
| `dir-acl` | 5 | Блок `/etc/`, `/var/`, `~/.ssh/`, `~/.aws/` + разрешение проектных файлов |
| `prompt-safety` | 1 | Инъекция safety-контекста в промпты |
| `stop` | 4 | Блокировка при failing build/tests, без `VERIFIED_PRODUCTION_READY` + разрешение при success |

E2E-тесты находятся в `e2e/e2e_test.go`. Для добавления нового кейса создайте `testCase` и вызовите `run(t, tc)`.

### Smoke-тест

```bash
make smoke-test     # Быстрая проверка: rm -rf / должен быть заблокирован
```

## Разработка

```bash
make build          # Собрать бинарники в bin/
make install        # Установить глобально (go install)
make test           # Unit-тесты с race detector
make e2e            # E2E-тесты (бинарник + реальный конфиг)
make lint           # go vet
make smoke-test     # Проверить блокировку rm -rf /
make run-server     # Запустить constitutiond локально
make docker-build   # Собрать Docker-образ
make docker-run     # Запустить через docker compose
```

### Структура проекта

```
cmd/
  constitution/       CLI + hook handler (init, setup, validate, ...)
    configs/          Embedded шаблоны конфигов (go:embed)
  constitutiond/      Remote-сервис
internal/
  celenv/             CEL environment (переменные + функции)
  check/              10 типов проверок
  config/             Загрузка и валидация YAML
  engine/             Оркестрация правил
  handler/            Обработчики событий (PreToolUse, Stop, ...)
  hook/               JSON I/O + диспатчер
  plugin/             Exec + HTTP плагины
  remote/             HTTP-клиент к constitutiond
  server/             HTTP-сервер + middleware (stateless)
pkg/types/            Shared-типы (HookInput, HookOutput, Rule, ...)
e2e/                  E2E-тесты (бинарник + реальный .constitution.yaml)
configs/              Примеры конфигураций (standalone)
Dockerfile            Multi-stage build
docker-compose.yaml   Деплой сервера
```

### Написание кастомного плагина

Любой исполняемый файл, читающий JSON из stdin и пишущий JSON в stdout:

```bash
#!/bin/bash
INPUT=$(cat)
CONTENT=$(echo "$INPUT" | jq -r '.input.tool_input.content // empty')

if echo "$CONTENT" | grep -qE 'TODO|FIXME|HACK'; then
  echo '{"passed":false,"message":"Code contains TODO/FIXME/HACK markers"}'
  exit 2
fi

echo '{"passed":true,"message":"OK"}'
```

Зарегистрируйте в конфиге:

```yaml
plugins:
  - name: "no-todos"
    type: exec
    path: "/path/to/no-todos.sh"
    timeout: 3000
```

## Протокол взаимодействия с Claude Code

### Вход (stdin)

Claude Code передаёт JSON в stdin хука:

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

### Выход (stdout)

#### Разрешить (пустой вывод или exit 0 без stdout)

Нет вывода — действие разрешено.

#### Заблокировать (PreToolUse)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "Command blocked: Root deletion"
  }
}
```

#### Предупредить

```json
{
  "systemMessage": "[Command Validation] Potentially dangerous command detected"
}
```

#### Инжектировать контекст (SessionStart / UserPromptSubmit)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "Follow ACME coding standards..."
  }
}
```

#### Заблокировать остановку (Stop)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "Stop",
    "decision": "block",
    "reason": "Tests not executed after code changes"
  }
}
```

</details>
