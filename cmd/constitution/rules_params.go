package main

import (
	"fmt"
	"os"
)

// wizardParams dispatches to the type-specific parameter wizard.
func wizardParams(checkType string) map[string]interface{} {
	switch checkType {
	case "secret_regex":
		return wizardParamsSecretRegex()
	case "dir_acl":
		return wizardParamsDirACL()
	case "cmd_validate":
		return wizardParamsCmdValidate()
	case "repo_access":
		return wizardParamsRepoAccess()
	case "cel":
		return wizardParamsCEL()
	case "linter":
		return wizardParamsLinter()
	case "secret_yelp":
		return wizardParamsSecretYelp()
	case "prompt_modify":
		return wizardParamsPromptModify()
	case "skill_inject":
		return wizardParamsSkillInject()
	case "cmd_check":
		return wizardParamsCmdCheck()
	default:
		printError(fmt.Sprintf("Неизвестный тип: %s", checkType))
		return map[string]interface{}{}
	}
}

// ─── secret_regex ───────────────────────────────────────────────────

func wizardParamsSecretRegex() map[string]interface{} {
	printSection("Настройка: Secret Regex Scanner")

	idx := promptChoice("Scan field (какое поле tool_input сканировать):", []string{
		"content     — Содержимое файла (Write)",
		"new_string  — Текст замены (Edit)",
		"command     — Bash-команда",
	}, 0)
	scanFields := []string{"content", "new_string", "command"}
	scanField := scanFields[idx]

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mПаттерны секретов\033[0m\n")
	printHint("Добавьте паттерны для обнаружения. Каждый имеет имя и regex.")

	var patterns []interface{}
	i := 1
	for {
		fmt.Fprintf(os.Stderr, "\n  Паттерн #%d:\n", i)
		name := promptStringRequired("Name")
		rx := promptRegex("Regex", "")
		patterns = append(patterns, map[string]interface{}{"name": name, "regex": rx})
		i++
		if !promptYN("Добавить ещё паттерн?", true) {
			break
		}
	}

	params := map[string]interface{}{
		"scan_field": scanField,
		"patterns":   patterns,
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Добавить allow_patterns (исключения)?", false) {
		printHint("Regex-паттерны для исключений (example keys, тестовые значения)")
		var allowPatterns []interface{}
		for {
			fmt.Fprintf(os.Stderr, "  Allow pattern: ")
			var val string
			fmt.Fscanln(os.Stdin, &val)
			if val == "" {
				break
			}
			allowPatterns = append(allowPatterns, val)
			if !promptYN("Ещё?", false) {
				break
			}
		}
		if len(allowPatterns) > 0 {
			params["allow_patterns"] = allowPatterns
		}
	}

	return params
}

// ─── dir_acl ────────────────────────────────────────────────────────

func wizardParamsDirACL() map[string]interface{} {
	printSection("Настройка: Directory ACL")

	modeIdx := promptChoice("Режим:", []string{
		"denylist   — Заблокировать указанные пути (всё остальное разрешено)",
		"allowlist  — Разрешить только указанные пути",
	}, 0)
	modes := []string{"denylist", "allowlist"}

	pathIdx := promptChoice("Path field (откуда брать путь файла):", []string{
		"auto       — Авто-определение (file_path, path, pattern)",
		"file_path  — Поле file_path",
		"path       — Поле path",
		"pattern    — Поле pattern",
	}, 0)
	pathFields := []string{"auto", "file_path", "path", "pattern"}

	allowWithin := promptYN("Allow within project? (пути внутри CWD всегда разрешены)", true)

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mGlob-паттерны путей\033[0m\n")
	printHint("Поддерживается ** для рекурсии, ~ для домашней директории")
	printHint("Примеры: /etc/**, ~/.ssh/**, **/.env, **/*.pem")

	patterns := promptStringLoop("Pattern", true)

	return map[string]interface{}{
		"mode":                 modes[modeIdx],
		"path_field":           pathFields[pathIdx],
		"allow_within_project": allowWithin,
		"patterns":             patterns,
	}
}

// ─── cmd_validate ───────────────────────────────────────────────────

func wizardParamsCmdValidate() map[string]interface{} {
	printSection("Настройка: Command Validator")

	fmt.Fprintf(os.Stderr, "  \033[1mDeny-паттерны (команды для блокировки)\033[0m\n")
	denyPatterns := promptPatternLoop("Deny pattern", true)

	params := map[string]interface{}{
		"deny_patterns": denyPatterns,
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Добавить allow-паттерны (исключения)?", false) {
		allowPatterns := promptPatternLoop("Allow pattern", false)
		if len(allowPatterns) > 0 {
			params["allow_patterns"] = allowPatterns
		}
	}

	return params
}

// ─── repo_access ────────────────────────────────────────────────────

func wizardParamsRepoAccess() map[string]interface{} {
	printSection("Настройка: Repository Access")

	modeIdx := promptChoice("Режим:", []string{
		"allowlist  — Разрешить только указанные репозитории",
		"denylist   — Заблокировать указанные репозитории",
	}, 0)
	modes := []string{"allowlist", "denylist"}

	detectIdx := promptChoice("Метод определения репозитория:", []string{
		"git_remote  — Из URL git remote (рекомендуется)",
		"directory   — По имени директории",
	}, 0)
	detects := []string{"git_remote", "directory"}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mПаттерны репозиториев\033[0m\n")
	printHint("Формат: github.com/org/repo или github.com/org/*")
	printHint("Примеры: github.com/acme-corp/*, github.com/acme-corp/my-repo")

	patterns := promptStringLoop("Pattern", true)

	return map[string]interface{}{
		"mode":        modes[modeIdx],
		"detect_from": detects[detectIdx],
		"patterns":    patterns,
	}
}

// ─── cel ────────────────────────────────────────────────────────────

func wizardParamsCEL() map[string]interface{} {
	printSection("Настройка: CEL Expression")

	printHint("Напишите CEL-выражение, которое возвращает true когда правило")
	printHint("должно СРАБОТАТЬ (заблокировать/предупредить/залогировать).")
	fmt.Fprintln(os.Stderr)
	printHint("Доступные переменные:")
	printHint("  session_id, cwd, hook_event_name, tool_name,")
	printHint("  tool_input (map), prompt, permission_mode, last_assistant_message")
	fmt.Fprintln(os.Stderr)
	printHint("Примеры:")
	printHint(`  tool_input.command.contains("git push") && tool_input.command.contains("main")`)
	printHint(`  tool_name == "Bash" && regex_match("curl.*\\|.*bash", tool_input.command)`)
	fmt.Fprintln(os.Stderr)

	expr := promptMultiline("Expression:")
	if expr == "" {
		expr = promptStringRequired("Expression (обязательно)")
	}

	return map[string]interface{}{
		"expression": expr,
	}
}

// ─── linter ─────────────────────────────────────────────────────────

func wizardParamsLinter() map[string]interface{} {
	printSection("Настройка: External Linter")

	printHint("Команда для запуска линтера. Используйте {file} как плейсхолдер.")
	printHint("Примеры: golangci-lint run --timeout=30s {file}")
	printHint("         eslint {file}")
	printHint("         ruff check {file}")
	fmt.Fprintln(os.Stderr)

	command := promptStringRequired("Command")

	params := map[string]interface{}{
		"command": command,
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Фильтр по расширениям файлов?", false) {
		printHint("Примеры: .go, .py, .js, .ts")
		exts := promptStringLoop("Extension", false)
		if len(exts) > 0 {
			params["file_extensions"] = exts
		}
	}

	wdIdx := promptChoice("Working directory:", []string{
		"project  — Из корня проекта (CWD)",
		"file     — Из директории файла",
	}, 0)
	wds := []string{"project", "file"}
	params["working_dir"] = wds[wdIdx]

	params["timeout"] = promptInt("Timeout (мс)", 30000, 1000, 300000)

	return params
}

// ─── secret_yelp ────────────────────────────────────────────────────

func wizardParamsSecretYelp() map[string]interface{} {
	printSection("Настройка: Yelp detect-secrets")

	printHint("Требуется: pip install detect-secrets")
	fmt.Fprintln(os.Stderr)

	binary := promptString("Путь к бинарнику", "detect-secrets")

	plugins := checklist("Плагины (детекторы):", []checklistItem{
		{"AWSKeyDetector", "AWS access/secret keys", true},
		{"ArtifactoryDetector", "Artifactory tokens", false},
		{"AzureStorageKeyDetector", "Azure storage keys", false},
		{"Base64HighEntropyString", "High-entropy base64 strings", false},
		{"BasicAuthDetector", "Basic auth credentials", true},
		{"GitHubTokenDetector", "GitHub tokens (ghp_, ghs_)", true},
		{"GitLabTokenDetector", "GitLab tokens", false},
		{"HexHighEntropyString", "High-entropy hex strings", false},
		{"JwtTokenDetector", "JWT tokens", true},
		{"KeywordDetector", "Keyword-based detection", true},
		{"PrivateKeyDetector", "Private keys (PEM)", true},
		{"SlackDetector", "Slack tokens", false},
		{"StripeDetector", "Stripe API keys", false},
		{"TwilioKeyDetector", "Twilio keys", false},
	})

	var pluginList []interface{}
	for _, p := range plugins {
		if !p.Selected {
			continue
		}
		entry := map[string]interface{}{"name": p.Label}
		// Entropy-based plugins have configurable limits
		if p.Label == "Base64HighEntropyString" {
			entry["limit"] = float64(promptInt("Base64 entropy limit (x10, e.g. 45=4.5)", 45, 10, 60)) / 10.0
		}
		if p.Label == "HexHighEntropyString" {
			entry["limit"] = float64(promptInt("Hex entropy limit (x10, e.g. 30=3.0)", 30, 10, 60)) / 10.0
		}
		pluginList = append(pluginList, entry)
	}

	params := map[string]interface{}{
		"binary": binary,
	}
	if len(pluginList) > 0 {
		params["plugins"] = pluginList
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Добавить exclude_secrets (regex-исключения)?", false) {
		excl := promptStringLoop("Exclude secret regex", false)
		if len(excl) > 0 {
			params["exclude_secrets"] = excl
		}
	}
	if promptYN("Добавить exclude_lines?", false) {
		excl := promptStringLoop("Exclude line pattern", false)
		if len(excl) > 0 {
			params["exclude_lines"] = excl
		}
	}

	return params
}

// ─── prompt_modify ──────────────────────────────────────────────────

func wizardParamsPromptModify() map[string]interface{} {
	printSection("Настройка: Prompt Modification")

	printHint("Инжект текста в промпты. Все поля опциональны,")
	printHint("но хотя бы одно должно быть заполнено.")
	fmt.Fprintln(os.Stderr)

	params := map[string]interface{}{}

	fmt.Fprintf(os.Stderr, "  \033[1mSystem context\033[0m (системный контекст):\n")
	sysCtx := promptMultiline("Текст:")
	if sysCtx != "" {
		params["system_context"] = sysCtx
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mPrepend\033[0m (добавить перед промптом):\n")
	prepend := promptMultiline("Текст:")
	if prepend != "" {
		params["prepend"] = prepend
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mAppend\033[0m (добавить после промпта):\n")
	appnd := promptMultiline("Текст:")
	if appnd != "" {
		params["append"] = appnd
	}

	if len(params) == 0 {
		printError("Нужно заполнить хотя бы одно поле")
		return wizardParamsPromptModify()
	}

	return params
}

// ─── skill_inject ───────────────────────────────────────────────────

func wizardParamsSkillInject() map[string]interface{} {
	printSection("Настройка: Skill/Context Injection")

	printHint("Инжект контекста при старте сессии.")
	printHint("Укажите inline-текст и/или путь к файлу (файл имеет приоритет).")
	fmt.Fprintln(os.Stderr)

	params := map[string]interface{}{}

	fmt.Fprintf(os.Stderr, "  \033[1mInline-контекст\033[0m:\n")
	ctx := promptMultiline("Текст:")
	if ctx != "" {
		params["context"] = ctx
	}

	fmt.Fprintln(os.Stderr)
	ctxFile := promptString("Путь к файлу (относительно проекта)", "")
	if ctxFile != "" {
		params["context_file"] = ctxFile
	}

	if len(params) == 0 {
		printError("Нужно указать текст или файл")
		return wizardParamsSkillInject()
	}

	return params
}

// ─── cmd_check ──────────────────────────────────────────────────────

func wizardParamsCmdCheck() map[string]interface{} {
	printSection("Настройка: Command Check")

	printHint("Shell-команда. Exit code 0 = pass, non-zero = fail.")
	printHint("Используйте {cwd} как плейсхолдер для директории проекта.")
	fmt.Fprintln(os.Stderr)
	printHint("Примеры:")
	printHint("  go test ./... -count=1")
	printHint("  test -f README.md")
	printHint("  make check")
	fmt.Fprintln(os.Stderr)

	command := promptStringRequired("Command")

	wdIdx := promptChoice("Working directory:", []string{
		"project  — Из корня проекта (CWD)",
		"Указать путь вручную",
	}, 0)
	wd := "project"
	if wdIdx == 1 {
		wd = promptStringRequired("Working dir path")
	}

	timeout := promptInt("Timeout (мс)", 30000, 1000, 600000)

	return map[string]interface{}{
		"command":     command,
		"working_dir": wd,
		"timeout":     timeout,
	}
}
