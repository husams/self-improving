// Package judge provides an LLM-backed rubric scorer for skillbench cases.
//
// A Judge inspects a finished case run (test case, skill content, event trace,
// final assistant message) and returns sub-metric scores aligned with the
// rubric keys authored on the test case (task_success, skill_adherence,
// output_quality). The metrics package consumes these via ScoreWithVerdict
// to blend or replace heuristic values without changing weights or caps.
package judge

import (
	"context"

	"skillbench/internal/model"
)

// Judge produces a Verdict for a single case run.
type Judge interface {
	Score(ctx context.Context, in Input) (Verdict, error)
}

// Input is the data a Judge needs to score a run.
type Input struct {
	Case   model.TestCase
	Skill  model.SkillInfo
	Events []model.Event
	Final  string
}

// Verdict is a per-sub-metric judgement. Nil pointer fields mean "not judged"
// and the heuristic value is preserved by the metrics layer.
type Verdict struct {
	TaskSuccess    *float64
	SkillAdherence *float64
	OutputQuality  *float64
	Notes          []string
	// Status is one of:
	//   "ok"               — judge ran and produced a verdict
	//   "skipped"          — judge intentionally did not run
	//   "fallback:<reason>"— judge failed and fell back
	Status string
}

// Float is a helper for constructing pointer-valued verdict fields in tests
// and callers.
func Float(v float64) *float64 { return &v }
