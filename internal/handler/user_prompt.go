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

// UserPrompt handles UserPromptSubmit events (prompt modification).
type UserPrompt struct {
	registry *check.Registry
}

func NewUserPrompt(registry *check.Registry) *UserPrompt {
	return &UserPrompt{registry: registry}
}

func (h *UserPrompt) EventName() string { return "UserPromptSubmit" }

func (h *UserPrompt) Handle(ctx context.Context, input *types.HookInput, rules []types.Rule) (*types.HookOutput, int) {
	var contextParts []string

	for _, rule := range rules {
		if rule.Remote {
			continue
		}

		c, err := h.registry.Get(rule.Check.Type)
		if err != nil {
			slog.Warn("unknown check type", "type", rule.Check.Type, "rule", rule.ID)
			if rule.Severity == types.SeverityBlock {
				f := false
				return &types.HookOutput{Continue: &f, StopReason: fmt.Sprintf("Check %q unavailable: %v", rule.Check.Type, err)}, 2
			}
			continue
		}
		if err := c.Init(rule.Check.Params); err != nil {
			slog.Error("check init failed", "rule", rule.ID, "error", err)
			if rule.Severity == types.SeverityBlock {
				f := false
				return &types.HookOutput{Continue: &f, StopReason: fmt.Sprintf("Check %q init failed: %v", rule.ID, err)}, 2
			}
			continue
		}

		result, err := c.Execute(ctx, input)
		if err != nil {
			slog.Error("check execution failed", "rule", rule.ID, "error", err)
			if rule.Severity == types.SeverityBlock {
				f := false
				return &types.HookOutput{Continue: &f, StopReason: fmt.Sprintf("Check %q failed: %v", rule.ID, err)}, 2
			}
			continue
		}

		if !result.Passed && rule.Severity == types.SeverityBlock {
			msg := rule.Message
			if msg == "" {
				msg = result.Message
			}
			f := false
			return &types.HookOutput{
				Continue:   &f,
				StopReason: msg,
			}, 2
		}

		if result.AdditionalContext != "" {
			contextParts = append(contextParts, result.AdditionalContext)
		}
	}

	if len(contextParts) > 0 {
		return hook.BuildContextOutput("UserPromptSubmit", strings.Join(contextParts, "\n")), 0
	}

	return nil, 0
}
