package check

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func TestCmdCheck_PassingCommand(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{"command": "true"}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	result, err := c.Execute(context.Background(), &types.HookInput{CWD: "/tmp"})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true for 'true' command")
	}
}

func TestCmdCheck_FailingCommand(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{"command": "false"}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	result, err := c.Execute(context.Background(), &types.HookInput{CWD: "/tmp"})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected Passed=false for 'false' command")
	}
}

func TestCmdCheck_CapturesOutput(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{"command": "echo hello world"}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	result, err := c.Execute(context.Background(), &types.HookInput{CWD: "/tmp"})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if !strings.Contains(result.AdditionalContext, "hello world") {
		t.Errorf("expected output to contain 'hello world', got %q", result.AdditionalContext)
	}
}

func TestCmdCheck_FailingCommandCapturesOutput(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{"command": "echo failure && exit 1"}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	result, err := c.Execute(context.Background(), &types.HookInput{CWD: "/tmp"})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected Passed=false")
	}
	if !strings.Contains(result.Message, "failure") {
		t.Errorf("expected Message to contain 'failure', got %q", result.Message)
	}
}

func TestCmdCheck_Timeout(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{
		"command": "sleep 10",
		"timeout": float64(100), // 100ms
	}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	result, err := c.Execute(context.Background(), &types.HookInput{CWD: "/tmp"})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// Timeout should cause non-zero exit → Passed=false
	if result.Passed {
		t.Error("expected Passed=false for timed-out command")
	}
}

func TestCmdCheck_WorkingDir(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{"command": "pwd"}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	cwd, _ := os.Getwd()
	result, err := c.Execute(context.Background(), &types.HookInput{CWD: cwd})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(result.AdditionalContext, cwd) {
		t.Errorf("expected working dir %q in output, got %q", cwd, result.AdditionalContext)
	}
}

func TestCmdCheck_CWDSubstitution(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{"command": "echo {cwd}"}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	cwd, _ := os.Getwd()
	result, err := c.Execute(context.Background(), &types.HookInput{CWD: cwd})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(result.AdditionalContext, cwd) {
		t.Errorf("expected {cwd} substitution to contain %q, got %q", cwd, result.AdditionalContext)
	}
}

func TestCmdCheck_MissingCommand(t *testing.T) {
	c := &CmdCheck{}
	err := c.Init(map[string]interface{}{})
	if err == nil {
		t.Error("expected Init error for missing command")
	}
}

func TestCmdCheck_EmptyCommand(t *testing.T) {
	c := &CmdCheck{}
	err := c.Init(map[string]interface{}{"command": ""})
	if err == nil {
		t.Error("expected Init error for empty command")
	}
}

func TestCmdCheck_DefaultTimeout(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{"command": "true"}); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	if c.timeout.Seconds() != 30 {
		t.Errorf("expected default timeout 30s, got %v", c.timeout)
	}
}

func TestCmdCheck_CustomWorkingDir(t *testing.T) {
	c := &CmdCheck{}
	if err := c.Init(map[string]interface{}{
		"command":     "pwd",
		"working_dir": "/tmp",
	}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	result, err := c.Execute(context.Background(), &types.HookInput{CWD: "/some/other/dir"})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// Should use /tmp, not /some/other/dir
	if !strings.Contains(result.AdditionalContext, "/tmp") {
		t.Errorf("expected /tmp in output, got %q", result.AdditionalContext)
	}
}
