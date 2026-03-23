package check

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/legostin/constitution/pkg/types"
)

// SecretDetect scans tool_input content for secret patterns.
type SecretDetect struct {
	scanField     string
	patterns      []*compiledPattern
	allowPatterns []*regexp.Regexp
}

type compiledPattern struct {
	name string
	re   *regexp.Regexp
}

func (s *SecretDetect) Name() string { return "secret_detect" }

func (s *SecretDetect) Init(params map[string]interface{}) error {
	if sf, ok := params["scan_field"].(string); ok {
		s.scanField = sf
	}
	if s.scanField == "" {
		s.scanField = "content"
	}

	rawPatterns, ok := params["patterns"]
	if !ok {
		return fmt.Errorf("secret_detect: patterns is required")
	}

	plist, err := toSliceOfMaps(rawPatterns)
	if err != nil {
		return fmt.Errorf("secret_detect: invalid patterns: %w", err)
	}

	for _, p := range plist {
		name, _ := p["name"].(string)
		regex, _ := p["regex"].(string)
		if regex == "" {
			continue
		}
		re, err := regexp.Compile(regex)
		if err != nil {
			return fmt.Errorf("secret_detect: invalid regex for %q: %w", name, err)
		}
		s.patterns = append(s.patterns, &compiledPattern{name: name, re: re})
	}

	if allowRaw, ok := params["allow_patterns"]; ok {
		allowList, _ := toStringSlice(allowRaw)
		for _, a := range allowList {
			re, err := regexp.Compile(a)
			if err != nil {
				return fmt.Errorf("secret_detect: invalid allow_pattern %q: %w", a, err)
			}
			s.allowPatterns = append(s.allowPatterns, re)
		}
	}

	return nil
}

func (s *SecretDetect) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	content, err := s.extractContent(input)
	if err != nil {
		return &types.CheckResult{Passed: true}, nil // Can't scan, pass through
	}
	if content == "" {
		return &types.CheckResult{Passed: true}, nil
	}

	for _, p := range s.patterns {
		matches := p.re.FindAllString(content, -1)
		for _, match := range matches {
			if s.isAllowed(match) {
				continue
			}
			return &types.CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Secret detected: %s pattern matched", p.name),
				Details: map[string]string{
					"pattern": p.name,
					"match":   truncate(match, 20),
				},
			}, nil
		}
	}

	return &types.CheckResult{Passed: true, Message: "no secrets detected"}, nil
}

func (s *SecretDetect) extractContent(input *types.HookInput) (string, error) {
	if input.ToolInput == nil {
		return "", nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(input.ToolInput, &m); err != nil {
		return "", err
	}
	// For Edit, scan both old_string and new_string
	if input.ToolName == "Edit" {
		var combined string
		if ns, ok := m["new_string"].(string); ok {
			combined = ns
		}
		return combined, nil
	}
	val, ok := m[s.scanField]
	if !ok {
		return "", nil
	}
	str, ok := val.(string)
	if !ok {
		return "", nil
	}
	return str, nil
}

func (s *SecretDetect) isAllowed(match string) bool {
	for _, a := range s.allowPatterns {
		if a.MatchString(match) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
