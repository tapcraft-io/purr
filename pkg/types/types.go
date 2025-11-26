package types

import "time"

// Mode represents the current interaction mode
type Mode int

const (
	ModeTyping Mode = iota
	ModeSelectingResource
	ModeSelectingFile
	ModeViewingHistory
	ModeViewingOutput
	ModeConfirming
	ModeError
)

// CompletionType represents what kind of completion is needed
type CompletionType int

const (
	CompletionNamespace CompletionType = iota
	CompletionResourceName
	CompletionContainer
	CompletionFile
	CompletionOutputFormat
	CompletionContext
	CompletionNode
)

// CompletionNeeded represents a missing field that needs user input
type CompletionNeeded struct {
	Type     CompletionType
	Flag     string
	Required bool
}

// ParsedCommand represents a parsed kubectl command
type ParsedCommand struct {
	Raw          string
	Verb         string
	Resource     string
	ResourceName string
	Namespace    string
	Flags        map[string]string
	BoolFlags    map[string]bool
	Files        []string
	IsComplete   bool
	NeedsInput   []CompletionNeeded
	IsValid      bool
	Errors       []string
}

// HistoryEntry represents a command in the history
type HistoryEntry struct {
	Command   string
	Timestamp time.Time
	Success   bool
	Context   string
	Namespace string
}

// ListItem represents an item that can be selected from a list
type ListItem struct {
	Title       string
	Description string
	Metadata    map[string]string
}

func (i ListItem) FilterValue() string {
	return i.Title
}
