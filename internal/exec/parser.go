package exec

import (
	"strings"

	"github.com/tapcraft-io/purr/pkg/types"
)

// Parser parses kubectl commands
type Parser struct{}

// NewParser creates a new command parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a kubectl command string
func (p *Parser) Parse(command string) *types.ParsedCommand {
	cmd := &types.ParsedCommand{
		Raw:        command,
		Flags:      make(map[string]string),
		BoolFlags:  make(map[string]bool),
		Files:      make([]string, 0),
		IsValid:    true,
		Errors:     make([]string, 0),
		NeedsInput: make([]types.CompletionNeeded, 0),
	}

	command = strings.TrimSpace(command)

	// Shell commands (prefixed with !) are not kubectl commands
	if strings.HasPrefix(command, "!") {
		cmd.IsValid = false
		cmd.Errors = append(cmd.Errors, "shell command")
		return cmd
	}

	// Remove "kubectl" prefix if present
	command = strings.TrimPrefix(command, "kubectl ")
	command = strings.TrimSpace(command)

	if command == "" {
		cmd.IsValid = false
		cmd.Errors = append(cmd.Errors, "empty command")
		return cmd
	}

	// Split into tokens
	tokens := tokenize(command)
	if len(tokens) == 0 {
		cmd.IsValid = false
		cmd.Errors = append(cmd.Errors, "no command specified")
		return cmd
	}

	// Parse verb (first token)
	cmd.Verb = tokens[0]
	position := 1

	// Parse flags and positional arguments
	for position < len(tokens) {
		token := tokens[position]

		// Handle flags
		if strings.HasPrefix(token, "--") {
			flagName := strings.TrimPrefix(token, "--")

			// Check if it's a boolean flag
			if isBooleanFlag(flagName) {
				cmd.BoolFlags[flagName] = true
				position++
				continue
			}

			// Check if next token is the value
			if position+1 < len(tokens) && !strings.HasPrefix(tokens[position+1], "-") {
				flagValue := tokens[position+1]
				cmd.Flags[flagName] = flagValue

				// Store specific flags
				switch flagName {
				case "namespace":
					cmd.Namespace = flagValue
				case "filename":
					cmd.Files = append(cmd.Files, flagValue)
				}

				position += 2
				continue
			} else {
				// Flag expects a value but none provided
				cmd.NeedsInput = append(cmd.NeedsInput, types.CompletionNeeded{
					Type:     getFlagCompletionType(flagName),
					Flag:     flagName,
					Required: isRequiredFlag(cmd.Verb, flagName),
				})
				position++
				continue
			}
		} else if strings.HasPrefix(token, "-") && len(token) == 2 {
			// Short flag
			flagName := strings.TrimPrefix(token, "-")

			// Check if it's a boolean flag
			if isBooleanShortFlag(flagName) {
				cmd.BoolFlags[expandShortFlag(flagName)] = true
				position++
				continue
			}

			// Check if next token is the value
			if position+1 < len(tokens) && !strings.HasPrefix(tokens[position+1], "-") {
				flagValue := tokens[position+1]
				fullFlag := expandShortFlag(flagName)
				cmd.Flags[fullFlag] = flagValue

				// Store specific flags
				switch fullFlag {
				case "namespace":
					cmd.Namespace = flagValue
				case "filename":
					cmd.Files = append(cmd.Files, flagValue)
				}

				position += 2
				continue
			} else {
				// Flag expects a value but none provided
				fullFlag := expandShortFlag(flagName)
				cmd.NeedsInput = append(cmd.NeedsInput, types.CompletionNeeded{
					Type:     getFlagCompletionType(fullFlag),
					Flag:     fullFlag,
					Required: isRequiredFlag(cmd.Verb, fullFlag),
				})
				position++
				continue
			}
		} else {
			// Positional argument
			if cmd.Resource == "" {
				cmd.Resource = normalizeResourceType(token)
			} else if cmd.ResourceName == "" {
				cmd.ResourceName = token
			}
			position++
		}
	}

	// Check if command needs more input
	p.checkCompletions(cmd)

	return cmd
}

// tokenize splits a command string into tokens
func tokenize(command string) []string {
	// Simple tokenization - doesn't handle complex quoting
	return strings.Fields(command)
}

// isBooleanFlag checks if a flag is a boolean flag
func isBooleanFlag(flag string) bool {
	boolFlags := []string{
		"all-namespaces", "A",
		"watch", "w",
		"force",
		"dry-run",
		"follow", "f",
		"help", "h",
		"no-headers",
		"show-labels",
		"wide",
	}

	for _, bf := range boolFlags {
		if flag == bf {
			return true
		}
	}

	return false
}

// isBooleanShortFlag checks if a short flag is boolean
func isBooleanShortFlag(flag string) bool {
	return isBooleanFlag(flag)
}

// expandShortFlag expands a short flag to its long form
func expandShortFlag(short string) string {
	expansions := map[string]string{
		"n": "namespace",
		"f": "filename",
		"o": "output",
		"l": "selector",
		"c": "container",
		"A": "all-namespaces",
		"w": "watch",
		"h": "help",
	}

	if long, ok := expansions[short]; ok {
		return long
	}

	return short
}

// getFlagCompletionType returns the completion type for a flag
func getFlagCompletionType(flag string) types.CompletionType {
	switch flag {
	case "namespace":
		return types.CompletionNamespace
	case "filename":
		return types.CompletionFile
	case "output":
		return types.CompletionOutputFormat
	case "container":
		return types.CompletionContainer
	case "context":
		return types.CompletionContext
	default:
		return types.CompletionNamespace
	}
}

// isRequiredFlag checks if a flag is required for a given verb
func isRequiredFlag(verb, flag string) bool {
	// Most flags are optional
	requiredFlags := map[string][]string{
		"apply": {"filename"},
	}

	if required, ok := requiredFlags[verb]; ok {
		for _, rf := range required {
			if rf == flag {
				return true
			}
		}
	}

	return false
}

// normalizeResourceType normalizes a resource type alias to its full form
func normalizeResourceType(resource string) string {
	aliases := map[string]string{
		"po":     "pods",
		"svc":    "services",
		"deploy": "deployments",
		"rs":     "replicasets",
		"rc":     "replicationcontrollers",
		"ds":     "daemonsets",
		"sts":    "statefulsets",
		"cm":     "configmaps",
		"secret": "secrets",
		"ing":    "ingresses",
		"ns":     "namespaces",
		"no":     "nodes",
		"pv":     "persistentvolumes",
		"pvc":    "persistentvolumeclaims",
		"sa":     "serviceaccounts",
		"cj":     "cronjobs",
	}

	if full, ok := aliases[resource]; ok {
		return full
	}

	return resource
}

// checkCompletions determines what completions are needed
func (p *Parser) checkCompletions(cmd *types.ParsedCommand) {
	// Check if verb needs a resource
	verbsNeedingResource := []string{"get", "describe", "delete", "edit", "logs", "exec"}
	needsResource := false
	for _, v := range verbsNeedingResource {
		if cmd.Verb == v {
			needsResource = true
			break
		}
	}

	if needsResource && cmd.Resource == "" {
		cmd.NeedsInput = append(cmd.NeedsInput, types.CompletionNeeded{
			Type:     types.CompletionResourceName,
			Required: true,
		})
		cmd.IsComplete = false
		return
	}

	// Check if resource needs a name
	verbsNeedingResourceName := []string{"describe", "delete", "edit", "logs", "exec"}
	needsResourceName := false
	for _, v := range verbsNeedingResourceName {
		if cmd.Verb == v {
			needsResourceName = true
			break
		}
	}

	if needsResourceName && cmd.ResourceName == "" && cmd.Resource != "" {
		cmd.NeedsInput = append(cmd.NeedsInput, types.CompletionNeeded{
			Type:     types.CompletionResourceName,
			Required: true,
		})
		cmd.IsComplete = false
		return
	}

	// Check for namespace flag value needed
	if _, ok := cmd.Flags["namespace"]; !ok && cmd.Namespace == "" {
		// Namespace is optional, not adding to NeedsInput unless explicitly requested
	}

	// If no required inputs are missing, command is complete
	hasRequired := false
	for _, need := range cmd.NeedsInput {
		if need.Required {
			hasRequired = true
			break
		}
	}

	cmd.IsComplete = !hasRequired
}

// GetResourceTypes returns a list of common resource types
func GetResourceTypes() []string {
	return []string{
		"pods",
		"services",
		"deployments",
		"replicasets",
		"statefulsets",
		"daemonsets",
		"jobs",
		"cronjobs",
		"configmaps",
		"secrets",
		"ingresses",
		"persistentvolumes",
		"persistentvolumeclaims",
		"nodes",
		"namespaces",
		"serviceaccounts",
		"roles",
		"rolebindings",
		"clusterroles",
		"clusterrolebindings",
	}
}
