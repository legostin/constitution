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

	cwd, _ := os.Getwd()
	sources := config.DiscoverConfigSources(*configPath, cwd)

	if len(sources) == 0 {
		fmt.Fprintln(os.Stderr, "  \033[31m✗\033[0m No config files found")
		fmt.Fprintln(os.Stderr, "    Run: constitution init")
		os.Exit(1)
	}

	// Show all discovered sources
	fmt.Fprintln(os.Stderr, "  Config sources:")
	for _, src := range sources {
		fmt.Fprintf(os.Stderr, "    [%s] %s\n", src.Level, src.Path)
	}

	layers, errs := config.LoadAll(sources)
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "  \033[31m✗\033[0m %v\n", err)
	}
	if len(layers) == 0 {
		fmt.Fprintln(os.Stderr, "  \033[31m✗\033[0m No valid configs loaded")
		os.Exit(1)
	}

	result := config.MergePolicies(layers)

	// Show conflicts
	for _, c := range result.Conflicts {
		fmt.Fprintf(os.Stderr, "  \033[33m⚠\033[0m Rule %q: %s %s→%s by %s (%s)\n",
			c.RuleID, c.Field, c.HigherValue, c.LowerValue, c.LowerLevel, c.Action)
	}

	policy := result.Policy
	enabled := 0
	for _, r := range policy.Rules {
		if r.Enabled {
			enabled++
		}
	}

	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m Merged: %d rules (%d enabled) from %d sources\n",
		len(policy.Rules), enabled, len(layers))
	if policy.Remote.Enabled {
		fmt.Fprintf(os.Stderr, "    Remote: %s (fallback: %s)\n", policy.Remote.URL, policy.Remote.Fallback)
	}
	fmt.Fprintln(os.Stderr)
}
