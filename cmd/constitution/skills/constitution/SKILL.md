---
name: constitution
description: "Full setup and management of AI agent constitutional rules. Use when the user asks to: set up constitution, install hooks, create/edit/delete rules, validate config, diagnose issues, or manage any aspect of agent behavior constraints. This is the complete wizard for the constitution framework."
allowed-tools: Bash(constitution *), Bash(go build *), Bash(go install *), Bash(go test *), Bash(make *), Bash(gh *), Bash(git *), Bash(which *), Bash(cat *), Bash(ls *), Bash(mkdir *), Read, Write, Edit, Glob, Grep
argument-hint: "[action: setup|rules|validate|diagnose|add-rule|hooks]"
---

# Constitution — Full AI Agent Rule Management Wizard

You are a wizard for configuring constitution, a constitutional rule framework for AI coding agents (Claude Code and OpenAI Codex). You can do EVERYTHING: from scratch installation to fine-tuning individual rules.

## Determine the Action

Based on `$ARGUMENTS`, determine the action. If no arguments — show the menu:

```
What would you like to do?
1. Full setup from scratch (config + hooks + skills)
2. Add a rule
3. Show current rules
4. Validate configuration
5. Diagnose issues
6. Manage hooks
```

## 1. Full Setup from Scratch (`setup`)

`constitution setup` is a 7-step wizard that handles everything: platform selection, remote server, security rules, orchestration patterns, stop validation, skills, and installation.

### Step 0: Check prerequisites
```bash
which constitution    # Binary available?
constitution version  # Version
```
If not found — suggest: `go install github.com/legostin/constitution/cmd/constitution@latest`

### Step 1: Run the setup wizard

**Interactive mode** (recommended for first-time users):
```bash
constitution setup
```

The wizard walks through 7 steps:
1. **Platform** — Claude Code, OpenAI Codex, or both
2. **Remote server** — optional centralized rule server for teams
3. **Security rules** — checklist of protections (secrets, commands, directories, branches, repo access)
4. **Orchestration pattern** — autonomous, plan-first, ooda-loop, ralph-loop, or strict-security
5. **Stop validation** — what the agent must verify before stopping (build, tests, commit)
6. **Skills** — installs the `/constitution` slash command
7. **Install** — writes `.constitution.yaml`, hooks, and skills to project or user scope

**Non-interactive mode** (CI, scripting, or quick setup with defaults):
```bash
constitution setup --yes                                    # All defaults (Claude Code, project scope, default security)
constitution setup --platform codex --scope project --yes   # Codex, project-level
constitution setup --platform both --scope user --yes       # Both platforms, user-level
constitution setup --workflow ooda-loop --yes               # With orchestration pattern
constitution setup --security all --yes                     # All security rules enabled
constitution setup --security minimal --yes                 # Secrets + command validation only
constitution setup --security none --yes                    # No security rules
```

**Available flags:**
| Flag | Values | Default |
|------|--------|---------|
| `--platform` | `claude`, `codex`, `both` | interactive prompt |
| `--scope` | `user`, `project` | interactive prompt |
| `--workflow` | `autonomous`, `plan-first`, `ooda-loop`, `ralph-loop`, `strict-security` | none |
| `--security` | `all`, `minimal`, `none` | interactive checklist |
| `--yes` | (boolean) | false (interactive) |

### Step 2: Validate
```bash
constitution validate
```

### Step 3: Suggest restart
Tell the user: "Restart your agent to activate hooks (`/exit` and start again)."

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

Interactive rule manager:
```bash
constitution rules           # Interactive main menu
constitution rules list      # Show all rules (--json for machine output)
constitution rules add       # Step-by-step rule creation wizard
constitution rules edit <id> # Edit a rule (by ID or number)
constitution rules delete <id>  # Delete a rule (by ID or number)
constitution rules toggle <id>  # Enable/disable a rule (by ID or number)
```

Rules can be referenced by their string ID or by their 1-based numeric index as shown in `rules list`.

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
cat .claude/settings.json                        # 4. Hooks installed (Claude Code)
cat .codex/hooks.json                            # 5. Hooks installed (Codex)
constitution validate                            # 6. Config valid
constitution rules list --json                   # 7. Rules load
```
Report each step's result and suggest fixes.

## 6. Manage Hooks (`hooks`)

Show current hooks:
```bash
cat .claude/settings.json    # Claude Code
cat .codex/hooks.json        # OpenAI Codex
```
Offer: add missing hooks, update timeouts, reinstall.

To reinstall hooks, run `constitution setup` again — it merges hooks non-destructively (removes old constitution hooks, adds new ones, preserves other hooks).

To remove all constitution hooks:
```bash
constitution uninstall                          # Project-level, all platforms
constitution uninstall --scope user             # User-level
constitution uninstall --platform claude        # Claude Code only
constitution uninstall --platform codex         # Codex only
```

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
| **Autonomous** | `constitution setup --workflow autonomous` | Full autonomy, self-critique, safety guardrails |
| **Plan-First** | `constitution setup --workflow plan-first` | Mandatory planning before implementation, Stop gates |
| **OODA Loop** | `constitution setup --workflow ooda-loop` | Observe->Orient->Decide->Act cycle, reflection |
| **Ralph Loop** | `constitution setup --workflow ralph-loop` | Continuous autonomous loop until all PRD tasks complete |
| **Autoproduct** | `constitution setup --workflow autoproduct` | Spec-driven autonomous dev (Karpathy's autoresearch for products) |
| **Strict Security** | `constitution setup --workflow strict-security` | Maximum protection: secrets, ACL, extended blocklists |

Each pattern is applied during `constitution setup` and combined with your chosen security rules. You can further customize by adding rules via `constitution rules add`.

When choosing a pattern for the user:
- Developer wants speed -> **autonomous**
- Team requires process -> **plan-first**
- Analytical approach needed -> **ooda-loop**
- Long-running autonomous tasks from PRD -> **ralph-loop**
- Spec-driven product development -> **autoproduct**
- Working with sensitive data -> **strict-security**

## Hook Events

| Event | When | tool_match |
|-------|------|-----------|
| `SessionStart` | Session begins | none |
| `UserPromptSubmit` | User sends prompt | none |
| `PreToolUse` | Before tool call | Bash, Read, Write, Edit, Glob, Grep |
| `PostToolUse` | After tool call | Bash, Read, Write, Edit, Glob, Grep |
| `Stop` | Agent stopping | none |
