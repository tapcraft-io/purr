package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tapcraft-io/purr/pkg/types"
)

// View renders the entire UI
func (m Model) View() string {
	if m.quitting {
		return "Thanks for using Purr! 🐱\n"
	}

	// Show error if present
	if m.err != nil {
		return m.renderError()
	}

	// Show loading state if cache not ready
	if !m.ready {
		return m.renderLoading()
	}

	// Render based on current mode
	switch m.mode {
	case types.ModeTyping:
		return m.renderTypingMode()
	case types.ModeSelectingResource:
		return m.renderSelectingResourceMode()
	case types.ModeSelectingFile:
		return m.renderSelectingFileMode()
	case types.ModeViewingHistory:
		return m.renderViewingHistoryMode()
	case types.ModeViewingOutput:
		return m.renderViewingOutputMode()
	case types.ModeConfirming:
		return m.renderConfirmingMode()
	case types.ModeError:
		return m.renderError()
	default:
		return m.renderTypingMode()
	}
}

// renderLoading renders the loading screen
func (m Model) renderLoading() string {
	var b strings.Builder

	// Title
	title := RenderTitle("Purr", m.context)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Spinner
	b.WriteString(m.spinner.View())
	b.WriteString(" Initializing cache...\n\n")

	b.WriteString(RenderHelp("Please wait while we fetch resources from your cluster."))

	return b.String()
}

// renderError renders an error screen
func (m Model) renderError() string {
	var b strings.Builder

	title := RenderTitle("Purr", m.context)
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(RenderError("Error: " + m.err.Error()))
	} else if m.cmdError != nil {
		b.WriteString(RenderError("Command failed: " + m.cmdError.Error()))
		b.WriteString("\n\n")
		b.WriteString(m.cmdOutput)
	}

	b.WriteString("\n\n")
	b.WriteString(RenderHelp("[Enter] to continue  [Ctrl+C] quit"))

	return b.String()
}

// renderTypingMode renders the main typing mode
func (m Model) renderTypingMode() string {
	var b strings.Builder

	// Title bar
	title := RenderTitle("Purr", m.context)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Command input with custom ghost text
	b.WriteString(RenderPrompt())

	// Render the input field
	inputView := m.commandInput.View()
	b.WriteString(inputView)

	// The textinput already shows ghost text for the current suggestion
	// so we don't need to add extra ghost text here

	b.WriteString("\n")

	// Show suggestion list below input with scrolling window
	if len(m.suggestions) > 0 {
		suggestionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))          // lighter gray
		selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true) // pink/magenta for selected
		maxVisible := 10

		// Calculate the visible window to keep selected item in view
		startIdx := 0
		endIdx := len(m.suggestions)

		if len(m.suggestions) > maxVisible {
			// Calculate window that keeps selected item visible
			// Try to keep selected item in the middle when possible
			halfWindow := maxVisible / 2

			if m.suggestionIndex <= halfWindow {
				// Near the start - show from beginning
				startIdx = 0
				endIdx = maxVisible
			} else if m.suggestionIndex >= len(m.suggestions)-halfWindow {
				// Near the end - show last items
				startIdx = len(m.suggestions) - maxVisible
				endIdx = len(m.suggestions)
			} else {
				// In the middle - center around selected
				startIdx = m.suggestionIndex - halfWindow
				endIdx = startIdx + maxVisible
			}
		}

		// Show scroll indicator at top if not showing from start
		if startIdx > 0 {
			b.WriteString(suggestionStyle.Render(fmt.Sprintf("  ↑ %d more above", startIdx)))
			b.WriteString("\n")
		}

		for i := startIdx; i < endIdx; i++ {
			sug := m.suggestions[i]
			if i == m.suggestionIndex {
				b.WriteString(selectedStyle.Render("→ " + sug))
			} else {
				b.WriteString(suggestionStyle.Render("  " + sug))
			}
			b.WriteString("\n")
		}

		// Show scroll indicator at bottom if more items below
		if endIdx < len(m.suggestions) {
			b.WriteString(suggestionStyle.Render(fmt.Sprintf("  ↓ %d more below", len(m.suggestions)-endIdx)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Show status message if present
	if m.statusMsg != "" {
		b.WriteString(RenderInfo(m.statusMsg))
		b.WriteString("\n\n")
	}

	// Show panes if there are any, otherwise show last output
	if len(m.panes) > 0 {
		b.WriteString(m.renderPanes())
	} else if m.cmdOutput != "" {
		// Show last output in viewport if available (limited height to leave room for input)
		// Calculate available height for output
		// Reserve space for: title(2) + prompt(1) + suggestions(~12) + help(2) + padding(3) = ~20 lines
		maxOutputHeight := m.height - 20
		if maxOutputHeight < 5 {
			maxOutputHeight = 5
		}
		if maxOutputHeight > 20 {
			maxOutputHeight = 20 // Cap output height
		}

		// Create a limited viewport view
		lines := strings.Split(m.cmdOutput, "\n")
		displayLines := lines
		hasMore := false
		if len(lines) > maxOutputHeight {
			displayLines = lines[:maxOutputHeight]
			hasMore = true
		}

		outputStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Width(m.width - 4)

		output := strings.Join(displayLines, "\n")
		if hasMore {
			moreStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
			output += "\n" + moreStyle.Render(fmt.Sprintf("... %d more lines (Ctrl+O to view full output)", len(lines)-maxOutputHeight))
		}

		b.WriteString(outputStyle.Render(output))
		b.WriteString("\n\n")
	}

	// Help bar
	help := m.renderHelpBar()
	b.WriteString(help)

	return b.String()
}

// renderSelectingResourceMode renders the resource selection mode
func (m Model) renderSelectingResourceMode() string {
	var b strings.Builder

	// Title bar
	title := RenderTitle("Purr", m.context)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Resource list
	b.WriteString(m.resourceList.View())
	b.WriteString("\n\n")

	// Help
	b.WriteString(RenderHelp("[↑↓] navigate  [Enter] select  [Esc] cancel  [/] search"))

	return b.String()
}

// renderSelectingFileMode renders the file selection mode
func (m Model) renderSelectingFileMode() string {
	var b strings.Builder

	// Title bar
	title := RenderTitle("Purr", m.context)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Current directory
	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	b.WriteString(dirStyle.Render("📁 " + m.filePicker.CurrentDirectory))
	b.WriteString("\n\n")

	// File picker
	b.WriteString(m.filePicker.View())
	b.WriteString("\n\n")

	// Help
	b.WriteString(RenderHelp("[↑↓/jk] navigate  [Enter/l] open/select  [←/h/Backspace] back  [Esc] cancel"))

	return b.String()
}

// renderViewingHistoryMode renders the history viewing mode
func (m Model) renderViewingHistoryMode() string {
	var b strings.Builder

	// Title bar
	title := RenderTitle("Purr", m.context)
	b.WriteString(title)
	b.WriteString("\n\n")

	// History list
	b.WriteString(m.historyList.View())
	b.WriteString("\n\n")

	// Help
	b.WriteString(RenderHelp("[↑↓] navigate  [Enter] execute  [e] edit  [Esc] cancel  [/] search"))

	return b.String()
}

// renderViewingOutputMode renders the output viewing mode
func (m Model) renderViewingOutputMode() string {
	var b strings.Builder

	// Title bar
	title := RenderTitle("Purr", m.context)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Show last command
	b.WriteString(promptStyle.Render("$ "))
	b.WriteString(m.lastCmd)
	b.WriteString("\n\n")

	// Show output in viewport
	viewportContent := m.viewport.View()
	b.WriteString(viewportStyle.Render(viewportContent))
	b.WriteString("\n\n")

	// Show success or error indicator
	if m.cmdError != nil {
		b.WriteString(RenderError("Command failed"))
		b.WriteString("\n")
	} else {
		b.WriteString(RenderSuccess("Command succeeded"))
		b.WriteString("\n")
	}

	// Help
	b.WriteString(RenderHelp("[n] new command  [r] re-run  [e] edit  [↑↓] scroll  [Ctrl+C] quit"))

	return b.String()
}

// renderConfirmingMode renders the confirmation dialog
func (m Model) renderConfirmingMode() string {
	var b strings.Builder

	// Title bar
	title := RenderTitle("Purr", m.context)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Warning
	b.WriteString(RenderWarning("⚠ Destructive Operation"))
	b.WriteString("\n\n")

	// Show command
	b.WriteString("Command: ")
	b.WriteString(highlightStyle.Render(m.lastCmd))
	b.WriteString("\n\n")

	// Confirmation prompt
	b.WriteString("This command may delete or modify resources.\n")
	b.WriteString("Are you sure you want to continue?\n\n")

	b.WriteString(RenderHelp("[y] yes  [n] no"))

	return b.String()
}

// renderPanes renders all command panes in a tiled layout
func (m Model) renderPanes() string {
	if len(m.panes) == 0 {
		return ""
	}

	var b strings.Builder

	// Calculate dimensions for panes
	// For now, we'll use a simple horizontal tiling
	availableWidth := m.width - 4
	availableHeight := m.height - 25 // Reserve space for input, suggestions, help

	// Each pane gets equal width
	paneWidth := availableWidth / len(m.panes)
	if paneWidth < 20 {
		paneWidth = 20 // Minimum width
	}

	// Render panes side by side
	var paneViews []string
	for i, pane := range m.panes {
		isActive := i == m.activePaneIndex

		// Create border style based on active state
		var borderStyle lipgloss.Style
		if isActive {
			borderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("212")). // Pink for active
				Padding(0, 1).
				Width(paneWidth - 2).
				Height(availableHeight)
		} else {
			borderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")). // Gray for inactive
				Padding(0, 1).
				Width(paneWidth - 2).
				Height(availableHeight)
		}

		// Create header with command and status
		statusSymbol := "●"
		statusColor := "yellow"
		switch pane.Status {
		case types.PaneStatusRunning:
			statusSymbol = "●"
			statusColor = "green"
		case types.PaneStatusCompleted:
			statusSymbol = "✓"
			statusColor = "blue"
		case types.PaneStatusError:
			statusSymbol = "✗"
			statusColor = "red"
		}

		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Bold(true)
		cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

		header := fmt.Sprintf("%s %s",
			statusStyle.Render(statusSymbol),
			cmdStyle.Render(truncate(pane.Command, paneWidth-6)),
		)

		// Get output content
		content := pane.Output.String()
		if content == "" {
			content = "Waiting for output..."
		}

		// Limit output to available height
		lines := strings.Split(content, "\n")
		maxLines := availableHeight - 3 // Reserve space for header
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:] // Show most recent lines
		}
		displayContent := strings.Join(lines, "\n")

		// Combine header and content
		paneContent := header + "\n" + strings.Repeat("─", paneWidth-4) + "\n" + displayContent

		paneView := borderStyle.Render(paneContent)
		paneViews = append(paneViews, paneView)
	}

	// Join panes horizontally
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, paneViews...))
	b.WriteString("\n\n")

	return b.String()
}

// renderHelpBar renders the help bar at the bottom
func (m Model) renderHelpBar() string {
	items := []string{
		"[Tab] accept",
		"[↑↓] cycle",
		"[@] file",
		"[Ctrl+R] history",
	}

	// Add pane-specific help if there are panes
	if len(m.panes) > 0 {
		items = append(items, "[Ctrl+]] next pane", "[Ctrl+[] prev pane", "[Ctrl+W] close pane")
	}

	// Add output-specific help if there's output
	if m.cmdOutput != "" {
		items = append(items, "[Ctrl+O] full output", "[Ctrl+L] clear")
	}

	items = append(items, "[Ctrl+C] quit")

	return RenderHelp(strings.Join(items, "  "))
}

// Width returns the terminal width
func (m Model) Width() int {
	return m.width
}

// Height returns the terminal height
func (m Model) Height() int {
	return m.height
}

// IsReady returns true if the model is ready
func (m Model) IsReady() bool {
	return m.ready
}

// centerHorizontal centers text horizontally
func centerHorizontal(width int, text string) string {
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return text
	}

	padding := (width - textWidth) / 2
	return strings.Repeat(" ", padding) + text
}

// wrapText wraps text to fit within a given width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		// If adding this word would exceed width, start a new line
		if currentLine.Len()+len(word)+1 > width {
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}
		}

		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}

	// Add the last line
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

// truncate truncates a string to a maximum length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	if max <= 3 {
		return s[:max]
	}

	return s[:max-3] + "..."
}

// padRight pads a string to the right
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// formatDuration formats a duration string to be more human-readable
func formatDuration(duration string) string {
	// Simple formatting - could be enhanced
	return duration
}

// formatStatus formats a status string with color
func formatStatus(status string) string {
	indicator := RenderStatus(status)
	return fmt.Sprintf("%s %s", indicator, status)
}
