package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/legostin/constitution/pkg/types"
)

func cmdRules(args []string) {
	if len(args) == 0 {
		rulesMainMenu()
		return
	}
	switch args[0] {
	case "list":
		cmdRulesList(args[1:])
	case "add":
		cmdRulesAdd(args[1:])
	case "edit":
		if len(args) < 2 {
			printError("Укажите ID правила: constitution rules edit <id>")
			os.Exit(1)
		}
		cmdRulesEdit(args[1])
	case "delete":
		if len(args) < 2 {
			printError("Укажите ID правила: constitution rules delete <id>")
			os.Exit(1)
		}
		cmdRulesDelete(args[1], args[2:])
	case "toggle":
		if len(args) < 2 {
			printError("Укажите ID правила: constitution rules toggle <id>")
			os.Exit(1)
		}
		cmdRulesToggle(args[1], args[2:])
	case "help", "--help", "-h":
		rulesHelp()
	default:
		printError(fmt.Sprintf("Неизвестная подкоманда: %s", args[0]))
		rulesHelp()
		os.Exit(1)
	}
}

func rulesMainMenu() {
	policy, _, err := loadLocalConfig()
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	for {
		fmt.Fprintf(os.Stderr, "\n\033[1mConstitution Rule Manager\033[0m\n")
		fmt.Fprintf(os.Stderr, "  \033[2m%d правил (%d включено)\033[0m\n\n", len(policy.Rules), countEnabled(policy))

		idx := promptChoice("Что сделать:", []string{
			"Показать все правила",
			"Добавить новое правило",
			"Редактировать правило",
			"Удалить правило",
			"Включить/выключить правило",
			"Выход",
		}, 5)

		switch idx {
		case 0:
			cmdRulesList(nil)
		case 1:
			cmdRulesAdd(nil)
			// Reload after add
			policy, _, _ = loadLocalConfig()
		case 2:
			cmdRulesList(nil)
			id := promptString("ID правила для редактирования", "")
			if id != "" {
				cmdRulesEdit(id)
				policy, _, _ = loadLocalConfig()
			}
		case 3:
			cmdRulesList(nil)
			id := promptString("ID правила для удаления", "")
			if id != "" {
				cmdRulesDelete(id, nil)
				policy, _, _ = loadLocalConfig()
			}
		case 4:
			cmdRulesList(nil)
			id := promptString("ID правила для переключения", "")
			if id != "" {
				cmdRulesToggle(id, nil)
				policy, _, _ = loadLocalConfig()
			}
		case 5:
			return
		}
	}
}

func cmdRulesList(args []string) {
	jsonMode := hasFlag(args, "--json")

	policy, configPath, err := loadLocalConfig()
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	// JSON output mode — write to stdout for programmatic use
	if jsonMode {
		data, _ := json.MarshalIndent(policy.Rules, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Fprintf(os.Stderr, "\n\033[1mПравила\033[0m (%s)\n\n", configPath)

	if len(policy.Rules) == 0 {
		printHint("Нет правил. Запустите 'constitution rules add' для создания.")
		return
	}

	// Header
	fmt.Fprintf(os.Stderr, "  \033[2m%-4s %-22s %-10s %-8s %-4s %-18s %s\033[0m\n",
		"#", "ID", "Статус", "Severity", "Pri", "Events", "Type")
	fmt.Fprintf(os.Stderr, "  \033[2m%s\033[0m\n", strings.Repeat("─", 85))

	for i, r := range policy.Rules {
		status := "\033[32mВКЛЮЧЕНО\033[0m "
		if !r.Enabled {
			status = "\033[2mвыключено\033[0m"
		}
		events := strings.Join(r.HookEvents, ",")
		if len(events) > 18 {
			events = events[:15] + "..."
		}

		fmt.Fprintf(os.Stderr, "  %-4d %-22s %s  %-8s %-4d %-18s %s\n",
			i+1, r.ID, status, r.Severity, r.Priority, events, r.Check.Type)
	}

	enabled := countEnabled(policy)
	fmt.Fprintf(os.Stderr, "\n  %d правил (%d включено, %d выключено)\n\n",
		len(policy.Rules), enabled, len(policy.Rules)-enabled)
}

func cmdRulesDelete(id string, args []string) {
	yes := hasFlag(args, "--yes")

	policy, configPath, err := loadLocalConfig()
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	idx := findRuleIndex(policy, id)
	if idx < 0 {
		printError(fmt.Sprintf("Правило %q не найдено", id))
		printAvailableIDs(policy)
		os.Exit(1)
	}

	rule := policy.Rules[idx]
	if !yes {
		fmt.Fprintf(os.Stderr, "\n  Удаление правила: \033[1m%s\033[0m (%s)\n", rule.ID, rule.Name)
		previewRuleYAML(rule)
		if !promptYN("Удалить?", false) {
			fmt.Fprintln(os.Stderr, "  Отменено.")
			return
		}
	}

	policy.Rules = append(policy.Rules[:idx], policy.Rules[idx+1:]...)
	if err := saveConfig(configPath, policy); err != nil {
		printError(err.Error())
		os.Exit(1)
	}
	printSuccess(fmt.Sprintf("Правило %q удалено", id))
}

func cmdRulesToggle(id string, args []string) {
	yes := hasFlag(args, "--yes")

	policy, configPath, err := loadLocalConfig()
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	idx := findRuleIndex(policy, id)
	if idx < 0 {
		printError(fmt.Sprintf("Правило %q не найдено", id))
		printAvailableIDs(policy)
		os.Exit(1)
	}

	rule := &policy.Rules[idx]
	newStatus := "ВЫКЛЮЧЕНО"
	if !rule.Enabled {
		newStatus = "ВКЛЮЧЕНО"
	}

	if !yes {
		oldStatus := "ВКЛЮЧЕНО"
		if !rule.Enabled {
			oldStatus = "ВЫКЛЮЧЕНО"
		}
		fmt.Fprintf(os.Stderr, "\n  Правило %q сейчас %s.\n", id, oldStatus)
		action := "Выключить"
		if !rule.Enabled {
			action = "Включить"
		}
		if !promptYN(fmt.Sprintf("%s?", action), true) {
			fmt.Fprintln(os.Stderr, "  Отменено.")
			return
		}
	}

	rule.Enabled = !rule.Enabled
	if err := saveConfig(configPath, policy); err != nil {
		printError(err.Error())
		os.Exit(1)
	}
	printSuccess(fmt.Sprintf("Правило %q теперь %s", id, newStatus))
}

// ─── Helpers ────────────────────────────────────────────────────────

func findRuleIndex(policy *types.Policy, id string) int {
	for i, r := range policy.Rules {
		if r.ID == id {
			return i
		}
	}
	return -1
}

func countEnabled(policy *types.Policy) int {
	n := 0
	for _, r := range policy.Rules {
		if r.Enabled {
			n++
		}
	}
	return n
}

func printAvailableIDs(policy *types.Policy) {
	if len(policy.Rules) == 0 {
		return
	}
	ids := make([]string, len(policy.Rules))
	for i, r := range policy.Rules {
		ids[i] = r.ID
	}
	printHint("Доступные ID: " + strings.Join(ids, ", "))
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func rulesHelp() {
	fmt.Fprint(os.Stderr, `
Usage:
  constitution rules             Интерактивное главное меню
  constitution rules list        Показать все правила
  constitution rules add         Пошаговый визард создания правила
  constitution rules edit <id>   Редактировать правило
  constitution rules delete <id> Удалить правило
  constitution rules toggle <id> Включить/выключить правило

`)
}
