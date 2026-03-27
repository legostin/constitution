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
		printError(fmt.Sprintf("Правило %q не найдено", id))
		printAvailableIDs(policy)
		return
	}

	rule := &policy.Rules[idx]

	fmt.Fprintf(os.Stderr, "\n\033[1mРедактирование правила: %s\033[0m\n", rule.ID)
	previewRuleYAML(*rule)

	for {
		idx := promptChoice("Что редактировать:", []string{
			fmt.Sprintf("Идентификация  (name=%q, desc=%q)", rule.Name, rule.Description),
			fmt.Sprintf("Поведение      (enabled=%v, severity=%s, priority=%d)", rule.Enabled, rule.Severity, rule.Priority),
			fmt.Sprintf("Hook events    (%v)", rule.HookEvents),
			fmt.Sprintf("Tool filter    (%v)", rule.ToolMatch),
			fmt.Sprintf("Check params   (type=%s)", rule.Check.Type),
			fmt.Sprintf("Message        (%q)", rule.Message),
			"Готово — сохранить и выйти",
			"Отмена — выйти без сохранения",
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
			if promptYN("Сохранить изменения?", true) {
				if err := saveConfig(configPath, policy); err != nil {
					printError(err.Error())
					return
				}
				printSuccess(fmt.Sprintf("Правило %q обновлено", rule.ID))
			}
			return
		case 7: // Cancel
			fmt.Fprintln(os.Stderr, "  Отменено.")
			return
		}

		fmt.Fprintln(os.Stderr)
		previewRuleYAML(*rule)
	}
}

func editIdentity(rule *types.Rule, policy *types.Policy) {
	printSection("Редактирование: Идентификация")

	name := promptString("Name", rule.Name)
	rule.Name = name

	desc := promptString("Description", rule.Description)
	rule.Description = desc
}

func editBehavior(rule *types.Rule) {
	printSection("Редактирование: Поведение")

	rule.Enabled = promptYN("Включено?", rule.Enabled)
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
	printSection("Редактирование: Hook Events")

	allEvents := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}
	allDescs := []string{
		"Начало сессии",
		"Промпт пользователя",
		"Перед инструментом",
		"После инструмента",
		"Завершение",
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
		printError("Нужно хотя бы одно событие, изменения не применены")
		return
	}
	rule.HookEvents = events
}

func editToolMatch(rule *types.Rule) {
	printSection("Редактирование: Tool Filter")

	allTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"}
	allDescs := []string{"Shell", "Чтение", "Создание", "Редактирование", "Поиск файлов", "Поиск в тексте"}

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

	items = checklist("Инструменты:", items)

	var tools []string
	for _, item := range items {
		if item.Selected {
			tools = append(tools, item.Label)
		}
	}
	rule.ToolMatch = tools
}

func editCheck(rule *types.Rule) {
	printSection("Редактирование: Check Type & Params")

	if promptYN(fmt.Sprintf("Сменить тип проверки? (сейчас: %s)", rule.Check.Type), false) {
		var options []string
		for _, ct := range checkTypes {
			options = append(options, fmt.Sprintf("%-14s — %s", ct.name, ct.desc))
		}
		idx := promptChoice("Новый тип:", options, 0)
		rule.Check.Type = checkTypes[idx].name
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Перенастроить параметры?", true) {
		rule.Check.Params = wizardParams(rule.Check.Type)
	}
}

func editMessage(rule *types.Rule) {
	printSection("Редактирование: Message")
	rule.Message = promptString("Message", rule.Message)
}
