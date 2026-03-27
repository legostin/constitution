package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var skillFiles = map[string]string{
	"constitution":       skillConstitution,
	"constitution-rules": skillConstitutionRules,
}

func cmdSkill(args []string) {
	if len(args) == 0 {
		skillHelp()
		return
	}
	switch args[0] {
	case "install":
		cmdSkillInstall(args[1:])
	case "uninstall":
		cmdSkillUninstall(args[1:])
	case "list":
		cmdSkillList()
	case "help", "--help", "-h":
		skillHelp()
	default:
		printError(fmt.Sprintf("Неизвестная подкоманда: %s", args[0]))
		skillHelp()
		os.Exit(1)
	}
}

func cmdSkillInstall(args []string) {
	fs := flag.NewFlagSet("skill install", flag.ExitOnError)
	scope := fs.String("scope", "", "Scope: user (all projects) or project (this project)")
	quiet := fs.Bool("quiet", false, "Suppress output (for non-interactive use)")
	fs.Parse(args)

	var skillDir string
	if *scope != "" {
		switch *scope {
		case "user":
			skillDir = filepath.Join(homeDir(), ".claude", "skills")
		case "project":
			skillDir = filepath.Join(".claude", "skills")
		default:
			fmt.Fprintf(os.Stderr, "Unknown scope: %s (user or project)\n", *scope)
			os.Exit(1)
		}
	} else {
		idx := promptChoice("Куда установить skills:", []string{
			"User-level   (~/.claude/skills/) — все проекты",
			"Project-level (.claude/skills/)  — этот проект",
		}, 0)
		switch idx {
		case 0:
			skillDir = filepath.Join(homeDir(), ".claude", "skills")
		case 1:
			skillDir = filepath.Join(".claude", "skills")
		}
	}

	installed := 0
	for name, content := range skillFiles {
		dir := filepath.Join(skillDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			if !*quiet {
				printError(fmt.Sprintf("Ошибка создания %s: %v", dir, err))
			}
			continue
		}
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			if !*quiet {
				printError(fmt.Sprintf("Ошибка записи %s: %v", path, err))
			}
			continue
		}
		if !*quiet {
			printSuccess(fmt.Sprintf("/%s → %s", name, path))
		}
		installed++
	}

	if !*quiet {
		fmt.Fprintln(os.Stderr)
		if installed > 0 {
			printSuccess(fmt.Sprintf("%d skill(s) установлено", installed))
			printHint("Перезапустите Claude Code для активации")
		}
	}
}

func cmdSkillUninstall(args []string) {
	fs := flag.NewFlagSet("skill uninstall", flag.ExitOnError)
	scope := fs.String("scope", "", "Scope: user or project")
	fs.Parse(args)

	dirs := []string{}
	if *scope == "user" || *scope == "" {
		dirs = append(dirs, filepath.Join(homeDir(), ".claude", "skills"))
	}
	if *scope == "project" || *scope == "" {
		dirs = append(dirs, filepath.Join(".claude", "skills"))
	}

	removed := 0
	for _, baseDir := range dirs {
		for name := range skillFiles {
			dir := filepath.Join(baseDir, name)
			if _, err := os.Stat(dir); err == nil {
				os.RemoveAll(dir)
				printSuccess(fmt.Sprintf("Удалён: %s", dir))
				removed++
			}
		}
	}

	if removed == 0 {
		printHint("Constitution skills не найдены")
	} else {
		printSuccess(fmt.Sprintf("%d skill(s) удалено", removed))
	}
}

func cmdSkillList() {
	dirs := []struct {
		label string
		path  string
	}{
		{"user", filepath.Join(homeDir(), ".claude", "skills")},
		{"project", filepath.Join(".claude", "skills")},
	}

	found := 0
	for _, d := range dirs {
		for name := range skillFiles {
			path := filepath.Join(d.path, name, "SKILL.md")
			if _, err := os.Stat(path); err == nil {
				fmt.Fprintf(os.Stderr, "  [%s] /%s → %s\n", d.label, name, path)
				found++
			}
		}
	}

	if found == 0 {
		printHint("Constitution skills не установлены")
		printHint("Запустите: constitution skill install")
	}
}

func skillHelp() {
	fmt.Fprint(os.Stderr, `
Usage:
  constitution skill install               Установить Claude Code skills
  constitution skill install --scope user  В ~/.claude/skills/ (все проекты)
  constitution skill install --scope project  В .claude/skills/ (этот проект)
  constitution skill uninstall             Удалить skills
  constitution skill list                  Показать установленные

Skills:
  /constitution        — управление правилами, валидация, диагностика
  /constitution-rules  — быстрое создание правил через диалог

`)
}
