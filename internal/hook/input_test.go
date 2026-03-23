package hook

import (
	"strings"
	"testing"
)

func TestReadInput_PreToolUse(t *testing.T) {
	json := `{
		"session_id": "sess-123",
		"cwd": "/home/user/project",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "rm -rf /"}
	}`
	input, err := ReadInput(strings.NewReader(json))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.SessionID != "sess-123" {
		t.Errorf("session_id = %q, want %q", input.SessionID, "sess-123")
	}
	if input.HookEventName != "PreToolUse" {
		t.Errorf("hook_event_name = %q, want %q", input.HookEventName, "PreToolUse")
	}
	if input.ToolName != "Bash" {
		t.Errorf("tool_name = %q, want %q", input.ToolName, "Bash")
	}

	m, err := input.ToolInputMap()
	if err != nil {
		t.Fatalf("ToolInputMap error: %v", err)
	}
	if cmd, ok := m["command"].(string); !ok || cmd != "rm -rf /" {
		t.Errorf("tool_input.command = %v, want %q", m["command"], "rm -rf /")
	}
}

func TestReadInput_SessionStart(t *testing.T) {
	json := `{
		"session_id": "sess-456",
		"cwd": "/project",
		"hook_event_name": "SessionStart",
		"source": "cli"
	}`
	input, err := ReadInput(strings.NewReader(json))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.HookEventName != "SessionStart" {
		t.Errorf("hook_event_name = %q, want %q", input.HookEventName, "SessionStart")
	}
	if input.Source != "cli" {
		t.Errorf("source = %q, want %q", input.Source, "cli")
	}
}

func TestReadInput_InvalidJSON(t *testing.T) {
	_, err := ReadInput(strings.NewReader("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadInput_EmptyInput(t *testing.T) {
	_, err := ReadInput(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}
