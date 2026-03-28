package types

import "encoding/json"

// HookInput is the universal input parsed from stdin JSON.
// Fields are populated depending on the hook_event_name.
type HookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	PermissionMode string `json:"permission_mode,omitempty"`
	HookEventName  string `json:"hook_event_name"`

	// Codex-specific
	TurnID string `json:"turn_id,omitempty"`

	// PreToolUse / PostToolUse
	ToolName  string          `json:"tool_name,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`

	// PostToolUse
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`

	// UserPromptSubmit
	Prompt string `json:"prompt,omitempty"`

	// SessionStart
	Source    string `json:"source,omitempty"`
	Model    string `json:"model,omitempty"`
	AgentType string `json:"agent_type,omitempty"`

	// Stop / SubagentStop
	StopHookActive       bool   `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`

	// SessionEnd
	Reason string `json:"reason,omitempty"`

	// Notification
	Message          string `json:"message,omitempty"`
	Title            string `json:"title,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`

	// PreCompact / PostCompact
	Trigger            string `json:"trigger,omitempty"`
	CustomInstructions string `json:"custom_instructions,omitempty"`
	CompactSummary     string `json:"compact_summary,omitempty"`

	// Failure events
	Error        string `json:"error,omitempty"`
	ErrorDetails string `json:"error_details,omitempty"`
	IsInterrupt  bool   `json:"is_interrupt,omitempty"`
}

// ToolInputMap parses the raw tool_input JSON into a map.
func (h *HookInput) ToolInputMap() (map[string]interface{}, error) {
	if h.ToolInput == nil {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(h.ToolInput, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// HookOutput is the universal output written to stdout JSON.
type HookOutput struct {
	Continue       *bool           `json:"continue,omitempty"`
	StopReason     string          `json:"stopReason,omitempty"`
	SuppressOutput *bool           `json:"suppressOutput,omitempty"`
	SystemMessage  string          `json:"systemMessage,omitempty"`
	Decision       string          `json:"decision,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	HookSpecific   json.RawMessage `json:"hookSpecificOutput,omitempty"`
}

// PreToolUseOutput is for hookSpecificOutput in PreToolUse responses.
type PreToolUseOutput struct {
	HookEventName            string          `json:"hookEventName"`
	PermissionDecision       string          `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string          `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             json.RawMessage `json:"updatedInput,omitempty"`
	AdditionalContext        string          `json:"additionalContext,omitempty"`
}

// PostToolUseOutput is for hookSpecificOutput in PostToolUse responses.
type PostToolUseOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// SessionStartOutput is for hookSpecificOutput in SessionStart responses.
type SessionStartOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// UserPromptOutput is for hookSpecificOutput in UserPromptSubmit responses.
type UserPromptOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// StopOutput is for hookSpecificOutput in Stop responses.
type StopOutput struct {
	HookEventName string `json:"hookEventName"`
	Decision      string `json:"decision,omitempty"`
	Reason        string `json:"reason,omitempty"`
}
