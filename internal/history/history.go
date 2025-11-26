package history

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/sahilm/fuzzy"
	"github.com/tapcraft-io/purr/pkg/types"
)

// History manages command history
type History struct {
	commands []types.HistoryEntry
	maxSize  int
	filepath string
	mu       sync.RWMutex
}

// NewHistory creates a new history manager
func NewHistory(maxSize int, filepath string) (*History, error) {
	h := &History{
		commands: make([]types.HistoryEntry, 0, maxSize),
		maxSize:  maxSize,
		filepath: filepath,
	}

	// Try to load existing history
	if err := h.Load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return h, nil
}

// Add adds a command to history
func (h *History) Add(cmd string, success bool, ctx, ns string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry := types.HistoryEntry{
		Command:   cmd,
		Timestamp: time.Now(),
		Success:   success,
		Context:   ctx,
		Namespace: ns,
	}

	// Add to beginning
	h.commands = append([]types.HistoryEntry{entry}, h.commands...)

	// Trim to max size
	if len(h.commands) > h.maxSize {
		h.commands = h.commands[:h.maxSize]
	}
}

// Get returns the most recent n commands
func (h *History) Get(n int) []types.HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n > len(h.commands) {
		n = len(h.commands)
	}

	result := make([]types.HistoryEntry, n)
	copy(result, h.commands[:n])
	return result
}

// GetAll returns all commands
func (h *History) GetAll() []types.HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]types.HistoryEntry, len(h.commands))
	copy(result, h.commands)
	return result
}

// Search searches history with fuzzy matching
func (h *History) Search(query string) []types.HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if query == "" {
		return h.GetAll()
	}

	// Build list of commands for fuzzy search
	commands := make([]string, len(h.commands))
	for i, entry := range h.commands {
		commands[i] = entry.Command
	}

	// Fuzzy search
	matches := fuzzy.Find(query, commands)

	// Build result from matches
	result := make([]types.HistoryEntry, 0, len(matches))
	for _, match := range matches {
		if match.Index < len(h.commands) {
			result = append(result, h.commands[match.Index])
		}
	}

	return result
}

// Filter filters history by context, namespace, and success
func (h *History) Filter(ctx, ns string, successOnly bool) []types.HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]types.HistoryEntry, 0)
	for _, entry := range h.commands {
		if ctx != "" && entry.Context != ctx {
			continue
		}
		if ns != "" && entry.Namespace != ns {
			continue
		}
		if successOnly && !entry.Success {
			continue
		}
		result = append(result, entry)
	}

	return result
}

// Delete removes a command from history by index
func (h *History) Delete(index int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if index < 0 || index >= len(h.commands) {
		return
	}

	h.commands = append(h.commands[:index], h.commands[index+1:]...)
}

// Save persists history to disk
func (h *History) Save() error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.MarshalIndent(h.commands, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.filepath, data, 0644)
}

// Load loads history from disk
func (h *History) Load() error {
	data, err := os.ReadFile(h.filepath)
	if err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	return json.Unmarshal(data, &h.commands)
}

// Clear removes all commands from history
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.commands = make([]types.HistoryEntry, 0, h.maxSize)
}

// ToListItems converts history entries to list items for display
func (h *History) ToListItems(entries []types.HistoryEntry) []types.ListItem {
	items := make([]types.ListItem, len(entries))
	for i, entry := range entries {
		desc := entry.Timestamp.Format("2006-01-02 15:04:05")
		if entry.Context != "" {
			desc += " | " + entry.Context
		}
		if entry.Namespace != "" {
			desc += "/" + entry.Namespace
		}
		if !entry.Success {
			desc += " | ✗ failed"
		}

		successStr := "false"
		if entry.Success {
			successStr = "true"
		}

		items[i] = types.ListItem{
			Title:       entry.Command,
			Description: desc,
			Metadata: map[string]string{
				"timestamp": entry.Timestamp.Format(time.RFC3339),
				"context":   entry.Context,
				"namespace": entry.Namespace,
				"success":   successStr,
			},
		}
	}
	return items
}
