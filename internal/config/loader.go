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
//	Level 0 (Global):     $CONSTITUTION_GLOBAL_CONFIG or /etc/constitution/global.yaml
//	Level 1 (Enterprise): $CONSTITUTION_ENTERPRISE_CONFIG
//	Level 2 (User):       ~/.config/constitution/constitution.yaml
//	Level 3 (Project):    {cwd}/.constitution.yaml or {cwd}/.claude/constitution.yaml
//
// The explicit parameter (--config flag) and $CONSTITUTION_CONFIG are treated as user level.
func DiscoverConfigSources(explicit, cwd string) []ConfigSource {
	var sources []ConfigSource

	// Level 0: Global
	if globalPath := os.Getenv("CONSTITUTION_GLOBAL_CONFIG"); globalPath != "" {
		if fileExists(globalPath) {
			sources = append(sources, ConfigSource{Path: globalPath, Level: types.LevelGlobal})
		}
	} else if fileExists("/etc/constitution/global.yaml") {
		sources = append(sources, ConfigSource{Path: "/etc/constitution/global.yaml", Level: types.LevelGlobal})
	}

	// Level 1: Enterprise / Organization
	if enterprise := os.Getenv("CONSTITUTION_ENTERPRISE_CONFIG"); enterprise != "" {
		if fileExists(enterprise) {
			sources = append(sources, ConfigSource{Path: enterprise, Level: types.LevelEnterprise})
		}
	}

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
