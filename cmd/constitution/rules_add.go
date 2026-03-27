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
	{"secret_regex", "Сканировать контент на секреты (regex)"},
	{"dir_acl", "Контроль доступа к файлам/директориям"},
	{"cmd_validate", "Валидация bash-команд"},
	{"repo_access", "Контроль репозиториев"},
	{"cel", "Кастомные CEL-выражения (продвинутый)"},
	{"linter", "Запуск внешнего линтера"},
	{"secret_yelp", "Yelp detect-secrets (28+ детекторов)"},
	{"prompt_modify", "Модификация промптов"},
	{"skill_inject", "Инжект контекста при старте"},
	{"cmd_check", "Запуск shell-команды (pass/fail по exit code)"},
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
			printError(fmt.Sprintf("Ошибка чтения stdin: %v", err))
			os.Exit(1)
		}
		var rule types.Rule
		if err := json.Unmarshal(data, &rule); err != nil {
			printError(fmt.Sprintf("Невалидный JSON: %v", err))
			os.Exit(1)
		}
		policy.Rules = append(policy.Rules, rule)
		if err := saveConfig(configPath, policy); err != nil {
			printError(err.Error())
			os.Exit(1)
		}
		printSuccess(fmt.Sprintf("Правило %q добавлено", rule.ID))
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
				printError(fmt.Sprintf("Невалидный JSON в --params: %v", err))
				os.Exit(1)
			}
			rule.Check.Params = params
		}

		// Validate
		if len(rule.HookEvents) == 0 {
			printError("--events обязателен")
			os.Exit(1)
		}
		if rule.Check.Type == "" {
			printError("--check-type обязателен")
			os.Exit(1)
		}

		policy.Rules = append(policy.Rules, rule)
		if err := saveConfig(configPath, policy); err != nil {
			printError(err.Error())
			os.Exit(1)
		}
		printSuccess(fmt.Sprintf("Правило %q добавлено в %s", rule.ID, configPath))
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
	printSection("Шаг 6/7: Параметры")
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

	if !promptYN("Сохранить это правило?", true) {
		fmt.Fprintln(os.Stderr, "  Отменено.")
		return
	}

	policy.Rules = append(policy.Rules, rule)
	if err := saveConfig(configPath, policy); err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	printSuccess(fmt.Sprintf("Правило %q добавлено в %s", id, configPath))
	printHint("Запустите 'constitution validate' для проверки конфигурации")
}

// ─── Wizard Steps ───────────────────────────────────────────────────

func wizardIdentity(existingIDs map[string]bool) (id, name, desc string) {
	printSection("Шаг 1/7: Идентификация правила")

	printHint("Rule ID (уникальный, lowercase, дефисы)")
	printHint("Примеры: secret-write, cmd-validate, lint-go")
	fmt.Fprintln(os.Stderr)

	for {
		id = promptStringRequired("ID")
		if !ruleIDRegex.MatchString(id) {
			printError("ID должен состоять из a-z, 0-9 и дефисов, начинаться с буквы")
			continue
		}
		if existingIDs[id] {
			printError(fmt.Sprintf("ID %q уже существует", id))
			continue
		}
		break
	}

	// Default name from ID: "cmd-validate" -> "Cmd Validate"
	defaultName := strings.ReplaceAll(id, "-", " ")
	defaultName = strings.Title(defaultName)
	name = promptString("Название", defaultName)

	desc = promptString("Описание (опционально)", "")
	return
}

func wizardBehavior() (enabled bool, severity types.Severity, priority int) {
	printSection("Шаг 2/7: Поведение")

	enabled = promptYN("Включить правило сразу?", true)
	fmt.Fprintln(os.Stderr)
	severity = promptSeverity()
	fmt.Fprintln(os.Stderr)
	priority = promptInt("Priority (меньше = проверяется раньше, 1-100)", 10, 1, 100)
	return
}

func wizardHookEvents() []string {
	printSection("Шаг 3/7: Когда запускать")

	for {
		items := checklist("Выберите hook events:", []checklistItem{
			{"SessionStart", "Начало сессии Claude Code", false},
			{"UserPromptSubmit", "Пользователь отправляет промпт", false},
			{"PreToolUse", "Перед использованием инструмента", false},
			{"PostToolUse", "После использования инструмента", false},
			{"Stop", "Агент завершает работу", false},
		})

		var events []string
		for _, item := range items {
			if item.Selected {
				events = append(events, item.Label)
			}
		}
		if len(events) == 0 {
			printError("Выберите хотя бы одно событие")
			continue
		}
		return events
	}
}

func wizardToolMatch(hookEvents []string) []string {
	// Only show tool match if tool-related events are selected
	hasToolEvent := false
	for _, e := range hookEvents {
		if e == "PreToolUse" || e == "PostToolUse" {
			hasToolEvent = true
			break
		}
	}
	if !hasToolEvent {
		printSection("Шаг 4/7: Фильтр инструментов")
		printHint("Пропущено — нет tool-related событий")
		return nil
	}

	printSection("Шаг 4/7: Фильтр инструментов")
	printHint("Оставьте всё невыбранным чтобы срабатывать на ВСЕ инструменты")
	fmt.Fprintln(os.Stderr)

	items := checklist("Какие инструменты?", []checklistItem{
		{"Bash", "Shell-команды", false},
		{"Read", "Чтение файлов", false},
		{"Write", "Создание файлов", false},
		{"Edit", "Редактирование файлов", false},
		{"Glob", "Поиск файлов", false},
		{"Grep", "Поиск в содержимом", false},
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
	printSection("Шаг 5/7: Тип проверки")

	var options []string
	for _, ct := range checkTypes {
		options = append(options, fmt.Sprintf("%-14s — %s", ct.name, ct.desc))
	}

	idx := promptChoice("Какую проверку выполнять:", options, 0)
	return checkTypes[idx].name
}

func wizardMessage() string {
	printSection("Шаг 7/7: Сообщение и подтверждение")

	printHint("Custom message (показывается при блокировке/предупреждении)")
	printHint("Оставьте пустым для сообщения по умолчанию")
	fmt.Fprintln(os.Stderr)

	return promptString("Message", "")
}
