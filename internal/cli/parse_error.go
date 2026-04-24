package cli

import (
	"strings"

	"github.com/austinjan/aascribe/internal/apperr"
)

type ParseErrorKind string

const (
	ParseErrorNoSubcommand         ParseErrorKind = "no_subcommand"
	ParseErrorMissingFlagValue     ParseErrorKind = "missing_flag_value"
	ParseErrorInvalidFlagValue     ParseErrorKind = "invalid_flag_value"
	ParseErrorUnknownGlobalFlag    ParseErrorKind = "unknown_global_flag"
	ParseErrorUnknownCommand       ParseErrorKind = "unknown_command"
	ParseErrorMissingNestedCommand ParseErrorKind = "missing_nested_command"
	ParseErrorUnknownNestedCommand ParseErrorKind = "unknown_nested_command"
	ParseErrorDoesNotAcceptArgs    ParseErrorKind = "does_not_accept_args"
	ParseErrorMissingRequiredArg   ParseErrorKind = "missing_required_arg"
	ParseErrorMissingRequiredFlag  ParseErrorKind = "missing_required_flag"
	ParseErrorTooManyArgs          ParseErrorKind = "too_many_args"
)

type ParseError struct {
	base        *apperr.Error
	Kind        ParseErrorKind
	Scope       string
	Token       string
	Suggestions []string
}

func (e *ParseError) Error() string {
	return e.base.Error()
}

func (e *ParseError) AppError() *apperr.Error {
	return e.base
}

func newParseError(kind ParseErrorKind, scope, token string, message string, args ...any) *ParseError {
	return &ParseError{
		base:  apperr.InvalidArguments(message, args...),
		Kind:  kind,
		Scope: scope,
		Token: token,
	}
}

func newUnknownCommandError(token string) *ParseError {
	err := newParseError(ParseErrorUnknownCommand, "root", token, "Unknown subcommand %s.", token)
	err.Suggestions = suggestCommand(token, topLevelCommands())
	return err
}

func newUnknownNestedCommandError(scope, token string, candidates []string) *ParseError {
	err := newParseError(ParseErrorUnknownNestedCommand, scope, token, "Unknown %s subcommand %s.", scope, token)
	err.Suggestions = suggestCommand(token, candidates)
	return err
}

func newUnknownGlobalFlagError(token string) *ParseError {
	err := newParseError(ParseErrorUnknownGlobalFlag, "global", token, "Unknown global flag %s.", token)
	err.Suggestions = suggestCommand(token, []string{"--store", "--format", "--quiet", "--verbose", "--help", "--version"})
	return err
}

func suggestCommand(token string, candidates []string) []string {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}

	type suggestion struct {
		value string
		score int
	}

	results := make([]suggestion, 0, len(candidates))
	for _, candidate := range candidates {
		score := suggestionScore(token, candidate)
		if score < 0 {
			continue
		}
		results = append(results, suggestion{value: candidate, score: score})
	}
	if len(results) == 0 {
		return nil
	}

	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score < results[i].score || (results[j].score == results[i].score && results[j].value < results[i].value) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	limit := 3
	if len(results) < limit {
		limit = len(results)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, results[i].value)
	}
	return out
}

func suggestionScore(token, candidate string) int {
	normalizedToken := strings.TrimPrefix(token, "-")
	normalizedCandidate := strings.TrimPrefix(candidate, "-")
	if normalizedToken == normalizedCandidate {
		return 0
	}
	if strings.HasPrefix(normalizedCandidate, normalizedToken) || strings.HasPrefix(normalizedToken, normalizedCandidate) {
		return 1
	}
	distance := levenshtein(normalizedToken, normalizedCandidate)
	maxLen := len(normalizedCandidate)
	if len(normalizedToken) > maxLen {
		maxLen = len(normalizedToken)
	}
	threshold := 2
	if maxLen >= 10 {
		threshold = 3
	}
	if distance > threshold {
		return -1
	}
	return distance
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min3(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func topLevelCommands() []string {
	return []string{"init", "logs", "output", "operation", "index", "describe", "remember", "consolidate", "recall", "chat", "summarize", "list", "show", "forget"}
}
