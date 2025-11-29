package kubecomplete

import (
	"fmt"
	"os"
	"strings"
)

func debugLog(msg string) {
	f, err := os.OpenFile("/tmp/purr-completer-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", msg)
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
	debugLog(fmt.Sprintf("=== Complete called: line=%q, cursor=%d ===", line, cursor))

	if c.Registry == nil {
		debugLog("Registry is nil, returning empty")
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

	debugLog(fmt.Sprintf("tokens=%v, hasTrailingSpace=%v", tokens, hasTrailingSpace))

	if len(tokens) == 0 {
		debugLog("No tokens, suggesting top-level commands")
		return c.suggestTopLevelCommands("")
	}

	cmd, pathLen := c.Registry.MatchCommand(tokens)
	debugLog(fmt.Sprintf("MatchCommand result: cmd=%v, pathLen=%d", cmd != nil, pathLen))

	if cmd == nil {
		debugLog("No command match, checking for subcommands")
		// No exact command match - check if we're building a subcommand
		// e.g., "rollout " or "rollout" or "rollout re" should suggest subcommands
		subcommands := c.suggestSubcommands(tokens)
		debugLog(fmt.Sprintf("suggestSubcommands returned %d results", len(subcommands)))
		if len(subcommands) > 0 {
			return subcommands
		}
		// Otherwise suggest top-level command names
		debugLog(fmt.Sprintf("No subcommands, suggesting top-level with prefix=%q", tokens[0]))
		return c.suggestTopLevelCommands(tokens[0])
	}

	// Check if there are subcommands available (e.g., "rollout" -> "rollout restart")
	// This handles cases like typing "rollout" where we match the command but
	// subcommands exist that should be suggested
	if !hasTrailingSpace && pathLen == len(tokens) {
		debugLog("Checking for subcommands (complete command, no trailing space)")
		// We matched a complete command but might have subcommands
		subcommands := c.suggestSubcommands(tokens)
		debugLog(fmt.Sprintf("suggestSubcommands returned %d results", len(subcommands)))
		if len(subcommands) > 0 {
			return subcommands
		}
	}

	args := tokens[pathLen:] // after command path
	debugLog(fmt.Sprintf("args=%v (tokens after command path)", args))

	// Case 1: We're typing a flag value (e.g., "get pods -n d")
	// Check if second-to-last arg is a flag and last arg is not a flag
	if !hasTrailingSpace && len(args) >= 2 {
		secondToLast := args[len(args)-2]
		lastArg := args[len(args)-1]
		if isFlagToken(secondToLast) && !isFlagToken(lastArg) {
			debugLog(fmt.Sprintf("Typing flag value: flag=%s, value=%s", secondToLast, lastArg))
			// We're typing a flag value - suggest completions for that flag
			// Pass args without the partial value so suggestAfterFlag can identify the flag
			return c.suggestAfterFlag(cmd, ctx, args[:len(args)-1], true)
		}
	}

	// Case 2: We just finished typing a flag and added space - suggest flag value
	if hasTrailingSpace && len(args) > 0 && isFlagToken(args[len(args)-1]) {
		debugLog(fmt.Sprintf("Just typed flag with space: %s", args[len(args)-1]))
		return c.suggestAfterFlag(cmd, ctx, args, hasTrailingSpace)
	}

	// Case 3: Otherwise - suggest positionals and flags
	debugLog("Calling suggestPositionalsAndFlags")
	return c.suggestPositionalsAndFlags(cmd, ctx, args, hasTrailingSpace)
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

// suggestSubcommands suggests the next part of a multi-part command
// e.g., for ["rollout"], suggest ["restart", "status", "pause", ...]
// Handles partial matches: ["rollout", "re"] suggests "restart"
func (c *Completer) suggestSubcommands(tokens []string) []Suggestion {
	if c.Registry == nil || len(tokens) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var out []Suggestion

	// Check all commands in registry
	// For "rollout re", we match commands starting with "rollout" and filter by "re" prefix
	for _, cmd := range c.Registry.Commands {
		if len(cmd.Spec.Path) < len(tokens) {
			continue
		}

		// Try exact match first (all tokens must match exactly)
		exactMatch := true
		for i := 0; i < len(tokens)-1; i++ {
			if i >= len(cmd.Spec.Path) || cmd.Spec.Path[i] != tokens[i] {
				exactMatch = false
				break
			}
		}

		if !exactMatch {
			continue
		}

		// For the last token, try both exact and prefix match
		lastIdx := len(tokens) - 1
		lastToken := tokens[lastIdx]

		if lastIdx < len(cmd.Spec.Path) {
			pathToken := cmd.Spec.Path[lastIdx]

			// Exact match - suggest next token
			if pathToken == lastToken {
				if len(cmd.Spec.Path) > len(tokens) {
					nextToken := cmd.Spec.Path[len(tokens)]
					if !seen[nextToken] {
						seen[nextToken] = true
						out = append(out, Suggestion{
							Value:       nextToken,
							Kind:        SuggestCommand,
							Description: "",
							Score:       50,
						})
					}
				}
			} else if strings.HasPrefix(pathToken, lastToken) {
				// Prefix match - suggest this token
				if !seen[pathToken] {
					seen[pathToken] = true
					out = append(out, Suggestion{
						Value:       pathToken,
						Kind:        SuggestCommand,
						Description: "",
						Score:       scorePrefix(pathToken, lastToken),
					})
				}
			}
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

func (c *Completer) suggestAfterFlag(cmd *CommandRuntime, ctx CompletionContext, args []string, hasTrailingSpace bool) []Suggestion {
	if len(args) == 0 {
		return nil
	}
	flagToken := args[len(args)-1]
	primary, ok := cmd.AliasToPrimary[flagToken]
	if !ok {
		// unknown flag → fall back
		return c.suggestPositionalsAndFlags(cmd, ctx, args, hasTrailingSpace)
	}

	flagDesc, ok := cmd.Spec.Flags[primary]
	if !ok || flagDesc.After == nil {
		// flag doesn't take a value
		return c.suggestPositionalsAndFlags(cmd, ctx, args, hasTrailingSpace)
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

func (c *Completer) suggestPositionalsAndFlags(cmd *CommandRuntime, ctx CompletionContext, args []string, hasTrailingSpace bool) []Suggestion {
	spec := cmd.Spec

	debugLog(fmt.Sprintf("suggestPositionalsAndFlags: args=%v, hasTrailingSpace=%v, numPositionals=%d", args, hasTrailingSpace, len(spec.Positionals)))

	usedFlags := parseUsedFlags(cmd, args)
	posIndex := countSatisfiedPositionals(spec.Positionals, cmd, args, hasTrailingSpace)

	debugLog(fmt.Sprintf("posIndex=%d (satisfied positionals)", posIndex))

	var out []Suggestion

	// 1. Suggest next positional (if any)
	if posIndex < len(spec.Positionals) {
		td := &spec.Positionals[posIndex]
		debugLog(fmt.Sprintf("Suggesting positional %d, kind=%s", posIndex, td.Kind))
		out = append(out, c.suggestForPositional(cmd, ctx, td, args)...)
	} else if posIndex > 0 && posIndex == len(spec.Positionals) {
		debugLog("All positionals satisfied, checking for resource name suggestions")
		// All positionals are satisfied, but if the first positional was a resource type,
		// suggest resource names for that type (e.g., "rollout restart deployment" -> suggest deployment names)
		firstPos := &spec.Positionals[0]
		debugLog(fmt.Sprintf("First positional kind=%s", firstPos.Kind))
		if firstPos.Kind == TokenResourceType || firstPos.Kind == TokenResourceName {
			// Get the resource type from the first non-flag arg
			resourceType := getFirstNonFlagArg(args)
			debugLog(fmt.Sprintf("Resource type from args: %q", resourceType))
			if resourceType != "" && c.Cache != nil {
				// Extract namespace from flags if specified
				ns := extractNamespaceFromArgs(cmd, args)
				if ns == "" {
					ns = ctx.CurrentNamespace
				}
				debugLog(fmt.Sprintf("Looking up resource names for type=%s, namespace=%s", resourceType, ns))
				names := c.Cache.ResourceNames(resourceType, ns)
				debugLog(fmt.Sprintf("Found %d resource names", len(names)))
				for _, name := range names {
					out = append(out, Suggestion{
						Value:       name,
						Kind:        SuggestResourceName,
						Description: fmt.Sprintf("%s in %s", resourceType, ns),
						Score:       55, // Higher than flags
					})
				}
			}
		}
	}

	// 2. Suggest flags (not yet used), with weighted scores
	flagCount := 0
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
		flagCount++
	}

	debugLog(fmt.Sprintf("Added %d flags to suggestions", flagCount))
	debugLog(fmt.Sprintf("Total suggestions before sort: %d", len(out)))

	sortSuggestions(out)
	return out
}

// getFirstNonFlagArg returns the first argument that isn't a flag or flag value
func getFirstNonFlagArg(args []string) string {
	i := 0
	for i < len(args) {
		if !isFlagToken(args[i]) {
			return args[i]
		}
		// Skip flag and potentially its value
		if i+1 < len(args) && !isFlagToken(args[i+1]) {
			i += 2
		} else {
			i++
		}
	}
	return ""
}

// extractNamespaceFromArgs extracts the namespace value from -n or --namespace flags
func extractNamespaceFromArgs(cmd *CommandRuntime, args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" || args[i] == "--namespace" {
			if i+1 < len(args) && !isFlagToken(args[i+1]) {
				return args[i+1]
			}
		}
	}
	return ""
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

func countSatisfiedPositionals(positionals []TokenDescriptor, cmd *CommandRuntime, args []string, hasTrailingSpace bool) int {
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
		// Don't count the last token as satisfied if there's no trailing space
		// (it's still being typed, so we should suggest completions for it)
		isLastToken := (i == len(args)-1)
		if isLastToken && !hasTrailingSpace {
			// This is a partial token being typed - don't count it as satisfied
			break
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

		// For certain commands, if we can't infer the kind, suggest resource types first
		if kind == "" && len(args) == 0 {
			// First positional with no args - suggest resource type instead
			// This handles commands like "logs", "describe", "delete", etc.
			return c.suggestResourceTypes(cmd, ctx, td)
		}

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
