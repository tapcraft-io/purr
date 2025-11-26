package tui

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// KubectlCompleter uses kubectl's native __complete command for suggestions
type KubectlCompleter struct {
	timeout time.Duration
}

// NewKubectlCompleter creates a new completer that delegates to kubectl
func NewKubectlCompleter() *KubectlCompleter {
	return &KubectlCompleter{
		timeout: 500 * time.Millisecond, // Fast timeout for responsive UX
	}
}

// Complete gets completions from kubectl's native completion system
func (k *KubectlCompleter) Complete(input string) []string {
	// kubectl __complete expects the command and an empty string for word to complete
	ctx, cancel := context.WithTimeout(context.Background(), k.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "__complete", input, "")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to our basic heuristics if kubectl fails
		return nil
	}

	lines := strings.Split(string(output), "\n")
	var suggestions []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			// Skip empty lines and completion directives (e.g., ":4")
			continue
		}
		suggestions = append(suggestions, line)
	}

	return suggestions
}

// GetFullSuggestions returns full command suggestions for textinput
func (k *KubectlCompleter) GetFullSuggestions(input string) []string {
	completions := k.Complete(input)
	if len(completions) == 0 {
		return nil
	}

	var fullSuggestions []string
	trimmed := strings.TrimSpace(input)

	// Determine if we're completing the current token or adding a new one
	hasTrailingSpace := len(input) > 0 && input[len(input)-1] == ' '

	if hasTrailingSpace {
		// Append suggestions as new tokens
		for _, comp := range completions {
			fullSuggestions = append(fullSuggestions, trimmed+" "+comp)
		}
	} else {
		// Replace last token with suggestion
		parts := strings.Fields(trimmed)
		if len(parts) == 0 {
			return completions
		}

		prefix := ""
		if len(parts) > 1 {
			prefix = strings.Join(parts[:len(parts)-1], " ") + " "
		}

		for _, comp := range completions {
			fullSuggestions = append(fullSuggestions, prefix+comp)
		}
	}

	return fullSuggestions
}
