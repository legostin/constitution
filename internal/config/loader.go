package config

import (
	"os"
	"path/filepath"
)

// FindConfigPath resolves the configuration file path using priority order:
//  1. Explicit path (from --config flag)
//  2. $CONSTITUTION_CONFIG env var
//  3. {cwd}/.constitution.yaml
//  4. {cwd}/.claude/constitution.yaml
//  5. ~/.config/constitution/constitution.yaml
//  6. $CONSTITUTION_ENTERPRISE_CONFIG env var (enterprise admin)
func FindConfigPath(explicit, cwd string) string {
	if explicit != "" {
		return explicit
	}

	if envPath := os.Getenv("CONSTITUTION_CONFIG"); envPath != "" {
		if fileExists(envPath) {
			return envPath
		}
	}

	candidates := []string{
		filepath.Join(cwd, ".constitution.yaml"),
		filepath.Join(cwd, ".claude", "constitution.yaml"),
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "constitution", "constitution.yaml"))
	}

	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}

	// Enterprise config (set by org admin via Claude Code enterprise settings)
	if enterprise := os.Getenv("CONSTITUTION_ENTERPRISE_CONFIG"); enterprise != "" {
		if fileExists(enterprise) {
			return enterprise
		}
	}

	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
