package kubecomplete

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
)

// CommandRuntime is the compiled form used by the engine.
type CommandRuntime struct {
	Spec           *CommandSpec
	Key            string            // "get", "rollout restart"
	AliasToPrimary map[string]string // "--namespace" -> "-n"
}

type Registry struct {
	Commands map[string]*CommandRuntime // key: strings.Join(Path, " ")
}

// LoadRootSpecFromFile reads kubectl_commands.json.
func LoadRootSpecFromFile(path string) (*RootSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root RootSpec
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	return &root, nil
}

func NewRegistry(root *RootSpec) *Registry {
	cmds := make(map[string]*CommandRuntime, len(root.Commands))
	for i := range root.Commands {
		spec := &root.Commands[i]
		key := strings.Join(spec.Path, " ")
		rt := &CommandRuntime{
			Spec:           spec,
			Key:            key,
			AliasToPrimary: make(map[string]string),
		}
		for primary, flag := range spec.Flags {
			// ensure mapping includes all forms
			rt.AliasToPrimary[flag.Primary] = primary
			for _, alias := range flag.Aliases {
				rt.AliasToPrimary[alias] = primary
			}
			// some JSONs may use map key != Primary, be safe:
			if primary != flag.Primary {
				rt.AliasToPrimary[primary] = primary
			}
		}
		cmds[key] = rt
	}
	return &Registry{Commands: cmds}
}

// MatchCommand finds the longest matching command path in tokens.
// returns (command, numberOfTokensConsumedFromStart)
func (r *Registry) MatchCommand(tokens []string) (*CommandRuntime, int) {
	if len(tokens) == 0 {
		return nil, 0
	}
	for i := len(tokens); i > 0; i-- {
		key := strings.Join(tokens[:i], " ")
		if cmd, ok := r.Commands[key]; ok {
			return cmd, i
		}
	}
	return nil, 0
}

// TopLevelCommands returns unique first tokens for suggestion.
func (r *Registry) TopLevelCommands() []string {
	seen := make(map[string]struct{})
	var out []string
	for key := range r.Commands {
		parts := strings.Split(key, " ")
		if len(parts) == 0 {
			continue
		}
		first := parts[0]
		if _, ok := seen[first]; !ok {
			seen[first] = struct{}{}
			out = append(out, first)
		}
	}
	sort.Strings(out)
	return out
}
