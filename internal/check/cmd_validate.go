package check

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/legostin/constitution/pkg/types"
)

// CmdValidate checks bash commands against deny/allow patterns.
type CmdValidate struct {
	denyPatterns  []*cmdPattern
	allowPatterns []*cmdPattern
}

type cmdPattern struct {
	name string
	re   *regexp.Regexp
}

func (c *CmdValidate) Name() string { return "cmd_validate" }

func (c *CmdValidate) Init(params map[string]interface{}) error {
	if denyRaw, ok := params["deny_patterns"]; ok {
		patterns, err := toSliceOfMaps(denyRaw)
		if err != nil {
			return fmt.Errorf("cmd_validate: invalid deny_patterns: %w", err)
		}
		for _, p := range patterns {
			name, _ := p["name"].(string)
			regex, _ := p["regex"].(string)
			if regex == "" {
				continue
			}
			ci, _ := p["case_insensitive"].(bool)
			if ci {
				regex = "(?i)" + regex
			}
			re, err := regexp.Compile(regex)
			if err != nil {
				return fmt.Errorf("cmd_validate: invalid regex for %q: %w", name, err)
			}
			c.denyPatterns = append(c.denyPatterns, &cmdPattern{name: name, re: re})
		}
	}

	if allowRaw, ok := params["allow_patterns"]; ok {
		patterns, err := toSliceOfMaps(allowRaw)
		if err != nil {
			return fmt.Errorf("cmd_validate: invalid allow_patterns: %w", err)
		}
		for _, p := range patterns {
			name, _ := p["name"].(string)
			regex, _ := p["regex"].(string)
			if regex == "" {
				continue
			}
			re, err := regexp.Compile(regex)
			if err != nil {
				return fmt.Errorf("cmd_validate: invalid allow regex for %q: %w", name, err)
			}
			c.allowPatterns = append(c.allowPatterns, &cmdPattern{name: name, re: re})
		}
	}

	return nil
}

func (c *CmdValidate) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	command := c.extractCommand(input)
	if command == "" {
		return &types.CheckResult{Passed: true}, nil
	}

	// Check allow patterns first (exceptions)
	for _, ap := range c.allowPatterns {
		if ap.re.MatchString(command) {
			return &types.CheckResult{Passed: true, Message: "allowed by exception: " + ap.name}, nil
		}
	}

	// Check deny patterns
	for _, dp := range c.denyPatterns {
		if dp.re.MatchString(command) {
			return &types.CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Command blocked: %s", dp.name),
				Details: map[string]string{
					"pattern": dp.name,
					"command": truncate(command, 100),
				},
			}, nil
		}
	}

	return &types.CheckResult{Passed: true, Message: "command allowed"}, nil
}

func (c *CmdValidate) extractCommand(input *types.HookInput) string {
	if input.ToolInput == nil {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(input.ToolInput, &m); err != nil {
		return ""
	}
	cmd, _ := m["command"].(string)
	return cmd
}
