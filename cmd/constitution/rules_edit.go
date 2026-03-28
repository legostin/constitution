package main

import (
	"fmt"
	"os"

	"github.com/legostin/constitution/pkg/types"
)

func cmdRulesEdit(id string) {
	policy, configPath, err := loadLocalConfig()
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	idx := findRuleIndex(policy, id)
	if idx < 0 {
		printError(fmt.Sprintf("Rule %q not found", id))
		printAvailableIDs(policy)
		return
	}

	rule := &policy.Rules[idx]

	fmt.Fprintf(os.Stderr, "\n\033[1mEditing rule: %s\033[0m\n", rule.ID)
	previewRuleYAML(*rule)

	for {
		idx := promptChoice("What to edit:", []string{
			fmt.Sprintf("Identity       (name=%q, desc=%q)", rule.Name, rule.Description),
			fmt.Sprintf("Behavior       (enabled=%v, severity=%s, priority=%d)", rule.Enabled, rule.Severity, rule.Priority),
			fmt.Sprintf("Hook events    (%v)", rule.HookEvents),
			fmt.Sprintf("Tool filter    (%v)", rule.ToolMatch),
			fmt.Sprintf("Check params   (type=%s)", rule.Check.Type),
			fmt.Sprintf("Message        (%q)", rule.Message),
			"Done — save and exit",
			"Cancel — exit without saving",
		}, 6)

		switch idx {
		case 0: // Identity
			editIdentity(rule, policy)
		case 1: // Behavior
			editBehavior(rule)
		case 2: // Hook Events
			editHookEvents(rule)
		case 3: // Tool Match
			editToolMatch(rule)
		case 4: // Check type + params
			editCheck(rule)
		case 5: // Message
			editMessage(rule)
		case 6: // Save
			previewRuleYAML(*rule)
			if promptYN("Save changes?", true) {
				if err := saveConfig(configPath, policy); err != nil {
					printError(err.Error())
					return
				}
				printSuccess(fmt.Sprintf("Rule %q updated", rule.ID))
			}
			return
		case 7: // Cancel
			fmt.Fprintln(os.Stderr, "  Cancelled.")
			return
		}

		fmt.Fprintln(os.Stderr)
		previewRuleYAML(*rule)
	}
}

func editIdentity(rule *types.Rule, policy *types.Policy) {
	printSection("Edit: Identity")

	name := promptString("Name", rule.Name)
	rule.Name = name

	desc := promptString("Description", rule.Description)
	rule.Description = desc
}

func editBehavior(rule *types.Rule) {
	printSection("Edit: Behavior")

	rule.Enabled = promptYN("Enabled?", rule.Enabled)
	fmt.Fprintln(os.Stderr)
	rule.Severity = promptSeverity()
	fmt.Fprintln(os.Stderr)

	defPriority := rule.Priority
	if defPriority < 1 {
		defPriority = 10
	}
	rule.Priority = promptInt("Priority (1-100)", defPriority, 1, 100)
}

func editHookEvents(rule *types.Rule) {
	printSection("Edit: Hook Events")

	allEvents := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}
	allDescs := []string{
		"Session start",
		"User prompt",
		"Before tool",
		"After tool",
		"Stop",
	}

	currentSet := make(map[string]bool)
	for _, e := range rule.HookEvents {
		currentSet[e] = true
	}

	var items []checklistItem
	for i, e := range allEvents {
		items = append(items, checklistItem{
			Label:       e,
			Description: allDescs[i],
			Selected:    currentSet[e],
		})
	}

	items = checklist("Hook events:", items)

	var events []string
	for _, item := range items {
		if item.Selected {
			events = append(events, item.Label)
		}
	}
	if len(events) == 0 {
		printError("At least one event required, changes not applied")
		return
	}
	rule.HookEvents = events
}

func editToolMatch(rule *types.Rule) {
	printSection("Edit: Tool Filter")

	allTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"}
	allDescs := []string{"Shell", "Reading", "Creating", "Editing", "File search", "Content search"}

	currentSet := make(map[string]bool)
	for _, t := range rule.ToolMatch {
		currentSet[t] = true
	}

	var items []checklistItem
	for i, t := range allTools {
		items = append(items, checklistItem{
			Label:       t,
			Description: allDescs[i],
			Selected:    currentSet[t],
		})
	}

	items = checklist("Tools:", items)

	var tools []string
	for _, item := range items {
		if item.Selected {
			tools = append(tools, item.Label)
		}
	}
	rule.ToolMatch = tools
}

func editCheck(rule *types.Rule) {
	printSection("Edit: Check Type & Params")

	if promptYN(fmt.Sprintf("Change check type? (current: %s)", rule.Check.Type), false) {
		var options []string
		for _, ct := range checkTypes {
			options = append(options, fmt.Sprintf("%-14s — %s", ct.name, ct.desc))
		}
		idx := promptChoice("New type:", options, 0)
		rule.Check.Type = checkTypes[idx].name
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Reconfigure parameters?", true) {
		rule.Check.Params = wizardParams(rule.Check.Type)
	}
}

func editMessage(rule *types.Rule) {
	printSection("Edit: Message")
	rule.Message = promptString("Message", rule.Message)
}
