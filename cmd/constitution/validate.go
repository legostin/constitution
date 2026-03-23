package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/legostin/constitution/internal/config"
)

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := fs.String("config", "", "Config file path (default: auto-discover)")
	fs.Parse(args)

	cfgPath := *configPath
	if cfgPath == "" {
		cwd, _ := os.Getwd()
		cfgPath = config.FindConfigPath("", cwd)
	}

	if cfgPath == "" {
		fmt.Fprintln(os.Stderr, "  \033[31m✗\033[0m No config file found")
		fmt.Fprintln(os.Stderr, "    Run: constitution init")
		os.Exit(1)
	}

	policy, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  \033[31m✗\033[0m %s: %v\n", cfgPath, err)
		os.Exit(1)
	}

	enabled := 0
	for _, r := range policy.Rules {
		if r.Enabled {
			enabled++
		}
	}

	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m %s\n", cfgPath)
	fmt.Fprintf(os.Stderr, "    %d rules (%d enabled)\n", len(policy.Rules), enabled)
	if policy.Remote.Enabled {
		fmt.Fprintf(os.Stderr, "    Remote: %s (fallback: %s)\n", policy.Remote.URL, policy.Remote.Fallback)
	}
	fmt.Fprintln(os.Stderr)
}
