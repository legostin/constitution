package check

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func newReadInput(cwd, filePath string) *types.HookInput {
	toolInput, _ := json.Marshal(map[string]string{"file_path": filePath})
	return &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Read",
		ToolInput:     toolInput,
		CWD:           cwd,
	}
}

func TestDirACL_DenyList(t *testing.T) {
	d := &DirACL{}
	err := d.Init(map[string]interface{}{
		"mode":       "denylist",
		"path_field": "file_path",
		"patterns":   []interface{}{"/etc/**", "/var/**"},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		blocked bool
	}{
		{"etc passwd", "/etc/passwd", true},
		{"var log", "/var/log/syslog", true},
		{"home file", "/home/user/file.txt", false},
		{"project file", "/project/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := d.Execute(context.Background(), newReadInput("/project", tt.path))
			if err != nil {
				t.Fatalf("Execute error: %v", err)
			}
			if result.Passed == tt.blocked {
				if tt.blocked {
					t.Errorf("path %q should be blocked", tt.path)
				} else {
					t.Errorf("path %q should be allowed", tt.path)
				}
			}
		})
	}
}

func TestDirACL_DenySecretFiles(t *testing.T) {
	d := &DirACL{}
	err := d.Init(map[string]interface{}{
		"mode":       "denylist",
		"path_field": "file_path",
		"patterns":   []interface{}{"**/.env", "**/.env.*", "**/credentials.json", "**/*.pem", "**/*.key"},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		blocked bool
	}{
		{"env file", "/project/.env", true},
		{"env local", "/project/.env.local", true},
		{"credentials", "/project/credentials.json", true},
		{"pem file", "/project/server.pem", true},
		{"key file", "/project/private.key", true},
		{"go file", "/project/main.go", false},
		{"json file", "/project/config.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := d.Execute(context.Background(), newReadInput("/project", tt.path))
			if err != nil {
				t.Fatalf("Execute error: %v", err)
			}
			if result.Passed == tt.blocked {
				if tt.blocked {
					t.Errorf("path %q should be blocked", tt.path)
				} else {
					t.Errorf("path %q should be allowed", tt.path)
				}
			}
		})
	}
}

func TestDirACL_AllowWithinProject(t *testing.T) {
	d := &DirACL{}
	err := d.Init(map[string]interface{}{
		"mode":                 "denylist",
		"path_field":           "file_path",
		"patterns":             []interface{}{"/etc/**"},
		"allow_within_project": true,
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	// File within project should be allowed even if it matched a general deny
	result, err := d.Execute(context.Background(), newReadInput("/project", "/project/src/main.go"))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Passed {
		t.Error("file within project should be allowed")
	}

	// File outside project should still be blocked
	result, err = d.Execute(context.Background(), newReadInput("/project", "/etc/passwd"))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("/etc/passwd should be blocked")
	}
}

func TestDirACL_AutoPathField(t *testing.T) {
	d := &DirACL{}
	err := d.Init(map[string]interface{}{
		"mode":       "denylist",
		"path_field": "auto",
		"patterns":   []interface{}{"/etc/**"},
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	// Test with file_path field
	toolInput, _ := json.Marshal(map[string]string{"file_path": "/etc/passwd"})
	input := &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Read",
		ToolInput:     toolInput,
		CWD:           "/project",
	}
	result, err := d.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("/etc/passwd should be blocked with auto path detection")
	}

	// Test with path field (Glob tool)
	toolInput, _ = json.Marshal(map[string]string{"path": "/etc", "pattern": "*.conf"})
	input = &types.HookInput{
		HookEventName: "PreToolUse",
		ToolName:      "Glob",
		ToolInput:     toolInput,
		CWD:           "/project",
	}
	result, err = d.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Passed {
		t.Error("/etc should be blocked with auto path detection on Glob")
	}
}
