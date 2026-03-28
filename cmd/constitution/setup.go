package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type hookEntry struct {
	Matcher string     `json:"matcher"`
	Hooks   []hookDef  `json:"hooks"`
}

type hookDef struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

func cmdSetup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	scope := fs.String("scope", "", "Settings scope: user, project (default: interactive)")
	remote := fs.String("remote", "", "Quick setup: create remote config + install hooks")
	all := fs.Bool("all", false, "Select all default hooks (skip interactive checklist)")
	platform := fs.String("platform", "claude", "Target platform: claude, codex")
	fs.Parse(args)

	fmt.Fprintln(os.Stderr)

	// Quick remote setup
	if *remote != "" {
		quickRemoteSetup(*remote)
		return
	}

	// ── Select hooks ──
	items := []checklistItem{
		{"SessionStart", "repo validation, skill injection", true},
		{"UserPromptSubmit", "prompt modification, safety context", true},
		{"PreToolUse: Bash", "command validation, block dangerous commands", true},
		{"PreToolUse: Files", "secret detection, directory ACL (Read/Write/Edit)", true},
		{"PreToolUse: Search", "directory access control (Glob/Grep)", true},
		{"PostToolUse: Files", "run linters after file changes (Write/Edit)", false},
		{"Stop", "final validation before agent stops", true},
	}

	if !*all {
		items = checklist("Select hooks to install:", items)
	}
	fmt.Fprintln(os.Stderr)

	selected := 0
	for _, item := range items {
		if item.Selected {
			selected++
		}
	}
	if selected == 0 {
		fmt.Fprintln(os.Stderr, "  No hooks selected.")
		return
	}
	fmt.Fprintf(os.Stderr, "  \033[32m%d hook(s) selected\033[0m\n\n", selected)

	// ── Build hooks JSON ──
	var hooks map[string]interface{}
	if *platform == "codex" {
		hooks = buildCodexHooksJSON(items)
	} else {
		hooks = buildHooksJSON(items)
	}

	// ── Select scope ──
	var settingsFile string
	isCodex := *platform == "codex"
	platformDir := ".claude"
	configFile := "settings.json"
	if isCodex {
		platformDir = ".codex"
		configFile = "hooks.json"
	}

	if *scope != "" {
		switch *scope {
		case "user":
			settingsFile = filepath.Join(homeDir(), platformDir, configFile)
		case "project":
			settingsFile = filepath.Join(platformDir, configFile)
		}
	} else {
		userPath := filepath.Join("~", platformDir, configFile)
		projPath := filepath.Join(platformDir, configFile)
		idx := promptChoice("Where to install hooks?", []string{
			fmt.Sprintf("User-level   (%s) — all projects", userPath),
			fmt.Sprintf("Project-level (%s)  — this project", projPath),
			"Print only   — show JSON, don't write",
		}, 0)
		switch idx {
		case 0:
			settingsFile = filepath.Join(homeDir(), platformDir, configFile)
		case 1:
			settingsFile = filepath.Join(platformDir, configFile)
		case 2:
			settingsFile = ""
		}
	}

	fmt.Fprintln(os.Stderr)

	// ── Apply ──
	hooksWrapped := map[string]interface{}{"hooks": hooks}
	pretty, _ := json.MarshalIndent(hooksWrapped, "", "  ")
	fmt.Fprintf(os.Stderr, "%s\n\n", pretty)

	if settingsFile == "" {
		fmt.Fprintf(os.Stderr, "  Paste the above into your %s config\n", *platform)
		return
	}

	if isCodex {
		applyHooksCodex(settingsFile, hooks)
	} else {
		applyHooks(settingsFile, hooks)
	}

	// Offer to install skills
	fmt.Fprintln(os.Stderr)
	platformLabel := "Claude Code"
	if isCodex {
		platformLabel = "Codex"
	}
	if promptYN(fmt.Sprintf("Install %s skills (/constitution)?", platformLabel), true) {
		skillScope := "project"
		if strings.Contains(settingsFile, homeDir()) {
			skillScope = "user"
		}
		installArgs := []string{"--scope", skillScope}
		if isCodex {
			installArgs = append(installArgs, "--platform", "codex")
		}
		cmdSkillInstall(installArgs)
	}

	if isCodex {
		printHint("Enable hooks in Codex: set codex_hooks = true in config.toml")
	}
}

func quickRemoteSetup(remoteURL string) {
	// 1. Create remote config if no config exists
	if _, err := os.Stat(".constitution.yaml"); os.IsNotExist(err) {
		content := strings.ReplaceAll(remoteConfigTemplate, "{{REMOTE_URL}}", remoteURL)
		if err := os.WriteFile(".constitution.yaml", []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "  Error writing .constitution.yaml: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m .constitution.yaml created (remote: %s)\n", remoteURL)
	}

	// 2. Install all default hooks to user settings
	items := []checklistItem{
		{"SessionStart", "", true},
		{"UserPromptSubmit", "", true},
		{"PreToolUse: Bash", "", true},
		{"PreToolUse: Files", "", true},
		{"PreToolUse: Search", "", true},
		{"PostToolUse: Files", "", false},
		{"Stop", "", false},
	}
	hooks := buildHooksJSON(items)
	settingsFile := filepath.Join(homeDir(), ".claude", "settings.json")
	applyHooks(settingsFile, hooks)
}

func buildHooksJSON(items []checklistItem) map[string]interface{} {
	hooks := make(map[string]interface{})

	entry := func(matcher string, timeout int) hookEntry {
		return hookEntry{
			Matcher: matcher,
			Hooks:   []hookDef{{Type: "command", Command: "constitution", Timeout: timeout}},
		}
	}

	if items[0].Selected { // SessionStart
		hooks["SessionStart"] = []hookEntry{entry("", 5)}
	}
	if items[1].Selected { // UserPromptSubmit
		hooks["UserPromptSubmit"] = []hookEntry{entry("", 5)}
	}

	var pre []hookEntry
	if items[2].Selected { // PreToolUse: Bash
		pre = append(pre, entry("Bash", 5))
	}
	if items[3].Selected { // PreToolUse: Files
		pre = append(pre, entry("Read|Write|Edit", 5))
	}
	if items[4].Selected { // PreToolUse: Search
		pre = append(pre, entry("Glob|Grep", 3))
	}
	if len(pre) > 0 {
		hooks["PreToolUse"] = pre
	}

	if items[5].Selected { // PostToolUse: Files
		hooks["PostToolUse"] = []hookEntry{entry("Write|Edit", 60)}
	}
	if items[6].Selected { // Stop
		hooks["Stop"] = []hookEntry{entry("", 180)}
	}

	return hooks
}

// buildCodexHooksJSON builds hooks for OpenAI Codex.
// Codex currently only supports Bash tool.
func buildCodexHooksJSON(items []checklistItem) map[string]interface{} {
	hooks := make(map[string]interface{})

	type codexHookDef struct {
		Type          string `json:"type"`
		Command       string `json:"command"`
		Timeout       int    `json:"timeout"`
		StatusMessage string `json:"statusMessage,omitempty"`
	}
	type codexHookEntry struct {
		Matcher string         `json:"matcher,omitempty"`
		Hooks   []codexHookDef `json:"hooks"`
	}

	entry := func(matcher, status string, timeout int) codexHookEntry {
		return codexHookEntry{
			Matcher: matcher,
			Hooks:   []codexHookDef{{Type: "command", Command: "constitution", Timeout: timeout, StatusMessage: status}},
		}
	}

	if items[0].Selected { // SessionStart
		hooks["SessionStart"] = []codexHookEntry{entry("", "Checking constitution rules", 5)}
	}
	if items[1].Selected { // UserPromptSubmit
		hooks["UserPromptSubmit"] = []codexHookEntry{entry("", "Validating prompt", 5)}
	}

	// Codex only supports Bash tool currently
	if items[2].Selected || items[3].Selected || items[4].Selected { // Any PreToolUse
		hooks["PreToolUse"] = []codexHookEntry{entry("Bash", "Validating command", 5)}
	}

	if items[5].Selected { // PostToolUse
		hooks["PostToolUse"] = []codexHookEntry{entry("Bash", "Checking output", 60)}
	}
	if items[6].Selected { // Stop
		hooks["Stop"] = []codexHookEntry{entry("", "Running completion checks", 180)}
	}

	return hooks
}

// applyHooksCodex writes a standalone hooks.json for Codex.
func applyHooksCodex(hooksFile string, hooks map[string]interface{}) {
	os.MkdirAll(filepath.Dir(hooksFile), 0o755)

	// Codex hooks.json is standalone — just {"hooks": {...}}
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(hooksFile); err == nil {
		json.Unmarshal(data, &existing)
	}

	// Remove old constitution hooks and merge new ones
	existingHooks, _ := existing["hooks"].(map[string]interface{})
	if existingHooks == nil {
		existingHooks = make(map[string]interface{})
	}

	for event, entries := range hooks {
		existingHooks[event] = entries
	}

	existing["hooks"] = existingHooks

	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(hooksFile, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError writing %s: %v\033[0m\n", hooksFile, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m Hooks written to %s\n", hooksFile)
	fmt.Fprintln(os.Stderr, "  Restart Codex for hooks to take effect.")
	fmt.Fprintln(os.Stderr)
}

func applyHooks(settingsFile string, hooks map[string]interface{}) {
	// Read existing settings
	os.MkdirAll(filepath.Dir(settingsFile), 0o755)
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(settingsFile); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to parse %s: %v\n", settingsFile, err)
			return
		}
	}

	// Remove existing constitution hooks, then add new ones
	existingHooks, _ := existing["hooks"].(map[string]interface{})
	if existingHooks == nil {
		existingHooks = make(map[string]interface{})
	}

	for event, entries := range hooks {
		// Filter out existing constitution hooks for this event
		var kept []interface{}
		if arr, ok := existingHooks[event].([]interface{}); ok {
			for _, e := range arr {
				if m, ok := e.(map[string]interface{}); ok {
					if innerHooks, ok := m["hooks"].([]interface{}); ok {
						isConstitution := false
						for _, h := range innerHooks {
							if hm, ok := h.(map[string]interface{}); ok {
								if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "constitution") {
									isConstitution = true
								}
							}
						}
						if !isConstitution {
							kept = append(kept, e)
						}
					}
				}
			}
		}

		// Append new hooks
		newEntries, _ := entries.([]hookEntry)
		for _, ne := range newEntries {
			kept = append(kept, ne)
		}
		existingHooks[event] = kept
	}

	existing["hooks"] = existingHooks

	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(settingsFile, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31mError writing %s: %v\033[0m\n", settingsFile, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m Hooks written to %s\n", settingsFile)
	fmt.Fprintln(os.Stderr, "  Restart Claude Code for hooks to take effect.")
	fmt.Fprintln(os.Stderr)
}

func cmdUninstall(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	scope := fs.String("scope", "user", "Settings scope: user, project")
	fs.Parse(args)

	var settingsFile string
	switch *scope {
	case "project":
		settingsFile = filepath.Join(".claude", "settings.json")
	default:
		settingsFile = filepath.Join(homeDir(), ".claude", "settings.json")
	}

	data, err := os.ReadFile(settingsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  No settings file found: %s\n", settingsFile)
		return
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Fprintf(os.Stderr, "  Error parsing %s: %v\n", settingsFile, err)
		return
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "  No hooks found.")
		return
	}

	removed := 0
	for event, entries := range hooks {
		arr, ok := entries.([]interface{})
		if !ok {
			continue
		}
		var kept []interface{}
		for _, e := range arr {
			m, ok := e.(map[string]interface{})
			if !ok {
				kept = append(kept, e)
				continue
			}
			innerHooks, ok := m["hooks"].([]interface{})
			if !ok {
				kept = append(kept, e)
				continue
			}
			isConstitution := false
			for _, h := range innerHooks {
				if hm, ok := h.(map[string]interface{}); ok {
					if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "constitution") {
						isConstitution = true
					}
				}
			}
			if isConstitution {
				removed++
			} else {
				kept = append(kept, e)
			}
		}
		if len(kept) > 0 {
			hooks[event] = kept
		} else {
			delete(hooks, event)
		}
	}

	if removed == 0 {
		fmt.Fprintln(os.Stderr, "  No constitution hooks found.")
		return
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	out, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsFile, out, 0o644)

	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m Removed %d constitution hook(s) from %s\n", removed, settingsFile)
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}
