package types

import (
	"testing"
)

func TestListItem_FilterValue(t *testing.T) {
	item := ListItem{
		Title:       "my-pod",
		Description: "Running",
		Metadata:    map[string]string{"status": "Running"},
	}

	if item.FilterValue() != "my-pod" {
		t.Errorf("FilterValue() = %s, want %s", item.FilterValue(), "my-pod")
	}
}

func TestParsedCommand_Structure(t *testing.T) {
	cmd := ParsedCommand{
		Raw:          "kubectl get pods",
		Verb:         "get",
		Resource:     "pods",
		ResourceName: "",
		Namespace:    "default",
		Flags:        make(map[string]string),
		BoolFlags:    make(map[string]bool),
		Files:        []string{},
		IsComplete:   true,
		NeedsInput:   []CompletionNeeded{},
		IsValid:      true,
		Errors:       []string{},
	}

	if cmd.Verb != "get" {
		t.Errorf("Verb = %s, want get", cmd.Verb)
	}
	if cmd.Resource != "pods" {
		t.Errorf("Resource = %s, want pods", cmd.Resource)
	}
	if !cmd.IsComplete {
		t.Error("Expected command to be complete")
	}
	if !cmd.IsValid {
		t.Error("Expected command to be valid")
	}
}

func TestCompletionNeeded_Structure(t *testing.T) {
	completion := CompletionNeeded{
		Type:     CompletionNamespace,
		Flag:     "namespace",
		Required: true,
	}

	if completion.Type != CompletionNamespace {
		t.Errorf("Type = %v, want CompletionNamespace", completion.Type)
	}
	if !completion.Required {
		t.Error("Expected completion to be required")
	}
}

func TestHistoryEntry_Structure(t *testing.T) {
	entry := HistoryEntry{
		Command:   "kubectl get pods",
		Success:   true,
		Context:   "production",
		Namespace: "default",
	}

	if entry.Command != "kubectl get pods" {
		t.Errorf("Command = %s, want 'kubectl get pods'", entry.Command)
	}
	if !entry.Success {
		t.Error("Expected success to be true")
	}
}

func TestMode_Values(t *testing.T) {
	modes := []Mode{
		ModeTyping,
		ModeSelectingResource,
		ModeSelectingFile,
		ModeViewingHistory,
		ModeViewingOutput,
		ModeConfirming,
		ModeError,
	}

	// Check that modes are unique
	seen := make(map[Mode]bool)
	for _, mode := range modes {
		if seen[mode] {
			t.Errorf("Duplicate mode value: %v", mode)
		}
		seen[mode] = true
	}

	if len(seen) != 7 {
		t.Errorf("Expected 7 unique modes, got %d", len(seen))
	}
}

func TestCompletionType_Values(t *testing.T) {
	types := []CompletionType{
		CompletionNamespace,
		CompletionResourceName,
		CompletionContainer,
		CompletionFile,
		CompletionOutputFormat,
		CompletionContext,
		CompletionNode,
	}

	// Check that types are unique
	seen := make(map[CompletionType]bool)
	for _, ct := range types {
		if seen[ct] {
			t.Errorf("Duplicate completion type value: %v", ct)
		}
		seen[ct] = true
	}

	if len(seen) != 7 {
		t.Errorf("Expected 7 unique completion types, got %d", len(seen))
	}
}

func TestListItem_WithMetadata(t *testing.T) {
	metadata := map[string]string{
		"namespace": "default",
		"status":    "Running",
		"age":       "2d",
	}

	item := ListItem{
		Title:       "my-pod",
		Description: "A running pod",
		Metadata:    metadata,
	}

	if item.Metadata["namespace"] != "default" {
		t.Errorf("Namespace = %s, want default", item.Metadata["namespace"])
	}
	if item.Metadata["status"] != "Running" {
		t.Errorf("Status = %s, want Running", item.Metadata["status"])
	}
	if len(item.Metadata) != 3 {
		t.Errorf("Expected 3 metadata entries, got %d", len(item.Metadata))
	}
}

func TestParsedCommand_WithFlags(t *testing.T) {
	cmd := ParsedCommand{
		Raw:      "kubectl get pods -n default -o json",
		Verb:     "get",
		Resource: "pods",
		Flags: map[string]string{
			"namespace": "default",
			"output":    "json",
		},
		BoolFlags: map[string]bool{
			"watch": false,
		},
	}

	if cmd.Flags["namespace"] != "default" {
		t.Errorf("Namespace flag = %s, want default", cmd.Flags["namespace"])
	}
	if cmd.Flags["output"] != "json" {
		t.Errorf("Output flag = %s, want json", cmd.Flags["output"])
	}
	if cmd.BoolFlags["watch"] {
		t.Error("Watch flag should be false")
	}
}

func TestParsedCommand_WithErrors(t *testing.T) {
	cmd := ParsedCommand{
		Raw:     "invalid command",
		IsValid: false,
		Errors:  []string{"unknown verb", "missing resource"},
	}

	if cmd.IsValid {
		t.Error("Command should be invalid")
	}
	if len(cmd.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(cmd.Errors))
	}
}

func TestParsedCommand_WithFiles(t *testing.T) {
	cmd := ParsedCommand{
		Raw:   "kubectl apply -f deployment.yaml -f service.yaml",
		Verb:  "apply",
		Files: []string{"deployment.yaml", "service.yaml"},
	}

	if len(cmd.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(cmd.Files))
	}
	if cmd.Files[0] != "deployment.yaml" {
		t.Errorf("First file = %s, want deployment.yaml", cmd.Files[0])
	}
}

func TestCompletionNeeded_Optional(t *testing.T) {
	completion := CompletionNeeded{
		Type:     CompletionOutputFormat,
		Flag:     "output",
		Required: false,
	}

	if completion.Required {
		t.Error("Completion should not be required")
	}
}
