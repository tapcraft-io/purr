package tui

import (
	"context"
	"fmt"
	"os"
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
	"github.com/tapcraft-io/purr/internal/kubecomplete"
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
	cache      k8s.Cache
	context    string
	namespace  string
	kubeconfig string

	// Command State
	currentCmd *types.ParsedCommand
	lastCmd    string
	cmdOutput  string
	cmdError   error

	// Services
	history   *history.History
	executor  *exec.Executor
	parser    *exec.Parser
	completer *kubecomplete.Completer

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
	// Don't suggest for shell commands
	if strings.HasPrefix(strings.TrimSpace(input), "!") {
		return nil
	}

	// Don't suggest if no completer
	if m.completer == nil {
		return nil
	}

	// Use the new kubecomplete engine
	ctx := kubecomplete.CompletionContext{
		Line:             input,
		Cursor:           len(input),
		CurrentNamespace: m.namespace,
	}

	suggestions := m.completer.Complete(input, len(input), ctx)
	if len(suggestions) == 0 {
		return nil
	}

	// Check if user is typing a partial token (no trailing space)
	hasTrailingSpace := len(input) > 0 && input[len(input)-1] == ' '
	trimmed := strings.TrimSpace(input)
	tokens := strings.Fields(trimmed)

	// Determine if we should filter by current partial token
	var currentPartial string
	if !hasTrailingSpace && len(tokens) > 0 {
		// Check if the current tokens match a complete command
		// If so, we're suggesting the next token, not completing the command
		cmd, pathLen := m.completer.Registry.MatchCommand(tokens)
		if cmd != nil && pathLen == len(tokens) {
			// Tokens match a complete command - don't filter
			currentPartial = ""
		} else {
			// Either no command match or partial match - filter by last token
			currentPartial = tokens[len(tokens)-1]
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

	return result
}

