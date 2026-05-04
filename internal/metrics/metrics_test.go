package metrics

import (
	"reflect"
	"testing"

	"skillbench/internal/judge"
	"skillbench/internal/model"
)

func ptr(v float64) *float64 { return &v }

func TestScoreCapsDeterministicFailure(t *testing.T) {
	tc := model.TestCase{
		ExpectedSkill: "cognee-memory",
		Assertions: model.Assertions{
			FinalContains: []string{"missing"},
		},
	}
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "cognee-memory"},
		{Type: model.EventAssistant, Text: "completed something useful with a reasonably long final message"},
	}
	m := Score(tc, events)
	if !m.DeterministicFail {
		t.Fatal("expected deterministic failure")
	}
	if m.Overall > 60 {
		t.Fatalf("cap not applied: %.1f", m.Overall)
	}
}

func TestScoreSuccess(t *testing.T) {
	tc := model.TestCase{
		ExpectedSkill: "cognee-memory",
		Assertions: model.Assertions{
			FinalContains: []string{"dataset"},
			ToolCalled:    []string{"cognee"},
		},
	}
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "cognee-memory"},
		{Type: model.EventToolCall, Name: "cognee"},
		{Type: model.EventToolResult, Text: "ok"},
		{Type: model.EventAssistant, Text: "The dataset ingest completed successfully with traceable output."},
	}
	m := Score(tc, events)
	if m.Overall < 80 {
		t.Fatalf("score too low: %.1f", m.Overall)
	}
}

// Regression: deterministic-fail cap must hold even when the judge says 95.
func TestScoreWithVerdictDeterministicCapStillApplies(t *testing.T) {
	tc := model.TestCase{
		ExpectedSkill: "cognee-memory",
		Assertions: model.Assertions{
			FinalContains: []string{"missing-token-that-wont-match"},
		},
		Rubric: map[string]string{
			"task_success":    "Did the agent succeed?",
			"skill_adherence": "Was the skill followed?",
			"output_quality":  "Was the answer clear?",
		},
	}
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "cognee-memory"},
		{Type: model.EventAssistant, Text: "completed something useful with a reasonably long final message"},
	}
	v := judge.Verdict{
		TaskSuccess:    ptr(95),
		SkillAdherence: ptr(95),
		OutputQuality:  ptr(95),
		Status:         "ok",
	}
	m := ScoreWithVerdict(tc, events, v)
	if !m.DeterministicFail {
		t.Fatal("expected deterministic failure")
	}
	if m.Overall > 60 {
		t.Fatalf("deterministic cap not applied: overall=%.1f", m.Overall)
	}
}

// Regression: with no verdict, ScoreWithVerdict must equal Score bit-for-bit
// for an existing case, and Score must remain bit-identical to its prior
// behavior (assertions, notes, judge_status absent).
func TestScoreWithEmptyVerdictBitIdentical(t *testing.T) {
	tc := model.TestCase{
		ExpectedSkill: "cognee-memory",
		Assertions: model.Assertions{
			FinalContains: []string{"dataset"},
			ToolCalled:    []string{"cognee"},
		},
	}
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "cognee-memory"},
		{Type: model.EventToolCall, Name: "cognee"},
		{Type: model.EventToolResult, Text: "ok"},
		{Type: model.EventAssistant, Text: "The dataset ingest completed successfully with traceable output."},
	}
	a := Score(tc, events)
	b := ScoreWithVerdict(tc, events, judge.Verdict{})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Score and ScoreWithVerdict(empty) diverged:\n a=%#v\n b=%#v", a, b)
	}
	if a.JudgeStatus != "" {
		t.Fatalf("JudgeStatus should be empty for default path: %q", a.JudgeStatus)
	}
}

// When a judge supplies a higher TaskSuccess and assertions all pass, the
// task_success metric is the 50/50 blend.
func TestScoreWithVerdictBlendsTaskSuccess(t *testing.T) {
	tc := model.TestCase{
		Assertions: model.Assertions{FinalContains: []string{"foo"}},
		Rubric:     map[string]string{"task_success": "Did it work?"},
	}
	events := []model.Event{{Type: model.EventAssistant, Text: "foo here is the answer with enough length"}}
	v := judge.Verdict{TaskSuccess: ptr(60), Status: "ok"}
	m := ScoreWithVerdict(tc, events, v)
	// All 1 assertion passes -> assertionRatio=1.0 -> 100. Blend with 60 -> 80.
	if m.TaskSuccess != 80 {
		t.Fatalf("expected blended TaskSuccess=80, got %.1f", m.TaskSuccess)
	}
}

// When no assertions exist but a rubric task_success is authored and the
// judge scores it, the metric is judge-only.
func TestScoreWithVerdictJudgeOnlyWhenNoAssertions(t *testing.T) {
	tc := model.TestCase{
		Rubric: map[string]string{"task_success": "Did it work?"},
	}
	events := []model.Event{{Type: model.EventAssistant, Text: "ok"}}
	v := judge.Verdict{TaskSuccess: ptr(42), Status: "ok"}
	m := ScoreWithVerdict(tc, events, v)
	if m.TaskSuccess != 42 {
		t.Fatalf("expected judge-only TaskSuccess=42, got %.1f", m.TaskSuccess)
	}
}

// Verdict without a corresponding rubric question must NOT override TaskSuccess.
func TestScoreWithVerdictTaskSuccessIgnoredWithoutRubric(t *testing.T) {
	tc := model.TestCase{
		Assertions: model.Assertions{FinalContains: []string{"foo"}},
		// no rubric.task_success
	}
	events := []model.Event{{Type: model.EventAssistant, Text: "foo here is the answer with enough length"}}
	v := judge.Verdict{TaskSuccess: ptr(0), Status: "ok"}
	m := ScoreWithVerdict(tc, events, v)
	if m.TaskSuccess != 100 {
		t.Fatalf("rubric-less judge value should be ignored, got %.1f", m.TaskSuccess)
	}
}

// JudgeStatus is propagated and notes are prefixed.
func TestScoreWithVerdictPropagatesStatusAndNotes(t *testing.T) {
	tc := model.TestCase{}
	events := []model.Event{{Type: model.EventAssistant, Text: "answer"}}
	v := judge.Verdict{Status: "fallback:run: x", Notes: []string{"reason"}}
	m := ScoreWithVerdict(tc, events, v)
	if m.JudgeStatus != "fallback:run: x" {
		t.Fatalf("JudgeStatus=%q", m.JudgeStatus)
	}
	found := false
	for _, n := range m.Notes {
		if n == "judge: reason" {
			found = true
		}
	}
	if !found {
		t.Fatalf("judge note not propagated: %v", m.Notes)
	}
}
