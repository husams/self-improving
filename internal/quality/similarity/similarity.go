// Package similarity scores a candidate string (typically the final assistant
// message) against a reference answer. Backends are pluggable behind the
// Method enum so callers can pick rouge (default, no deps), bleu, or — when
// Slice 5 lands — llm. An "embedding" method is reserved but deferred.
package similarity

import (
	"context"
	"fmt"
	"strings"
)

// Method is the chosen backend for a similarity check.
type Method string

const (
	MethodROUGE     Method = "rouge"
	MethodBLEU      Method = "bleu"
	MethodLLM       Method = "llm"
	MethodEmbedding Method = "embedding" // deferred; not implemented in v1
)

// LLMRunner is the function the LLM backend calls. Returns score 0-100, notes,
// and an error. Errors trigger a fallback to ROUGE.
type LLMRunner func(ctx context.Context, candidate, reference string) (float64, []string, error)

// Score is the per-backend signature. Returns a 0-100 float and per-call notes.
type Score struct {
	Value float64
	Notes []string
}

// Compute dispatches to the configured backend. Unknown methods fall back to
// ROUGE with a note. Errors from the LLM backend are handled inside the LLM
// implementation and surfaced as fallback notes; Compute itself does not
// return an error.
func Compute(ctx context.Context, method Method, candidate, reference string, llm LLMRunner) Score {
	switch normalizeMethod(method) {
	case MethodROUGE:
		return Score{Value: ROUGE_L(candidate, reference)}
	case MethodBLEU:
		return Score{Value: BLEU4(candidate, reference)}
	case MethodLLM:
		return computeLLM(ctx, candidate, reference, llm)
	case MethodEmbedding:
		return Score{
			Value: ROUGE_L(candidate, reference),
			Notes: []string{"embedding backend deferred to v2; falling back to rouge"},
		}
	default:
		return Score{
			Value: ROUGE_L(candidate, reference),
			Notes: []string{fmt.Sprintf("unknown similarity method %q; falling back to rouge", method)},
		}
	}
}

func normalizeMethod(m Method) Method {
	return Method(strings.ToLower(strings.TrimSpace(string(m))))
}

// tokenize lowercases and splits on whitespace + common punctuation. Used by
// both ROUGE and BLEU.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	repl := strings.NewReplacer(
		".", " ", ",", " ", ";", " ", ":", " ", "!", " ", "?", " ",
		"\"", " ", "'", " ", "(", " ", ")", " ", "[", " ", "]", " ",
		"\n", " ", "\t", " ",
	)
	s = repl.Replace(s)
	fields := strings.Fields(s)
	return fields
}
