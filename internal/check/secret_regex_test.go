package check

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func TestSecretDetect_AWSKey(t *testing.T) {
	s := &SecretDetect{}
	err := s.Init(map[string]interface{}{
		"scan_field": "content",
		"patterns": []interface{}{
			map[string]interface{}{"name": "AWS Key", "regex": "AKIA[0-9A-Z]{16}"},
		},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]string{
		"content": "aws_key = AKIAIOSFODNN7ABCDEFG",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	result, err := s.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected check to fail for AWS key")
	}
}

func TestSecretDetect_AllowPattern(t *testing.T) {
	s := &SecretDetect{}
	err := s.Init(map[string]interface{}{
		"scan_field": "content",
		"patterns": []interface{}{
			map[string]interface{}{"name": "AWS Key", "regex": "AKIA[0-9A-Z]{16}"},
		},
		"allow_patterns": []interface{}{"AKIAIOSFODNN7EXAMPLE"},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]string{
		"content": "example_key = AKIAIOSFODNN7EXAMPLE",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	result, err := s.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected check to pass for allowed pattern")
	}
}

func TestSecretDetect_NoSecret(t *testing.T) {
	s := &SecretDetect{}
	err := s.Init(map[string]interface{}{
		"scan_field": "content",
		"patterns": []interface{}{
			map[string]interface{}{"name": "AWS Key", "regex": "AKIA[0-9A-Z]{16}"},
		},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]string{
		"content": "func main() { fmt.Println(\"hello\") }",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	result, err := s.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected check to pass for clean content")
	}
}

func TestSecretDetect_GitHubToken(t *testing.T) {
	s := &SecretDetect{}
	err := s.Init(map[string]interface{}{
		"scan_field": "content",
		"patterns": []interface{}{
			map[string]interface{}{"name": "GitHub Token", "regex": "gh[ps]_[A-Za-z0-9_]{36,}"},
		},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]string{
		"content": "token = ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	result, err := s.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected check to fail for GitHub token")
	}
}

func TestSecretDetect_EditNewString(t *testing.T) {
	s := &SecretDetect{}
	err := s.Init(map[string]interface{}{
		"scan_field": "content",
		"patterns": []interface{}{
			map[string]interface{}{"name": "AWS Key", "regex": "AKIA[0-9A-Z]{16}"},
		},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]interface{}{
		"old_string": "placeholder",
		"new_string": "key = AKIAIOSFODNN7ABCDEFG",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Edit",
		ToolInput:     toolInput,
	}
	result, err := s.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected check to fail for AWS key in Edit new_string")
	}
}
