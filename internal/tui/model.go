package tui

import (
	"context"
	"fmt"
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
	ready     bool
	quitting  bool
	err       error
	statusMsg string
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
