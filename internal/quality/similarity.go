package quality

import (
	"context"
	"fmt"
	"strings"

	"skillbench/internal/quality/similarity"
)

// SimilarityCheck scores in.Final against Reference using Method. With a
// non-zero Threshold it gates to binary 100/0 (passed iff score/100 >=
// Threshold); otherwise the raw 0-100 score flows through and Passed = score>0.
type SimilarityCheck struct {
	Reference string
	Method    string
	Threshold float64
	LLMRunner SimilarityRunner
}

func (SimilarityCheck) Name() string { return "similarity" }

func (s SimilarityCheck) Run(ctx context.Context, in Input) Result {
	if s.Reference == "" {
		return Result{Name: "similarity", Score: 0, Passed: false, Note: "reference is empty"}
	}
	method := similarity.Method(s.Method)
	var llm similarity.LLMRunner
	if s.LLMRunner != nil {
		llm = similarity.LLMRunner(s.LLMRunner)
	}
	sc := similarity.Compute(ctx, method, in.Final, s.Reference, llm)
	noteParts := []string{fmt.Sprintf("%s=%.1f", s.Method, sc.Value)}
	noteParts = append(noteParts, sc.Notes...)
	if s.Threshold > 0 {
		if sc.Value/100.0 >= s.Threshold {
			return Result{Name: "similarity", Score: 100, Passed: true, Note: strings.Join(append(noteParts, fmt.Sprintf("threshold=%.2f passed", s.Threshold)), "; ")}
		}
		return Result{Name: "similarity", Score: 0, Passed: false, Note: strings.Join(append(noteParts, fmt.Sprintf("threshold=%.2f failed", s.Threshold)), "; ")}
	}
	passed := sc.Value > 0
	return Result{Name: "similarity", Score: sc.Value, Passed: passed, Note: strings.Join(noteParts, "; ")}
}
