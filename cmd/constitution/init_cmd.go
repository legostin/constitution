package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var workflowTemplates = map[string]string{
	"autonomous":      autonomousTemplate,
	"plan-first":      planFirstTemplate,
	"ooda-loop":       oodaLoopTemplate,
	"strict-security": strictSecurityTemplate,
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	template := fs.String("template", "", "Config template: full, minimal (default: interactive)")
	workflow := fs.String("workflow", "", "Orchestration pattern: autonomous, plan-first, ooda-loop, strict-security")
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
		content = strings.ReplaceAll(remoteConfigTemplate, "{{REMOTE_URL}}", *remote)
		fmt.Fprintf(os.Stderr, "\n  Creating remote config → %s\n", dest)
		fmt.Fprintf(os.Stderr, "  Server: %s\n\n", *remote)
	} else if *workflow != "" {
		// Workflow pattern (non-interactive)
		wf, ok := workflowTemplates[*workflow]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown workflow: %s\n", *workflow)
			fmt.Fprintln(os.Stderr, "Available: autonomous, plan-first, ooda-loop, strict-security")
			os.Exit(1)
		}
		content = wf
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
		idx := promptChoice("Choose config type:", []string{
			"Full           — all checks with examples",
			"Minimal        — secrets + command validation",
			"Remote         — connect to existing server",
			"─── Orchestration Patterns ───",
			"Autonomous     — full autonomy with safety guardrails",
			"Plan-First     — plan → execute → test workflow",
			"OODA Loop      — observe → orient → decide → act cycle",
			"Strict Security — maximum protection, extended blocklists",
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
		case 3:
			// Separator — re-prompt
			fmt.Fprintln(os.Stderr, "  Please select a specific pattern:")
			return
		case 4:
			content = autonomousTemplate
		case 5:
			content = planFirstTemplate
		case 6:
			content = oodaLoopTemplate
		case 7:
			content = strictSecurityTemplate
		}
	}

	if content == "" {
		fmt.Fprintln(os.Stderr, "No template selected.")
		return
	}

	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", dest, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m %s created\n\n", dest)
	fmt.Fprintln(os.Stderr, "  Next: constitution setup")
	fmt.Fprintln(os.Stderr)
}
