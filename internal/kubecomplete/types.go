package kubecomplete

import "sort"

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

// Suggestion model
type SuggestionKind string

const (
	SuggestCommand      SuggestionKind = "command"
	SuggestFlag         SuggestionKind = "flag"
	SuggestFlagValue    SuggestionKind = "flag-value"
	SuggestResourceType SuggestionKind = "resource-type"
	SuggestResourceName SuggestionKind = "resource-name"
	SuggestNamespace    SuggestionKind = "namespace"
	SuggestContainer    SuggestionKind = "container"
	SuggestOther        SuggestionKind = "other"
)

type Suggestion struct {
	Value       string
	Kind        SuggestionKind
	Description string
	Score       float64
}

// CompletionContext holds context for completion
type CompletionContext struct {
	Line             string
	Cursor           int
	CurrentNamespace string
}

// Helper function to sort suggestions
func sortSuggestions(s []Suggestion) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Score == s[j].Score {
			return s[i].Value < s[j].Value
		}
		return s[i].Score > s[j].Score
	})
}
