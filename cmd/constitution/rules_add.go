package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/legostin/constitution/pkg/types"
)

var ruleIDRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// checkTypeNames and their descriptions for the wizard.
var checkTypes = []struct {
	name string
	desc string
}{
	{"secret_regex", "Scan content for secrets (regex)"},
	{"dir_acl", "File/directory access control"},
	{"cmd_validate", "Bash command validation"},
	{"repo_access", "Repository access control"},
	{"cel", "Custom CEL expressions (advanced)"},
	{"linter", "Run external linter"},
	{"secret_yelp", "Yelp detect-secrets (28+ detectors)"},
	{"prompt_modify", "Prompt modification"},
	{"skill_inject", "Context injection at session start"},
	{"cmd_check", "Run shell command (pass/fail on exit code)"},
}

func cmdRulesAdd(args []string) {
	// Parse flags for non-interactive mode
	fs := flag.NewFlagSet("rules add", flag.ExitOnError)
	fID := fs.String("id", "", "Rule ID")
	fName := fs.String("name", "", "Rule name")
	fDesc := fs.String("description", "", "Rule description")
	fSeverity := fs.String("severity", "block", "Severity: block|warn|audit")
	fPriority := fs.Int("priority", 10, "Priority (1-100)")
	fEvents := fs.String("events", "", "Hook events (comma-separated)")
	fTools := fs.String("tools", "", "Tool match (comma-separated)")
	fCheckType := fs.String("check-type", "", "Check type")
	fParams := fs.String("params", "", "Check params as JSON string")
	fMessage := fs.String("message", "", "Custom block/warn message")
	fJSON := fs.Bool("json", false, "Read full rule as JSON from stdin")
	fEnabled := fs.Bool("enabled", true, "Enable rule")
	fs.Parse(args)

	policy, configPath, err := loadLocalConfig()
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	// Non-interactive: JSON from stdin
	if *fJSON {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			printError(fmt.Sprintf("Failed to read stdin: %v", err))
			os.Exit(1)
		}
		var rule types.Rule
		if err := json.Unmarshal(data, &rule); err != nil {
			printError(fmt.Sprintf("Invalid JSON: %v", err))
			os.Exit(1)
		}
		policy.Rules = append(policy.Rules, rule)
		if err := saveConfig(configPath, policy); err != nil {
			printError(err.Error())
			os.Exit(1)
		}
		printSuccess(fmt.Sprintf("Rule %q added", rule.ID))
		return
	}

	// Non-interactive: flags mode
	if *fID != "" {
		rule := types.Rule{
			ID:          *fID,
			Name:        *fName,
			Description: *fDesc,
			Enabled:     *fEnabled,
			Priority:    *fPriority,
			Severity:    types.Severity(*fSeverity),
			Message:     *fMessage,
		}
		if rule.Name == "" {
			rule.Name = *fID
		}
		if *fEvents != "" {
			rule.HookEvents = strings.Split(*fEvents, ",")
		}
		if *fTools != "" {
			rule.ToolMatch = strings.Split(*fTools, ",")
		}
		if *fCheckType != "" {
			rule.Check.Type = *fCheckType
		}
		if *fParams != "" {
			var params map[string]interface{}
			if err := json.Unmarshal([]byte(*fParams), &params); err != nil {
				printError(fmt.Sprintf("Invalid JSON in --params: %v", err))
				os.Exit(1)
			}
			rule.Check.Params = params
		}

		// Validate
		if len(rule.HookEvents) == 0 {
			printError("--events is required")
			os.Exit(1)
		}
		if rule.Check.Type == "" {
			printError("--check-type is required")
			os.Exit(1)
		}

		policy.Rules = append(policy.Rules, rule)
		if err := saveConfig(configPath, policy); err != nil {
			printError(err.Error())
			os.Exit(1)
		}
		printSuccess(fmt.Sprintf("Rule %q added to %s", rule.ID, configPath))
		return
	}

	// Interactive wizard mode
	existingIDs := make(map[string]bool)
	for _, r := range policy.Rules {
		existingIDs[r.ID] = true
	}

	// Step 1: Identity
	id, name, desc := wizardIdentity(existingIDs)

	// Step 2: Behavior
	enabled, severity, priority := wizardBehavior()

	// Step 3: Hook Events
	hookEvents := wizardHookEvents()

	// Step 4: Tool Match
	toolMatch := wizardToolMatch(hookEvents)

	// Step 5: Check Type
	checkType := wizardCheckType()

	// Step 6: Parameters
	printSection("Step 6/7: Parameters")
	params := wizardParams(checkType)

	// Step 7: Message + Preview
	message := wizardMessage()

	rule := types.Rule{
		ID:          id,
		Name:        name,
		Description: desc,
		Enabled:     enabled,
		Priority:    priority,
		Severity:    severity,
		HookEvents:  hookEvents,
		ToolMatch:   toolMatch,
		Check: types.CheckConfig{
			Type:   checkType,
			Params: params,
		},
		Message: message,
	}

	previewRuleYAML(rule)

	if !promptYN("Save this rule?", true) {
		fmt.Fprintln(os.Stderr, "  Cancelled.")
		return
	}

	policy.Rules = append(policy.Rules, rule)
	if err := saveConfig(configPath, policy); err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	printSuccess(fmt.Sprintf("Rule %q added to %s", id, configPath))
	printHint("Run 'constitution validate' to verify configuration")
}

// ─── Wizard Steps ───────────────────────────────────────────────────

func wizardIdentity(existingIDs map[string]bool) (id, name, desc string) {
	printSection("Step 1/7: Rule Identity")

	printHint("Rule ID (unique, lowercase, hyphens)")
	printHint("Examples: secret-write, cmd-validate, lint-go")
	fmt.Fprintln(os.Stderr)

	for {
		id = promptStringRequired("ID")
		if !ruleIDRegex.MatchString(id) {
			printError("ID must contain only a-z, 0-9 and hyphens, start with a letter")
			continue
		}
		if existingIDs[id] {
			printError(fmt.Sprintf("ID %q already exists", id))
			continue
		}
		break
	}

	// Default name from ID: "cmd-validate" -> "Cmd Validate"
	defaultName := strings.ReplaceAll(id, "-", " ")
	defaultName = strings.Title(defaultName)
	name = promptString("Name", defaultName)

	desc = promptString("Description (optional)", "")
	return
}

func wizardBehavior() (enabled bool, severity types.Severity, priority int) {
	printSection("Step 2/7: Behavior")

	enabled = promptYN("Enable rule immediately?", true)
	fmt.Fprintln(os.Stderr)
	severity = promptSeverity()
	fmt.Fprintln(os.Stderr)
	priority = promptInt("Priority (lower = checked first, 1-100)", 10, 1, 100)
	return
}

func wizardHookEvents() []string {
	printSection("Step 3/7: Hook Events")

	for {
		items := checklist("Select hook events:", []checklistItem{
			{"SessionStart", "Claude Code session begins", false},
			{"UserPromptSubmit", "User sends a prompt", false},
			{"PreToolUse", "Before tool execution", false},
			{"PostToolUse", "After tool execution", false},
			{"Stop", "Agent is stopping", false},
		})

		var events []string
		for _, item := range items {
			if item.Selected {
				events = append(events, item.Label)
			}
		}
		if len(events) == 0 {
			printError("Select at least one event")
			continue
		}
		return events
	}
}

func wizardToolMatch(hookEvents []string) []string {
	hasToolEvent := false
	for _, e := range hookEvents {
		if e == "PreToolUse" || e == "PostToolUse" {
			hasToolEvent = true
			break
		}
	}
	if !hasToolEvent {
		printSection("Step 4/7: Tool Filter")
		printHint("Skipped — no tool-related events selected")
		return nil
	}

	printSection("Step 4/7: Tool Filter")
	printHint("Leave all unselected to match ALL tools")
	fmt.Fprintln(os.Stderr)

	items := checklist("Which tools?", []checklistItem{
		{"Bash", "Shell commands", false},
		{"Read", "File reading", false},
		{"Write", "File creation", false},
		{"Edit", "File editing", false},
		{"Glob", "File search", false},
		{"Grep", "Content search", false},
	})

	var tools []string
	for _, item := range items {
		if item.Selected {
			tools = append(tools, item.Label)
		}
	}
	return tools
}

func wizardCheckType() string {
	printSection("Step 5/7: Check Type")

	var options []string
	for _, ct := range checkTypes {
		options = append(options, fmt.Sprintf("%-14s — %s", ct.name, ct.desc))
	}

	idx := promptChoice("Which check to perform:", options, 0)
	return checkTypes[idx].name
}

func wizardMessage() string {
	printSection("Step 7/7: Message & Confirmation")

	printHint("Custom message (shown on block/warning)")
	printHint("Leave empty for default message")
	fmt.Fprintln(os.Stderr)

	return promptString("Message", "")
}
