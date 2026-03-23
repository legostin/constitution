package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

// ExecPlugin runs an external binary. Communication is via stdin JSON / stdout JSON.
type ExecPlugin struct {
	name    string
	path    string
	timeout time.Duration
}

// NewExecPlugin creates an ExecPlugin from config.
func NewExecPlugin(cfg types.PluginConfig) (*ExecPlugin, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("exec plugin %q: path is required", cfg.Name)
	}
	timeout := 10 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Millisecond
	}
	return &ExecPlugin{
		name:    cfg.Name,
		path:    cfg.Path,
		timeout: timeout,
	}, nil
}

func (p *ExecPlugin) Name() string { return p.name }

type execPluginInput struct {
	Input  *types.HookInput       `json:"input"`
	Params map[string]interface{} `json:"params"`
}

func (p *ExecPlugin) Execute(ctx context.Context, input *types.HookInput, params map[string]interface{}) (*types.CheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	payload := execPluginInput{Input: input, Params: params}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("exec plugin %q: marshal error: %w", p.name, err)
	}

	cmd := exec.CommandContext(ctx, p.path)
	cmd.Stdin = bytes.NewReader(data)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Exit 0 = passed, Exit 2 = blocked, other = error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 2 {
				// Blocking result
				var result types.CheckResult
				if json.Unmarshal(stdout.Bytes(), &result) == nil {
					result.Passed = false
					return &result, nil
				}
				return &types.CheckResult{
					Passed:  false,
					Message: stderr.String(),
				}, nil
			}
		}
		return nil, fmt.Errorf("exec plugin %q: %w (stderr: %s)", p.name, err, stderr.String())
	}

	var result types.CheckResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return &types.CheckResult{Passed: true}, nil
	}
	return &result, nil
}

func (p *ExecPlugin) Close() error { return nil }
