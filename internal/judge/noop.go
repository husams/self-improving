package judge

import "context"

// NoopJudge returns an empty Verdict with Status "skipped". It is the default
// Judge when no LLM judge is configured: the metrics layer sees nil pointer
// fields and falls through to the existing heuristic scoring untouched.
type NoopJudge struct{}

func (NoopJudge) Score(_ context.Context, _ Input) (Verdict, error) {
	return Verdict{Status: "skipped"}, nil
}
