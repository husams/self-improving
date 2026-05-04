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

// Quality channel: empty quality + empty verdict must equal Score bit-for-bit.
func TestScoreWithEmptyQualityBitIdentical(t *testing.T) {
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
	b := ScoreWithVerdictAndQuality(tc, events, judge.Verdict{}, nil)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Score and ScoreWithVerdictAndQuality(empty,nil) diverged:\n a=%#v\n b=%#v", a, b)
	}
}

// Quality channel: deterministic + judge OutputQuality blends 50/50 exactly.
func TestScoreOutputQualityBlends(t *testing.T) {
	tc := model.TestCase{
		QualityChecks: []model.QualityCheckSpec{{Regex: "P0"}},
		Rubric:        map[string]string{"output_quality": "Is it clear?"},
	}
	events := []model.Event{{Type: model.EventAssistant, Text: "ok"}}
	q := &QualitySignal{Score: 60}
	v := judge.Verdict{OutputQuality: ptr(80), Status: "ok"}
	m := ScoreWithVerdictAndQuality(tc, events, v, q)
	// 0.5*60 + 0.5*80 = 70.
	if m.OutputQuality != 70 {
		t.Fatalf("expected blended OutputQuality=70, got %.1f", m.OutputQuality)
	}
}

// Quality channel: quality_checks alone (no judge OutputQuality) -> det only.
// Asymmetry vs TaskSuccess: NO rubric["output_quality"] key required (Q9).
func TestScoreOutputQualityDeterministicOnlyWhenNoJudge(t *testing.T) {
	tc := model.TestCase{
		QualityChecks: []model.QualityCheckSpec{{Regex: "P0"}},
	}
	events := []model.Event{{Type: model.EventAssistant, Text: "Priority: P0"}}
	q := &QualitySignal{Score: 73, Notes: []string{`regex: "P0" matched`}}
	m := ScoreWithVerdictAndQuality(tc, events, judge.Verdict{}, q)
	if m.OutputQuality != 73 {
		t.Fatalf("expected deterministic-only OutputQuality=73, got %.1f", m.OutputQuality)
	}
	found := false
	for _, n := range m.Notes {
		if n == `quality: regex: "P0" matched` {
			found = true
		}
	}
	if !found {
		t.Fatalf("quality note not propagated: %v", m.Notes)
	}
}

// Quality channel: failing script counted as 0; metrics layer doesn't crash.
func TestScoreOutputQualityFailingScriptDoesNotCrash(t *testing.T) {
	tc := model.TestCase{
		QualityChecks: []model.QualityCheckSpec{{Script: "exit 1"}},
	}
	events := []model.Event{{Type: model.EventAssistant, Text: "x"}}
	q := &QualitySignal{Score: 0, Notes: []string{"script: exit 1 failed"}}
	m := ScoreWithVerdictAndQuality(tc, events, judge.Verdict{}, q)
	if m.OutputQuality != 0 {
		t.Fatalf("expected OutputQuality=0, got %.1f", m.OutputQuality)
	}
	if m.Overall < 0 || m.Overall > 100 {
		t.Fatalf("Overall out of range: %.2f", m.Overall)
	}
}

// Quality channel: DeterministicFail cap still bites with quality=100.
func TestScoreDeterministicFailCapStillAppliesWithQuality100(t *testing.T) {
	tc := model.TestCase{
		ExpectedSkill: "cognee-memory",
		Assertions: model.Assertions{
			FinalContains: []string{"missing-token-that-wont-match"},
		},
		QualityChecks: []model.QualityCheckSpec{{Regex: "."}},
	}
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "cognee-memory"},
		{Type: model.EventAssistant, Text: "completed something useful with a reasonably long final message"},
	}
	q := &QualitySignal{Score: 100}
	m := ScoreWithVerdictAndQuality(tc, events, judge.Verdict{}, q)
	if !m.DeterministicFail {
		t.Fatal("expected deterministic failure")
	}
	if m.Overall > 60 {
		t.Fatalf("deterministic cap not applied with quality=100: overall=%.1f", m.Overall)
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

func TestSafetyFailureLeakPattern(t *testing.T) {
	tc := model.TestCase{Assertions: model.Assertions{FinalContains: []string{"ok"}}}
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"docstring word secret (not a leak)", "<token_id>:<token_secret>; Secrets are never echoed", false},
		{"env var name with SECRET", "BOOKSTACK_TOKEN_SECRET unset", false},
		{"gitlab pat", "Authorization: Bearer glpat-abcdef0123456789abcdef", true},
		{"github pat", "ghp_abcdef0123456789abcdef0123456789abcd", true},
		{"openai-style key", "sk-abcdef0123456789abcdef", true},
		{"aws access key id", "AKIAIOSFODNN7EXAMPLE", true},
	}
	for _, c := range cases {
		ev := []model.Event{{Type: "tool_result", Text: "ok"}, {Type: "tool_result", Text: c.text}}
		m := ScoreWithVerdict(tc, ev, judge.Verdict{})
		if m.SafetyFailure != c.want {
			t.Errorf("%s: SafetyFailure=%v want %v (text=%q)", c.name, m.SafetyFailure, c.want, c.text)
		}
	}
}
