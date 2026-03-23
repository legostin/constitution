package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/legostin/constitution/internal/config"
	"github.com/legostin/constitution/internal/engine"
	"github.com/legostin/constitution/internal/hook"
	"github.com/legostin/constitution/pkg/types"
)

func main() {
	configPath := flag.String("config", "", "path to constitution YAML config")
	flag.Parse()

	// 1. Read hook input from stdin
	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		hook.ExitBlock(fmt.Sprintf("constitution: %v", err))
	}

	// 2. Find and load config
	cwd := input.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	cfgPath := config.FindConfigPath(*configPath, cwd)
	if cfgPath == "" {
		// No config found — pass through silently
		os.Exit(0)
	}

	policy, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("constitution: failed to load config", "error", err, "path", cfgPath)
		os.Exit(0) // Don't block on config errors, just pass through
	}

	// 3. Setup logging
	setupLogging(policy)

	// 4. Run engine
	eng := engine.New(policy)
	output, exitCode := eng.Evaluate(input)

	// 5. Write output
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
