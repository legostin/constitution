package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

type checklistItem struct {
	Label       string
	Description string
	Selected    bool
}

// checklist shows an interactive multi-select list.
// Returns the items with Updated .Selected fields.
func checklist(title string, items []checklistItem) []checklistItem {
	fd := int(os.Stdin.Fd())

	// Switch to raw mode for keypress reading
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback: return items as-is if terminal not available
		return items
	}
	defer term.Restore(fd, oldState)

	cursor := 0
	buf := make([]byte, 3)

	draw := func(first bool) {
		if !first {
			// Move cursor up to redraw
			fmt.Fprintf(os.Stderr, "\033[%dA", len(items)+3)
		}
		fmt.Fprintf(os.Stderr, "\033[1m%s\033[0m\r\n", title)
		fmt.Fprintf(os.Stderr, "\033[2m  ↑/↓ navigate · Space toggle · a all · n none · Enter confirm\033[0m\r\n")
		fmt.Fprintf(os.Stderr, "\r\n")

		for i, item := range items {
			check := " "
			if item.Selected {
				check = "\033[32m✔\033[0m"
			}
			prefix := "  "
			if i == cursor {
				prefix = "\033[36m▸\033[0m "
			}
			label := item.Label
			if i == cursor {
				label = "\033[1m" + label + "\033[0m"
			}
			fmt.Fprintf(os.Stderr, "  %s [%s] %s \033[2m%s\033[0m\r\n", prefix, check, label, item.Description)
		}
	}

	draw(true)

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}

		switch {
		// Arrow up
		case n == 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'A':
			if cursor > 0 {
				cursor--
			}
		// Arrow down
		case n == 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'B':
			if cursor < len(items)-1 {
				cursor++
			}
		// Space — toggle
		case n == 1 && buf[0] == ' ':
			items[cursor].Selected = !items[cursor].Selected
		// Enter — confirm
		case n == 1 && buf[0] == '\r':
			return items
		// 'a' — select all
		case n == 1 && buf[0] == 'a':
			for i := range items {
				items[i].Selected = true
			}
		// 'n' — select none
		case n == 1 && buf[0] == 'n':
			for i := range items {
				items[i].Selected = false
			}
		// 'q' or Ctrl-C — abort
		case n == 1 && (buf[0] == 'q' || buf[0] == 3):
			return items
		}

		draw(false)
	}

	return items
}

// promptChoice shows a simple numbered menu and returns the 0-based index.
func promptChoice(title string, options []string, defaultIdx int) int {
	fmt.Fprintf(os.Stderr, "\033[1m%s\033[0m\n", title)
	for i, opt := range options {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, opt)
	}
	fmt.Fprintf(os.Stderr, "\n")

	var choice int
	fmt.Fprintf(os.Stderr, "  Choose [%d]: ", defaultIdx+1)
	_, err := fmt.Fscanf(os.Stdin, "%d", &choice)
	if err != nil || choice < 1 || choice > len(options) {
		return defaultIdx
	}
	return choice - 1
}

// promptString asks for a string input.
func promptString(prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "  %s: ", prompt)
	}
	var val string
	fmt.Fscanln(os.Stdin, &val)
	if val == "" {
		return defaultVal
	}
	return val
}

// promptYN asks a yes/no question.
func promptYN(prompt string, defaultYes bool) bool {
	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}
	fmt.Fprintf(os.Stderr, "  %s %s: ", prompt, suffix)
	var val string
	fmt.Fscanln(os.Stdin, &val)
	if val == "" {
		return defaultYes
	}
	return val == "y" || val == "Y"
}
