package quality_test

import (
	"context"
	"path/filepath"
	"testing"

	"skillbench/internal/judge"
	"skillbench/internal/metrics"
	"skillbench/internal/model"
	"skillbench/internal/quality"
)

// Integration smoke test: load the real bug-triage cases (now with
// quality_checks on outage-payments), simulate a passing run, run the
// quality channel + a stubbed judge, and verify the blended OutputQuality
// + cap behavior end-to-end. No live LLM.
func TestIntegrationBugTriageQualityBlend(t *testing.T) {
	casesPath := filepath.Join("..", "..", "experiments", "bug-triage", "cases.yaml")
	cases, err := model.LoadCases(casesPath)
	if err != nil {
		t.Fatalf("load cases: %v", err)
	}
	var tc model.TestCase
	for _, c := range cases {
		if c.ID == "outage-payments" {
			tc = c
			break
		}
	}
	if tc.ID == "" {
		t.Fatal("outage-payments case not found")
	}
	if len(tc.QualityChecks) == 0 {
		t.Fatalf("expected quality_checks on outage-payments; got %#v", tc.QualityChecks)
	}

	finalMsg := "Priority: P0\nRationale: revenue impact during the payment outage\nImpact: high\nRecommended Action: page the on-call engineer"
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "bug-triage"},
		{Type: model.EventAssistant, Text: finalMsg},
	}

	// Build the quality runner with default options (rouge similarity; no LLM).
	qr, err := quality.Build(tc, quality.BuildOptions{})
	if err != nil {
		t.Fatalf("quality.Build: %v", err)
	}
	if qr == nil {
		t.Fatal("expected non-nil quality runner")
	}
	sig := qr.Run(context.Background(), quality.Input{Case: tc, Final: finalMsg})
	if sig == nil {
		t.Fatal("expected QualitySignal")
	}
	// regex P0 -> 100; rouge similarity threshold 0.4 -> binary 100 (final
	// substantially overlaps the reference). Mean should be 100.
	if sig.Score != 100 {
		t.Fatalf("expected quality score=100 (both checks pass), got %.1f; notes=%v", sig.Score, sig.Notes)
	}

	// Stubbed judge for the OutputQuality blend assertion.
	stub := judge.LLMJudge{
		Agent: model.AgentClaude,
		Run: func(_ context.Context, _ model.Agent, _ string) (string, error) {
			return `{"task_success":90,"skill_adherence":90,"output_quality":80}`, nil
		},
		Fallback: judge.NoopJudge{},
	}
	v, err := stub.Score(context.Background(), judge.Input{
		Case:   tc,
		Skill:  model.SkillInfo{Name: "bug-triage", Content: "---\nname: bug-triage\n---\nbody"},
		Events: events,
		Final:  finalMsg,
	})
	if err != nil {
		t.Fatal(err)
	}

	qSig := &metrics.QualitySignal{Score: sig.Score, Notes: sig.Notes}
	m := metrics.ScoreWithVerdictAndQuality(tc, events, v, qSig)

	// Blend: 0.5 * 100 (det) + 0.5 * 80 (judge) = 90.
	if m.OutputQuality != 90 {
		t.Fatalf("OutputQuality: want 90 (blend), got %.1f", m.OutputQuality)
	}
	if m.DeterministicFail {
		t.Fatal("unexpected deterministic failure on a passing run")
	}
	// Quality notes propagated.
	var sawQuality bool
	for _, n := range m.Notes {
		if len(n) >= 9 && n[:9] == "quality: " {
			sawQuality = true
			break
		}
	}
	if !sawQuality {
		t.Fatalf("expected quality: notes propagated, got %v", m.Notes)
	}
}

// DeterministicFail cap still bites even when quality_checks score 100.
func TestIntegrationBugTriageQualityDoesNotMaskCap(t *testing.T) {
	casesPath := filepath.Join("..", "..", "experiments", "bug-triage", "cases.yaml")
	cases, err := model.LoadCases(casesPath)
	if err != nil {
		t.Fatalf("load cases: %v", err)
	}
	var tc model.TestCase
	for _, c := range cases {
		if c.ID == "outage-payments" {
			tc = c
			break
		}
	}

	// Final message missing required substrings -> assertion failure ->
	// DeterministicFail. But quality regex+similarity might match enough.
	finalMsg := "P0 revenue impact during the payment outage"
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "bug-triage"},
		{Type: model.EventAssistant, Text: finalMsg},
	}

	qr, err := quality.Build(tc, quality.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sig := qr.Run(context.Background(), quality.Input{Case: tc, Final: finalMsg})

	qSig := &metrics.QualitySignal{Score: sig.Score, Notes: sig.Notes}
	m := metrics.ScoreWithVerdictAndQuality(tc, events, judge.Verdict{}, qSig)

	if !m.DeterministicFail {
		t.Fatalf("expected deterministic failure; m=%+v", m)
	}
	if m.Overall > 60 {
		t.Fatalf("DeterministicFail cap not applied: overall=%.1f", m.Overall)
	}
}
