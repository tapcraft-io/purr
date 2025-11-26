package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Executor executes kubectl commands
type Executor struct {
	kubectlPath string
}

// ExecuteResult contains the result of a kubectl execution
type ExecuteResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Error    error
}

// NewExecutor creates a new kubectl executor
func NewExecutor() (*Executor, error) {
	// Find kubectl in PATH
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("kubectl not found in PATH: %w", err)
	}

	return &Executor{
		kubectlPath: kubectlPath,
	}, nil
}

// Execute runs a kubectl command
func (e *Executor) Execute(ctx context.Context, args []string) *ExecuteResult {
	start := time.Now()
	result := &ExecuteResult{}

	cmd := exec.CommandContext(ctx, e.kubectlPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		result.Error = err
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// ExecuteString runs a command from a string. Commands starting with "!" are
// executed directly in the shell, all others are treated as kubectl commands.
func (e *Executor) ExecuteString(ctx context.Context, command string) *ExecuteResult {
	trimmed := strings.TrimSpace(command)

	if strings.HasPrefix(trimmed, "!") {
		shellCmd := strings.TrimSpace(strings.TrimPrefix(trimmed, "!"))
		if shellCmd == "" {
			return &ExecuteResult{Error: fmt.Errorf("empty shell command"), ExitCode: 1}
		}
		return e.executeShell(ctx, shellCmd)
	}

	// Parse command string into args
	args := parseCommandString(trimmed)
	return e.Execute(ctx, args)
}

// executeShell runs a command directly in the shell
func (e *Executor) executeShell(ctx context.Context, command string) *ExecuteResult {
	start := time.Now()
	result := &ExecuteResult{}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		result.Error = err
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// parseCommandString splits a command string into arguments
// This is a simple implementation - doesn't handle quotes perfectly
func parseCommandString(command string) []string {
	// Remove "kubectl" prefix if present
	command = strings.TrimPrefix(command, "kubectl ")
	command = strings.TrimSpace(command)

	if command == "" {
		return []string{}
	}

	// Simple split on whitespace
	// TODO: Handle quoted strings properly
	return strings.Fields(command)
}

// IsDestructive checks if a command is destructive (requires confirmation)
func IsDestructive(command string) bool {
	trimmed := strings.TrimSpace(command)

	if strings.HasPrefix(trimmed, "!") {
		return false
	}

	args := strings.Fields(trimmed)
	if len(args) == 0 {
		return false
	}

	// Check for destructive verbs
	verb := args[0]
	destructiveVerbs := []string{
		"delete",
		"drain",
		"cordon",
		"rollout",
	}

	for _, dv := range destructiveVerbs {
		if verb == dv {
			return true
		}
	}

	// Check for --force flag
	for _, arg := range args {
		if arg == "--force" {
			return true
		}
	}

	return false
}

// GetCommandVerb extracts the kubectl verb from a command string
func GetCommandVerb(command string) string {
	command = strings.TrimSpace(command)

	if strings.HasPrefix(command, "!") {
		return ""
	}

	command = strings.TrimPrefix(command, "kubectl ")

	args := strings.Fields(command)
	if len(args) == 0 {
		return ""
	}

	return args[0]
}
