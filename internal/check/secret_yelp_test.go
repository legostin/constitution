package check

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func detectSecretsAvailable() bool {
	_, err := exec.LookPath("detect-secrets")
	return err == nil
}

func TestDetectSecrets_Init_BinaryNotFound(t *testing.T) {
	d := &DetectSecrets{}
	err := d.Init(map[string]interface{}{
		"binary": "nonexistent-binary-12345",
	})
	if err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestDetectSecrets_GenerateBaseline(t *testing.T) {
	d := &DetectSecrets{}
	baseline, err := d.generateBaseline(map[string]interface{}{
		"plugins": []interface{}{
			map[string]interface{}{"name": "AWSKeyDetector"},
			map[string]interface{}{"name": "Base64HighEntropyString", "limit": 4.5},
		},
		"filters": []interface{}{
			map[string]interface{}{"path": "detect_secrets.filters.allowlist.is_line_allowlisted"},
		},
	})
	if err != nil {
		t.Fatalf("generateBaseline error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(baseline, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	plugins, ok := parsed["plugins_used"].([]interface{})
	if !ok {
		t.Fatal("plugins_used missing or wrong type")
	}
	if len(plugins) != 2 {
		t.Errorf("plugins count = %d, want 2", len(plugins))
	}

	p1, _ := plugins[0].(map[string]interface{})
	if p1["name"] != "AWSKeyDetector" {
		t.Errorf("first plugin = %v, want AWSKeyDetector", p1["name"])
	}

	p2, _ := plugins[1].(map[string]interface{})
	if p2["name"] != "Base64HighEntropyString" {
		t.Errorf("second plugin = %v, want Base64HighEntropyString", p2["name"])
	}
	if p2["limit"] != 4.5 {
		t.Errorf("limit = %v, want 4.5", p2["limit"])
	}

	filters, ok := parsed["filters_used"].([]interface{})
	if !ok {
		t.Fatal("filters_used missing or wrong type")
	}
	if len(filters) != 1 {
		t.Errorf("filters count = %d, want 1", len(filters))
	}
}

func TestDetectSecrets_GenerateBaseline_NoPlugins(t *testing.T) {
	d := &DetectSecrets{}
	baseline, err := d.generateBaseline(map[string]interface{}{})
	if err != nil {
		t.Fatalf("generateBaseline error: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(baseline, &parsed)

	if _, ok := parsed["plugins_used"]; ok {
		t.Error("plugins_used should not be present when no plugins configured")
	}
}

func TestParseStringOutput(t *testing.T) {
	output := `AWSKeyDetector          : True  (unverified)
ArtifactoryDetector     : False
Base64HighEntropyString : False (3.584)
GitHubTokenDetector     : True  (unverified)
PrivateKeyDetector      : False`

	matched := parseStringOutput(output)
	if len(matched) != 2 {
		t.Fatalf("matched count = %d, want 2", len(matched))
	}
	if matched[0] != "AWSKeyDetector" {
		t.Errorf("first match = %q, want AWSKeyDetector", matched[0])
	}
	if matched[1] != "GitHubTokenDetector" {
		t.Errorf("second match = %q, want GitHubTokenDetector", matched[1])
	}
}

func TestParseStringOutput_NoMatch(t *testing.T) {
	output := `AWSKeyDetector          : False
GitHubTokenDetector     : False`

	matched := parseStringOutput(output)
	if len(matched) != 0 {
		t.Errorf("matched count = %d, want 0", len(matched))
	}
}

func TestDetectSecrets_ScanLine_AWSKey(t *testing.T) {
	if !detectSecretsAvailable() {
		t.Skip("detect-secrets not in PATH")
	}

	d := &DetectSecrets{}
	err := d.Init(map[string]interface{}{
		"plugins": []interface{}{
			map[string]interface{}{"name": "AWSKeyDetector"},
		},

	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]string{
		"content": "aws_access_key_id = AKIAIOSFODNN7ABCDEFG",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	result, err := d.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected check to fail for AWS key")
	}
	if result.Details["detectors"] == "" {
		t.Error("expected detectors in details")
	}
}

func TestDetectSecrets_MultilineContent_AWSKey(t *testing.T) {
	if !detectSecretsAvailable() {
		t.Skip("detect-secrets not in PATH")
	}

	d := &DetectSecrets{}
	err := d.Init(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]string{
		"content": "some code\naws_access_key_id = AKIAIOSFODNN7ABCDEFG\nmore code",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	result, err := d.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("expected check to fail for AWS key in multiline content")
	}
}

func TestDetectSecrets_CleanContent(t *testing.T) {
	if !detectSecretsAvailable() {
		t.Skip("detect-secrets not in PATH")
	}

	d := &DetectSecrets{}
	err := d.Init(map[string]interface{}{

	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]string{
		"content": "func main() {\n\tfmt.Println(\"hello\")\n}",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	result, err := d.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected check to pass for clean content, got: %s", result.Message)
	}
}

func TestDetectSecrets_ExcludeLines(t *testing.T) {
	if !detectSecretsAvailable() {
		t.Skip("detect-secrets not in PATH")
	}

	d := &DetectSecrets{}
	err := d.Init(map[string]interface{}{
		"exclude_lines": []interface{}{"pragma: allowlist"},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	toolInput, _ := json.Marshal(map[string]string{
		"content": "key = AKIAIOSFODNN7ABCDEFG  # pragma: allowlist secret",
	})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Write",
		ToolInput:     toolInput,
	}
	result, err := d.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("expected excluded line to pass")
	}
}
