package check

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func newCmdInput(command string) *types.HookInput {
	toolInput, _ := json.Marshal(map[string]string{"command": command})
	return &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput,
	}
}

func TestCmdValidate_BlocksDangerous(t *testing.T) {
	c := &CmdValidate{}
	err := c.Init(map[string]interface{}{
		"deny_patterns": []interface{}{
			map[string]interface{}{"name": "Root deletion", "regex": `rm\s+-rf\s+/`},
			map[string]interface{}{"name": "Force push", "regex": `git\s+push\s+.*--force`},
			map[string]interface{}{"name": "Drop database", "regex": `\bdrop\s+database\b`, "case_insensitive": true},
		},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	tests := []struct {
		name    string
		command string
		blocked bool
	}{
		{"rm -rf /", "rm -rf /", true},
		{"rm -rf /tmp", "rm -rf /tmp", true},
		{"safe rm", "rm -rf ./build", false},
		{"force push", "git push origin main --force", true},
		{"normal push", "git push origin feature", false},
		{"drop database", "DROP DATABASE mydb", true},
		{"safe query", "SELECT * FROM users", false},
		{"ls", "ls -la", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.Execute(context.Background(), newCmdInput(tt.command))
			if err != nil {
				t.Fatalf("Execute error: %v", err)
			}
			if result.Passed == tt.blocked {
				if tt.blocked {
					t.Errorf("command %q should be blocked", tt.command)
				} else {
					t.Errorf("command %q should be allowed", tt.command)
				}
			}
		})
	}
}

func TestCmdValidate_AllowException(t *testing.T) {
	c := &CmdValidate{}
	err := c.Init(map[string]interface{}{
		"deny_patterns": []interface{}{
			map[string]interface{}{"name": "Sudo", "regex": `\bsudo\b`},
		},
		"allow_patterns": []interface{}{
			map[string]interface{}{"name": "Sudo apt", "regex": `sudo\s+apt`},
		},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	// sudo apt should be allowed (exception)
	result, err := c.Execute(context.Background(), newCmdInput("sudo apt install vim"))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("sudo apt should be allowed by exception")
	}

	// generic sudo should be blocked
	result, err = c.Execute(context.Background(), newCmdInput("sudo rm -rf /"))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("generic sudo should be blocked")
	}
}
