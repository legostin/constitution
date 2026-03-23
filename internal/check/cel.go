package check

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/legostin/constitution/internal/celenv"
	"github.com/legostin/constitution/pkg/types"
)

// CELCheck evaluates a CEL expression against the hook input.
type CELCheck struct {
	expression string
	env        *cel.Env
	program    cel.Program
}

func (c *CELCheck) Name() string { return "cel" }

func (c *CELCheck) Init(params map[string]interface{}) error {
	expr, ok := params["expression"].(string)
	if !ok || expr == "" {
		return fmt.Errorf("cel: expression is required")
	}
	c.expression = expr

	env, err := celenv.New()
	if err != nil {
		return fmt.Errorf("cel: failed to create environment: %w", err)
	}
	c.env = env

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("cel: compile error: %w", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return fmt.Errorf("cel: program error: %w", err)
	}
	c.program = prg

	return nil
}

func (c *CELCheck) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	vars := c.buildVars(input)
	out, _, err := c.program.Eval(vars)
	if err != nil {
		return nil, fmt.Errorf("cel: eval error: %w", err)
	}

	matched, ok := out.Value().(bool)
	if !ok {
		return nil, fmt.Errorf("cel: expression must return bool, got %T", out.Value())
	}

	if matched {
		// CEL expression matched = rule triggered = check fails
		return &types.CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("CEL rule matched: %s", c.expression),
		}, nil
	}

	return &types.CheckResult{Passed: true}, nil
}

func (c *CELCheck) buildVars(input *types.HookInput) map[string]interface{} {
	toolInput := make(map[string]interface{})
	if input.ToolInput != nil {
		_ = json.Unmarshal(input.ToolInput, &toolInput)
	}

	return map[string]interface{}{
		"session_id":      input.SessionID,
		"cwd":             input.CWD,
		"hook_event_name": input.HookEventName,
		"tool_name":       input.ToolName,
		"tool_input":      toolInput,
		"prompt":          input.Prompt,
		"permission_mode": input.PermissionMode,
	}
}
