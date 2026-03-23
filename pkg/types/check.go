package types

import "context"

// CheckResult is returned by every check execution.
type CheckResult struct {
	Passed            bool              `json:"passed"`
	Message           string            `json:"message"`
	Details           map[string]string `json:"details,omitempty"`
	UpdatedInput      map[string]interface{} `json:"updated_input,omitempty"`
	AdditionalContext string            `json:"additional_context,omitempty"`
}

// Check is the interface all checks must implement.
type Check interface {
	// Name returns the unique check type identifier.
	Name() string

	// Init is called once with check parameters from YAML config.
	Init(params map[string]interface{}) error

	// Execute runs the check against the hook input.
	Execute(ctx context.Context, input *HookInput) (*CheckResult, error)
}
