package check

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/legostin/constitution/pkg/types"
)

// DirACL checks file paths against allow/deny glob patterns.
type DirACL struct {
	mode              string // "allowlist" | "denylist"
	pathField         string // "file_path", "path", "pattern", "auto"
	patterns          []string
	allowWithinProject bool
}

func (d *DirACL) Name() string { return "dir_acl" }

func (d *DirACL) Init(params map[string]interface{}) error {
	if m, ok := params["mode"].(string); ok {
		d.mode = m
	}
	if d.mode == "" {
		d.mode = "denylist"
	}

	if pf, ok := params["path_field"].(string); ok {
		d.pathField = pf
	}
	if d.pathField == "" {
		d.pathField = "auto"
	}

	rawPatterns, ok := params["patterns"]
	if !ok {
		return fmt.Errorf("dir_acl: patterns is required")
	}
	pats, err := toStringSlice(rawPatterns)
	if err != nil {
		return fmt.Errorf("dir_acl: invalid patterns: %w", err)
	}
	// Expand ~ in patterns
	for i, p := range pats {
		pats[i] = expandHome(p)
	}
	d.patterns = pats

	if awp, ok := params["allow_within_project"].(bool); ok {
		d.allowWithinProject = awp
	}

	return nil
}

func (d *DirACL) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	path := d.extractPath(input)
	if path == "" {
		return &types.CheckResult{Passed: true}, nil
	}

	// Resolve to absolute path
	if !filepath.IsAbs(path) {
		path = filepath.Join(input.CWD, path)
	}
	path = filepath.Clean(path)

	// Check allow_within_project
	if d.allowWithinProject && input.CWD != "" {
		cwd := filepath.Clean(input.CWD)
		if strings.HasPrefix(path, cwd+string(filepath.Separator)) || path == cwd {
			if d.mode == "denylist" {
				// Even within project, check for explicit denies on the path itself
				for _, pattern := range d.patterns {
					if matchGlob(pattern, path) {
						return &types.CheckResult{
							Passed:  false,
							Message: fmt.Sprintf("Access denied: path %q matches deny pattern %q", path, pattern),
						}, nil
					}
				}
				return &types.CheckResult{Passed: true}, nil
			}
		}
	}

	matched := false
	var matchedPattern string
	for _, pattern := range d.patterns {
		if matchGlob(pattern, path) {
			matched = true
			matchedPattern = pattern
			break
		}
	}

	switch d.mode {
	case "denylist":
		if matched {
			return &types.CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Access denied: path %q matches deny pattern %q", path, matchedPattern),
			}, nil
		}
		return &types.CheckResult{Passed: true}, nil
	case "allowlist":
		if !matched {
			return &types.CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Access denied: path %q not in allowlist", path),
			}, nil
		}
		return &types.CheckResult{Passed: true}, nil
	default:
		return &types.CheckResult{Passed: true}, nil
	}
}

func (d *DirACL) extractPath(input *types.HookInput) string {
	if input.ToolInput == nil {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(input.ToolInput, &m); err != nil {
		return ""
	}

	if d.pathField != "auto" {
		val, _ := m[d.pathField].(string)
		return val
	}

	// Auto-detect: try common fields in order
	for _, field := range []string{"file_path", "path", "pattern"} {
		if val, ok := m[field].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

// matchGlob performs glob-like matching.
// Supports ** for recursive directory matching.
func matchGlob(pattern, path string) bool {
	pattern = expandHome(pattern)

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		// Convert ** glob to a simpler check
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")
			if prefix != "" && !strings.HasPrefix(path, prefix) {
				return false
			}
			if suffix != "" {
				// Check if any component matches the suffix pattern
				matched, _ := filepath.Match(suffix, filepath.Base(path))
				return matched
			}
			return prefix == "" || strings.HasPrefix(path, prefix)
		}
	}

	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}
	// Also match against just the filename
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
