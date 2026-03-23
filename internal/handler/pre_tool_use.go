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

// PreToolUse handles PreToolUse events.
type PreToolUse struct {
	registry *check.Registry
}

func NewPreToolUse(registry *check.Registry) *PreToolUse {
	return &PreToolUse{registry: registry}
}

func (h *PreToolUse) EventName() string { return "PreToolUse" }

func (h *PreToolUse) Handle(ctx context.Context, input *types.HookInput, rules []types.Rule) (*types.HookOutput, int) {
	var warnings []string

	for _, rule := range rules {
		if rule.Remote {
			continue
		}

		c, err := h.registry.Get(rule.Check.Type)
		if err != nil {
			slog.Warn("unknown check type", "type", rule.Check.Type, "rule", rule.ID)
			continue
		}
		if err := c.Init(rule.Check.Params); err != nil {
			slog.Error("check init failed", "rule", rule.ID, "error", err)
			continue
		}

		result, err := c.Execute(ctx, input)
		if err != nil {
			slog.Error("check execution failed", "rule", rule.ID, "error", err)
			continue
		}

		if !result.Passed {
			msg := rule.Message
			if msg == "" {
				msg = result.Message
			}

			switch rule.Severity {
			case types.SeverityBlock:
				slog.Info("rule blocked", "rule", rule.ID, "message", msg)
				return hook.BuildDenyOutput("PreToolUse", msg), 0
			case types.SeverityWarn:
				warnings = append(warnings, fmt.Sprintf("[%s] %s", rule.Name, msg))
			case types.SeverityAudit:
				slog.Info("audit", "rule", rule.ID, "message", msg)
			}
		}
	}

	if len(warnings) > 0 {
		return &types.HookOutput{
			SystemMessage: strings.Join(warnings, "\n"),
		}, 0
	}

	return nil, 0
}
