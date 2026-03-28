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
		printError(fmt.Sprintf("Unknown type: %s", checkType))
		return map[string]interface{}{}
	}
}

// ─── secret_regex ───────────────────────────────────────────────────

func wizardParamsSecretRegex() map[string]interface{} {
	printSection("Configure: Secret Regex Scanner")

	idx := promptChoice("Scan field (which tool_input field to scan):", []string{
		"content     — File content (Write)",
		"new_string  — Replacement text (Edit)",
		"command     — Bash command",
	}, 0)
	scanFields := []string{"content", "new_string", "command"}
	scanField := scanFields[idx]

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mSecret patterns\033[0m\n")
	printHint("Add patterns for detection. Each has a name and regex.")

	var patterns []interface{}
	i := 1
	for {
		fmt.Fprintf(os.Stderr, "\n  Pattern #%d:\n", i)
		name := promptStringRequired("Name")
		rx := promptRegex("Regex", "")
		patterns = append(patterns, map[string]interface{}{"name": name, "regex": rx})
		i++
		if !promptYN("Add another pattern?", true) {
			break
		}
	}

	params := map[string]interface{}{
		"scan_field": scanField,
		"patterns":   patterns,
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Add allow_patterns (exceptions)?", false) {
		printHint("Regex patterns for exceptions (example keys, test values)")
		var allowPatterns []interface{}
		for {
			fmt.Fprintf(os.Stderr, "  Allow pattern: ")
			var val string
			fmt.Fscanln(os.Stdin, &val)
			if val == "" {
				break
			}
			allowPatterns = append(allowPatterns, val)
			if !promptYN("More?", false) {
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
	printSection("Configure: Directory ACL")

	modeIdx := promptChoice("Mode:", []string{
		"denylist   — Block specified paths (everything else allowed)",
		"allowlist  — Allow only specified paths",
	}, 0)
	modes := []string{"denylist", "allowlist"}

	pathIdx := promptChoice("Path field (where to get the file path):", []string{
		"auto       — Auto-detect (file_path, path, pattern)",
		"file_path  — file_path field",
		"path       — path field",
		"pattern    — pattern field",
	}, 0)
	pathFields := []string{"auto", "file_path", "path", "pattern"}

	allowWithin := promptYN("Allow within project? (paths inside CWD always allowed)", true)

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mPath glob patterns\033[0m\n")
	printHint("Supports ** for recursion, ~ for home directory")
	printHint("Examples: /etc/**, ~/.ssh/**, **/.env, **/*.pem")

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
	printSection("Configure: Command Validator")

	fmt.Fprintf(os.Stderr, "  \033[1mDeny patterns (commands to block)\033[0m\n")
	denyPatterns := promptPatternLoop("Deny pattern", true)

	params := map[string]interface{}{
		"deny_patterns": denyPatterns,
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Add allow patterns (exceptions)?", false) {
		allowPatterns := promptPatternLoop("Allow pattern", false)
		if len(allowPatterns) > 0 {
			params["allow_patterns"] = allowPatterns
		}
	}

	return params
}

// ─── repo_access ────────────────────────────────────────────────────

func wizardParamsRepoAccess() map[string]interface{} {
	printSection("Configure: Repository Access")

	modeIdx := promptChoice("Mode:", []string{
		"allowlist  — Allow only specified repositories",
		"denylist   — Block specified repositories",
	}, 0)
	modes := []string{"allowlist", "denylist"}

	detectIdx := promptChoice("Repository detection method:", []string{
		"git_remote  — From git remote URL (recommended)",
		"directory   — By directory name",
	}, 0)
	detects := []string{"git_remote", "directory"}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mRepository patterns\033[0m\n")
	printHint("Format: github.com/org/repo or github.com/org/*")
	printHint("Examples: github.com/acme-corp/*, github.com/acme-corp/my-repo")

	patterns := promptStringLoop("Pattern", true)

	return map[string]interface{}{
		"mode":        modes[modeIdx],
		"detect_from": detects[detectIdx],
		"patterns":    patterns,
	}
}

// ─── cel ────────────────────────────────────────────────────────────

func wizardParamsCEL() map[string]interface{} {
	printSection("Configure: CEL Expression")

	printHint("Write a CEL expression that returns true when the rule")
	printHint("should TRIGGER (block/warn/log).")
	fmt.Fprintln(os.Stderr)
	printHint("Available variables:")
	printHint("  session_id, cwd, hook_event_name, tool_name,")
	printHint("  tool_input (map), prompt, permission_mode, last_assistant_message")
	fmt.Fprintln(os.Stderr)
	printHint("Examples:")
	printHint(`  tool_input.command.contains("git push") && tool_input.command.contains("main")`)
	printHint(`  tool_name == "Bash" && regex_match("curl.*\\|.*bash", tool_input.command)`)
	fmt.Fprintln(os.Stderr)

	expr := promptMultiline("Expression:")
	if expr == "" {
		expr = promptStringRequired("Expression (required)")
	}

	return map[string]interface{}{
		"expression": expr,
	}
}

// ─── linter ─────────────────────────────────────────────────────────

func wizardParamsLinter() map[string]interface{} {
	printSection("Configure: External Linter")

	printHint("Command to run the linter. Use {file} as a placeholder.")
	printHint("Examples: golangci-lint run --timeout=30s {file}")
	printHint("          eslint {file}")
	printHint("          ruff check {file}")
	fmt.Fprintln(os.Stderr)

	command := promptStringRequired("Command")

	params := map[string]interface{}{
		"command": command,
	}

	fmt.Fprintln(os.Stderr)
	if promptYN("Filter by file extensions?", false) {
		printHint("Examples: .go, .py, .js, .ts")
		exts := promptStringLoop("Extension", false)
		if len(exts) > 0 {
			params["file_extensions"] = exts
		}
	}

	wdIdx := promptChoice("Working directory:", []string{
		"project  — From project root (CWD)",
		"file     — From file directory",
	}, 0)
	wds := []string{"project", "file"}
	params["working_dir"] = wds[wdIdx]

	params["timeout"] = promptInt("Timeout (ms)", 30000, 1000, 300000)

	return params
}

// ─── secret_yelp ────────────────────────────────────────────────────

func wizardParamsSecretYelp() map[string]interface{} {
	printSection("Configure: Yelp detect-secrets")

	printHint("Required: pip install detect-secrets")
	fmt.Fprintln(os.Stderr)

	binary := promptString("Path to binary", "detect-secrets")

	plugins := checklist("Plugins (detectors):", []checklistItem{
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
	if promptYN("Add exclude_secrets (regex exceptions)?", false) {
		excl := promptStringLoop("Exclude secret regex", false)
		if len(excl) > 0 {
			params["exclude_secrets"] = excl
		}
	}
	if promptYN("Add exclude_lines?", false) {
		excl := promptStringLoop("Exclude line pattern", false)
		if len(excl) > 0 {
			params["exclude_lines"] = excl
		}
	}

	return params
}

// ─── prompt_modify ──────────────────────────────────────────────────

func wizardParamsPromptModify() map[string]interface{} {
	printSection("Configure: Prompt Modification")

	printHint("Text injection into prompts. All fields are optional,")
	printHint("but at least one must be filled.")
	fmt.Fprintln(os.Stderr)

	params := map[string]interface{}{}

	fmt.Fprintf(os.Stderr, "  \033[1mSystem context\033[0m:\n")
	sysCtx := promptMultiline("Text:")
	if sysCtx != "" {
		params["system_context"] = sysCtx
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mPrepend\033[0m (add before prompt):\n")
	prepend := promptMultiline("Text:")
	if prepend != "" {
		params["prepend"] = prepend
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  \033[1mAppend\033[0m (add after prompt):\n")
	appnd := promptMultiline("Text:")
	if appnd != "" {
		params["append"] = appnd
	}

	if len(params) == 0 {
		printError("At least one field must be filled")
		return wizardParamsPromptModify()
	}

	return params
}

// ─── skill_inject ───────────────────────────────────────────────────

func wizardParamsSkillInject() map[string]interface{} {
	printSection("Configure: Skill/Context Injection")

	printHint("Context injection at session start.")
	printHint("Specify inline text and/or path to a file (file takes priority).")
	fmt.Fprintln(os.Stderr)

	params := map[string]interface{}{}

	fmt.Fprintf(os.Stderr, "  \033[1mInline context\033[0m:\n")
	ctx := promptMultiline("Text:")
	if ctx != "" {
		params["context"] = ctx
	}

	fmt.Fprintln(os.Stderr)
	ctxFile := promptString("Path to file (relative to project)", "")
	if ctxFile != "" {
		params["context_file"] = ctxFile
	}

	if len(params) == 0 {
		printError("Must specify text or file")
		return wizardParamsSkillInject()
	}

	return params
}

// ─── cmd_check ──────────────────────────────────────────────────────

func wizardParamsCmdCheck() map[string]interface{} {
	printSection("Configure: Command Check")

	printHint("Shell command. Exit code 0 = pass, non-zero = fail.")
	printHint("Use {cwd} as a placeholder for the project directory.")
	fmt.Fprintln(os.Stderr)
	printHint("Examples:")
	printHint("  go test ./... -count=1")
	printHint("  test -f README.md")
	printHint("  make check")
	fmt.Fprintln(os.Stderr)

	command := promptStringRequired("Command")

	wdIdx := promptChoice("Working directory:", []string{
		"project  — From project root (CWD)",
		"Specify path manually",
	}, 0)
	wd := "project"
	if wdIdx == 1 {
		wd = promptStringRequired("Working dir path")
	}

	timeout := promptInt("Timeout (ms)", 30000, 1000, 600000)

	return map[string]interface{}{
		"command":     command,
		"working_dir": wd,
		"timeout":     timeout,
	}
}
