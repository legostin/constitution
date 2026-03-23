package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/legostin/constitution/internal/check"
	"github.com/legostin/constitution/internal/hook"
	"github.com/legostin/constitution/pkg/types"
)

// PostToolUse handles PostToolUse events (e.g., linting after writes).
type PostToolUse struct {
	registry *check.Registry
}

func NewPostToolUse(registry *check.Registry) *PostToolUse {
	return &PostToolUse{registry: registry}
}

func (h *PostToolUse) EventName() string { return "PostToolUse" }

func (h *PostToolUse) Handle(ctx context.Context, input *types.HookInput, rules []types.Rule) (*types.HookOutput, int) {
	var messages []string
	var contextParts []string

	for _, rule := range rules {
		if rule.Remote {
			continue
		}

		c, err := h.registry.Get(rule.Check.Type)
		if err != nil {
			slog.Warn("unknown check type", "type", rule.Check.Type, "rule", rule.ID)
			if rule.Severity == types.SeverityBlock {
				messages = append(messages, fmt.Sprintf("[%s] check %q unavailable: %v", rule.Name, rule.Check.Type, err))
			}
			continue
		}
		if err := c.Init(rule.Check.Params); err != nil {
			slog.Error("check init failed", "rule", rule.ID, "error", err)
			if rule.Severity == types.SeverityBlock {
				messages = append(messages, fmt.Sprintf("[%s] check init failed: %v", rule.Name, err))
			}
			continue
		}

		result, err := c.Execute(ctx, input)
		if err != nil {
			slog.Error("check execution failed", "rule", rule.ID, "error", err)
			if rule.Severity == types.SeverityBlock {
				messages = append(messages, fmt.Sprintf("[%s] check failed: %v", rule.Name, err))
			}
			continue
		}

		if !result.Passed {
			msg := rule.Message
			if msg == "" {
				msg = result.Message
			}
			messages = append(messages, fmt.Sprintf("[%s] %s", rule.Name, msg))
			slog.Info("post-tool check", "rule", rule.ID, "passed", false, "message", msg)
		}

		if result.AdditionalContext != "" {
			contextParts = append(contextParts, result.AdditionalContext)
		}
	}

	if len(contextParts) > 0 {
		output := hook.BuildContextOutput("PostToolUse", strings.Join(contextParts, "\n"))
		if len(messages) > 0 {
			output.SystemMessage = strings.Join(messages, "\n")
		}
		return output, 0
	}

	if len(messages) > 0 {
		return &types.HookOutput{
			SystemMessage: strings.Join(messages, "\n"),
		}, 0
	}

	return nil, 0
}
