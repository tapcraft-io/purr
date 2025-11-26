package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistory_AddAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Add some commands
	h.Add("kubectl get pods", true, "prod", "default")
	h.Add("kubectl get services", true, "prod", "default")
	h.Add("kubectl describe pod my-pod", false, "prod", "default")

	// Get all commands
	entries := h.Get(10)
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Check order (most recent first)
	if entries[0].Command != "kubectl describe pod my-pod" {
		t.Errorf("Expected most recent command first, got %s", entries[0].Command)
	}

	// Check success/failure
	if entries[0].Success {
		t.Error("Expected last command to be marked as failed")
	}
	if !entries[1].Success {
		t.Error("Expected second command to be marked as successful")
	}
}

func TestHistory_MaxSize(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	maxSize := 5
	h, err := NewHistory(maxSize, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Add more commands than max size
	for i := 0; i < 10; i++ {
		h.Add("kubectl get pods", true, "prod", "default")
	}

	entries := h.GetAll()
	if len(entries) != maxSize {
		t.Errorf("Expected max size %d, got %d", maxSize, len(entries))
	}
}

func TestHistory_Search(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Add various commands
	h.Add("kubectl get pods", true, "prod", "default")
	h.Add("kubectl get services", true, "prod", "default")
	h.Add("kubectl describe pod my-pod", true, "prod", "default")
	h.Add("kubectl logs my-pod", true, "prod", "default")

	tests := []struct {
		query    string
		expected int
	}{
		{"pods", 1}, // Should match "get pods" (fuzzy is strict)
		{"pod", 3},  // Should match "get pods", "describe pod", and "logs my-pod"
		{"services", 1},
		{"logs", 1},
		{"kubectl", 4}, // All commands
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := h.Search(tt.query)
			if len(results) != tt.expected {
				t.Errorf("Search(%s) returned %d results, expected %d",
					tt.query, len(results), tt.expected)
			}
		})
	}
}

func TestHistory_Filter(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Add commands with different contexts and namespaces
	h.Add("kubectl get pods", true, "prod", "default")
	h.Add("kubectl get services", true, "staging", "default")
	h.Add("kubectl describe pod my-pod", false, "prod", "kube-system")
	h.Add("kubectl logs my-pod", true, "prod", "default")

	// Filter by context
	prodEntries := h.Filter("prod", "", false)
	if len(prodEntries) != 3 {
		t.Errorf("Expected 3 prod entries, got %d", len(prodEntries))
	}

	// Filter by namespace
	defaultEntries := h.Filter("", "default", false)
	if len(defaultEntries) != 3 {
		t.Errorf("Expected 3 default namespace entries, got %d", len(defaultEntries))
	}

	// Filter by success
	successEntries := h.Filter("", "", true)
	if len(successEntries) != 3 {
		t.Errorf("Expected 3 successful entries, got %d", len(successEntries))
	}

	// Filter by context and namespace
	entries := h.Filter("prod", "default", false)
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}

func TestHistory_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	// Create history and add commands
	h1, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	h1.Add("kubectl get pods", true, "prod", "default")
	h1.Add("kubectl get services", true, "prod", "default")

	// Save
	if err := h1.Save(); err != nil {
		t.Fatalf("Failed to save history: %v", err)
	}

	// Load into new history
	h2, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create second history: %v", err)
	}

	entries := h2.GetAll()
	if len(entries) != 2 {
		t.Errorf("Expected 2 loaded entries, got %d", len(entries))
	}

	// Verify commands were preserved
	if entries[0].Command != "kubectl get services" {
		t.Errorf("Expected first command to be 'kubectl get services', got %s", entries[0].Command)
	}
}

func TestHistory_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Add commands
	h.Add("kubectl get pods", true, "prod", "default")
	h.Add("kubectl get services", true, "prod", "default")
	h.Add("kubectl describe pod", true, "prod", "default")

	// Delete middle entry
	h.Delete(1)

	entries := h.GetAll()
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries after delete, got %d", len(entries))
	}

	// Verify correct entry was deleted
	if entries[1].Command != "kubectl get pods" {
		t.Errorf("Wrong entry deleted")
	}
}

func TestHistory_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Add commands
	h.Add("kubectl get pods", true, "prod", "default")
	h.Add("kubectl get services", true, "prod", "default")

	// Clear
	h.Clear()

	entries := h.GetAll()
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(entries))
	}
}

func TestHistory_ToListItems(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	h.Add("kubectl get pods", true, "prod", "default")
	h.Add("kubectl get services", false, "staging", "kube-system")

	entries := h.GetAll()
	items := h.ToListItems(entries)

	if len(items) != 2 {
		t.Errorf("Expected 2 list items, got %d", len(items))
	}

	// Check metadata
	if items[0].Metadata["context"] != "staging" {
		t.Errorf("Expected context 'staging', got %s", items[0].Metadata["context"])
	}

	if items[0].Metadata["success"] != "false" {
		t.Errorf("Expected success 'false', got %s", items[0].Metadata["success"])
	}

	// Check that failure is shown in description
	if items[0].Description == "" {
		t.Error("Expected description for failed command")
	}
}

func TestHistory_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(1000, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				h.Add("kubectl get pods", true, "prod", "default")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	entries := h.GetAll()
	if len(entries) != 1000 {
		t.Errorf("Expected 1000 entries (max size), got %d", len(entries))
	}
}

func TestHistory_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "nonexistent.json")

	// Should not error when file doesn't exist
	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Should not error on non-existent file: %v", err)
	}

	entries := h.GetAll()
	if len(entries) != 0 {
		t.Errorf("Expected empty history for non-existent file, got %d entries", len(entries))
	}
}

func TestHistory_Timestamps(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	before := time.Now()
	h.Add("kubectl get pods", true, "prod", "default")
	after := time.Now()

	entries := h.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	timestamp := entries[0].Timestamp
	if timestamp.Before(before) || timestamp.After(after) {
		t.Errorf("Timestamp %v not between %v and %v", timestamp, before, after)
	}
}

func TestHistory_EmptySearch(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "history.json")

	h, err := NewHistory(100, histFile)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}

	h.Add("kubectl get pods", true, "prod", "default")
	h.Add("kubectl get services", true, "prod", "default")

	// Empty query should return all
	results := h.Search("")
	if len(results) != 2 {
		t.Errorf("Empty search should return all entries, got %d", len(results))
	}
}

func TestHistory_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	if err := os.WriteFile(histFile, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	// Should handle gracefully
	h, err := NewHistory(100, histFile)
	if err == nil {
		t.Error("Expected error when loading invalid JSON")
	}

	// Should still be usable
	if h != nil {
		h.Add("kubectl get pods", true, "prod", "default")
		entries := h.GetAll()
		if len(entries) != 1 {
			t.Errorf("Should be able to use history after load error, got %d entries", len(entries))
		}
	}
}
