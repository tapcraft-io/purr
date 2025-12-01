package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tapcraft-io/purr/internal/exec"
	"github.com/tapcraft-io/purr/internal/history"
	"github.com/tapcraft-io/purr/internal/k8s"
	"github.com/tapcraft-io/purr/internal/kubecomplete"
	"github.com/tapcraft-io/purr/pkg/types"
)

// PaneData holds the runtime data for a command pane
type PaneData struct {
	types.CommandPane
	Output   *strings.Builder // Pointer to avoid copy issues with BubbleTea
	Viewport viewport.Model
}

// Model represents the application state
type Model struct {
	// UI Components
	commandInput textinput.Model
	resourceList list.Model
	viewport     viewport.Model
	historyList  list.Model
	spinner      spinner.Model
	filePicker   filepicker.Model

	// Application State
	mode   types.Mode
	width  int
	height int

	// Kubernetes State
	cache      k8s.Cache
	context    string
	namespace  string
	kubeconfig string

	// Command State
	currentCmd *types.ParsedCommand
	lastCmd    string
	cmdOutput  string
	cmdError   error

	// Pane State (for parallel execution)
	panes           []PaneData
	activePaneIndex int
	nextPaneID      int

	// Services
	history   *history.History
	executor  *exec.Executor
	parser    *exec.Parser
	completer *kubecomplete.Completer

	// Autocomplete state
	suggestions     []string
	suggestionIndex int // Currently selected suggestion (0 = first)

	// Flags
	ready        bool
	quitting     bool
	err          error
	statusMsg    string
	ctrlCPressed int       // Track consecutive Ctrl+C presses
	ctrlCTime    time.Time // Track time of last Ctrl+C
}

// NewModel creates a new application model
func NewModel(cache k8s.Cache, hist *history.History, ctx, kubeconfig string, completer *kubecomplete.Completer) Model {
	// Initialize text input with suggestion support
	ti := textinput.New()
	ti.Placeholder = "get pods"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80
	ti.ShowSuggestions = true
	// Set initial suggestions to common commands
	ti.SetSuggestions([]string{"get", "describe", "logs", "apply", "delete", "exec", "create", "rollout", "scale"})

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

	// Initialize file picker
	fp := filepicker.New()
	fp.CurrentDirectory, _ = os.Getwd()
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = true
	fp.Height = 15

	return Model{
		commandInput: ti,
		resourceList: rl,
		viewport:     vp,
		historyList:  hl,
		spinner:      s,
		filePicker:   fp,
		mode:         types.ModeTyping,
		width:        80, // Sensible default, will be updated on WindowSizeMsg
		height:       24, // Sensible default, will be updated on WindowSizeMsg
		cache:        cache,
		history:      hist,
		context:      ctx,
		kubeconfig:   kubeconfig,
		executor:     executor,
		parser:       parser,
		completer:    completer,
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
func checkCacheReady(cache k8s.Cache) tea.Cmd {
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

// debugLog writes debug messages to a file
func debugLog(msg string) {
	f, err := os.OpenFile("/tmp/purr-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", msg)
}

// getAutocompleteSuggestions generates autocomplete suggestions based on current input
// Returns just the next token(s) to suggest, not full commands
func (m *Model) getAutocompleteSuggestions(input string) []string {
	debugLog(fmt.Sprintf("=== getAutocompleteSuggestions input=%q ===", input))

	// Don't suggest for shell commands
	if strings.HasPrefix(strings.TrimSpace(input), "!") {
		debugLog("skipping: shell command")
		return nil
	}

	// Don't suggest if no completer
	if m.completer == nil {
		debugLog("skipping: no completer")
		return nil
	}

	// Use the new kubecomplete engine
	ctx := kubecomplete.CompletionContext{
		Line:             input,
		Cursor:           len(input),
		CurrentNamespace: m.namespace,
	}

	suggestions := m.completer.Complete(input, len(input), ctx)
	debugLog(fmt.Sprintf("completer returned %d suggestions", len(suggestions)))
	if len(suggestions) > 0 {
		first := min(5, len(suggestions))
		debugLog(fmt.Sprintf("first few: %+v", suggestions[:first]))
	}

	if len(suggestions) == 0 {
		return nil
	}

	// Check if user is typing a partial token (no trailing space)
	hasTrailingSpace := len(input) > 0 && input[len(input)-1] == ' '
	trimmed := strings.TrimSpace(input)
	tokens := strings.Fields(trimmed)
	debugLog(fmt.Sprintf("hasTrailingSpace=%v, tokens=%v", hasTrailingSpace, tokens))

	// Determine if we should filter by current partial token
	var currentPartial string
	if !hasTrailingSpace && len(tokens) > 0 {
		// Check if the current tokens match a complete command
		// If so, we're suggesting the next token, not completing the command
		cmd, pathLen := m.completer.Registry.MatchCommand(tokens)
		debugLog(fmt.Sprintf("cmd=%v, pathLen=%d, len(tokens)=%d", cmd != nil, pathLen, len(tokens)))
		if cmd != nil && pathLen == len(tokens) {
			// Tokens match a complete command - don't filter
			currentPartial = ""
			debugLog("complete command match - no filtering")
		} else {
			// Either no command match or partial match - filter by last token
			currentPartial = tokens[len(tokens)-1]
			debugLog(fmt.Sprintf("partial/no match - filter by %q", currentPartial))
		}
	}

	result := make([]string, 0, len(suggestions))
	for _, sug := range suggestions {
		// Filter by partial token if we're typing one
		if currentPartial != "" && !strings.HasPrefix(sug.Value, currentPartial) {
			continue
		}

		result = append(result, sug.Value)

		if len(result) >= 20 { // Limit to 20 suggestions
			break
		}
	}

	debugLog(fmt.Sprintf("returning %d results: %v", len(result), result))
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper methods for pane management

// createPane creates a new pane for a command
func (m *Model) createPane(command string, cancel context.CancelFunc) int {
	paneID := m.nextPaneID
	m.nextPaneID++

	vp := viewport.New(80, 20)
	vp.Style = viewportStyle

	pane := PaneData{
		CommandPane: types.CommandPane{
			ID:        paneID,
			Command:   command,
			StartTime: time.Now(),
			Status:    types.PaneStatusRunning,
			Cancel:    cancel,
		},
		Output:   &strings.Builder{}, // Use pointer to avoid copy issues
		Viewport: vp,
	}

	m.panes = append(m.panes, pane)
	m.activePaneIndex = len(m.panes) - 1

	return paneID
}

// removePane removes a pane by index
func (m *Model) removePane(index int) {
	if index < 0 || index >= len(m.panes) {
		return
	}

	// Cancel the command if it's still running
	if m.panes[index].Cancel != nil {
		m.panes[index].Cancel()
	}

	// Remove the pane
	m.panes = append(m.panes[:index], m.panes[index+1:]...)

	// Adjust active pane index
	if m.activePaneIndex >= len(m.panes) && len(m.panes) > 0 {
		m.activePaneIndex = len(m.panes) - 1
	} else if len(m.panes) == 0 {
		m.activePaneIndex = 0
	}
}

// findPaneByID finds a pane by its ID and returns its index
func (m *Model) findPaneByID(paneID int) int {
	for i, pane := range m.panes {
		if pane.ID == paneID {
			return i
		}
	}
	return -1
}

// cyclePaneForward moves to the next pane
func (m *Model) cyclePaneForward() {
	if len(m.panes) == 0 {
		return
	}
	m.activePaneIndex = (m.activePaneIndex + 1) % len(m.panes)
}

// cyclePaneBackward moves to the previous pane
func (m *Model) cyclePaneBackward() {
	if len(m.panes) == 0 {
		return
	}
	m.activePaneIndex--
	if m.activePaneIndex < 0 {
		m.activePaneIndex = len(m.panes) - 1
	}
}

// isLongRunningCommand checks if a command is likely to be long-running
func isLongRunningCommand(command string) bool {
	trimmed := strings.TrimSpace(command)

	// Shell commands with certain patterns are long-running
	if strings.HasPrefix(trimmed, "!") {
		shellCmd := strings.TrimSpace(strings.TrimPrefix(trimmed, "!"))
		// Note: top/htop require TTY, use "top -b" for batch mode
		longRunningPrefixes := []string{
			"tail -f",
			"tail -F",
			"watch",
			"top -b", // batch mode only
			"vmstat",
			"iostat",
			"dmesg -w",
		}

		for _, prefix := range longRunningPrefixes {
			if strings.HasPrefix(shellCmd, prefix) {
				return true
			}
		}
	}

	// kubectl commands that stream
	if strings.Contains(trimmed, "logs") && strings.Contains(trimmed, "-f") {
		return true
	}

	return false
}
