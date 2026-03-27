package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/legostin/constitution/pkg/types"
)

// WriteOutput writes a HookOutput as JSON to the given writer (typically os.Stdout).
func WriteOutput(w io.Writer, output *types.HookOutput) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("failed to write hook output: %w", err)
	}
	return nil
}

// BuildAllowOutput creates an output that allows the action to proceed.
func BuildAllowOutput(eventName string) *types.HookOutput {
	return &types.HookOutput{}
}

// BuildDenyOutput creates an output that blocks a PreToolUse action.
func BuildDenyOutput(eventName, reason string) *types.HookOutput {
	specific := types.PreToolUseOutput{
		HookEventName:            eventName,
		PermissionDecision:       "deny",
		PermissionDecisionReason: reason,
	}
	raw, _ := json.Marshal(specific)
	return &types.HookOutput{
		HookSpecific: raw,
	}
}

// BuildWarnOutput creates an output that allows the action but adds a system message.
func BuildWarnOutput(warnings []string) *types.HookOutput {
	return &types.HookOutput{
		SystemMessage: strings.Join(warnings, "\n"),
	}
}

// BuildStopBlockOutput creates an output that blocks the agent from stopping.
// Claude Code expects decision/reason at the top level for Stop events.
func BuildStopBlockOutput(reason string) *types.HookOutput {
	return &types.HookOutput{
		Decision: "block",
		Reason:   reason,
	}
}

// BuildContextOutput creates an output that injects additional context.
func BuildContextOutput(eventName, context string) *types.HookOutput {
	switch eventName {
	case "SessionStart":
		specific := types.SessionStartOutput{
			HookEventName:     eventName,
			AdditionalContext: context,
		}
		raw, _ := json.Marshal(specific)
		return &types.HookOutput{HookSpecific: raw}
	case "UserPromptSubmit":
		specific := types.UserPromptOutput{
			HookEventName:     eventName,
			AdditionalContext: context,
		}
		raw, _ := json.Marshal(specific)
		return &types.HookOutput{HookSpecific: raw}
	case "PostToolUse":
		specific := types.PostToolUseOutput{
			HookEventName:     eventName,
			AdditionalContext: context,
		}
		raw, _ := json.Marshal(specific)
		return &types.HookOutput{HookSpecific: raw}
	default:
		return &types.HookOutput{SystemMessage: context}
	}
}

// ExitBlock writes a blocking error to stderr and exits with code 2.
func ExitBlock(reason string) {
	fmt.Fprintln(os.Stderr, reason)
	os.Exit(2)
}
