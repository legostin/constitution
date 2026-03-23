package handler

import (
	"context"
	"log/slog"

	"github.com/legostin/constitution/internal/check"
	"github.com/legostin/constitution/internal/hook"
	"github.com/legostin/constitution/pkg/types"
)

// Stop handles Stop events (completion validation).
type Stop struct {
	registry *check.Registry
}

func NewStop(registry *check.Registry) *Stop {
	return &Stop{registry: registry}
}

func (h *Stop) EventName() string { return "Stop" }

func (h *Stop) Handle(ctx context.Context, input *types.HookInput, rules []types.Rule) (*types.HookOutput, int) {
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

		if !result.Passed && rule.Severity == types.SeverityBlock {
			msg := rule.Message
			if msg == "" {
				msg = result.Message
			}
			slog.Info("stop blocked", "rule", rule.ID, "message", msg)
			return hook.BuildStopBlockOutput(msg), 0
		}
	}

	return nil, 0
}
