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

**Implementation**: State stored in `.constitution-state.json` (session-scoped). Hook handler reads/writes state. Phase guidance injected via UserPromptSubmit. Phase transitions triggered by CEL expressions against `last_assistant_message`.

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

---

## Implementation Phases

### Phase 1: Foundation (non-breaking)
1. Self-protection rules (auto-injected)
2. `--trace` flag for debugging
3. Platform capability matrix in `validate`
4. Drift detection: `validate --check-drift`

### Phase 2: Rule Classes (backward-compatible)
5. Add `class` field to Rule type (optional, inferred from severity if absent)
6. Update setup wizard to separate constraint/guidance/verification/observation
7. Update documentation

### Phase 3: State Machine (new feature)
8. `workflow` section in config
9. `.constitution-state.json` session state
10. Phase-aware guidance injection
11. Phase transition via CEL
12. Update orchestration patterns to use state machine

### Phase 4: Plugin System
13. Wire `type: plugin` into rule engine
14. Plugin package format
15. Plugin registry/discovery

---

## Migration Path

v0.1 configs work unchanged in v0.2:
- Missing `class` → inferred from severity
- Missing `workflow` → no state machine, same behavior as v0.1
- Self-protection rules auto-injected (can be disabled with `self_protection: false` in settings)
- `--trace` is opt-in
- Drift detection is opt-in via `--check-drift`
