package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/legostin/constitution/internal/config"
	"github.com/legostin/constitution/internal/engine"
	"github.com/legostin/constitution/internal/hook"
	"github.com/legostin/constitution/pkg/types"
	"golang.org/x/term"
)

var version = "dev"

func main() {
	// If stdin is a pipe (not TTY) and no subcommand — hook handler mode
	if !term.IsTerminal(int(os.Stdin.Fd())) && len(os.Args) <= 1 {
		runHookHandler()
		return
	}

	// CLI mode
	if len(os.Args) < 2 {
		cmdHelp()
		return
	}
	runCLI(os.Args[1:])
}

func runCLI(args []string) {
	switch args[0] {
	case "init":
		cmdInit(args[1:])
	case "setup":
		cmdSetup(args[1:])
	case "validate":
		cmdValidate(args[1:])
	case "uninstall":
		cmdUninstall(args[1:])
	case "version", "--version", "-v":
		fmt.Fprintf(os.Stderr, "constitution %s\n", version)
	case "help", "--help", "-h":
		cmdHelp()
	default:
		// Could be --config flag for hook mode
		if args[0] == "--config" || args[0] == "-config" {
			runHookHandler()
			return
		}
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		cmdHelp()
		os.Exit(1)
	}
}

func cmdHelp() {
	fmt.Fprint(os.Stderr, `Constitution — rule enforcement for Claude Code

Usage:
  constitution                 Hook handler mode (reads JSON from stdin)
  constitution init            Create .constitution.yaml from template
  constitution setup           Install hooks into Claude Code settings
  constitution validate        Validate configuration file
  constitution uninstall       Remove hooks from Claude Code settings
  constitution version         Show version

Examples:
  constitution init --template minimal
  constitution init --remote https://constitution.company.com
  constitution setup --scope user
  constitution setup --remote https://constitution.company.com
  constitution validate --config .constitution.yaml

`)
}

func runHookHandler() {
	configPath := ""
	for i, arg := range os.Args {
		if (arg == "--config" || arg == "-config") && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
		}
	}

	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		hook.ExitBlock(fmt.Sprintf("constitution: %v", err))
	}

	cwd := input.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	sources := config.DiscoverConfigSources(configPath, cwd)
	if len(sources) == 0 {
		os.Exit(0)
	}

	layers, errs := config.LoadAll(sources)
	for _, err := range errs {
		slog.Warn("constitution: config load error", "error", err)
	}
	if len(layers) == 0 {
		slog.Error("constitution: no valid configs loaded")
		os.Exit(0)
	}

	merged := config.MergePolicies(layers)
	for _, c := range merged.Conflicts {
		slog.Warn("constitution: config merge conflict",
			"rule", c.RuleID,
			"field", c.Field,
			"higher_level", c.HigherLevel.String(),
			"lower_level", c.LowerLevel.String(),
			"action", c.Action,
		)
	}

	policy := merged.Policy
	setupLogging(policy)

	eng := engine.New(policy)
	output, exitCode := eng.Evaluate(input)

	if output != nil {
		if err := hook.WriteOutput(os.Stdout, output); err != nil {
			slog.Error("constitution: failed to write output", "error", err)
		}
	}

	os.Exit(exitCode)
}

func setupLogging(policy *types.Policy) {
	if policy.Settings.LogFile != "" {
		f, err := os.OpenFile(policy.Settings.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			level := slog.LevelInfo
			switch policy.Settings.LogLevel {
			case "debug":
				level = slog.LevelDebug
			case "warn":
				level = slog.LevelWarn
			case "error":
				level = slog.LevelError
			}
			slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})))
		}
	}
}
