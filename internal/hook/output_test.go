package hook

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func TestWriteOutput(t *testing.T) {
	var buf bytes.Buffer
	output := &types.HookOutput{
		SystemMessage: "test warning",
	}
	if err := WriteOutput(&buf, output); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded types.HookOutput
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if decoded.SystemMessage != "test warning" {
		t.Errorf("systemMessage = %q, want %q", decoded.SystemMessage, "test warning")
	}
}

func TestBuildDenyOutput(t *testing.T) {
	output := BuildDenyOutput("PreToolUse", "blocked: dangerous command")
	if output.HookSpecific == nil {
		t.Fatal("hookSpecificOutput is nil")
	}
	var specific types.PreToolUseOutput
	if err := json.Unmarshal(output.HookSpecific, &specific); err != nil {
		t.Fatalf("failed to unmarshal hookSpecificOutput: %v", err)
	}
	if specific.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want %q", specific.PermissionDecision, "deny")
	}
	if specific.PermissionDecisionReason != "blocked: dangerous command" {
		t.Errorf("reason = %q, want %q", specific.PermissionDecisionReason, "blocked: dangerous command")
	}
}

func TestBuildWarnOutput(t *testing.T) {
	output := BuildWarnOutput([]string{"warn1", "warn2"})
	if output.SystemMessage != "warn1\nwarn2" {
		t.Errorf("systemMessage = %q, want %q", output.SystemMessage, "warn1\nwarn2")
	}
}

func TestBuildContextOutput_SessionStart(t *testing.T) {
	output := BuildContextOutput("SessionStart", "extra context")
	if output.HookSpecific == nil {
		t.Fatal("hookSpecificOutput is nil")
	}
	var specific types.SessionStartOutput
	if err := json.Unmarshal(output.HookSpecific, &specific); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if specific.AdditionalContext != "extra context" {
		t.Errorf("additionalContext = %q, want %q", specific.AdditionalContext, "extra context")
	}
}

func TestBuildContextOutput_UserPromptSubmit(t *testing.T) {
	output := BuildContextOutput("UserPromptSubmit", "prompt context")
	var specific types.UserPromptOutput
	if err := json.Unmarshal(output.HookSpecific, &specific); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if specific.AdditionalContext != "prompt context" {
		t.Errorf("additionalContext = %q, want %q", specific.AdditionalContext, "prompt context")
	}
}
