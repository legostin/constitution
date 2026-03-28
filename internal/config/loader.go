package config

import (
	"os"
	"path/filepath"

	"github.com/legostin/constitution/pkg/types"
)

// DiscoverConfigSources finds all available config files across all authority levels.
// Returns sources ordered from highest authority (global) to lowest (project).
//
// Authority levels:
//
//	Level 0 (Global):     Reserved for model/platform developers. Not managed by constitution.
//	Level 1 (Enterprise): Reserved for LLM provider/platform. Not managed by constitution.
//	Level 2 (User):       ~/.config/constitution/constitution.yaml
//	Level 3 (Project):    {cwd}/.constitution.yaml or {cwd}/.claude/constitution.yaml
//
// Constitution manages levels 2 (User) and 3 (Project).
// Levels 0-1 exist in the type system for forward compatibility with platform-level
// rule injection, but constitution does not discover or create configs at these levels.
// The explicit parameter (--config flag) and $CONSTITUTION_CONFIG are treated as user level.
func DiscoverConfigSources(explicit, cwd string) []ConfigSource {
	var sources []ConfigSource

	// Levels 0-1 (Global/Enterprise) are reserved for platform use.
	// Constitution does not search for configs at these levels.

	// Level 2: User
	if home, err := os.UserHomeDir(); err == nil {
		userPath := filepath.Join(home, ".config", "constitution", "constitution.yaml")
		if fileExists(userPath) {
			sources = append(sources, ConfigSource{Path: userPath, Level: types.LevelUser})
		}
	}

	// Level 2: --config flag or $CONSTITUTION_CONFIG (user-level authority)
	if explicit != "" {
		if fileExists(explicit) {
			sources = append(sources, ConfigSource{Path: explicit, Level: types.LevelUser})
		}
	} else if envPath := os.Getenv("CONSTITUTION_CONFIG"); envPath != "" {
		if fileExists(envPath) {
			sources = append(sources, ConfigSource{Path: envPath, Level: types.LevelUser})
		}
	}

	// Level 3: Project
	projectCandidates := []string{
		filepath.Join(cwd, ".constitution.yaml"),
		filepath.Join(cwd, ".claude", "constitution.yaml"),
	}
	for _, c := range projectCandidates {
		if fileExists(c) {
			sources = append(sources, ConfigSource{Path: c, Level: types.LevelProject})
			break
		}
	}

	return sources
}

// FindConfigPath returns the single highest-priority config path.
// Deprecated: Use DiscoverConfigSources + LoadAll + MergePolicies instead.
func FindConfigPath(explicit, cwd string) string {
	sources := DiscoverConfigSources(explicit, cwd)
	if len(sources) == 0 {
		return ""
	}
	return sources[len(sources)-1].Path
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
