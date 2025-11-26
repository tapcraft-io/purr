package exec

import (
	"testing"

	"github.com/tapcraft-io/purr/pkg/types"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name          string
		input         string
		expectedVerb  string
		expectedRes   string
		expectedName  string
		expectedNS    string
		expectedValid bool
	}{
		{
			name:          "Simple get pods",
			input:         "get pods",
			expectedVerb:  "get",
			expectedRes:   "pods",
			expectedValid: true,
		},
		{
			name:          "Get pods with namespace",
			input:         "get pods -n default",
			expectedVerb:  "get",
			expectedRes:   "pods",
			expectedNS:    "default",
			expectedValid: true,
		},
		{
			name:          "Get specific pod",
			input:         "get pods my-pod",
			expectedVerb:  "get",
			expectedRes:   "pods",
			expectedName:  "my-pod",
			expectedValid: true,
		},
		{
			name:          "Describe with long namespace flag",
			input:         "describe pod my-pod --namespace production",
			expectedVerb:  "describe",
			expectedRes:   "pod",
			expectedName:  "my-pod",
			expectedNS:    "production",
			expectedValid: true,
		},
		{
			name:          "Delete deployment",
			input:         "delete deployment my-deploy -n staging",
			expectedVerb:  "delete",
			expectedRes:   "deployment",
			expectedName:  "my-deploy",
			expectedNS:    "staging",
			expectedValid: true,
		},
		{
			name:          "Logs command",
			input:         "logs my-pod",
			expectedVerb:  "logs",
			expectedRes:   "my-pod",
			expectedName:  "",
			expectedValid: true,
		},
		{
			name:          "Empty command",
			input:         "",
			expectedValid: false,
		},
		{
			name:          "Apply with file flag",
			input:         "apply --filename deployment.yaml",
			expectedVerb:  "apply",
			expectedRes:   "",
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.input)

			if result.IsValid != tt.expectedValid {
				t.Errorf("Expected IsValid=%v, got %v", tt.expectedValid, result.IsValid)
			}

			if tt.expectedValid {
				if result.Verb != tt.expectedVerb {
					t.Errorf("Expected verb=%s, got %s", tt.expectedVerb, result.Verb)
				}
				if result.Resource != tt.expectedRes {
					t.Errorf("Expected resource=%s, got %s", tt.expectedRes, result.Resource)
				}
				if result.ResourceName != tt.expectedName {
					t.Errorf("Expected resource name=%s, got %s", tt.expectedName, result.ResourceName)
				}
				if result.Namespace != tt.expectedNS {
					t.Errorf("Expected namespace=%s, got %s", tt.expectedNS, result.Namespace)
				}
			}
		})
	}
}

func TestParser_NormalizeResourceType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"po", "pods"},
		{"pods", "pods"},
		{"svc", "services"},
		{"deploy", "deployments"},
		{"deployment", "deployment"},
		{"cm", "configmaps"},
		{"secret", "secrets"},
		{"ing", "ingresses"},
		{"ns", "namespaces"},
		{"no", "nodes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeResourceType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeResourceType(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParser_FlagParsing(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name         string
		input        string
		expectedFlag string
		expectedVal  string
	}{
		{
			name:         "Short namespace flag",
			input:        "get pods -n production",
			expectedFlag: "namespace",
			expectedVal:  "production",
		},
		{
			name:         "Long namespace flag",
			input:        "get pods --namespace production",
			expectedFlag: "namespace",
			expectedVal:  "production",
		},
		{
			name:         "Output flag",
			input:        "get pods -o json",
			expectedFlag: "output",
			expectedVal:  "json",
		},
		{
			name:         "Filename flag",
			input:        "apply --filename deployment.yaml",
			expectedFlag: "filename",
			expectedVal:  "deployment.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.input)
			if val, ok := result.Flags[tt.expectedFlag]; !ok || val != tt.expectedVal {
				t.Errorf("Expected flag %s=%s, got %s", tt.expectedFlag, tt.expectedVal, val)
			}
		})
	}
}

func TestParser_BooleanFlags(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name         string
		input        string
		expectedFlag string
	}{
		{
			name:         "All namespaces flag",
			input:        "get pods --all-namespaces",
			expectedFlag: "all-namespaces",
		},
		{
			name:         "Watch flag",
			input:        "get pods -w",
			expectedFlag: "watch",
		},
		{
			name:         "Force flag",
			input:        "delete pod my-pod --force",
			expectedFlag: "force",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.input)
			if !result.BoolFlags[tt.expectedFlag] {
				t.Errorf("Expected boolean flag %s to be true", tt.expectedFlag)
			}
		})
	}
}

func TestIsDestructive(t *testing.T) {
	tests := []struct {
		command  string
		expected bool
	}{
		{"get pods", false},
		{"describe pod my-pod", false},
		{"delete pod my-pod", true},
		{"delete deployment my-deploy", true},
		{"drain node my-node", true},
		{"apply -f deployment.yaml --force", true},
		{"logs my-pod", false},
		{"exec my-pod -- ls", false},
		{"rollout restart deployment my-deploy", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := IsDestructive(tt.command)
			if result != tt.expected {
				t.Errorf("IsDestructive(%s) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestGetCommandVerb(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"kubectl get pods", "get"},
		{"get pods", "get"},
		{"describe pod my-pod", "describe"},
		{"delete deployment my-deploy", "delete"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := GetCommandVerb(tt.command)
			if result != tt.expected {
				t.Errorf("GetCommandVerb(%s) = %s, want %s", tt.command, result, tt.expected)
			}
		})
	}
}

func TestParser_CompletionNeeds(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name                 string
		input                string
		expectsCompletion    bool
		expectedCompletionType types.CompletionType
	}{
		{
			name:              "Namespace flag without value",
			input:             "get pods -n ",
			expectsCompletion: true,
			expectedCompletionType: types.CompletionNamespace,
		},
		{
			name:              "Filename flag without value",
			input:             "apply --filename ",
			expectsCompletion: true,
			expectedCompletionType: types.CompletionFile,
		},
		{
			name:              "Complete command",
			input:             "get pods -n default",
			expectsCompletion: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.input)
			hasCompletion := len(result.NeedsInput) > 0

			if hasCompletion != tt.expectsCompletion {
				t.Errorf("Expected NeedsInput length > 0: %v, got %v", tt.expectsCompletion, hasCompletion)
			}

			if tt.expectsCompletion && len(result.NeedsInput) > 0 {
				if result.NeedsInput[0].Type != tt.expectedCompletionType {
					t.Errorf("Expected completion type %v, got %v",
						tt.expectedCompletionType, result.NeedsInput[0].Type)
				}
			}
		})
	}
}
