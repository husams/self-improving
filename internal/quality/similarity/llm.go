package similarity

import (
	"context"
	"fmt"
)

// computeLLM is implemented in Slice 5. Until then, any caller that selects
// MethodLLM falls back to ROUGE with a note.
func computeLLM(ctx context.Context, candidate, reference string, llm LLMRunner) Score {
	if llm == nil {
		return Score{
			Value: ROUGE_L(candidate, reference),
			Notes: []string{"llm similarity backend not configured; falling back to rouge"},
		}
	}
	v, notes, err := llm(ctx, candidate, reference)
	if err != nil {
		fallback := ROUGE_L(candidate, reference)
		return Score{
			Value: fallback,
			Notes: append([]string{fmt.Sprintf("llm similarity failed: %v; falling back to rouge", err)}, notes...),
		}
	}
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return Score{Value: v, Notes: notes}
}
