package exec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// PaneOutputMsg represents a chunk of output for a command pane
// It includes a NextCmd that should be called to continue streaming
type PaneOutputMsg struct {
	PaneID  int
	Output  string
	NextCmd tea.Cmd // Call this to continue streaming
}

// PaneCompleteMsg indicates a pane command has completed
type PaneCompleteMsg struct {
	PaneID   int
	ExitCode int
	Error    error
}

// ExecuteStreaming runs a command and streams output via tea messages
func (e *Executor) ExecuteStreaming(ctx context.Context, command string, paneID int) tea.Cmd {
	trimmed := strings.TrimSpace(command)

	var cmd *exec.Cmd
	if strings.HasPrefix(trimmed, "!") {
		shellCmd := strings.TrimSpace(strings.TrimPrefix(trimmed, "!"))
		if shellCmd == "" {
			return func() tea.Msg {
				return PaneCompleteMsg{
					PaneID:   paneID,
					ExitCode: 1,
					Error:    fmt.Errorf("empty shell command"),
				}
			}
		}
		cmd = exec.CommandContext(ctx, "sh", "-c", shellCmd)
	} else {
		args := parseCommandString(trimmed)
		cmd = exec.CommandContext(ctx, e.kubectlPath, args...)
	}

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return func() tea.Msg {
			return PaneCompleteMsg{
				PaneID:   paneID,
				ExitCode: -1,
				Error:    fmt.Errorf("failed to create stdout pipe: %w", err),
			}
		}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return func() tea.Msg {
			return PaneCompleteMsg{
				PaneID:   paneID,
				ExitCode: -1,
				Error:    fmt.Errorf("failed to create stderr pipe: %w", err),
			}
		}
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return func() tea.Msg {
			return PaneCompleteMsg{
				PaneID:   paneID,
				ExitCode: -1,
				Error:    fmt.Errorf("failed to start command: %w", err),
			}
		}
	}

	// Create channel for streaming
	outputChan := make(chan string, 100)

	// Start goroutine to read output
	go func() {
		defer close(outputChan)

		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		// Increase buffer size for long lines
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			select {
			case outputChan <- scanner.Text() + "\n":
			case <-ctx.Done():
				return
			}
		}

		// Wait for command to finish
		cmd.Wait()
	}()

	// Return a command that waits for the first output
	return waitForPaneOutput(paneID, outputChan, cmd)
}

// waitForPaneOutput returns a tea.Cmd that waits for output from a pane
func waitForPaneOutput(paneID int, outputChan <-chan string, cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		// Try to collect some output with a small timeout
		var output strings.Builder
		timeout := time.NewTimer(100 * time.Millisecond)
		defer timeout.Stop()

		// Collect output until timeout or channel closed
		for {
			select {
			case line, ok := <-outputChan:
				if !ok {
					// Channel closed - command finished
					if output.Len() > 0 {
						// Return final output with no next command
						return PaneOutputMsg{
							PaneID:  paneID,
							Output:  output.String(),
							NextCmd: completionCmd(paneID, cmd),
						}
					}
					// No output, just return completion
					return completionMsg(paneID, cmd)
				}
				output.WriteString(line)
				// Reset timer to collect more output in this batch
				if !timeout.Stop() {
					select {
					case <-timeout.C:
					default:
					}
				}
				timeout.Reset(50 * time.Millisecond)

			case <-timeout.C:
				// Timeout - return what we have and schedule next read
				if output.Len() > 0 {
					return PaneOutputMsg{
						PaneID:  paneID,
						Output:  output.String(),
						NextCmd: waitForPaneOutput(paneID, outputChan, cmd),
					}
				}
				// No output yet, keep waiting (return another wait command)
				return PaneOutputMsg{
					PaneID:  paneID,
					Output:  "",
					NextCmd: waitForPaneOutput(paneID, outputChan, cmd),
				}
			}
		}
	}
}

// completionCmd returns a command that produces the completion message
func completionCmd(paneID int, cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		return completionMsg(paneID, cmd)
	}
}

// completionMsg creates the completion message for a pane
func completionMsg(paneID int, cmd *exec.Cmd) PaneCompleteMsg {
	exitCode := 0
	var err error
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		exitCode = cmd.ProcessState.ExitCode()
		err = fmt.Errorf("command exited with code %d", exitCode)
	}
	return PaneCompleteMsg{PaneID: paneID, ExitCode: exitCode, Error: err}
}
