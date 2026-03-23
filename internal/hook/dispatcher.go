package hook

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/legostin/constitution/pkg/types"
)

// FilterRules returns rules applicable to the given hook event and tool name.
// Rules are sorted by priority (ascending).
func FilterRules(rules []types.Rule, eventName, toolName string) []types.Rule {
	var matched []types.Rule
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if !matchesEvent(r.HookEvents, eventName) {
			continue
		}
		if len(r.ToolMatch) > 0 && toolName != "" {
			if !matchesTool(r.ToolMatch, toolName) {
				continue
			}
		}
		matched = append(matched, r)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Priority < matched[j].Priority
	})
	return matched
}

func matchesEvent(events []string, target string) bool {
	for _, e := range events {
		if e == target {
			return true
		}
	}
	return false
}

func matchesTool(patterns []string, toolName string) bool {
	for _, p := range patterns {
		if p == toolName {
			return true
		}
		// Try as regex
		re, err := regexp.Compile(fmt.Sprintf("^%s$", p))
		if err != nil {
			continue
		}
		if re.MatchString(toolName) {
			return true
		}
	}
	return false
}
