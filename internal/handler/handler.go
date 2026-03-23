package handler

import (
	"context"

	"github.com/legostin/constitution/pkg/types"
)

// Handler processes a specific hook event type.
type Handler interface {
	// EventName returns the hook event this handler is for.
	EventName() string

	// Handle processes the input and returns the output.
	Handle(ctx context.Context, input *types.HookInput, rules []types.Rule) (*types.HookOutput, int)
}
