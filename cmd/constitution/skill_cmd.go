package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var skillFiles = map[string]string{
	"constitution": skillConstitution,
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
		printError(fmt.Sprintf("Unknown subcommand: %s", args[0]))
		skillHelp()
		os.Exit(1)
	}
}

func cmdSkillInstall(args []string) {
	fs := flag.NewFlagSet("skill install", flag.ExitOnError)
	scope := fs.String("scope", "", "Scope: user (all projects) or project (this project)")
	quiet := fs.Bool("quiet", false, "Suppress output (for non-interactive use)")
	plat := fs.String("platform", "claude", "Platform: claude, codex")
	fs.Parse(args)

	platformDir := ".claude"
	if *plat == "codex" {
		platformDir = ".codex"
	}

	var skillDir string
	if *scope != "" {
		switch *scope {
		case "user":
			skillDir = filepath.Join(homeDir(), platformDir, "skills")
		case "project":
			skillDir = filepath.Join(platformDir, "skills")
		default:
			fmt.Fprintf(os.Stderr, "Unknown scope: %s (user or project)\n", *scope)
			os.Exit(1)
		}
	} else {
		idx := promptChoice("Where to install skills:", []string{
			fmt.Sprintf("User-level   (~/%s/skills/) — all projects", platformDir),
			fmt.Sprintf("Project-level (%s/skills/)  — this project", platformDir),
		}, 0)
		switch idx {
		case 0:
			skillDir = filepath.Join(homeDir(), platformDir, "skills")
		case 1:
			skillDir = filepath.Join(platformDir, "skills")
		}
	}

	installed := 0
	for name, content := range skillFiles {
		dir := filepath.Join(skillDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			if !*quiet {
				printError(fmt.Sprintf("Error creating %s: %v", dir, err))
			}
			continue
		}
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			if !*quiet {
				printError(fmt.Sprintf("Error writing %s: %v", path, err))
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
			printSuccess(fmt.Sprintf("%d skill(s) installed", installed))
			printHint("Restart Claude Code to activate")
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
				printSuccess(fmt.Sprintf("Removed: %s", dir))
				removed++
			}
		}
	}

	if removed == 0 {
		printHint("Constitution skills not found")
	} else {
		printSuccess(fmt.Sprintf("%d skill(s) removed", removed))
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
		printHint("Constitution skills not installed")
		printHint("Run: constitution skill install")
	}
}

func skillHelp() {
	fmt.Fprint(os.Stderr, `
Usage:
  constitution skill install               Install Claude Code skills
  constitution skill install --scope user  To ~/.claude/skills/ (all projects)
  constitution skill install --scope project  To .claude/skills/ (this project)
  constitution skill uninstall             Remove skills
  constitution skill list                  Show installed skills

Skills:
  /constitution        — rule management, validation, diagnostics
  /constitution-rules  — quick rule creation via dialog

`)
}
