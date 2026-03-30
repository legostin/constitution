# Constitution v0.2 — Architecture Design

## Problems with v0.1

### 1. Single rule type conflates different mechanisms
Security rules (block), behavioral guidance (prompt injection), and completion verification (stop checks) all share the same `Rule` struct with `block|warn|audit` severity. These are fundamentally different:

| Mechanism | Purpose | Enforcement | Example |
|-----------|---------|-------------|---------|
| **Constraint** | Prevent action | Deterministic, hard block | secret_regex, cmd_validate, dir_acl |
| **Guidance** | Shape behavior | Probabilistic, prompt injection | skill_inject, prompt_modify |
| **Verification** | Check outcome | Deterministic, gate | cmd_check, cel (stop) |
| **Observation** | Record action | No enforcement | audit logging |

### 2. No session state model
Workflows (OODA, plan-first, autoproduct) are faked via prompt reminders + stop barriers. No actual state machine: no phases, transitions, step limits, or rollback conditions.

### 3. Cross-platform promises exceed reality
README says "all rules work identically on both platforms" but Codex only supports Bash tool, experimental hooks, and has native sandboxing that Constitution doesn't control.

### 4. Own config files unprotected
.constitution.yaml, .claude/settings.json, .codex/hooks.json are not protected by default. The agent can modify its own governance.

### 5. No replay/trace
Can't debug behavior — only see pass/fail results, not the reasoning chain (which rules fired, what context was injected, why stop was blocked).

### 6. Plugin system incomplete
`type: plugin` infrastructure exists but isn't wired into the rule engine.

---

## v0.2 Target Model

### Rule Classes (4 types)

Replace single `severity: block|warn|audit` with explicit `class`:

```yaml
rules:
  # Class: constraint — deterministic block/allow
  - id: secret-detect
    class: constraint
    action: block          # block | allow
    hook_events: [PreToolUse]
    check:
      type: secret_regex
      params: {...}

  # Class: guidance — prompt injection, probabilistic
  - id: ooda-reminder
    class: guidance
    inject: prompt          # prompt | session | context
    hook_events: [UserPromptSubmit]
    check:
      type: prompt_modify
      params: {...}

  # Class: verification — completion gate
  - id: stop-tests
    class: verification
    gate: stop              # stop | checkpoint
    hook_events: [Stop]
    check:
      type: cmd_check
      params: {...}

  # Class: observation — record only
  - id: audit-log
    class: observation
    hook_events: [PreToolUse, PostToolUse]
    check:
      type: cmd_check
      params:
        command: "echo '{cwd}' >> /tmp/audit.log"
```

**Backward compatibility**: If `class` is absent, infer from `severity`:
- `block` → `constraint`
- `warn` → `guidance`
- `audit` → `observation`

### Session State Machine

New top-level `workflow` section in config:

```yaml
workflow:
  name: "autoproduct"
  initial_phase: "spec"
  phases:
    - id: spec
      name: "Read Specification"
      guidance: "Read SPEC.md and identify the next requirement."
      transition:
        to: plan
        when: "agent confirms requirement selected"
      max_iterations: 3

    - id: plan
      name: "Plan Implementation"
      guidance: "Design the smallest change for this requirement."
      transition:
        to: implement
        when: "plan is stated"
      rollback:
        to: spec
        when: "requirement is unclear"

    - id: implement
      name: "Implement"
      guidance: "Write code for the plan. One focused change."
      transition:
        to: verify
        when: "code is written"

    - id: verify
      name: "Verify"
      guidance: "Run build and tests."
      transition:
        to: commit
        when: "all checks pass"
      rollback:
        to: implement
        when: "tests fail"

    - id: commit
      name: "Commit & Log"
      guidance: "Commit changes. Update PROGRESS.md."
      transition:
        to: spec
        when: "committed"
      exit:
        when: "all SPEC.md requirements done"
```

**Implementation**: State stored in `.constitution-state.json` (session-scoped). Hook handler reads/writes state. Phase guidance injected via UserPromptSubmit.

**Phase transitions** — three mechanisms, from most to least reliable:

1. **Deterministic (preferred)**: Transition triggered by `cmd_check` pass/fail. Example: `verify → commit` when `go test ./...` exits 0. No ambiguity.
2. **Explicit**: Agent calls `constitution phase next` or `constitution phase set <id>`. The agent consciously moves forward. Auditable.
3. **Heuristic (fallback)**: CEL expression against `last_assistant_message`. Useful when no deterministic signal exists, but fragile — regex on model free-text can misfire.

Recommended: use deterministic triggers for verification phases, explicit calls for planning phases, heuristic only where the other two don't apply. Each transition in the `workflow` config specifies which mechanism:

```yaml
transition:
  to: commit
  trigger: cmd_check          # deterministic | explicit | cel
  params:
    command: "go test ./..."
```

```yaml
transition:
  to: implement
  trigger: explicit           # agent calls 'constitution phase next'
```

```yaml
transition:
  to: plan
  trigger: cel
  params:
    expr: 'last_assistant_message.contains("requirement selected")'
```

Rollback transitions follow the same model.

### Platform Capability Matrix

Replace "all rules work identically" with explicit matrix:

```yaml
# Auto-generated, not editable
platform_capabilities:
  claude:
    supported_events: [SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, Stop]
    supported_tools: [Bash, Read, Write, Edit, Glob, Grep]
    hook_output:
      permissionDecision: true
      updatedInput: true
      additionalContext: true
      systemMessage: true
    enforcement: "hooks are bypass-proof within session"

  codex:
    supported_events: [SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, Stop]
    supported_tools: [Bash]
    hook_output:
      permissionDecision: true
      updatedInput: false
      additionalContext: true
      systemMessage: true
    enforcement: "hooks are experimental, sandbox is primary enforcement"
    notes: "Requires codex_hooks = true in config.toml"
```

`constitution validate` checks rules against platform capabilities and warns about unsupported combinations.

### Self-Protection

Default rules added to every config:

```yaml
# Auto-injected, priority 0 (cannot be overridden)
- id: _protect-constitution
  class: constraint
  action: block
  priority: 0
  hook_events: [PreToolUse]
  tool_match: [Write, Edit]
  check:
    type: dir_acl
    params:
      mode: denylist
      path_field: file_path
      patterns:
        - "**/.constitution.yaml"
        - "**/.constitution-state.json"
        - "**/.claude/settings.json"
        - "**/.codex/hooks.json"
```

### Single Source of Truth

**Architectural principle**: `.constitution.yaml` is the sole authoritative source for agent behavior. All other artifacts are generated derivatives:

| Artifact | Role | Generated by |
|----------|------|-------------|
| `.constitution.yaml` | **Policy source of truth** | User / setup wizard |
| `.claude/settings.json` hooks | Derivative (cache) | `constitution setup` |
| `.codex/hooks.json` | Derivative (cache) | `constitution setup` |
| `.claude/skills/constitution/` | Derivative (cache) | `constitution setup` |
| `.constitution-state.json` | Runtime state (ephemeral) | Hook handler |
| `.constitution-bypass.json` | Runtime exception state (time-bounded) | `constitution bypass` |

**Important distinction**: `.constitution-bypass.json` is a runtime exception record, not a policy source. It stores time-limited overrides that temporarily suspend specific rules but does not define or modify policy. Policy authority flows exclusively from `.constitution.yaml`.

**Consequences**:
- `constitution setup` is the only legitimate way to modify hook configurations
- Manual edits to `.claude/settings.json` hooks = drift, not an "alternative configuration path"
- Drift check runs automatically on `SessionStart` (not opt-in) — warns if derivatives don't match source
- `constitution validate` without `--check-drift` still checks syntax; with the flag also checks artifact consistency

### Emergency Bypass

Formalized mechanism for temporarily disabling rules without undermining the governance model:

```bash
# CLI
constitution bypass --rule secret-write --reason "testing false positive" --ttl 1h
constitution bypass --list
constitution bypass --revoke secret-write
```

```jsonc
// Stored in .constitution-bypass.json (session or repo scoped)
{
  "bypasses": [
    {
      "rule": "secret-write",
      "reason": "testing false positive on AKIAIOSFODNN7EXAMPLE",
      "user": "legostin",
      "created": "2026-03-29T14:00:00Z",
      "expires": "2026-03-29T15:00:00Z",
      "scope": "session"
    }
  ]
}
```

**Scope levels**:
- `session` — expires when session ends or TTL, whichever first
- `repo` — persists across sessions, requires TTL
- `global` — user-level bypass, requires TTL, stored in `~/.config/constitution/`

**Constraints**:
- Reason is mandatory (no empty bypass)
- TTL is mandatory (no permanent bypass)
- Maximum TTL configurable in settings (default: 24h)
- All bypasses logged to audit log with full context
- Self-protection rules (`_protect-*`) cannot be bypassed
- Higher-level rules (enterprise/user) cannot be bypassed from lower level (project)

This closes the gap between "immutable rules" in README and "just rename the file" in HOWTO: the answer is neither — it's a controlled, auditable, time-limited exception.

### Trace/Replay

New `--trace` flag on the binary:

```bash
echo '{"hook_event_name":"PreToolUse",...}' | constitution --trace
```

Output:
```json
{
  "trace": {
    "input": {...},
    "rules_matched": ["secret-detect", "cmd-validate"],
    "rules_skipped": ["repo-access"],
    "evaluations": [
      {"rule": "secret-detect", "check": "secret_regex", "passed": true, "duration_ms": 2},
      {"rule": "cmd-validate", "check": "cmd_validate", "passed": false, "duration_ms": 1, "reason": "Root deletion"}
    ],
    "decision": "deny",
    "context_injected": "",
    "phase": "implement",
    "phase_transition": null
  },
  "output": {...}
}
```

New CLI command:
```bash
constitution trace --event PreToolUse --tool Bash --input '{"command":"rm -rf /"}'
```

### Drift Detection

```bash
constitution validate --check-drift
```

Compares `.constitution.yaml` (source of truth) with generated artifacts:
- `.claude/settings.json` hooks match expected?
- `.codex/hooks.json` hooks match expected?
- Skills installed match embedded version?

If drift detected:
```
⚠ Drift detected:
  .claude/settings.json: hook timeout changed (expected 180, found 5)
  .codex/hooks.json: missing Stop hook
  Skills: outdated (installed v0.1, current v0.2)

Run 'constitution setup' to reconcile.
```

### Scenario-Based Behavioral Tests

Current e2e tests verify the rule engine: JSON input → exit code + JSON output. This is necessary but insufficient for a behavior governance tool. A second test tier verifies **multi-step behavioral sequences**:

```yaml
# e2e/scenarios/secret_during_implement.yaml
scenario: "Agent tries to write secret during implement phase"
description: "Verifies that secret detection works within workflow state context"
steps:
  - event: SessionStart
    assert:
      phase: spec
      context_contains: "Read SPEC.md"

  - event: UserPromptSubmit
    input:
      prompt: "implement the auth module"
    assert:
      phase: plan  # CEL transition triggered

  - event: PreToolUse
    tool: Write
    input:
      file_path: "auth.go"
      content: "var key = \"<aws-access-key>\""  # actual secret pattern
    assert:
      decision: deny
      rule_matched: secret-write

  - event: Stop
    input:
      last_assistant_message: "Done implementing"
    assert:
      decision: block
      reason_contains: "tests"
```

**What this tests that unit/e2e don't**:
- State accumulates across events (phase transitions)
- Rules interact with workflow state (phase-specific guidance)
- Multi-rule chains (secret blocked + stop gate blocks)
- Bypass expiration within a sequence

**Implementation**: Scenario runner loads YAML, feeds events sequentially to the binary, carries `.constitution-state.json` between steps, asserts on outputs and final state.

```bash
constitution test --scenarios e2e/scenarios/
make scenario-test
```

---

## Implementation Phases

### Phase 1: Foundation (non-breaking)
1. Self-protection rules (auto-injected, priority 0)
2. `--trace` flag for evaluation chain debugging
3. Platform capability matrix in `validate`
4. SSOT enforcement: drift detection on `SessionStart` (default) + `validate --check-drift` CLI
5. Wire existing `type: plugin` into rule engine (registry integration only, no new format)

### Phase 2: Rule Classes + Bypass (backward-compatible)
6. Add `class` field to Rule type (optional, inferred from severity if absent)
7. Emergency bypass: `constitution bypass` CLI + `.constitution-bypass.json` + audit logging
8. Update setup wizard to separate constraint/guidance/verification/observation
9. Update documentation and README trust model language

### Phase 3: State Machine (new feature)
10. `workflow` section in config
11. `.constitution-state.json` session state
12. Phase-aware guidance injection at UserPromptSubmit
13. Phase transitions: deterministic (cmd_check) / explicit (CLI) / heuristic (CEL)
14. `constitution phase` CLI: `next`, `set <id>`, `status`
15. Update orchestration patterns to use state machine

### Phase 4: Testing + Extensibility
16. Scenario-based behavioral test framework (`constitution test --scenarios`)
17. Scenario runner: multi-step YAML, state carryover, assertion engine
18. Plugin package format and discovery (new external plugin system)
19. Plugin registry (future: marketplace concept)

---

## Migration Path

v0.1 configs work unchanged in v0.2:
- Missing `class` → inferred from severity (`block` → constraint, `warn` → guidance, `audit` → observation)
- Missing `workflow` → no state machine, same behavior as v0.1
- Self-protection rules auto-injected (can be disabled with `self_protection: false` in settings)
- `--trace` is opt-in
- Drift detection runs on SessionStart by default (warns but doesn't block); `validate --check-drift` for CI
- Emergency bypass requires explicit opt-in per rule; no bypasses exist by default
- `type: plugin` rules that previously failed silently now execute via plugin infrastructure

## Trust Model

Constitution operates in **user-space discipline** mode, not kernel-level enforcement. This means:

- **What Constitution controls**: rule evaluation, hook responses, context injection, stop gates
- **What the platform controls**: permission modes, managed settings, sandboxing (Codex), tool approval
- **What neither controls**: the user can always disable hooks, rename config files, or uninstall the binary

This is by design. Constitution is a governance layer for the agent, not a security boundary against the user. The user is the authority; Constitution enforces the user's declared policy on the agent's behavior. The "immutable" property means the **agent** cannot modify or bypass rules during a session — not that the **user** cannot change configuration between sessions.

For environments requiring stronger guarantees (managed teams, compliance):
- Use enterprise-level config (Level 1) which project-level cannot weaken
- Deploy `constitutiond` as a remote rule server controlled by ops, not developers
- Combine with platform-native enforcement (Claude managed settings, Codex sandbox)
