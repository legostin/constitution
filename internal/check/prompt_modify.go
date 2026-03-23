package check

import (
	"context"

	"github.com/legostin/constitution/pkg/types"
)

// PromptModify injects additional context into user prompts.
type PromptModify struct {
	systemContext string
	prepend       string
	append_       string
}

func (p *PromptModify) Name() string { return "prompt_modify" }

func (p *PromptModify) Init(params map[string]interface{}) error {
	if sc, ok := params["system_context"].(string); ok {
		p.systemContext = sc
	}
	if pre, ok := params["prepend"].(string); ok {
		p.prepend = pre
	}
	if app, ok := params["append"].(string); ok {
		p.append_ = app
	}
	return nil
}

func (p *PromptModify) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	var context string

	if p.systemContext != "" {
		context = p.systemContext
	}
	if p.prepend != "" {
		if context != "" {
			context = p.prepend + "\n" + context
		} else {
			context = p.prepend
		}
	}
	if p.append_ != "" {
		if context != "" {
			context = context + "\n" + p.append_
		} else {
			context = p.append_
		}
	}

	return &types.CheckResult{
		Passed:            true,
		Message:           "prompt context injected",
		AdditionalContext: context,
	}, nil
}
