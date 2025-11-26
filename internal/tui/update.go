package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tapcraft-io/purr/internal/exec"
	"github.com/tapcraft-io/purr/pkg/types"
)

// Update handles all state updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
		m.resourceList.SetWidth(msg.Width - 4)
		m.resourceList.SetHeight(msg.Height - 6)
		m.historyList.SetWidth(msg.Width - 4)
		m.historyList.SetHeight(msg.Height - 6)
		m.commandInput.Width = msg.Width - 6

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case cacheReadyMsg:
		m.ready = true
		m.statusMsg = "Cache ready"

	case commandResultMsg:
		m.cmdOutput = msg.result.Stdout
		if msg.result.Error != nil {
			m.cmdError = msg.result.Error
			m.cmdOutput += "\n" + msg.result.Stderr
			m.mode = types.ModeViewingOutput
			m.history.Add(msg.cmd, false, m.context, m.namespace)
		} else {
			m.cmdError = nil
			m.mode = types.ModeViewingOutput
			m.history.Add(msg.cmd, true, m.context, m.namespace)
		}
		m.viewport.SetContent(m.cmdOutput)
		m.viewport.GotoTop()
		// Save history after command execution
		_ = m.history.Save()

	case errMsg:
		m.err = msg.err
		m.mode = types.ModeError

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update active component based on mode
	switch m.mode {
	case types.ModeTyping:
		m.commandInput, cmd = m.commandInput.Update(msg)
		cmds = append(cmds, cmd)

	case types.ModeSelectingResource:
		m.resourceList, cmd = m.resourceList.Update(msg)
		cmds = append(cmds, cmd)

	case types.ModeViewingHistory:
		m.historyList, cmd = m.historyList.Update(msg)
		cmds = append(cmds, cmd)

	case types.ModeViewingOutput:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Global keybindings
	switch msg.String() {
	case "ctrl+c":
		if m.mode == types.ModeViewingOutput {
			// First Ctrl+C returns to typing mode
			m.mode = types.ModeTyping
			m.commandInput.Focus()
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "esc":
		// Cancel current operation and return to typing
		if m.mode != types.ModeTyping {
			m.mode = types.ModeTyping
			m.commandInput.Focus()
			return m, nil
		}
	}

	// Mode-specific keybindings
	switch m.mode {
	case types.ModeTyping:
		return m.handleTypingMode(msg)

	case types.ModeSelectingResource:
		return m.handleSelectingResourceMode(msg)

	case types.ModeViewingHistory:
		return m.handleViewingHistoryMode(msg)

	case types.ModeViewingOutput:
		return m.handleViewingOutputMode(msg)
	}

	return m, tea.Batch(cmds...)
}

// handleTypingMode handles key presses in typing mode
func (m Model) handleTypingMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg.String() {
	case "enter":
		// Execute command
		command := m.commandInput.Value()
		if command == "" {
			return m, nil
		}

		// Ensure it starts with kubectl
		if !strings.HasPrefix(command, "kubectl ") {
			command = "kubectl " + command
		}

		m.lastCmd = command
		m.statusMsg = "Executing command..."

		// Check if destructive
		if m.parser != nil && exec.IsDestructive(command) {
			m.mode = types.ModeConfirming
			return m, nil
		}

		// Execute the command
		if m.executor != nil {
			return m, executeCommand(m.executor, command)
		}

	case "ctrl+r":
		// Open history
		m.mode = types.ModeViewingHistory
		entries := m.history.GetAll()
		items := convertToListItems(m.history.ToListItems(entries))
		m.historyList.SetItems(items)
		return m, nil

	case "ctrl+l":
		// Clear screen
		m.cmdOutput = ""
		m.viewport.SetContent("")
		m.commandInput.SetValue("")
		return m, nil

	case "tab":
		// Trigger autocomplete
		command := m.commandInput.Value()
		if command == "" {
			return m, nil
		}

		// Parse command to see what completions are needed
		if m.parser != nil {
			parsed := m.parser.Parse(command)
			m.currentCmd = parsed

			// Check if we need to show namespace picker
			if strings.HasSuffix(command, "-n ") || strings.HasSuffix(command, "--namespace ") {
				return m.showNamespacePicker()
			}

			// Check if we need to show resource picker
			if parsed.Resource != "" && parsed.ResourceName == "" {
				return m.showResourcePicker(parsed.Resource, parsed.Namespace)
			}
		}
	}

	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleSelectingResourceMode handles key presses in resource selection mode
func (m Model) handleSelectingResourceMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Get selected item
		if selected, ok := m.resourceList.SelectedItem().(listItem); ok {
			// Append to command
			currentCmd := m.commandInput.Value()
			currentCmd = strings.TrimRight(currentCmd, " ")
			m.commandInput.SetValue(currentCmd + " " + selected.item.Title)

			// Return to typing mode
			m.mode = types.ModeTyping
			m.commandInput.Focus()
		}
		return m, nil

	case "esc":
		m.mode = types.ModeTyping
		m.commandInput.Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.resourceList, cmd = m.resourceList.Update(msg)
	return m, cmd
}

// handleViewingHistoryMode handles key presses in history viewing mode
func (m Model) handleViewingHistoryMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Execute selected command
		if selected, ok := m.historyList.SelectedItem().(listItem); ok {
			command := selected.item.Title
			m.commandInput.SetValue(command)
			m.mode = types.ModeTyping
			m.commandInput.Focus()

			// Optionally execute immediately
			if strings.HasPrefix(command, "kubectl ") {
				m.lastCmd = command
				if m.executor != nil {
					return m, executeCommand(m.executor, command)
				}
			}
		}
		return m, nil

	case "e":
		// Edit command before executing
		if selected, ok := m.historyList.SelectedItem().(listItem); ok {
			command := selected.item.Title
			m.commandInput.SetValue(command)
			m.mode = types.ModeTyping
			m.commandInput.Focus()
		}
		return m, nil

	case "esc":
		m.mode = types.ModeTyping
		m.commandInput.Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.historyList, cmd = m.historyList.Update(msg)
	return m, cmd
}

// handleViewingOutputMode handles key presses in output viewing mode
func (m Model) handleViewingOutputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "n", "q":
		// New command
		m.mode = types.ModeTyping
		m.commandInput.Focus()
		m.commandInput.SetValue("")
		return m, nil

	case "r":
		// Re-run last command
		if m.lastCmd != "" && m.executor != nil {
			return m, executeCommand(m.executor, m.lastCmd)
		}
		return m, nil

	case "e":
		// Edit and re-run
		if m.lastCmd != "" {
			m.commandInput.SetValue(m.lastCmd)
			m.mode = types.ModeTyping
			m.commandInput.Focus()
		}
		return m, nil

	case "esc":
		m.mode = types.ModeTyping
		m.commandInput.Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// showNamespacePicker shows the namespace picker
func (m Model) showNamespacePicker() (tea.Model, tea.Cmd) {
	if m.cache == nil || !m.cache.IsReady() {
		return m, nil
	}

	namespaces := m.cache.GetNamespaces()
	items := make([]types.ListItem, len(namespaces))
	for i, ns := range namespaces {
		items[i] = types.ListItem{
			Title:       ns,
			Description: "Namespace",
		}
	}

	m.resourceList.Title = "Select Namespace"
	m.resourceList.SetItems(convertToListItems(items))
	m.mode = types.ModeSelectingResource
	return m, nil
}

// showResourcePicker shows the resource picker for a specific resource type
func (m Model) showResourcePicker(resourceType, namespace string) (tea.Model, tea.Cmd) {
	if m.cache == nil || !m.cache.IsReady() {
		return m, nil
	}

	if namespace == "" {
		namespace = m.namespace
	}

	items := m.cache.GetResourceByType(resourceType, namespace)
	if len(items) == 0 {
		return m, nil
	}

	m.resourceList.Title = "Select " + resourceType
	m.resourceList.SetItems(convertToListItems(items))
	m.mode = types.ModeSelectingResource
	return m, nil
}

// KeyMap defines the keybindings
type KeyMap struct {
	Quit   key.Binding
	Enter  key.Binding
	Back   key.Binding
	Tab    key.Binding
	History key.Binding
	Clear  key.Binding
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("ctrl+c/q", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "execute/select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back/cancel"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "autocomplete"),
		),
		History: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "history"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "clear"),
		),
	}
}
