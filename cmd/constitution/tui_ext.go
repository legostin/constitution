package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/legostin/constitution/pkg/types"
)

// ─── Output helpers ─────────────────────────────────────────────────

func printSection(title string) {
	fmt.Fprintf(os.Stderr, "\n\033[1m━━━ %s ━━━\033[0m\n\n", title)
}

func printHint(text string) {
	fmt.Fprintf(os.Stderr, "  \033[2m%s\033[0m\n", text)
}

func printSuccess(text string) {
	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m %s\n", text)
}

func printError(text string) {
	fmt.Fprintf(os.Stderr, "  \033[31m✗\033[0m %s\n", text)
}

// ─── Input helpers ──────────────────────────────────────────────────

// promptInt asks for an integer within [min, max].
func promptInt(prompt string, defaultVal, min, max int) int {
	for {
		fmt.Fprintf(os.Stderr, "  %s [%d]: ", prompt, defaultVal)
		var val int
		_, err := fmt.Fscanf(os.Stdin, "%d", &val)
		// consume rest of line
		var discard string
		fmt.Fscanln(os.Stdin, &discard)

		if err != nil {
			return defaultVal
		}
		if val < min || val > max {
			printError(fmt.Sprintf("Должно быть от %d до %d", min, max))
			continue
		}
		return val
	}
}

// promptStringRequired asks for a non-empty string, re-prompting until provided.
func promptStringRequired(prompt string) string {
	for {
		fmt.Fprintf(os.Stderr, "  %s: ", prompt)
		var val string
		fmt.Fscanln(os.Stdin, &val)
		val = strings.TrimSpace(val)
		if val != "" {
			return val
		}
		printError("Обязательное поле, введите значение")
	}
}

// promptMultiline reads lines until an empty line is entered.
// Returns the joined text.
func promptMultiline(prompt string) string {
	fmt.Fprintf(os.Stderr, "  %s\n", prompt)
	printHint("(пустая строка для завершения)")

	var lines []string
	for {
		fmt.Fprintf(os.Stderr, "  > ")
		var line string
		fmt.Fscanln(os.Stdin, &line)
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// promptRegex asks for a regex string, validates it compiles, re-prompts on error.
func promptRegex(prompt, defaultVal string) string {
	for {
		var val string
		if defaultVal != "" {
			fmt.Fprintf(os.Stderr, "  %s [%s]: ", prompt, defaultVal)
		} else {
			fmt.Fprintf(os.Stderr, "  %s: ", prompt)
		}
		fmt.Fscanln(os.Stdin, &val)
		val = strings.TrimSpace(val)
		if val == "" {
			if defaultVal != "" {
				return defaultVal
			}
			printError("Обязательное поле")
			continue
		}
		if _, err := regexp.Compile(val); err != nil {
			printError(fmt.Sprintf("Невалидный regex: %v", err))
			continue
		}
		printSuccess("Валидный regex")
		return val
	}
}

// promptSeverity shows a severity chooser and returns the typed Severity.
func promptSeverity() types.Severity {
	idx := promptChoice("Severity — что происходит при срабатывании:", []string{
		"block  — Заблокировать действие",
		"warn   — Разрешить + предупреждение",
		"audit  — Разрешить тихо, только логирование",
	}, 0)
	switch idx {
	case 1:
		return types.SeverityWarn
	case 2:
		return types.SeverityAudit
	default:
		return types.SeverityBlock
	}
}

// promptPatternLoop collects a list of {name, regex} patterns interactively.
// Returns a slice of map[string]interface{}.
func promptPatternLoop(label string, required bool) []interface{} {
	var patterns []interface{}
	i := 1
	for {
		fmt.Fprintf(os.Stderr, "\n  %s #%d:\n", label, i)
		name := promptStringRequired("Name")
		rx := promptRegex("Regex", "")
		p := map[string]interface{}{"name": name, "regex": rx}

		caseSens := promptYN("Case insensitive?", false)
		if caseSens {
			p["case_insensitive"] = true
		}

		patterns = append(patterns, p)
		i++

		if !promptYN(fmt.Sprintf("Добавить ещё %s?", label), !required || len(patterns) == 0) {
			break
		}
	}
	return patterns
}

// promptStringLoop collects a list of strings interactively.
func promptStringLoop(label string, required bool) []interface{} {
	var items []interface{}
	i := 1
	for {
		fmt.Fprintf(os.Stderr, "  %s #%d: ", label, i)
		var val string
		fmt.Fscanln(os.Stdin, &val)
		val = strings.TrimSpace(val)
		if val == "" {
			if required && len(items) == 0 {
				printError("Нужно добавить хотя бы один элемент")
				continue
			}
			break
		}
		items = append(items, val)
		i++
		if !promptYN("Добавить ещё?", len(items) == 0) {
			break
		}
	}
	return items
}
