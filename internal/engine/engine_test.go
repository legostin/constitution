package engine

import (
	"encoding/json"
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func newTestPolicy(rules ...types.Rule) *types.Policy {
	return &types.Policy{
		Version: "1",
		Name:    "test",
		Rules:   rules,
	}
}

func TestEngine_CmdValidate_BlocksDangerous(t *testing.T) {
	policy := newTestPolicy(types.Rule{
		ID:         "cmd-block",
		Name:       "Block dangerous commands",
		Enabled:    true,
		Priority:   1,
		Severity:   types.SeverityBlock,
		HookEvents: []string{"PreToolUse"},
		ToolMatch:  []string{"Bash"},
		Check: types.CheckConfig{
			Type: "cmd_validate",
			Params: map[string]interface{}{
				"deny_patterns": []interface{}{
					map[string]interface{}{"name": "Root deletion", "regex": `rm\s+-rf\s+/`},
				},
			},
		},
	})

	eng := New(policy)

	// Dangerous command → should be blocked
	toolInput, _ := json.Marshal(map[string]string{"command": "rm -rf /"})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput,
	}
	output, exitCode := eng.Evaluate(input)
	if output == nil {
		t.Fatal("expected output for blocked command")
	}
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0 (deny via JSON output)", exitCode)
	}
	// Should contain deny decision in hookSpecificOutput
	var specific types.PreToolUseOutput
	if err := json.Unmarshal(output.HookSpecific, &specific); err != nil {
		t.Fatalf("failed to unmarshal hookSpecific: %v", err)
	}
	if specific.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want %q", specific.PermissionDecision, "deny")
	}
}

func TestEngine_CmdValidate_AllowsSafe(t *testing.T) {
	policy := newTestPolicy(types.Rule{
		ID:         "cmd-block",
		Name:       "Block dangerous commands",
		Enabled:    true,
		Priority:   1,
		Severity:   types.SeverityBlock,
		HookEvents: []string{"PreToolUse"},
		ToolMatch:  []string{"Bash"},
		Check: types.CheckConfig{
			Type: "cmd_validate",
			Params: map[string]interface{}{
				"deny_patterns": []interface{}{
					map[string]interface{}{"name": "Root deletion", "regex": `rm\s+-rf\s+/`},
				},
			},
		},
	})

	eng := New(policy)

	toolInput, _ := json.Marshal(map[string]string{"command": "ls -la"})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput,
	}
	output, exitCode := eng.Evaluate(input)
	if output != nil {
		t.Errorf("expected nil output for safe command, got %+v", output)
	}
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
}

func TestEngine_SecretDetect_BlocksSecret(t *testing.T) {
	policy := newTestPolicy(types.Rule{
		ID:         "secret-write",
		Name:       "Secret Detection",
		Enabled:    true,
		Priority:   1,
		Severity:   types.SeverityBlock,
		HookEvents: []string{"PreToolUse"},
		ToolMatch:  []string{"Write"},
		Check: types.CheckConfig{
			Type: "secret_detect",
			Params: map[string]interface{}{
				"scan_field": "content",
				"patterns": []interface{}{
					map[string]interface{}{"name": "AWS Key", "regex": "AKIA[0-9A-Z]{16}"},
				},
			},
		},
	})

	eng := New(policy)

	toolInput, _ := json.Marshal(map[string]interface{}{
		"file_path": "/project/config.go",
		"content":   "var key = \"AKIAIOSFODNN7ABCDEFG\"",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	output, _ := eng.Evaluate(input)
	if output == nil {
		t.Fatal("expected output for blocked secret")
	}
	var specific types.PreToolUseOutput
	json.Unmarshal(output.HookSpecific, &specific)
	if specific.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want %q", specific.PermissionDecision, "deny")
	}
}

func TestEngine_WarnSeverity(t *testing.T) {
	policy := newTestPolicy(types.Rule{
		ID:         "cmd-warn",
		Name:       "Warn on sudo",
		Enabled:    true,
		Priority:   1,
		Severity:   types.SeverityWarn,
		HookEvents: []string{"PreToolUse"},
		ToolMatch:  []string{"Bash"},
		Check: types.CheckConfig{
			Type: "cmd_validate",
			Params: map[string]interface{}{
				"deny_patterns": []interface{}{
					map[string]interface{}{"name": "Sudo", "regex": `\bsudo\b`},
				},
			},
		},
	})

	eng := New(policy)

	toolInput, _ := json.Marshal(map[string]string{"command": "sudo apt install vim"})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput,
	}
	output, _ := eng.Evaluate(input)
	if output == nil {
		t.Fatal("expected output with warning")
	}
	if output.SystemMessage == "" {
		t.Error("expected system message with warning")
	}
	// Should NOT have deny decision — just a warning
	if output.HookSpecific != nil {
		t.Error("warn severity should not produce hookSpecificOutput with deny")
	}
}

func TestEngine_NoMatchingRules(t *testing.T) {
	policy := newTestPolicy(types.Rule{
		ID:         "cmd-block",
		Name:       "Block commands",
		Enabled:    true,
		Priority:   1,
		Severity:   types.SeverityBlock,
		HookEvents: []string{"PreToolUse"},
		ToolMatch:  []string{"Bash"},
		Check: types.CheckConfig{
			Type: "cmd_validate",
			Params: map[string]interface{}{
				"deny_patterns": []interface{}{
					map[string]interface{}{"name": "test", "regex": "test"},
				},
			},
		},
	})

	eng := New(policy)

	// Different event — no rules should match
	input := &types.HookInput{
		HookEventName: "Notification",
	}
	output, exitCode := eng.Evaluate(input)
	if output != nil {
		t.Error("expected nil output for no matching rules")
	}
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
}

func TestEngine_CELRule(t *testing.T) {
	policy := newTestPolicy(types.Rule{
		ID:         "cel-test",
		Name:       "Block main push",
		Enabled:    true,
		Priority:   1,
		Severity:   types.SeverityBlock,
		HookEvents: []string{"PreToolUse"},
		ToolMatch:  []string{"Bash"},
		Check: types.CheckConfig{
			Type: "cel",
			Params: map[string]interface{}{
				"expression": `tool_input.command.contains("git push") && tool_input.command.contains("main")`,
			},
		},
	})

	eng := New(policy)

	toolInput, _ := json.Marshal(map[string]string{"command": "git push origin main"})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput,
	}
	output, _ := eng.Evaluate(input)
	if output == nil {
		t.Fatal("expected output for CEL-blocked command")
	}
	var specific types.PreToolUseOutput
	json.Unmarshal(output.HookSpecific, &specific)
	if specific.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want %q", specific.PermissionDecision, "deny")
	}
}

func TestEngine_MultipleRules_PriorityOrder(t *testing.T) {
	policy := newTestPolicy(
		types.Rule{
			ID:         "warn-rule",
			Name:       "Warn first",
			Enabled:    true,
			Priority:   10, // Lower priority (runs second)
			Severity:   types.SeverityWarn,
			HookEvents: []string{"PreToolUse"},
			ToolMatch:  []string{"Bash"},
			Check: types.CheckConfig{
				Type: "cmd_validate",
				Params: map[string]interface{}{
					"deny_patterns": []interface{}{
						map[string]interface{}{"name": "any", "regex": ".*"},
					},
				},
			},
		},
		types.Rule{
			ID:         "block-rule",
			Name:       "Block critical",
			Enabled:    true,
			Priority:   1, // Higher priority (runs first)
			Severity:   types.SeverityBlock,
			HookEvents: []string{"PreToolUse"},
			ToolMatch:  []string{"Bash"},
			Check: types.CheckConfig{
				Type: "cmd_validate",
				Params: map[string]interface{}{
					"deny_patterns": []interface{}{
						map[string]interface{}{"name": "Root deletion", "regex": `rm\s+-rf\s+/`},
					},
				},
			},
		},
	)

	eng := New(policy)

	toolInput, _ := json.Marshal(map[string]string{"command": "rm -rf /"})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput,
	}
	output, _ := eng.Evaluate(input)
	if output == nil {
		t.Fatal("expected output")
	}
	// Should be blocked by the block rule (priority 1), not just warned
	var specific types.PreToolUseOutput
	json.Unmarshal(output.HookSpecific, &specific)
	if specific.PermissionDecision != "deny" {
		t.Errorf("expected deny from block rule, got %q", specific.PermissionDecision)
	}
}

func TestEngine_SessionStart_Blocks(t *testing.T) {
	policy := newTestPolicy(types.Rule{
		ID:         "repo-block",
		Name:       "Repo check",
		Enabled:    true,
		Priority:   1,
		Severity:   types.SeverityBlock,
		HookEvents: []string{"SessionStart"},
		Check: types.CheckConfig{
			Type: "repo_access",
			Params: map[string]interface{}{
				"mode":        "allowlist",
				"patterns":    []interface{}{"github.com/allowed-org/*"},
				"detect_from": "directory",
			},
		},
	})

	eng := New(policy)

	input := &types.HookInput{
		HookEventName: "SessionStart",
		CWD:           "/home/user/blocked-project",
	}
	output, exitCode := eng.Evaluate(input)
	if output == nil {
		t.Fatal("expected output for blocked session")
	}
	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}
	if output.Continue == nil || *output.Continue != false {
		t.Error("expected continue=false for blocked session")
	}
}
