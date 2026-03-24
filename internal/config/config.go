package config

import (
	"fmt"
	"os"

	"github.com/legostin/constitution/pkg/types"
	"gopkg.in/yaml.v3"
)

// ConfigSource represents one discovered config file and its authority level.
type ConfigSource struct {
	Path  string
	Level types.ConfigLevel
}

// LayeredPolicy is a policy loaded from a specific config level.
type LayeredPolicy struct {
	Policy *types.Policy
	Level  types.ConfigLevel
	Path   string
}

// LoadAll loads policies from all discovered config sources.
// Each policy's rules are stamped with their source level and file path.
func LoadAll(sources []ConfigSource) ([]LayeredPolicy, []error) {
	var policies []LayeredPolicy
	var errs []error

	for _, src := range sources {
		policy, err := Load(src.Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("level %s (%s): %w", src.Level, src.Path, err))
			continue
		}
		for i := range policy.Rules {
			policy.Rules[i].Source = src.Level
			policy.Rules[i].SourceFile = src.Path
		}
		policies = append(policies, LayeredPolicy{
			Policy: policy,
			Level:  src.Level,
			Path:   src.Path,
		})
	}
	return policies, errs
}

// Load reads and parses a YAML policy file from the given path.
func Load(path string) (*types.Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses YAML bytes into a Policy.
func Parse(data []byte) (*types.Policy, error) {
	var policy types.Policy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if err := validate(&policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

func validate(p *types.Policy) error {
	if p.Version == "" {
		return fmt.Errorf("config: version is required")
	}
	seen := make(map[string]bool)
	for i, r := range p.Rules {
		if r.ID == "" {
			return fmt.Errorf("config: rule[%d] missing id", i)
		}
		if seen[r.ID] {
			return fmt.Errorf("config: duplicate rule id %q", r.ID)
		}
		seen[r.ID] = true
		if len(r.HookEvents) == 0 {
			return fmt.Errorf("config: rule %q has no hook_events", r.ID)
		}
		if r.Check.Type == "" {
			return fmt.Errorf("config: rule %q has no check type", r.ID)
		}
		switch r.Severity {
		case types.SeverityBlock, types.SeverityWarn, types.SeverityAudit:
			// ok
		default:
			return fmt.Errorf("config: rule %q has invalid severity %q", r.ID, r.Severity)
		}
	}
	return nil
}
