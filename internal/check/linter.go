package check

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

// Linter runs an external linter command on files after write/edit.
type Linter struct {
	fileExtensions []string
	command        string
	workingDir     string // "project" | "file"
	timeout        time.Duration
}

func (l *Linter) Name() string { return "linter" }

func (l *Linter) Init(params map[string]interface{}) error {
	if exts, ok := params["file_extensions"]; ok {
		l.fileExtensions, _ = toStringSlice(exts)
	}

	cmd, ok := params["command"].(string)
	if !ok || cmd == "" {
		return fmt.Errorf("linter: command is required")
	}
	l.command = cmd

	if wd, ok := params["working_dir"].(string); ok {
		l.workingDir = wd
	}
	if l.workingDir == "" {
		l.workingDir = "project"
	}

	l.timeout = 30 * time.Second
	if ms, ok := params["timeout"]; ok {
		switch v := ms.(type) {
		case int:
			l.timeout = time.Duration(v) * time.Millisecond
		case float64:
			l.timeout = time.Duration(v) * time.Millisecond
		}
	}

	return nil
}

func (l *Linter) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	filePath := l.extractFilePath(input)
	if filePath == "" {
		return &types.CheckResult{Passed: true}, nil
	}

	// Check file extension filter
	if len(l.fileExtensions) > 0 {
		ext := filepath.Ext(filePath)
		matched := false
		for _, e := range l.fileExtensions {
			if e == ext {
				matched = true
				break
			}
		}
		if !matched {
			return &types.CheckResult{Passed: true, Message: "file extension not in filter"}, nil
		}
	}

	// Determine working directory
	workDir := input.CWD
	if l.workingDir == "file" {
		workDir = filepath.Dir(filePath)
	}

	// Build command with file substitution
	cmdStr := strings.ReplaceAll(l.command, "{file}", filePath)

	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &types.CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Linter failed: %s", strings.TrimSpace(string(output))),
			AdditionalContext: fmt.Sprintf("Linter output for %s:\n%s", filepath.Base(filePath), string(output)),
		}, nil
	}

	result := &types.CheckResult{Passed: true, Message: "linter passed"}
	if len(output) > 0 {
		result.AdditionalContext = fmt.Sprintf("Linter output for %s:\n%s", filepath.Base(filePath), string(output))
	}
	return result, nil
}

func (l *Linter) extractFilePath(input *types.HookInput) string {
	if input.ToolInput == nil {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(input.ToolInput, &m); err != nil {
		return ""
	}
	fp, _ := m["file_path"].(string)
	return fp
}
