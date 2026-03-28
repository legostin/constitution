package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/legostin/constitution/pkg/types"
	"gopkg.in/yaml.v3"
)

type hookEntry struct {
	Matcher string    `json:"matcher"`
	Hooks   []hookDef `json:"hooks"`
}

type hookDef struct {
	Type          string `json:"type"`
	Command       string `json:"command"`
	Timeout       int    `json:"timeout"`
	StatusMessage string `json:"statusMessage,omitempty"`
}

// ─── Pre-built security rules ───────────────────────────────────────

var securityRuleItems = []struct {
	label string
	desc  string
	on    bool
	rules []types.Rule
}{
	{"Block reading secret files", ".env, .pem, .key, credentials.json", true, []types.Rule{{
		ID: "secret-read", Name: "Block Secret Files", Enabled: true, Priority: 1, Severity: types.SeverityBlock,
		HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Read"},
		Check: types.CheckConfig{Type: "dir_acl", Params: map[string]interface{}{
			"mode": "denylist", "path_field": "file_path",
			"patterns": []interface{}{"**/.env", "**/.env.*", "**/credentials.json", "**/service-account*.json", "**/*.pem", "**/*.key"},
		}},
	}}},
	{"Detect secrets in file writes", "AWS keys, GitHub tokens, private keys, JWT", true, []types.Rule{{
		ID: "secret-write", Name: "Secret Detection", Enabled: true, Priority: 1, Severity: types.SeverityBlock,
		HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Write", "Edit"},
		Check: types.CheckConfig{Type: "secret_regex", Params: map[string]interface{}{
			"scan_field": "content",
			"patterns": []interface{}{
				map[string]interface{}{"name": "AWS Access Key", "regex": "AKIA[0-9A-Z]{16}"},
				map[string]interface{}{"name": "GitHub Token", "regex": "gh[ps]_[A-Za-z0-9_]{36,}"},
				map[string]interface{}{"name": "Private Key", "regex": "-----BEGIN (RSA|EC|DSA|OPENSSH) PRIVATE KEY-----"},
				map[string]interface{}{"name": "JWT Token", "regex": "eyJ[A-Za-z0-9_-]{10,}\\.eyJ[A-Za-z0-9_-]{10,}\\.[A-Za-z0-9_\\-]+"},
			},
			"allow_patterns": []interface{}{"AKIAIOSFODNN7EXAMPLE", "(?i)test|example|dummy|placeholder"},
		}},
	}}},
	{"Block dangerous commands", "rm -rf /, chmod 777, curl|bash, force push, hard reset", true, []types.Rule{{
		ID: "cmd-validate", Name: "Command Validation", Enabled: true, Priority: 1, Severity: types.SeverityBlock,
		HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Bash"},
		Check: types.CheckConfig{Type: "cmd_validate", Params: map[string]interface{}{
			"deny_patterns": []interface{}{
				map[string]interface{}{"name": "Root deletion", "regex": "rm\\s+-rf\\s+/"},
				map[string]interface{}{"name": "World-writable permissions", "regex": "chmod\\s+777"},
				map[string]interface{}{"name": "Pipe to shell", "regex": "curl.*\\|\\s*(bash|sh|zsh)"},
				map[string]interface{}{"name": "Hard reset", "regex": "\\bgit\\s+reset\\s+--hard"},
				map[string]interface{}{"name": "Drop database", "regex": "\\bdrop\\s+database\\b", "case_insensitive": true},
			},
		}},
	}}},
	{"Block access to system directories", "/etc, ~/.ssh, ~/.aws", true, []types.Rule{{
		ID: "dir-acl", Name: "Directory ACL", Enabled: true, Priority: 2, Severity: types.SeverityBlock,
		HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Read", "Write", "Edit", "Glob", "Grep"},
		Check: types.CheckConfig{Type: "dir_acl", Params: map[string]interface{}{
			"mode": "denylist", "path_field": "auto",
			"patterns":             []interface{}{"/etc/**", "/var/**", "~/.ssh/**", "~/.aws/**", "~/.config/gcloud/**", "../**"},
			"allow_within_project": true,
		}},
	}}},
	{"Block push to protected branches", "main, master, develop", false, []types.Rule{{
		ID: "no-protected-push", Name: "Block Protected Branch Push", Enabled: true, Priority: 1, Severity: types.SeverityBlock,
		HookEvents: []string{"PreToolUse"}, ToolMatch: []string{"Bash"},
		Check: types.CheckConfig{Type: "cmd_validate", Params: map[string]interface{}{
			"deny_patterns": []interface{}{
				map[string]interface{}{"name": "Push to protected branch", "regex": "\\bgit\\s+push\\s+\\S+\\s+(main|master|develop|release)\\b"},
			},
		}},
	}}},
	{"Repository allowlist", "restrict to specific repos", false, []types.Rule{{
		ID: "repo-access", Name: "Repository Allowlist", Enabled: true, Priority: 1, Severity: types.SeverityBlock,
		HookEvents: []string{"SessionStart"},
		Check: types.CheckConfig{Type: "repo_access", Params: map[string]interface{}{
			"mode": "allowlist", "patterns": []interface{}{"github.com/your-org/*"}, "detect_from": "git_remote",
		}},
	}}},
}

var stopCheckItems = []struct {
	label string
	desc  string
	on    bool
	rule  types.Rule
}{
	{"No uncommitted changes", "git diff --quiet", true, types.Rule{
		ID: "stop-committed", Name: "No Uncommitted Changes", Enabled: true, Priority: 3, Severity: types.SeverityBlock,
		HookEvents: []string{"Stop"}, Message: "Uncommitted changes. Commit before stopping.",
		Check: types.CheckConfig{Type: "cmd_check", Params: map[string]interface{}{
			"command": "git diff --quiet && git diff --cached --quiet", "working_dir": "project", "timeout": 5000,
		}},
	}},
	{"Branch pushed to remote", "git push verification", false, types.Rule{
		ID: "stop-pushed", Name: "Branch Must Be Pushed", Enabled: true, Priority: 4, Severity: types.SeverityBlock,
		HookEvents: []string{"Stop"}, Message: "Branch not pushed.",
		Check: types.CheckConfig{Type: "cmd_check", Params: map[string]interface{}{
			"command": "BRANCH=$(git rev-parse --abbrev-ref HEAD); git fetch origin $BRANCH --quiet 2>/dev/null; git diff --quiet $BRANCH..origin/$BRANCH 2>/dev/null", "working_dir": "project", "timeout": 10000,
		}},
	}},
	{"PR exists for current branch", "skip on main/master", false, types.Rule{
		ID: "stop-pr-exists", Name: "PR Must Exist", Enabled: true, Priority: 5, Severity: types.SeverityBlock,
		HookEvents: []string{"Stop"}, Message: "No PR found. Create a PR before stopping.",
		Check: types.CheckConfig{Type: "cmd_check", Params: map[string]interface{}{
			"command": "BRANCH=$(git rev-parse --abbrev-ref HEAD); if [ \"$BRANCH\" = 'main' ] || [ \"$BRANCH\" = 'master' ]; then exit 0; fi; gh pr view --json state -q '.state' 2>/dev/null | grep -qE 'OPEN|MERGED'", "working_dir": "project", "timeout": 10000,
		}},
	}},
}

// ─── The Setup Wizard ───────────────────────────────────────────────

func cmdSetup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	scope := fs.String("scope", "", "Scope: user, project")
	platform := fs.String("platform", "", "Platform: claude, codex, both")
	workflow := fs.String("workflow", "", "Orchestration: autonomous, plan-first, ooda-loop, ralph-loop, strict-security")
	security := fs.String("security", "", "Security preset: all, minimal, none")
	yes := fs.Bool("yes", false, "Accept all defaults (non-interactive)")
	fs.Parse(args)

	fmt.Fprintf(os.Stderr, "\n\033[1mConstitution Setup\033[0m\n\n")

	// ── Step 1: Platform ──
	platforms := wizardPlatform(*platform, *yes)

	// ── Step 2: Remote server ──
	remoteURL, remoteToken := wizardRemote(*yes)

	// ── Step 3: Security rules ──
	rules := wizardSecurity(*security, *yes)

	// ── Step 4: Orchestration pattern ──
	patternRules := wizardOrchestration(*workflow, *yes)
	rules = append(rules, patternRules...)

	// ── Step 5: Stop validation ──
	stopInstructions, gitRules := wizardStop(*yes)
	if stopInstructions != "" {
		rules = append(rules, types.Rule{
			ID: "stop-validation", Name: "Stop Validation", Enabled: true, Priority: 10, Severity: types.SeverityBlock,
			HookEvents: []string{"Stop"},
			Message:    stopInstructions,
			Check: types.CheckConfig{Type: "cel", Params: map[string]interface{}{
				"expression": `hook_event_name == "Stop" && !last_assistant_message.contains("VERIFIED_COMPLETE")`,
			}},
		})
	}
	rules = append(rules, gitRules...)

	// ── Step 6: Skills ──
	skillPlatforms := wizardSkills(platforms, *yes)

	// ── Step 7: Install ──
	wizardInstall(platforms, rules, remoteURL, remoteToken, *scope, *yes, skillPlatforms)
}

// ─── Wizard Steps ───────────────────────────────────────────────────

func wizardPlatform(flag string, yes bool) []string {
	if flag != "" {
		if flag == "both" {
			return []string{"claude", "codex"}
		}
		return []string{flag}
	}
	if yes {
		return []string{"claude"}
	}

	printSection("Step 1: Platform")
	idx := promptChoice("Which AI agent platform do you use?", []string{
		"Claude Code",
		"OpenAI Codex",
		"Both",
	}, 0)
	switch idx {
	case 1:
		return []string{"codex"}
	case 2:
		return []string{"claude", "codex"}
	default:
		return []string{"claude"}
	}
}

func wizardRemote(yes bool) (url, tokenEnv string) {
	if yes {
		return "", ""
	}

	printSection("Step 2: Remote Server")
	printHint("A remote server provides centralized rules for your team.")
	printHint("Leave empty to skip (local rules only).")
	fmt.Fprintln(os.Stderr)

	url = promptString("Remote server URL", "")
	if url == "" {
		return "", ""
	}
	tokenEnv = promptString("Auth token env var", "CONSTITUTION_TOKEN")
	return
}

func wizardSecurity(flag string, yes bool) []types.Rule {
	if flag == "none" {
		return nil
	}
	if flag == "all" || yes {
		var rules []types.Rule
		for _, item := range securityRuleItems {
			if item.on {
				rules = append(rules, item.rules...)
			}
		}
		return rules
	}
	if flag == "minimal" {
		// Just secrets + commands
		var rules []types.Rule
		for _, item := range securityRuleItems[:3] {
			rules = append(rules, item.rules...)
		}
		return rules
	}

	printSection("Step 3: Security Rules")

	var items []checklistItem
	for _, sr := range securityRuleItems {
		items = append(items, checklistItem{sr.label, sr.desc, sr.on})
	}
	items = checklist("Select protections:", items)

	var rules []types.Rule
	for i, item := range items {
		if item.Selected {
			rules = append(rules, securityRuleItems[i].rules...)
		}
	}
	return rules
}

func wizardOrchestration(flag string, yes bool) []types.Rule {
	if flag != "" {
		return loadWorkflowRules(flag)
	}
	if yes {
		return nil
	}

	printSection("Step 4: Orchestration Pattern")

	idx := promptChoice("Apply an orchestration pattern?", []string{
		"None           — just security rules",
		"Autonomous     — full autonomy, self-critique, guardrails",
		"Plan-First     — plan -> execute -> test workflow",
		"OODA Loop      — observe -> orient -> decide -> act cycle",
		"Ralph Loop     — continuous autonomous loop until PRD complete",
		"Autoproduct    — spec-driven autonomous dev (inspired by Karpathy's autoresearch)",
		"Strict Security — maximum protection, extended blocklists",
	}, 0)

	names := []string{"", "autonomous", "plan-first", "ooda-loop", "ralph-loop", "autoproduct", "strict-security"}
	if idx == 0 {
		return nil
	}
	return loadWorkflowRules(names[idx])
}

func loadWorkflowRules(name string) []types.Rule {
	tmpl, ok := workflowTemplates[name]
	if !ok {
		printError(fmt.Sprintf("Unknown workflow: %s", name))
		return nil
	}
	var policy types.Policy
	if err := yaml.Unmarshal([]byte(tmpl), &policy); err != nil {
		printError(fmt.Sprintf("Failed to parse workflow template: %v", err))
		return nil
	}
	// Only return orchestration rules (skill_inject, prompt_modify), not security ones
	// to avoid duplicates with step 3
	var rules []types.Rule
	for _, r := range policy.Rules {
		if r.Check.Type == "skill_inject" || r.Check.Type == "prompt_modify" {
			rules = append(rules, r)
		}
	}
	return rules
}

func wizardStop(yes bool) (instructions string, gitRules []types.Rule) {
	if yes {
		return "Verify the project builds and all tests pass. Commit all changes. Include VERIFIED_COMPLETE in your final message.", nil
	}

	printSection("Step 5: Stop Validation")
	printHint("What should the agent verify before stopping?")
	printHint("The agent will figure out the right commands for your stack.")
	fmt.Fprintln(os.Stderr)

	defaultInstr := "Verify the project builds and all tests pass.\nCommit all changes.\nInclude VERIFIED_COMPLETE in your final message."
	printHint("Default: " + strings.ReplaceAll(defaultInstr, "\n", " "))
	fmt.Fprintln(os.Stderr)

	choice := promptChoice("Stop validation:", []string{
		"Use default instructions",
		"Enter custom instructions",
		"Skip — no stop checks",
	}, 0)

	switch choice {
	case 0:
		instructions = defaultInstr
	case 1:
		instructions = promptMultiline("Enter stop instructions:")
		if instructions != "" {
			instructions += "\nInclude VERIFIED_COMPLETE in your final message."
		}
	case 2:
		return "", nil
	}

	// Git checks
	fmt.Fprintln(os.Stderr)
	var gitItems []checklistItem
	for _, sc := range stopCheckItems {
		gitItems = append(gitItems, checklistItem{sc.label, sc.desc, sc.on})
	}
	gitItems = checklist("Git checks before stopping:", gitItems)

	for i, item := range gitItems {
		if item.Selected {
			gitRules = append(gitRules, stopCheckItems[i].rule)
		}
	}

	return
}

func wizardSkills(platforms []string, yes bool) []string {
	if yes {
		return platforms
	}

	printSection("Step 6: Skills")
	printHint("Skills add /constitution slash command to your agent.")
	fmt.Fprintln(os.Stderr)

	var items []checklistItem
	for _, p := range platforms {
		label := "Claude Code"
		if p == "codex" {
			label = "OpenAI Codex"
		}
		items = append(items, checklistItem{label, fmt.Sprintf(".%s/skills/", p[:5]+"e"[:min(1, 5-len(p))]), true})
	}

	// Simplify: just ask yes/no
	if promptYN("Install /constitution skill?", true) {
		return platforms
	}
	return nil
}

func wizardInstall(platforms []string, rules []types.Rule, remoteURL, tokenEnv, scopeFlag string, yes bool, skillPlatforms []string) {
	printSection("Step 7: Installing")

	// Determine scope
	scope := scopeFlag
	if scope == "" {
		if yes {
			scope = "project"
		} else {
			idx := promptChoice("Installation scope:", []string{
				"Project-level — this project only",
				"User-level    — all projects",
			}, 0)
			if idx == 1 {
				scope = "user"
			} else {
				scope = "project"
			}
		}
	}

	// Deduplicate rules by ID
	seen := make(map[string]bool)
	var dedupRules []types.Rule
	for _, r := range rules {
		if !seen[r.ID] {
			seen[r.ID] = true
			dedupRules = append(dedupRules, r)
		}
	}

	// Build policy
	policy := &types.Policy{
		Version: "1",
		Name:    "constitution",
		Rules:   dedupRules,
	}
	if remoteURL != "" {
		policy.Remote = types.RemoteConfig{
			Enabled:      true,
			URL:          remoteURL,
			AuthTokenEnv: tokenEnv,
			Timeout:      5000,
			Fallback:     "local-only",
		}
	}

	// Write .constitution.yaml
	configPath := ".constitution.yaml"
	data, _ := yaml.Marshal(policy)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		printError(fmt.Sprintf("Error writing %s: %v", configPath, err))
		os.Exit(1)
	}
	enabled := 0
	for _, r := range dedupRules {
		if r.Enabled {
			enabled++
		}
	}
	printSuccess(fmt.Sprintf("Config: %s (%d rules, %d enabled)", configPath, len(dedupRules), enabled))

	// Install hooks for each platform
	for _, p := range platforms {
		installPlatformHooks(p, scope)
	}

	// Install skills
	for _, p := range skillPlatforms {
		platformDir := ".claude"
		if p == "codex" {
			platformDir = ".codex"
		}
		skillDir := filepath.Join(platformDir, "skills")
		if scope == "user" {
			skillDir = filepath.Join(homeDir(), platformDir, "skills")
		}
		installSkills(skillDir)
	}

	fmt.Fprintln(os.Stderr)
	printSuccess("Setup complete!")
	printHint("Restart your agent to activate hooks.")
}

// ─── Platform-specific hook installation ────────────────────────────

func installPlatformHooks(platform, scope string) {
	isCodex := platform == "codex"
	platformDir := ".claude"
	configFile := "settings.json"
	if isCodex {
		platformDir = ".codex"
		configFile = "hooks.json"
	}

	var settingsFile string
	if scope == "user" {
		settingsFile = filepath.Join(homeDir(), platformDir, configFile)
	} else {
		settingsFile = filepath.Join(platformDir, configFile)
	}

	hooks := buildPlatformHooks(platform)

	if isCodex {
		applyHooksCodex(settingsFile, hooks)
	} else {
		applyHooks(settingsFile, hooks)
	}
}

func buildPlatformHooks(platform string) map[string]interface{} {
	hooks := make(map[string]interface{})

	entry := func(matcher string, timeout int, status string) hookEntry {
		h := hookEntry{
			Matcher: matcher,
			Hooks:   []hookDef{{Type: "command", Command: "constitution", Timeout: timeout, StatusMessage: status}},
		}
		return h
	}

	hooks["SessionStart"] = []hookEntry{entry("", 5, "")}
	hooks["UserPromptSubmit"] = []hookEntry{entry("", 5, "")}

	if platform == "codex" {
		// Codex only supports Bash
		hooks["PreToolUse"] = []hookEntry{entry("Bash", 5, "Validating command")}
		hooks["PostToolUse"] = []hookEntry{entry("Bash", 60, "")}
	} else {
		hooks["PreToolUse"] = []hookEntry{
			entry("Bash", 5, ""),
			entry("Read|Write|Edit", 5, ""),
			entry("Glob|Grep", 3, ""),
		}
	}

	hooks["Stop"] = []hookEntry{entry("", 180, "")}

	return hooks
}

// ─── Hook writers ───────────────────────────────────────────────────

func applyHooks(settingsFile string, hooks map[string]interface{}) {
	os.MkdirAll(filepath.Dir(settingsFile), 0o755)
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(settingsFile); err == nil {
		json.Unmarshal(data, &existing)
	}

	existingHooks, _ := existing["hooks"].(map[string]interface{})
	if existingHooks == nil {
		existingHooks = make(map[string]interface{})
	}

	for event, entries := range hooks {
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
		newEntries, _ := entries.([]hookEntry)
		for _, ne := range newEntries {
			kept = append(kept, ne)
		}
		existingHooks[event] = kept
	}

	existing["hooks"] = existingHooks
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(settingsFile, data, 0o644); err != nil {
		printError(fmt.Sprintf("Error writing %s: %v", settingsFile, err))
		return
	}
	printSuccess(fmt.Sprintf("Hooks: %s", settingsFile))
}

func applyHooksCodex(hooksFile string, hooks map[string]interface{}) {
	os.MkdirAll(filepath.Dir(hooksFile), 0o755)
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(hooksFile); err == nil {
		json.Unmarshal(data, &existing)
	}

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
		printError(fmt.Sprintf("Error writing %s: %v", hooksFile, err))
		return
	}
	printSuccess(fmt.Sprintf("Hooks: %s", hooksFile))
	printHint("Enable hooks in Codex: set codex_hooks = true in config.toml")
}

// ─── Skills & Uninstall ─────────────────────────────────────────────

func installSkills(baseDir string) {
	for name, content := range skillFiles {
		dir := filepath.Join(baseDir, name)
		os.MkdirAll(dir, 0o755)
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			printError(fmt.Sprintf("Error writing %s: %v", path, err))
			continue
		}
		printSuccess(fmt.Sprintf("Skill: /%s → %s", name, path))
	}
}

func cmdUninstall(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	scope := fs.String("scope", "project", "Scope: user, project")
	platform := fs.String("platform", "", "Platform: claude, codex, both (default: all found)")
	fs.Parse(args)

	platforms := []string{"claude", "codex"}
	if *platform == "claude" {
		platforms = []string{"claude"}
	} else if *platform == "codex" {
		platforms = []string{"codex"}
	}

	removed := 0
	for _, p := range platforms {
		platformDir := ".claude"
		configFile := "settings.json"
		if p == "codex" {
			platformDir = ".codex"
			configFile = "hooks.json"
		}

		var settingsFile string
		if *scope == "user" {
			settingsFile = filepath.Join(homeDir(), platformDir, configFile)
		} else {
			settingsFile = filepath.Join(platformDir, configFile)
		}

		data, err := os.ReadFile(settingsFile)
		if err != nil {
			continue
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(data, &settings); err != nil {
			continue
		}

		hooks, ok := settings["hooks"].(map[string]interface{})
		if !ok {
			continue
		}

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
				if !isConstitution {
					kept = append(kept, e)
				} else {
					removed++
				}
			}
			if len(kept) > 0 {
				hooks[event] = kept
			} else {
				delete(hooks, event)
			}
		}

		if len(hooks) == 0 {
			delete(settings, "hooks")
		}

		out, _ := json.MarshalIndent(settings, "", "  ")
		os.WriteFile(settingsFile, out, 0o644)
		printSuccess(fmt.Sprintf("Cleaned: %s", settingsFile))

		// Remove skills
		skillDir := filepath.Join(platformDir, "skills")
		if *scope == "user" {
			skillDir = filepath.Join(homeDir(), platformDir, "skills")
		}
		for name := range skillFiles {
			os.RemoveAll(filepath.Join(skillDir, name))
		}
	}

	if removed > 0 {
		printSuccess(fmt.Sprintf("Removed %d hook(s)", removed))
	} else {
		printHint("No constitution hooks found")
	}
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
