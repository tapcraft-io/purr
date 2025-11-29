## 1. Data structures + JSON loading

Your JSON root looks like:

```json
{
  "version": "1.0",
  "generated_from": "kubectl_reference___Kubernetes.md",
  "commands": [ ... ]
}
```

and each command matches the schema we designed earlier. 

### 1.1. Types

```go
package kubecomplete

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type TokenKind string

const (
	TokenLiteral                TokenKind = "literal"
	TokenResourceType           TokenKind = "resource-type"
	TokenResourceName           TokenKind = "resource-name"
	TokenResourceNameOrSelector TokenKind = "resource-name-or-selector"
	TokenResourceTarget         TokenKind = "resource-target"
	TokenNamespace              TokenKind = "namespace"
	TokenSelector               TokenKind = "selector"
	TokenContainerName          TokenKind = "container-name"
	TokenOutput                 TokenKind = "output"
	TokenDuration               TokenKind = "duration"
	TokenOther                  TokenKind = "other"
)

// TokenDescriptor is used for positionals and for `after` in flags.
type TokenDescriptor struct {
	Kind     TokenKind `json:"kind"`
	Role     string    `json:"role"`
	Required bool      `json:"required"`
	Value    string    `json:"value"`   // for runtime, ignore when loading
	Allowed  []string  `json:"allowed"` // optional fixed set of values
}

type FlagDescriptor struct {
	Kind        string           `json:"kind"` // "flag"
	Primary     string           `json:"primary"`
	Aliases     []string         `json:"aliases"`
	Role        string           `json:"role"`
	Required    bool             `json:"required"`
	After       *TokenDescriptor `json:"after"`       // nil if flag has no value
	Description string           `json:"description"` // short help
}

type CommandSpec struct {
	Path        []string                  `json:"path"`        // e.g. ["get"], ["rollout","restart"]
	Synopsis    string                    `json:"synopsis"`
	Description string                    `json:"description"`
	Positionals []TokenDescriptor         `json:"positionals"`
	Flags       map[string]FlagDescriptor `json:"flags"` // keyed by primary
}

type RootSpec struct {
	Version       string        `json:"version"`
	GeneratedFrom string        `json:"generated_from"`
	Commands      []CommandSpec `json:"commands"`
}
```

### 1.2. Runtime registry

We want a fast way to:

* map token slices → command
* handle flag aliases

```go
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
```

### 1.3. Command matching (supports subcommands)

We want **longest-prefix matching** on the tokens (without `kubectl`).

```go
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
```

---

## 2. Cluster cache (namespaces, resource names, etc.)

You said you’ll have a cache of:

* resource types (maybe per verb)
* resource names per type/namespace
* namespaces
* containers

Let’s model that:

```go
// ClusterCache is your abstraction over client-go/cache.
type ClusterCache interface {
	Namespaces() []string
	// Example: resources you care about for "get" etc.
	ResourceTypes() []string
	// Optional: narrower list per verb (e.g. "rollout" → deployments, daemonsets, statefulsets)
	ResourceTypesForCommand(path []string) []string

	// Names for a given resource type in a namespace.
	ResourceNames(kind, namespace string) []string

	// Container names for a pod/workload target.
	Containers(namespace, resourceKind, resourceName string) []string
}
```

You can implement this however you want (watchers, informers, polling, etc.) – the completer just calls it.

---

## 3. Suggestion model

```go
type SuggestionKind string

const (
	SuggestCommand       SuggestionKind = "command"
	SuggestFlag          SuggestionKind = "flag"
	SuggestFlagValue     SuggestionKind = "flag-value"
	SuggestResourceType  SuggestionKind = "resource-type"
	SuggestResourceName  SuggestionKind = "resource-name"
	SuggestNamespace     SuggestionKind = "namespace"
	SuggestContainer     SuggestionKind = "container"
	SuggestOther         SuggestionKind = "other"
)

type Suggestion struct {
	Value       string
	Kind        SuggestionKind
	Description string
	Score       float64
}
```

---

## 4. Completer: main entry point

```go
type CompletionContext struct {
	Line   string
	Cursor int

	CurrentNamespace string
}

type Completer struct {
	Registry *Registry
	Cache    ClusterCache
}

func NewCompleter(reg *Registry, cache ClusterCache) *Completer {
	return &Completer{
		Registry: reg,
		Cache:    cache,
	}
}

// Complete is the main entry: pass the full line and cursor pos (byte offset).
func (c *Completer) Complete(line string, cursor int, ctx CompletionContext) []Suggestion {
	if c.Registry == nil {
		return nil
	}
	if cursor < 0 || cursor > len(line) {
		cursor = len(line)
	}
	segment := line[:cursor]

	tokens := shellSplit(segment)
	tokens = normalizeKubectl(tokens)

	if len(tokens) == 0 {
		return c.suggestTopLevelCommands("")
	}

	cmd, pathLen := c.Registry.MatchCommand(tokens)
	if cmd == nil {
		// no command yet → suggest command names
		return c.suggestTopLevelCommands(tokens[0])
	}

	args := tokens[pathLen:] // after command path

	// Case 1: last token is an unfinished flag value for a flag with `after`
	if len(args) > 0 && isFlagToken(args[len(args)-1]) {
		return c.suggestAfterFlag(cmd, ctx, args)
	}

	// Otherwise: we are between args / after a completed argument → use positional + flags
	return c.suggestPositionalsAndFlags(cmd, ctx, args)
}
```

Token helpers:

```go
func shellSplit(s string) []string {
	return strings.Fields(s) // good enough; you can swap in a real shell parser later
}

func normalizeKubectl(tokens []string) []string {
	if len(tokens) > 0 && tokens[0] == "kubectl" {
		return tokens[1:]
	}
	return tokens
}

func isFlagToken(tok string) bool {
	return strings.HasPrefix(tok, "-")
}
```

---

## 5. Top-level command suggestions

```go
func (c *Completer) suggestTopLevelCommands(prefix string) []Suggestion {
	names := c.Registry.TopLevelCommands()
	var out []Suggestion
	for _, name := range names {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			out = append(out, Suggestion{
				Value:       name,
				Kind:        SuggestCommand,
				Description: "",
				Score:       scorePrefix(name, prefix),
			})
		}
	}
	sortSuggestions(out)
	return out
}

func scorePrefix(value, prefix string) float64 {
	if prefix == "" {
		return 0
	}
	if strings.HasPrefix(value, prefix) {
		return float64(len(prefix)) + 10
	}
	if strings.Contains(value, prefix) {
		return float64(len(prefix))
	}
	return 0
}

func sortSuggestions(s []Suggestion) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Score == s[j].Score {
			return s[i].Value < s[j].Value
		}
		return s[i].Score > s[j].Score
	})
}
```

---

## 6. “After flag” logic (e.g. `-n |` → namespaces)

We look at the last arg (a flag), resolve alias → primary, then look up the `after` descriptor to decide what values to suggest.

```go
func (c *Completer) suggestAfterFlag(cmd *CommandRuntime, ctx CompletionContext, args []string) []Suggestion {
	if len(args) == 0 {
		return nil
	}
	flagToken := args[len(args)-1]
	primary, ok := cmd.AliasToPrimary[flagToken]
	if !ok {
		// unknown flag → fall back
		return c.suggestPositionalsAndFlags(cmd, ctx, args)
	}

	flagDesc, ok := cmd.Spec.Flags[primary]
	if !ok || flagDesc.After == nil {
		// flag doesn't take a value
		return c.suggestPositionalsAndFlags(cmd, ctx, args)
	}

	td := flagDesc.After
	switch td.Kind {
	case TokenNamespace:
		return c.suggestNamespaces(ctx)
	case TokenOutput:
		return c.suggestEnumValues(td.Allowed, "Output format")
	case TokenSelector:
		// Usually freeform; you could still suggest recent selectors if you track them.
		return nil
	case TokenContainerName:
		// we *could* inspect earlier args to find pod/workload; for now just ask cache with empty.
		return c.suggestContainers(ctx, "", "", "")
	case TokenResourceType:
		return c.suggestResourceTypes(cmd, ctx, td)
	case TokenResourceName, TokenResourceNameOrSelector:
		kind := inferResourceKindFromArgs(cmd, args)
		return c.suggestResourceNames(ctx, kind, ctx.CurrentNamespace, td)
	case TokenDuration, TokenOther:
		// leave as freeform, unless Allowed is non-empty
		if len(td.Allowed) > 0 {
			return c.suggestEnumValues(td.Allowed, td.Role)
		}
		return nil
	default:
		return nil
	}
}
```

Helper to suggest enum-like values:

```go
func (c *Completer) suggestEnumValues(values []string, desc string) []Suggestion {
	if len(values) == 0 {
		return nil
	}
	out := make([]Suggestion, 0, len(values))
	for _, v := range values {
		out = append(out, Suggestion{
			Value:       v,
			Kind:        SuggestFlagValue,
			Description: desc,
			Score:       40,
		})
	}
	sortSuggestions(out)
	return out
}
```

Namespace / containers via cache:

```go
func (c *Completer) suggestNamespaces(ctx CompletionContext) []Suggestion {
	if c.Cache == nil {
		return nil
	}
	names := c.Cache.Namespaces()
	out := make([]Suggestion, 0, len(names))
	for _, ns := range names {
		score := 50.0
		if ctx.CurrentNamespace != "" && ns == ctx.CurrentNamespace {
			score += 10
		}
		out = append(out, Suggestion{
			Value:       ns,
			Kind:        SuggestNamespace,
			Description: "Namespace",
			Score:       score,
		})
	}
	sortSuggestions(out)
	return out
}

func (c *Completer) suggestContainers(ctx CompletionContext, kind, name, ns string) []Suggestion {
	if c.Cache == nil {
		return nil
	}
	if ns == "" {
		ns = ctx.CurrentNamespace
	}
	names := c.Cache.Containers(ns, kind, name)
	out := make([]Suggestion, 0, len(names))
	for _, cn := range names {
		out = append(out, Suggestion{
			Value:       cn,
			Kind:        SuggestContainer,
			Description: "Container",
			Score:       45,
		})
	}
	sortSuggestions(out)
	return out
}
```

Resource types / names via cache **plus** `allowed` list from your JSON:

```go
func (c *Completer) suggestResourceTypes(cmd *CommandRuntime, ctx CompletionContext, td *TokenDescriptor) []Suggestion {
	var types []string

	// If JSON lists Allowed, that wins (e.g. rollout restart: deployment|daemonset|statefulset)
	if len(td.Allowed) > 0 {
		types = td.Allowed
	} else if c.Cache != nil {
		// Command-specific override if provided
		if rt := c.Cache.ResourceTypesForCommand(cmd.Spec.Path); len(rt) > 0 {
			types = rt
		} else {
			types = c.Cache.ResourceTypes()
		}
	}

	out := make([]Suggestion, 0, len(types))
	for _, t := range types {
		out = append(out, Suggestion{
			Value:       t,
			Kind:        SuggestResourceType,
			Description: "Resource type",
			Score:       55,
		})
	}
	sortSuggestions(out)
	return out
}

func (c *Completer) suggestResourceNames(ctx CompletionContext, kind, ns string, td *TokenDescriptor) []Suggestion {
	if c.Cache == nil {
		return nil
	}
	if ns == "" {
		ns = ctx.CurrentNamespace
	}
	names := c.Cache.ResourceNames(kind, ns)
	out := make([]Suggestion, 0, len(names))
	for _, n := range names {
		out = append(out, Suggestion{
			Value:       n,
			Kind:        SuggestResourceName,
			Description: fmt.Sprintf("%s in %s", kind, ns),
			Score:       50,
		})
	}
	sortSuggestions(out)
	return out
}
```

`inferResourceKindFromArgs` can be simple heuristics:

```go
// Very rough heuristic: look for last non-flag token before current position,
// if it looks like TYPE/NAME, split on '/', else if there was an earlier resource-type positional, use that.
func inferResourceKindFromArgs(cmd *CommandRuntime, args []string) string {
	// Walk backwards, skip flags and their values
	i := len(args) - 1
	for i >= 0 {
		a := args[i]
		if isFlagToken(a) {
			primary, ok := cmd.AliasToPrimary[a]
			if !ok {
				i--
				continue
			}
			flag := cmd.Spec.Flags[primary]
			if flag.After != nil && i+1 < len(args) {
				i -= 2
			} else {
				i--
			}
			continue
		}
		// non-flag
		if strings.Contains(a, "/") {
			parts := strings.SplitN(a, "/", 2)
			return parts[0] // TYPE/NAME
		}
		// if it's the first positional, might be resource-type, but we have no list here;
		// you could try matching against Cache.ResourceTypes().
		i--
	}
	return ""
}
```

---

## 7. Positional + flag suggestion logic

This is where we:

* determine **how many positionals** are already satisfied
* suggest the **next positional**
* plus suggest **flags**, with priority rules (namespace flag first, etc.)

```go
func (c *Completer) suggestPositionalsAndFlags(cmd *CommandRuntime, ctx CompletionContext, args []string) []Suggestion {
	spec := cmd.Spec

	usedFlags := parseUsedFlags(cmd, args)
	posIndex := countSatisfiedPositionals(spec.Positionals, cmd, args)

	var out []Suggestion

	// 1. Suggest next positional (if any)
	if posIndex < len(spec.Positionals) {
		td := &spec.Positionals[posIndex]
		out = append(out, c.suggestForPositional(cmd, ctx, td, args)...)
	}

	// 2. Suggest flags (not yet used), with weighted scores
	for primary, flag := range spec.Flags {
		if usedFlags[primary] {
			continue
		}
		out = append(out, Suggestion{
			Value:       flag.Primary,
			Kind:        SuggestFlag,
			Description: flag.Description,
			Score:       scoreFlag(flag),
		})
	}

	sortSuggestions(out)
	return out
}

func parseUsedFlags(cmd *CommandRuntime, args []string) map[string]bool {
	used := make(map[string]bool)
	i := 0
	for i < len(args) {
		a := args[i]
		if !isFlagToken(a) {
			i++
			continue
		}
		primary, ok := cmd.AliasToPrimary[a]
		if !ok {
			i++
			continue
		}
		used[primary] = true
		flag := cmd.Spec.Flags[primary]
		if flag.After != nil && i+1 < len(args) {
			i += 2
		} else {
			i++
		}
	}
	return used
}

func countSatisfiedPositionals(positionals []TokenDescriptor, cmd *CommandRuntime, args []string) int {
	posIndex := 0
	i := 0
	for i < len(args) && posIndex < len(positionals) {
		a := args[i]
		if isFlagToken(a) {
			primary, ok := cmd.AliasToPrimary[a]
			if !ok {
				i++
				continue
			}
			flag := cmd.Spec.Flags[primary]
			if flag.After != nil && i+1 < len(args) {
				i += 2
			} else {
				i++
			}
			continue
		}
		// treat any non-flag as filling a positional slot
		posIndex++
		i++
	}
	return posIndex
}

func (c *Completer) suggestForPositional(cmd *CommandRuntime, ctx CompletionContext, td *TokenDescriptor, args []string) []Suggestion {
	switch td.Kind {
	case TokenResourceType:
		return c.suggestResourceTypes(cmd, ctx, td)
	case TokenResourceName, TokenResourceNameOrSelector:
		kind := inferResourceKindFromArgs(cmd, args)
		return c.suggestResourceNames(ctx, kind, ctx.CurrentNamespace, td)
	case TokenNamespace:
		return c.suggestNamespaces(ctx)
	case TokenContainerName:
		kind := inferResourceKindFromArgs(cmd, args)
		// you might also derive pod/workload name by scanning args; we keep it simple here.
		return c.suggestContainers(ctx, kind, "", "")
	case TokenOutput:
		return c.suggestEnumValues(td.Allowed, "Output format")
	default:
		return nil
	}
}

// Score flags so that namespace / selector flags come early, and required flags first.
func scoreFlag(f FlagDescriptor) float64 {
	score := 10.0
	if f.Required {
		score += 50
	}
	switch f.Role {
	case "namespace-scope":
		score += 40
	case "label-selector", "field-selector":
		score += 30
	case "output-format":
		score += 20
	case "container-selector":
		score += 18
	}
	// shorter flags are nicer to type, slight boost
	score += float64(5 - len(f.Primary))
	return score
}
```

This is where your “**if the command needs a namespace to run, suggest `-n` first**” preference is encoded:

* any flag with `role == "namespace-scope"` gets +40
* plus required flags get +50
* so `-n` floats to the top of the suggestion list for most namespaced commands.

If later you decide to mark some commands/positionals as cluster-scoped vs namespaced, you can refine `scoreFlag` based on `cmd.Spec.Path` or `positionals`.

---

## 8. Putting it together in your TUI

Example usage (pseudo-main):

```go
root, err := LoadRootSpecFromFile("kubectl_commands.json")
if err != nil {
	log.Fatal(err)
}
reg := NewRegistry(root)

cache := NewYourClusterCache() // your implementation
comp := NewCompleter(reg, cache)

// In your readline/bubbletea loop:
line := currentInputLine   // e.g. "kubectl get po -n "
cursor := len(line)
ctx := CompletionContext{
	Line:            line,
	Cursor:          cursor,
	CurrentNamespace: "default", // or from kubeconfig
}

suggestions := comp.Complete(line, cursor, ctx)
// Render suggestions in your UI (dropdown, list, etc.)
```
