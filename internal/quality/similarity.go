package quality

import (
	"context"
	"fmt"
)

// SimilarityCheck is the dispatch shell for similarity scoring. Backends are
// implemented in internal/quality/similarity (Slice 3 adds rouge/bleu;
// Slice 5 adds llm). Until then this stub returns score 0 + a note, but
// keeps the YAML/decode plumbing exercised.
type SimilarityCheck struct {
	Reference string
	Method    string
	Threshold float64
	LLMRunner SimilarityRunner
}

func (SimilarityCheck) Name() string { return "similarity" }

func (s SimilarityCheck) Run(_ context.Context, _ Input) Result {
	return Result{
		Name:   "similarity",
		Score:  0,
		Passed: false,
		Note:   fmt.Sprintf("similarity backend %q not yet implemented", s.Method),
	}
}
