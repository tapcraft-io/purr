package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	// Primary colors
	colorPrimary   = lipgloss.Color("#7D56F4") // Purple
	colorSecondary = lipgloss.Color("#FF6B9D") // Pink
	colorAccent    = lipgloss.Color("#00D9FF") // Cyan

	// Status colors
	colorSuccess = lipgloss.Color("#00D787") // Green
	colorWarning = lipgloss.Color("#FFB86C") // Orange
	colorError   = lipgloss.Color("#FF5555") // Red
	colorInfo    = lipgloss.Color("#8BE9FD") // Cyan

	// UI colors
	colorText    = lipgloss.Color("#F8F8F2") // White
	colorTextDim = lipgloss.Color("#6272A4") // Gray
	colorBorder  = lipgloss.Color("#44475A") // Dark gray
	colorBg      = lipgloss.Color("#282A36") // Background
	colorBgAlt   = lipgloss.Color("#21222C") // Alt background
)

// Style definitions
var (
	// Title bar
	titleStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			Padding(0, 1)

	contextStyle = lipgloss.NewStyle().
			Foreground(colorInfo).
			Padding(0, 1)

	// Command input
	inputStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Padding(0, 1)

	// Prompt
	promptStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Selected item in list
	selectedStyle = lipgloss.NewStyle().
			Foreground(colorBgAlt).
			Background(colorPrimary).
			Bold(true).
			Padding(0, 1)

	// Normal list item
	normalStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Padding(0, 1)

	// Success message
	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	// Error message
	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	// Warning message
	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	// Info message
	infoStyle = lipgloss.NewStyle().
			Foreground(colorInfo)

	// Help text
	helpStyle = lipgloss.NewStyle().
			Foreground(colorTextDim)

	// Border style
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	// Box style for pickers/dialogs
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2).
			Width(60)

	// Output viewport style
	viewportStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	// Description style (for list items)
	descriptionStyle = lipgloss.NewStyle().
				Foreground(colorTextDim)

	// Highlighted text
	highlightStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	// Dimmed text (for autocomplete suggestions)
	dimStyle = lipgloss.NewStyle().
			Foreground(colorTextDim)

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorPrimary)

	// Status indicator styles
	statusReadyStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	statusPendingStyle = lipgloss.NewStyle().
				Foreground(colorWarning).
				Bold(true)

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(colorError).
				Bold(true)
)

// Helper functions for styling

// RenderTitle renders the title bar
func RenderTitle(title string, context string) string {
	left := titleStyle.Render("Purr")
	right := contextStyle.Render("[context: " + context + "]")
	return lipgloss.JoinHorizontal(lipgloss.Left, left, right)
}

// RenderPrompt renders the command prompt
func RenderPrompt() string {
	return promptStyle.Render("> ")
}

// RenderSuccess renders a success message
func RenderSuccess(msg string) string {
	return successStyle.Render("✓ " + msg)
}

// RenderError renders an error message
func RenderError(msg string) string {
	return errorStyle.Render("✗ " + msg)
}

// RenderWarning renders a warning message
func RenderWarning(msg string) string {
	return warningStyle.Render("⚠ " + msg)
}

// RenderInfo renders an info message
func RenderInfo(msg string) string {
	return infoStyle.Render("ℹ " + msg)
}

// RenderHelp renders help text
func RenderHelp(text string) string {
	return helpStyle.Render(text)
}

// RenderBox renders content in a bordered box
func RenderBox(title, content string) string {
	titleRendered := titleStyle.Render(title)
	return boxStyle.Render(titleRendered + "\n\n" + content)
}

// RenderListItem renders a list item
func RenderListItem(title, description string, selected bool) string {
	if selected {
		titleRendered := selectedStyle.Render("❯ " + title)
		descRendered := descriptionStyle.Render("  " + description)
		return titleRendered + "\n" + descRendered
	}

	titleRendered := normalStyle.Render("  " + title)
	descRendered := descriptionStyle.Render("  " + description)
	return titleRendered + "\n" + descRendered
}

// RenderStatus renders a status indicator
func RenderStatus(status string) string {
	switch status {
	case "Running", "Ready", "Active", "Succeeded":
		return statusReadyStyle.Render("●")
	case "Pending", "Creating", "Updating":
		return statusPendingStyle.Render("●")
	case "Failed", "Error", "CrashLoopBackOff", "Unknown":
		return statusFailedStyle.Render("●")
	default:
		return helpStyle.Render("●")
	}
}

// GetMaxWidth returns the maximum width for a given screen width
func GetMaxWidth(screenWidth int) int {
	maxWidth := screenWidth - 4 // Account for padding
	if maxWidth < 40 {
		maxWidth = 40
	}
	return maxWidth
}

// GetMaxHeight returns the maximum height for a given screen height
func GetMaxHeight(screenHeight int) int {
	maxHeight := screenHeight - 6 // Account for title, prompt, help
	if maxHeight < 10 {
		maxHeight = 10
	}
	return maxHeight
}
