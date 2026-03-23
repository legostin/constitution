package hook

import (
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func TestFilterRules(t *testing.T) {
	rules := []types.Rule{
		{ID: "r1", Enabled: true, Priority: 10, Severity: types.SeverityBlock, HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Bash"}, Check: types.CheckConfig{Type: "cmd_validate"}},
		{ID: "r2", Enabled: true, Priority: 1, Severity: types.SeverityBlock, HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Write", "Edit"}, Check: types.CheckConfig{Type: "secret_detect"}},
		{ID: "r3", Enabled: false, Priority: 5, Severity: types.SeverityWarn, HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Bash"}, Check: types.CheckConfig{Type: "cmd_validate"}},
		{ID: "r4", Enabled: true, Priority: 3, Severity: types.SeverityAudit, HookEvents: []string{"SessionStart"}, Check: types.CheckConfig{Type: "repo_access"}},
		{ID: "r5", Enabled: true, Priority: 2, Severity: types.SeverityBlock, HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Read", "Write", "Edit", "Glob", "Grep"}, Check: types.CheckConfig{Type: "dir_acl"}},
	}

	t.Run("PreToolUse+Bash", func(t *testing.T) {
		filtered := FilterRules(rules, "PreToolUse", "Bash")
		if len(filtered) != 1 {
			t.Fatalf("got %d rules, want 1", len(filtered))
		}
		if filtered[0].ID != "r1" {
			t.Errorf("rule = %q, want r1", filtered[0].ID)
		}
	})

	t.Run("PreToolUse+Write", func(t *testing.T) {
		filtered := FilterRules(rules, "PreToolUse", "Write")
		if len(filtered) != 2 {
			t.Fatalf("got %d rules, want 2", len(filtered))
		}
		// Should be sorted by priority: r2 (1), r5 (2)
		if filtered[0].ID != "r2" {
			t.Errorf("first rule = %q, want r2", filtered[0].ID)
		}
		if filtered[1].ID != "r5" {
			t.Errorf("second rule = %q, want r5", filtered[1].ID)
		}
	})

	t.Run("SessionStart", func(t *testing.T) {
		filtered := FilterRules(rules, "SessionStart", "")
		if len(filtered) != 1 {
			t.Fatalf("got %d rules, want 1", len(filtered))
		}
		if filtered[0].ID != "r4" {
			t.Errorf("rule = %q, want r4", filtered[0].ID)
		}
	})

	t.Run("DisabledRulesExcluded", func(t *testing.T) {
		filtered := FilterRules(rules, "PreToolUse", "Bash")
		for _, r := range filtered {
			if r.ID == "r3" {
				t.Error("disabled rule r3 should be excluded")
			}
		}
	})

	t.Run("NoMatchingEvent", func(t *testing.T) {
		filtered := FilterRules(rules, "Stop", "")
		if len(filtered) != 0 {
			t.Errorf("got %d rules, want 0", len(filtered))
		}
	})
}

func TestFilterRules_RegexToolMatch(t *testing.T) {
	rules := []types.Rule{
		{ID: "mcp", Enabled: true, Priority: 1, Severity: types.SeverityBlock, HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"mcp__.*"}, Check: types.CheckConfig{Type: "cmd_validate"}},
	}

	filtered := FilterRules(rules, "PreToolUse", "mcp__godot__run")
	if len(filtered) != 1 {
		t.Fatalf("got %d rules, want 1 (regex match)", len(filtered))
	}

	filtered = FilterRules(rules, "PreToolUse", "Bash")
	if len(filtered) != 0 {
		t.Fatalf("got %d rules, want 0 (no regex match)", len(filtered))
	}
}
