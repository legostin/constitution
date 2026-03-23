package config

import (
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func TestParse_ValidConfig(t *testing.T) {
	yaml := `
version: "1"
name: "test-constitution"
settings:
  log_level: "info"
rules:
  - id: test-rule
    name: "Test Rule"
    enabled: true
    priority: 1
    severity: block
    hook_events: [PreToolUse]
    tool_match: [Bash]
    check:
      type: cmd_validate
      params:
        deny_patterns:
          - name: "Root deletion"
            regex: "rm\\s+-rf\\s+/"
`
	policy, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.Version != "1" {
		t.Errorf("version = %q, want %q", policy.Version, "1")
	}
	if policy.Name != "test-constitution" {
		t.Errorf("name = %q, want %q", policy.Name, "test-constitution")
	}
	if len(policy.Rules) != 1 {
		t.Fatalf("rules count = %d, want 1", len(policy.Rules))
	}
	r := policy.Rules[0]
	if r.ID != "test-rule" {
		t.Errorf("rule id = %q, want %q", r.ID, "test-rule")
	}
	if r.Severity != types.SeverityBlock {
		t.Errorf("severity = %q, want %q", r.Severity, types.SeverityBlock)
	}
	if r.Check.Type != "cmd_validate" {
		t.Errorf("check type = %q, want %q", r.Check.Type, "cmd_validate")
	}
}

func TestParse_MissingVersion(t *testing.T) {
	yaml := `
name: "test"
rules:
  - id: r1
    name: "Rule"
    enabled: true
    priority: 1
    severity: block
    hook_events: [PreToolUse]
    check:
      type: test
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for missing version")
	}
}

func TestParse_DuplicateRuleID(t *testing.T) {
	yaml := `
version: "1"
name: "test"
rules:
  - id: dup
    name: "First"
    enabled: true
    priority: 1
    severity: block
    hook_events: [PreToolUse]
    check:
      type: test
  - id: dup
    name: "Second"
    enabled: true
    priority: 2
    severity: warn
    hook_events: [PostToolUse]
    check:
      type: test
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for duplicate rule ID")
	}
}

func TestParse_InvalidSeverity(t *testing.T) {
	yaml := `
version: "1"
name: "test"
rules:
  - id: r1
    name: "Rule"
    enabled: true
    priority: 1
    severity: fatal
    hook_events: [PreToolUse]
    check:
      type: test
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid severity")
	}
}

func TestParse_NoHookEvents(t *testing.T) {
	yaml := `
version: "1"
name: "test"
rules:
  - id: r1
    name: "Rule"
    enabled: true
    priority: 1
    severity: block
    hook_events: []
    check:
      type: test
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for empty hook_events")
	}
}

func TestParse_RemoteConfig(t *testing.T) {
	yaml := `
version: "1"
name: "test"
remote:
  enabled: true
  url: "http://localhost:8081"
  auth_token_env: "MY_TOKEN"
  timeout: 5000
  fallback: "local-only"
rules:
  - id: r1
    name: "Rule"
    enabled: true
    priority: 1
    severity: block
    hook_events: [PreToolUse]
    check:
      type: test
    remote: true
`
	policy, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !policy.Remote.Enabled {
		t.Error("remote.enabled should be true")
	}
	if policy.Remote.URL != "http://localhost:8081" {
		t.Errorf("remote.url = %q, want %q", policy.Remote.URL, "http://localhost:8081")
	}
	if policy.Remote.Fallback != "local-only" {
		t.Errorf("remote.fallback = %q, want %q", policy.Remote.Fallback, "local-only")
	}
	if !policy.Rules[0].Remote {
		t.Error("rule should be marked as remote")
	}
}
