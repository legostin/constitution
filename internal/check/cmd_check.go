package check

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

// CmdCheck runs an arbitrary shell command and passes/fails based on exit code.
// Unlike Linter, it does not require a file path — suitable for Stop events.
type CmdCheck struct {
	command    string
	workingDir string // "project" (default) or absolute path
	timeout    time.Duration
}

func (c *CmdCheck) Name() string { return "cmd_check" }

func (c *CmdCheck) Init(params map[string]interface{}) error {
	cmd, ok := params["command"].(string)
	if !ok || cmd == "" {
		return fmt.Errorf("cmd_check: command is required")
	}
	c.command = cmd

	if wd, ok := params["working_dir"].(string); ok {
		c.workingDir = wd
	}
	if c.workingDir == "" {
		c.workingDir = "project"
	}

	c.timeout = 30 * time.Second
	if ms, ok := params["timeout"]; ok {
		switch v := ms.(type) {
		case int:
			c.timeout = time.Duration(v) * time.Millisecond
		case float64:
			c.timeout = time.Duration(v) * time.Millisecond
		}
	}

	return nil
}

func (c *CmdCheck) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	workDir := input.CWD
	if c.workingDir != "project" && c.workingDir != "" {
		workDir = c.workingDir
	}

	cmdStr := strings.ReplaceAll(c.command, "{cwd}", input.CWD)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		msg := fmt.Sprintf("Command failed: %s", strings.TrimSpace(string(output)))
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		return &types.CheckResult{
			Passed:            false,
			Message:           msg,
			AdditionalContext: string(output),
		}, nil
	}

	result := &types.CheckResult{Passed: true, Message: "command check passed"}
	if len(output) > 0 {
		result.AdditionalContext = string(output)
	}
	return result, nil
}
