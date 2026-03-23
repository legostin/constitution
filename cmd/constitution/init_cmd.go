package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	template := fs.String("template", "", "Config template: full, minimal (default: interactive)")
	remote := fs.String("remote", "", "Create remote-only config with this server URL")
	output := fs.String("output", ".constitution.yaml", "Output file path")
	fs.Parse(args)

	dest := *output

	// Check if file exists
	if _, err := os.Stat(dest); err == nil {
		if !promptYN(fmt.Sprintf("%s already exists. Overwrite?", dest), false) {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return
		}
	}

	var content string

	if *remote != "" {
		// Remote-only config
		content = strings.ReplaceAll(remoteConfigTemplate, "{{REMOTE_URL}}", *remote)
		fmt.Fprintf(os.Stderr, "\n  Creating remote config → %s\n", dest)
		fmt.Fprintf(os.Stderr, "  Server: %s\n\n", *remote)
	} else if *template != "" {
		switch *template {
		case "full":
			content = fullConfigTemplate
		case "minimal":
			content = minimalConfigTemplate
		default:
			fmt.Fprintf(os.Stderr, "Unknown template: %s (use: full, minimal)\n", *template)
			os.Exit(1)
		}
	} else {
		// Interactive
		fmt.Fprintln(os.Stderr)
		idx := promptChoice("Config template:", []string{
			"Full      — all checks with examples",
			"Minimal   — secrets + command validation",
			"Remote    — connect to existing server",
		}, 0)

		switch idx {
		case 0:
			content = fullConfigTemplate
		case 1:
			content = minimalConfigTemplate
		case 2:
			url := promptString("Server URL", "")
			if url == "" {
				fmt.Fprintln(os.Stderr, "URL is required for remote config.")
				os.Exit(1)
			}
			content = strings.ReplaceAll(remoteConfigTemplate, "{{REMOTE_URL}}", url)
		}
	}

	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", dest, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m %s created\n\n", dest)
	fmt.Fprintln(os.Stderr, "  Next: constitution setup")
	fmt.Fprintln(os.Stderr)
}
