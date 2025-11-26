package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tapcraft-io/purr/internal/exec"
	"github.com/tapcraft-io/purr/internal/history"
	"github.com/tapcraft-io/purr/internal/k8s"
	"github.com/tapcraft-io/purr/pkg/types"
)

// Model represents the application state
type Model struct {
	// UI Components
	commandInput textinput.Model
	resourceList list.Model
	viewport     viewport.Model
	historyList  list.Model
	spinner      spinner.Model

	// Application State
	mode   types.Mode
	width  int
	height int

	// Kubernetes State
	cache      *k8s.ResourceCache
	context    string
	namespace  string
	kubeconfig string

	// Command State
	currentCmd *types.ParsedCommand
	lastCmd    string
	cmdOutput  string
	cmdError   error

	// Services
	history  *history.History
	executor *exec.Executor
	parser   *exec.Parser

	// Flags
	ready          bool
	quitting       bool
	err            error
	statusMsg      string
	ctrlCPressed   int       // Track consecutive Ctrl+C presses
	ctrlCTime      time.Time // Track time of last Ctrl+C

	// Autocomplete
	suggestions    []string // Current autocomplete suggestions
	selectedSuggestion int  // Selected suggestion index
}

// NewModel creates a new application model
func NewModel(cache *k8s.ResourceCache, hist *history.History, ctx, kubeconfig string) Model {
	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "get pods"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	// Initialize executor and parser
	executor, err := exec.NewExecutor()
	if err != nil {
		// We'll handle this in the Init function
	}

	parser := exec.NewParser()

	// Initialize viewport
	vp := viewport.New(80, 20)
	vp.Style = viewportStyle

	// Initialize resource list
	delegate := list.NewDefaultDelegate()
	rl := list.New([]list.Item{}, delegate, 60, 20)
	rl.Title = "Select Resource"
	rl.SetShowStatusBar(false)
	rl.SetFilteringEnabled(true)

	// Initialize history list
	hl := list.New([]list.Item{}, delegate, 60, 20)
	hl.Title = "Command History"
	hl.SetShowStatusBar(false)
	hl.SetFilteringEnabled(true)

	return Model{
		commandInput: ti,
		resourceList: rl,
		viewport:     vp,
		historyList:  hl,
		spinner:      s,
		mode:         types.ModeTyping,
		cache:        cache,
		history:      hist,
		context:      ctx,
		kubeconfig:   kubeconfig,
		executor:     executor,
		parser:       parser,
		namespace:    "default",
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		spinner.Tick,
		checkCacheReady(m.cache),
	)
}

// Messages for async operations
type (
	cacheReadyMsg    struct{}
	commandResultMsg struct {
		result *exec.ExecuteResult
		cmd    string
	}
	errMsg struct{ err error }
)

// checkCacheReady checks if the cache is ready
func checkCacheReady(cache *k8s.ResourceCache) tea.Cmd {
	return func() tea.Msg {
		// Poll for cache readiness with a small delay
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		timeout := time.After(30 * time.Second)

		for {
			select {
			case <-timeout:
				// Timeout after 30 seconds
				return errMsg{err: fmt.Errorf("cache initialization timeout")}
			case <-ticker.C:
				if cache.IsReady() {
					return cacheReadyMsg{}
				}
			}
		}
	}
}

// executeCommand executes a command asynchronously
func executeCommand(executor *exec.Executor, command string) tea.Cmd {
	return func() tea.Msg {
		result := executor.ExecuteString(context.Background(), command)
		return commandResultMsg{
			result: result,
			cmd:    command,
		}
	}
}

// Item adapter for list.Item interface
type listItem struct {
	item types.ListItem
}

func (i listItem) FilterValue() string {
	return i.item.Title
}

func (i listItem) Title() string {
	return i.item.Title
}

func (i listItem) Description() string {
	return i.item.Description
}

// convertToListItems converts types.ListItem to list.Item
func convertToListItems(items []types.ListItem) []list.Item {
	result := make([]list.Item, len(items))
	for i, item := range items {
		result[i] = listItem{item: item}
	}
	return result
}

// getAutocompleteSuggestions generates autocomplete suggestions based on current input
func (m *Model) getAutocompleteSuggestions(input string) []string {
	trimmed := strings.TrimSpace(input)

	// Don't suggest for shell commands
	if strings.HasPrefix(trimmed, "!") {
		return nil
	}

	// Remove kubectl prefix if present
	if strings.HasPrefix(trimmed, "kubectl ") {
		trimmed = strings.TrimPrefix(trimmed, "kubectl ")
	}

	// If input is empty or just whitespace, suggest common commands
	if trimmed == "" {
		return []string{"get", "describe", "logs", "apply", "delete", "exec", "create", "rollout", "scale"}
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return []string{"get", "describe", "logs", "apply", "delete", "exec", "create", "rollout", "scale"}
	}

	// Determine if we're typing a partial token (no trailing space) or ready for next token
	hasTrailingSpace := len(input) > 0 && input[len(input)-1] == ' '
	lastToken := parts[len(parts)-1]

	// CASE 1: Typing/completing a command
	if len(parts) == 1 && !hasTrailingSpace {
		return m.suggestCommands(lastToken)
	}

	// CASE 2: Command complete, need next token
	if len(parts) == 1 && hasTrailingSpace {
		heuristic, ok := GetCommandHeuristic(parts[0])
		if !ok {
			return nil
		}

		// Check if command has subcommands (like rollout, config, etc.)
		if len(heuristic.RequiredArgs) > 0 && heuristic.RequiredArgs[0].Type == ArgTypeString {
			// Return subcommands from description
			return m.extractSubcommands(heuristic.RequiredArgs[0].Description)
		}

		// Otherwise suggest resource types
		return ResourceTypeCompletions
	}

	// CASE 3: Multi-part command - need to determine context
	cmd := parts[0]
	heuristic, ok := GetCommandHeuristic(cmd)
	if !ok {
		return nil
	}

	// Check if we're completing a subcommand
	if len(heuristic.RequiredArgs) > 0 && heuristic.RequiredArgs[0].Type == ArgTypeString {
		if len(parts) == 2 && !hasTrailingSpace {
			// Typing subcommand
			return m.filterSubcommands(m.extractSubcommands(heuristic.RequiredArgs[0].Description), lastToken)
		}
		if len(parts) == 2 && hasTrailingSpace {
			// Subcommand complete, suggest resource types
			return ResourceTypeCompletions
		}
		if len(parts) == 3 && !hasTrailingSpace {
			// Typing resource type after subcommand
			return m.filterResourceTypes(lastToken)
		}
	}

	// Check if last token is a flag
	if strings.HasPrefix(lastToken, "-") && !hasTrailingSpace {
		return m.suggestFlags(heuristic, lastToken)
	}

	// Check if previous token was a flag that needs a value
	if len(parts) >= 2 && strings.HasPrefix(parts[len(parts)-2], "-") {
		flagName := strings.TrimLeft(parts[len(parts)-2], "-")
		return m.suggestFlagValue(heuristic, flagName, lastToken, hasTrailingSpace)
	}

	// Check if we're completing a resource type
	resourceArgIndex := m.getResourceTypeArgIndex(heuristic)
	if resourceArgIndex >= 0 && len(parts) == resourceArgIndex+2 && !hasTrailingSpace {
		// +2 because: cmd is index 0, resource type is at resourceArgIndex+1 in parts
		return m.filterResourceTypes(lastToken)
	}

	// If resource type complete, suggest flags
	if resourceArgIndex >= 0 && len(parts) == resourceArgIndex+2 && hasTrailingSpace {
		return m.suggestCommonFlags(heuristic)
	}

	// If we have command + resource, suggest flags or resource names
	if len(parts) >= 2 && hasTrailingSpace {
		return m.suggestCommonFlags(heuristic)
	}

	return nil
}

// suggestCommands suggests commands matching the prefix
func (m *Model) suggestCommands(prefix string) []string {
	var suggestions []string
	for cmd := range KubectlHeuristics {
		if strings.HasPrefix(cmd, prefix) {
			suggestions = append(suggestions, cmd)
		}
	}
	return suggestions
}

// extractSubcommands extracts subcommands from description field
func (m *Model) extractSubcommands(description string) []string {
	// Look for patterns like "status|history|pause|resume|restart|undo"
	if strings.Contains(description, "|") {
		return strings.Split(description, "|")
	}
	return nil
}

// filterSubcommands filters subcommands by prefix
func (m *Model) filterSubcommands(subcommands []string, prefix string) []string {
	if len(subcommands) == 0 {
		return nil
	}
	var filtered []string
	for _, sub := range subcommands {
		if strings.HasPrefix(sub, prefix) {
			filtered = append(filtered, sub)
		}
	}
	return filtered
}

// filterResourceTypes filters resource types by prefix
func (m *Model) filterResourceTypes(prefix string) []string {
	var suggestions []string
	for _, resource := range ResourceTypeCompletions {
		if strings.HasPrefix(resource, prefix) {
			suggestions = append(suggestions, resource)
		}
	}
	return suggestions
}

// suggestFlags suggests flags matching the prefix
func (m *Model) suggestFlags(heuristic CommandHeuristic, prefix string) []string {
	var suggestions []string

	// Determine if we want short or long flags
	wantsShort := len(prefix) == 1 || (len(prefix) == 2 && !strings.HasPrefix(prefix, "--"))
	wantsLong := strings.HasPrefix(prefix, "--")

	for _, flag := range heuristic.Flags {
		if wantsShort && flag.Shorthand != "" {
			shortFlag := "-" + flag.Shorthand
			if strings.HasPrefix(shortFlag, prefix) {
				suggestions = append(suggestions, shortFlag)
			}
		}
		if wantsLong || !wantsShort {
			longFlag := "--" + flag.Name
			if strings.HasPrefix(longFlag, prefix) {
				suggestions = append(suggestions, longFlag)
			}
		}
	}
	return suggestions
}

// suggestCommonFlags suggests common flags (without prefix filter)
func (m *Model) suggestCommonFlags(heuristic CommandHeuristic) []string {
	var suggestions []string
	// Prioritize common flags
	commonFlagNames := []string{"namespace", "all-namespaces", "output", "selector", "watch", "follow"}

	for _, commonName := range commonFlagNames {
		for _, flag := range heuristic.Flags {
			if flag.Name == commonName {
				if flag.Shorthand != "" {
					suggestions = append(suggestions, "-"+flag.Shorthand)
				} else {
					suggestions = append(suggestions, "--"+flag.Name)
				}
				break
			}
		}
	}

	// Limit to top suggestions
	if len(suggestions) > 6 {
		return suggestions[:6]
	}
	return suggestions
}

// suggestFlagValue suggests values for a flag
func (m *Model) suggestFlagValue(heuristic CommandHeuristic, flagName, currentValue string, hasTrailingSpace bool) []string {
	// Find the flag spec
	var flagSpec *FlagSpec
	for i, flag := range heuristic.Flags {
		if flag.Name == flagName || flag.Shorthand == flagName {
			flagSpec = &heuristic.Flags[i]
			break
		}
	}

	if flagSpec == nil {
		return nil
	}

	// Handle specific flag completions
	switch flagSpec.Completion {
	case CompletionNamespace:
		// If we're here, user is typing the namespace name, not ready for picker yet
		if !hasTrailingSpace {
			return nil // Let them type, or they can use picker with second tab
		}
	case CompletionNone:
		// Check for specific flags with known values
		if flagName == "dry-run" || strings.Contains(flagName, "dry-run") {
			return DryRunValues
		}
		if flagName == "output" || flagName == "o" {
			return OutputFormatCompletions
		}
	}

	return nil
}

// getResourceTypeArgIndex finds the index of resource type argument
func (m *Model) getResourceTypeArgIndex(heuristic CommandHeuristic) int {
	for i, arg := range heuristic.RequiredArgs {
		if arg.Type == ArgTypeResourceType {
			return i
		}
	}
	return -1
}
