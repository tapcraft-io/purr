package tui

import (
	"fmt"
	"strings"
	"time"

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
			if m.history != nil {
				m.history.Add(msg.cmd, false, m.context, m.namespace)
			}
		} else {
			m.cmdError = nil
			if m.history != nil {
				m.history.Add(msg.cmd, true, m.context, m.namespace)
			}
		}
		m.viewport.SetContent(m.cmdOutput)
		m.viewport.GotoTop()
		// Save history after command execution
		if m.history != nil {
			_ = m.history.Save()
		}
		// Return to typing mode with cleared input and suggestions - output remains visible
		m.mode = types.ModeTyping
		m.commandInput.SetValue("")
		m.commandInput.Focus()
		// Reset suggestions to default commands
		m.suggestions = []string{"get", "describe", "logs", "apply", "delete", "exec", "create", "rollout", "scale"}
		m.suggestionIndex = 0
		m.commandInput.SetSuggestions(m.suggestions)

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

	case types.ModeSelectingFile:
		m.filePicker, cmd = m.filePicker.Update(msg)
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
		now := time.Now()
		// Reset counter if more than 1 second has passed since last Ctrl+C
		if now.Sub(m.ctrlCTime) > time.Second {
			m.ctrlCPressed = 0
		}

		m.ctrlCPressed++
		m.ctrlCTime = now

		// Require double Ctrl+C to quit
		if m.ctrlCPressed >= 2 {
			m.quitting = true
			return m, tea.Quit
		}

		// First Ctrl+C shows a hint
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil

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

	case types.ModeSelectingFile:
		return m.handleSelectingFileMode(msg)

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

	// Reset ctrl+c counter on any other key
	if msg.String() != "ctrl+c" {
		m.ctrlCPressed = 0
	}

	switch msg.String() {
	case "tab", "right":
		// Accept the currently selected suggestion
		if len(m.suggestions) > 0 && m.suggestionIndex < len(m.suggestions) {
			currentInput := m.commandInput.Value()
			suggestion := m.suggestions[m.suggestionIndex]

			// Determine how to append the suggestion
			if len(currentInput) > 0 && currentInput[len(currentInput)-1] != ' ' {
				// Replace the last partial token with the suggestion
				tokens := strings.Fields(strings.TrimSpace(currentInput))
				if len(tokens) > 0 {
					// Remove last partial token and add suggestion
					prefix := strings.TrimSuffix(currentInput, tokens[len(tokens)-1])
					m.commandInput.SetValue(prefix + suggestion + " ")
				}
			} else {
				// Append suggestion after the space
				m.commandInput.SetValue(currentInput + suggestion + " ")
			}
			m.commandInput.CursorEnd()

			// Update suggestions for new input and reset index
			m.suggestions = m.getAutocompleteSuggestions(m.commandInput.Value())
			m.suggestionIndex = 0
			m.commandInput.SetSuggestions(m.suggestions)
		}
		return m, nil

	case "enter":
		command, isShell, err := m.prepareCommand(m.commandInput.Value())
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}

		m.lastCmd = command
		m.statusMsg = "Executing command..."

		// Check if destructive
		if !isShell && m.parser != nil && exec.IsDestructive(command) {
			m.mode = types.ModeConfirming
			return m, nil
		}

		// Execute the command
		if m.executor != nil {
			return m, executeCommand(m.executor, command)
		}

	case "ctrl+r":
		// Open history
		if m.history != nil {
			m.mode = types.ModeViewingHistory
			entries := m.history.GetAll()
			items := convertToListItems(m.history.ToListItems(entries))
			m.historyList.SetItems(items)
		}
		return m, nil

	case "ctrl+o":
		// View full output
		if m.cmdOutput != "" {
			m.mode = types.ModeViewingOutput
		}
		return m, nil

	case "ctrl+l":
		// Clear screen
		m.cmdOutput = ""
		m.viewport.SetContent("")
		m.commandInput.SetValue("")
		m.suggestionIndex = 0
		m.commandInput.SetSuggestions([]string{"get", "describe", "logs", "apply", "delete", "exec", "create", "rollout", "scale"})
		return m, nil

	case "down", "ctrl+n":
		// Cycle to next suggestion
		if len(m.suggestions) > 0 {
			m.suggestionIndex++
			if m.suggestionIndex >= len(m.suggestions) {
				m.suggestionIndex = 0
			}
		}
		return m, nil

	case "up", "ctrl+p":
		// Cycle to previous suggestion
		if len(m.suggestions) > 0 {
			m.suggestionIndex--
			if m.suggestionIndex < 0 {
				m.suggestionIndex = len(m.suggestions) - 1
			}
		}
		return m, nil

	case "@":
		// Open file picker
		return m.showFilePicker()

	case "ctrl+space":
		// Show resource/namespace picker if applicable
		command := m.commandInput.Value()
		trimmedCmd := strings.TrimSpace(command)
		if trimmedCmd != "" && !strings.HasPrefix(trimmedCmd, "!") {
			// Parse command to see what completions are needed
			if m.parser != nil {
				parsed := m.parser.Parse(trimmedCmd)
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
		return m, nil
	}

	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)
	cmds = append(cmds, cmd)

	// Update autocomplete suggestions after every keystroke
	newSuggestions := m.getAutocompleteSuggestions(m.commandInput.Value())
	// Reset index if suggestions changed
	if len(newSuggestions) != len(m.suggestions) || (len(newSuggestions) > 0 && len(m.suggestions) > 0 && newSuggestions[0] != m.suggestions[0]) {
		m.suggestionIndex = 0
	}
	m.suggestions = newSuggestions
	// Still set them on the textinput for its built-in ghost text
	m.commandInput.SetSuggestions(m.suggestions)

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

			preparedCmd, _, err := m.prepareCommand(command)
			if err == nil && m.executor != nil {
				m.lastCmd = preparedCmd
				return m, executeCommand(m.executor, preparedCmd)
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
	case "n", "q", "esc":
		// New command - clear output and return to typing
		m.cmdOutput = ""
		m.viewport.SetContent("")
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
			m.cmdOutput = ""
			m.viewport.SetContent("")
			m.commandInput.SetValue(m.lastCmd)
			m.mode = types.ModeTyping
			m.commandInput.Focus()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// prepareCommand normalizes user input into an executable command string and
// reports whether it should be run as a shell command.
func (m Model) prepareCommand(raw string) (string, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, fmt.Errorf("please enter a command")
	}

	// Explicit shell command with ! prefix
	if strings.HasPrefix(trimmed, "!") {
		shell := strings.TrimSpace(strings.TrimPrefix(trimmed, "!"))
		if shell == "" {
			return "", true, fmt.Errorf("shell command cannot be empty")
		}
		return "!" + shell, true, nil
	}

	// Already has kubectl prefix
	if strings.HasPrefix(trimmed, "kubectl") {
		return trimmed, false, nil
	}

	// Check if the first word is a kubectl verb
	firstWord := strings.Fields(trimmed)[0]
	if m.isKubectlVerb(firstWord) {
		return "kubectl " + trimmed, false, nil
	}

	// Not a kubectl verb - run as shell command
	return "!" + trimmed, true, nil
}

// isKubectlVerb checks if the given word is a valid kubectl command/verb
func (m Model) isKubectlVerb(word string) bool {
	if m.completer == nil || m.completer.Registry == nil {
		// Fallback to common verbs if completer not available
		commonVerbs := map[string]bool{
			"get": true, "describe": true, "create": true, "delete": true,
			"apply": true, "edit": true, "logs": true, "exec": true,
			"port-forward": true, "proxy": true, "cp": true, "attach": true,
			"run": true, "expose": true, "set": true, "explain": true,
			"scale": true, "autoscale": true, "rollout": true, "label": true,
			"annotate": true, "config": true, "cluster-info": true, "top": true,
			"cordon": true, "uncordon": true, "drain": true, "taint": true,
			"certificate": true, "auth": true, "diff": true, "patch": true,
			"replace": true, "wait": true, "kustomize": true, "api-resources": true,
			"api-versions": true, "version": true, "plugin": true, "debug": true,
		}
		return commonVerbs[word]
	}

	// Check against the registry's known commands
	topLevelCmds := m.completer.Registry.TopLevelCommands()
	for _, cmd := range topLevelCmds {
		if cmd == word {
			return true
		}
	}
	return false
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

// showFilePicker opens the file picker dialog
func (m Model) showFilePicker() (tea.Model, tea.Cmd) {
	m.mode = types.ModeSelectingFile
	return m, m.filePicker.Init()
}

// handleSelectingFileMode handles key presses in file selection mode
func (m Model) handleSelectingFileMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = types.ModeTyping
		m.commandInput.Focus()
		return m, nil
	}

	// Let the filepicker handle its own keys
	var cmd tea.Cmd
	m.filePicker, cmd = m.filePicker.Update(msg)

	// Check if a file was selected
	if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
		// Insert the file path into the command
		currentCmd := m.commandInput.Value()
		m.commandInput.SetValue(currentCmd + path)
		m.commandInput.CursorEnd()

		// Return to typing mode
		m.mode = types.ModeTyping
		m.commandInput.Focus()
		return m, nil
	}

	// Check if user tried to select a disabled file
	if didSelect, _ := m.filePicker.DidSelectDisabledFile(msg); didSelect {
		m.statusMsg = "Cannot select this file type"
	}

	return m, cmd
}

// KeyMap defines the keybindings
type KeyMap struct {
	Quit    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Tab     key.Binding
	History key.Binding
	Clear   key.Binding
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
