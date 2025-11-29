package kubecomplete

import (
	"fmt"
	"strings"
)

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

	// Check if we have trailing space (user finished typing current token)
	hasTrailingSpace := len(segment) > 0 && (segment[len(segment)-1] == ' ' || segment[len(segment)-1] == '\t')

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

	// Case 1: We just finished typing a flag and added space - suggest flag value
	if hasTrailingSpace && len(args) > 0 && isFlagToken(args[len(args)-1]) {
		return c.suggestAfterFlag(cmd, ctx, args)
	}

	// Case 2: We're typing a flag (no trailing space) - this will be handled by positionals+flags
	// Case 3: Otherwise - suggest positionals and flags
	return c.suggestPositionalsAndFlags(cmd, ctx, args)
}

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
