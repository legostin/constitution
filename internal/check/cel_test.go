package check

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func TestCELCheck_SimpleMatch(t *testing.T) {
	c := &CELCheck{}
	err := c.Init(map[string]interface{}{
		"expression": `tool_input.command.contains("git push") && tool_input.command.contains("main")`,
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	// Should match
	toolInput, _ := json.Marshal(map[string]string{"command": "git push origin main --force"})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput,
	}
	result, err := c.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected CEL rule to match (block) for git push main")
	}

	// Should not match
	toolInput, _ = json.Marshal(map[string]string{"command": "git push origin feature"})
	input.ToolInput = toolInput
	result, err = c.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected CEL rule to not match for git push feature")
	}
}

func TestCELCheck_PermissionMode(t *testing.T) {
	c := &CELCheck{}
	err := c.Init(map[string]interface{}{
		"expression": `permission_mode == "plan"`,
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	input := &types.HookInput{
		HookEventName:  "PreToolUse",
		PermissionMode: "plan",
	}
	result, err := c.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected CEL rule to match for plan mode")
	}
}

func TestCELCheck_CWDCheck(t *testing.T) {
	c := &CELCheck{}
	err := c.Init(map[string]interface{}{
		"expression": `cwd.startsWith("/prod")`,
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	input := &types.HookInput{
		HookEventName: "PreToolUse",
		CWD:           "/prod/service-a",
	}
	result, err := c.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected CEL rule to match for /prod CWD")
	}

	input.CWD = "/home/user/project"
	result, err = c.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected CEL rule to not match for non-prod CWD")
	}
}

func TestCELCheck_InvalidExpression(t *testing.T) {
	c := &CELCheck{}
	err := c.Init(map[string]interface{}{
		"expression": `invalid $$$ expression`,
	})
	if err == nil {
		t.Error("expected error for invalid CEL expression")
	}
}

func TestCELCheck_MissingExpression(t *testing.T) {
	c := &CELCheck{}
	err := c.Init(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing expression")
	}
}

func TestCELCheck_LastAssistantMessage(t *testing.T) {
	c := &CELCheck{}
	err := c.Init(map[string]interface{}{
		"expression": `last_assistant_message.contains("done")`,
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	// Message contains "done" → CEL matches → Passed=false (rule triggered)
	input := &types.HookInput{
		HookEventName:       "Stop",
		LastAssistantMessage: "All done, tests pass.",
	}
	result, err := c.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected CEL to match when message contains 'done'")
	}

	// Message without "done" → CEL doesn't match → Passed=true
	input.LastAssistantMessage = "I made some changes."
	result, err = c.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected CEL to not match when message lacks 'done'")
	}
}

func TestCELCheck_LastAssistantMessageEmpty(t *testing.T) {
	c := &CELCheck{}
	err := c.Init(map[string]interface{}{
		"expression": `last_assistant_message.contains("test")`,
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	input := &types.HookInput{
		HookEventName:       "Stop",
		LastAssistantMessage: "",
	}
	result, err := c.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected CEL to not match for empty message")
	}
}
