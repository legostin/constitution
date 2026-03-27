package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/legostin/constitution/internal/config"
	"github.com/legostin/constitution/pkg/types"
	"gopkg.in/yaml.v3"
)

// loadLocalConfig finds and loads the project-level .constitution.yaml.
// Returns the policy, config file path, and error.
func loadLocalConfig() (*types.Policy, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("не удалось определить рабочую директорию: %w", err)
	}

	candidates := []string{
		filepath.Join(cwd, ".constitution.yaml"),
		filepath.Join(cwd, ".claude", "constitution.yaml"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			policy, err := config.Load(path)
			if err != nil {
				return nil, path, fmt.Errorf("ошибка загрузки %s: %w", path, err)
			}
			return policy, path, nil
		}
	}

	return nil, "", fmt.Errorf("файл .constitution.yaml не найден. Запустите 'constitution init'")
}

// saveConfig backs up the old config and writes the new one.
func saveConfig(path string, policy *types.Policy) error {
	// Backup
	if data, err := os.ReadFile(path); err == nil {
		_ = os.WriteFile(path+".bak", data, 0o644)
	}

	data, err := yaml.Marshal(policy)
	if err != nil {
		return fmt.Errorf("ошибка сериализации: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("ошибка записи %s: %w", path, err)
	}
	return nil
}

// previewRuleYAML prints a YAML representation of a single rule to stderr.
func previewRuleYAML(rule types.Rule) {
	// Wrap in a slice to get the "- id:" list format
	wrapper := struct {
		Rules []types.Rule `yaml:"rules"`
	}{Rules: []types.Rule{rule}}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		printError(fmt.Sprintf("Ошибка сериализации: %v", err))
		return
	}

	fmt.Fprintf(os.Stderr, "\n  \033[2m── Превью ──────────────────────────────────────\033[0m\n")
	for _, line := range splitLines(string(data)) {
		// Skip the "rules:" wrapper line
		if line == "rules:" {
			continue
		}
		fmt.Fprintf(os.Stderr, "  %s\n", line)
	}
	fmt.Fprintf(os.Stderr, "  \033[2m── Конец превью ────────────────────────────────\033[0m\n\n")
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range filepath.SplitList(s) {
		lines = append(lines, line)
	}
	// filepath.SplitList uses OS path separator, use strings instead
	result := []string{}
	start := 0
	for i, c := range s {
		if c == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
