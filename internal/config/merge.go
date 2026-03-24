package config

import (
	"fmt"

	"github.com/legostin/constitution/pkg/types"
)

// MergeConflict describes an attempted override violation during merge.
type MergeConflict struct {
	RuleID      string
	Field       string // "enabled", "severity", etc.
	HigherLevel types.ConfigLevel
	LowerLevel  types.ConfigLevel
	HigherValue string
	LowerValue  string
	Action      string // "ignored" | "strengthened"
}

// MergeResult contains the merged policy and any conflicts detected.
type MergeResult struct {
	Policy    *types.Policy
	Conflicts []MergeConflict
}

// ruleEntry tracks a rule and its merge insertion order.
type ruleEntry struct {
	rule  types.Rule
	order int
}

// MergePolicies merges multiple layered policies into a single Policy.
// Layers must be ordered from highest authority (index 0) to lowest.
//
// Merge semantics:
//   - Settings: first non-empty value from highest authority wins
//   - Remote: highest level that defines it wins entirely
//   - Plugins: union by name, higher level wins on collision
//   - Rules (same ID): higher level cannot be weakened; lower can strengthen
//   - Rules (unique ID): added as-is from whatever level defines them
func MergePolicies(layers []LayeredPolicy) *MergeResult {
	if len(layers) == 0 {
		return &MergeResult{Policy: &types.Policy{Version: "1"}}
	}
	if len(layers) == 1 {
		return &MergeResult{Policy: layers[0].Policy}
	}

	result := &MergeResult{}

	// Merge rules
	ruleMap := make(map[string]*ruleEntry)
	orderCounter := 0

	for _, layer := range layers {
		for _, rule := range layer.Policy.Rules {
			existing, exists := ruleMap[rule.ID]
			if !exists {
				ruleMap[rule.ID] = &ruleEntry{rule: rule, order: orderCounter}
				orderCounter++
				continue
			}

			// Rule already defined by a higher-authority level.
			// Lower level cannot weaken it.

			// Cannot disable a rule that higher level enabled
			if existing.rule.Enabled && !rule.Enabled {
				result.Conflicts = append(result.Conflicts, MergeConflict{
					RuleID:      rule.ID,
					Field:       "enabled",
					HigherLevel: existing.rule.Source,
					LowerLevel:  rule.Source,
					HigherValue: "true",
					LowerValue:  "false",
					Action:      "ignored",
				})
			}

			// Severity: cannot weaken, can strengthen
			higherRank := types.SeverityRank(existing.rule.Severity)
			lowerRank := types.SeverityRank(rule.Severity)
			if lowerRank < higherRank {
				result.Conflicts = append(result.Conflicts, MergeConflict{
					RuleID:      rule.ID,
					Field:       "severity",
					HigherLevel: existing.rule.Source,
					LowerLevel:  rule.Source,
					HigherValue: string(existing.rule.Severity),
					LowerValue:  string(rule.Severity),
					Action:      "ignored",
				})
			} else if lowerRank > higherRank {
				result.Conflicts = append(result.Conflicts, MergeConflict{
					RuleID:      rule.ID,
					Field:       "severity",
					HigherLevel: existing.rule.Source,
					LowerLevel:  rule.Source,
					HigherValue: string(existing.rule.Severity),
					LowerValue:  string(rule.Severity),
					Action:      "strengthened",
				})
				existing.rule.Severity = rule.Severity
			}

			// Priority: lower level can make more urgent (lower number) but not less
			if rule.Priority < existing.rule.Priority {
				existing.rule.Priority = rule.Priority
			}
		}
	}

	// Collect rules in insertion order
	rules := make([]types.Rule, 0, len(ruleMap))
	ordered := make([]*ruleEntry, 0, len(ruleMap))
	for _, entry := range ruleMap {
		ordered = append(ordered, entry)
	}
	sortByOrder(ordered)
	for _, entry := range ordered {
		rules = append(rules, entry.rule)
	}

	result.Policy = &types.Policy{
		Version:  layers[0].Policy.Version,
		Name:     mergeName(layers),
		Settings: mergeSettings(layers),
		Remote:   mergeRemote(layers),
		Plugins:  mergePlugins(layers),
		Rules:    rules,
	}

	return result
}

func mergeSettings(layers []LayeredPolicy) types.Settings {
	var s types.Settings
	for _, layer := range layers {
		ls := layer.Policy.Settings
		if s.LogLevel == "" && ls.LogLevel != "" {
			s.LogLevel = ls.LogLevel
		}
		if s.LogFile == "" && ls.LogFile != "" {
			s.LogFile = ls.LogFile
		}
	}
	return s
}

func mergeRemote(layers []LayeredPolicy) types.RemoteConfig {
	for _, layer := range layers {
		if layer.Policy.Remote.Enabled || layer.Policy.Remote.URL != "" {
			return layer.Policy.Remote
		}
	}
	return types.RemoteConfig{}
}

func mergePlugins(layers []LayeredPolicy) []types.PluginConfig {
	seen := make(map[string]bool)
	var result []types.PluginConfig
	for _, layer := range layers {
		for _, p := range layer.Policy.Plugins {
			if !seen[p.Name] {
				seen[p.Name] = true
				result = append(result, p)
			}
		}
	}
	return result
}

func mergeName(layers []LayeredPolicy) string {
	names := make([]string, 0, len(layers))
	for _, l := range layers {
		if l.Policy.Name != "" {
			names = append(names, fmt.Sprintf("%s(%s)", l.Policy.Name, l.Level))
		}
	}
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return layers[0].Policy.Name
	}
	return names[0] // Use highest-authority name
}

func sortByOrder(entries []*ruleEntry) {
	// Simple insertion sort — rule count is always small
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].order < entries[j-1].order; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}
