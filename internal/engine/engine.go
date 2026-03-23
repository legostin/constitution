package engine

import (
	"context"
	"time"

	"github.com/legostin/constitution/internal/check"
	"github.com/legostin/constitution/internal/handler"
	"github.com/legostin/constitution/internal/hook"
	"github.com/legostin/constitution/pkg/types"
)

// Engine orchestrates rule evaluation against hook input.
type Engine struct {
	policy   *types.Policy
	registry *check.Registry
	handlers map[string]handler.Handler
}

// New creates a new Engine with the given policy.
func New(policy *types.Policy) *Engine {
	reg := check.NewRegistry()
	e := &Engine{
		policy:   policy,
		registry: reg,
		handlers: make(map[string]handler.Handler),
	}

	// Register handlers for each event type
	e.handlers["PreToolUse"] = handler.NewPreToolUse(reg)
	e.handlers["PostToolUse"] = handler.NewPostToolUse(reg)
	e.handlers["SessionStart"] = handler.NewSessionStart(reg)
	e.handlers["UserPromptSubmit"] = handler.NewUserPrompt(reg)
	e.handlers["Stop"] = handler.NewStop(reg)

	return e
}

// Evaluate runs all applicable rules against the input.
// Returns the HookOutput and the exit code (0 = pass, 2 = block).
func (e *Engine) Evaluate(input *types.HookInput) (*types.HookOutput, int) {
	rules := hook.FilterRules(e.policy.Rules, input.HookEventName, input.ToolName)
	if len(rules) == 0 {
		return nil, 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h, ok := e.handlers[input.HookEventName]
	if !ok {
		// No specific handler — use a generic pass-through
		return nil, 0
	}

	return h.Handle(ctx, input, rules)
}
