package judge_test

import (
	"context"
	"path/filepath"
	"testing"

	"skillbench/internal/judge"
	"skillbench/internal/metrics"
	"skillbench/internal/model"
)

// Integration smoke test: load the real bug-triage cases, simulate a passing
// run with all assertions satisfied, stub the judge with canned JSON, and
// verify ScoreWithVerdict's blend/replace behavior end-to-end. No live LLM.
func TestIntegrationBugTriageJudgeBlend(t *testing.T) {
	casesPath := filepath.Join("..", "..", "experiments", "bug-triage", "cases.yaml")
	cases, err := model.LoadCases(casesPath)
	if err != nil {
		t.Fatalf("load cases: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases loaded")
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
	if tc.Rubric["task_success"] == "" || tc.Rubric["skill_adherence"] == "" || tc.Rubric["output_quality"] == "" {
		t.Fatalf("expected all three rubric keys, got %#v", tc.Rubric)
	}

	// Simulate a passing run: the final assistant message contains every
	// required substring, so all six final_contains assertions pass.
	finalMsg := "Priority: P0\nRationale: revenue impact\nImpact: high\nRecommended Action: page the on-call"
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "bug-triage"},
		{Type: model.EventAssistant, Text: finalMsg},
	}

	// Stub judge: canned-JSON returning 90 for every key.
	stub := judge.LLMJudge{
		Agent: model.AgentClaude,
		Run: func(_ context.Context, _ model.Agent, _ string) (string, error) {
			return `{"task_success":90,"skill_adherence":90,"output_quality":90,"notes":["all sections present"]}`, nil
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
		t.Fatalf("judge: %v", err)
	}
	if v.Status != "ok" {
		t.Fatalf("judge status=%q", v.Status)
	}

	m := metrics.ScoreWithVerdict(tc, events, v)

	// All 6 assertions pass -> assertionRatio=1.0 -> 100. Blend with 90 -> 95.
	if m.TaskSuccess != 95 {
		t.Fatalf("TaskSuccess: want 95 (blend), got %.1f", m.TaskSuccess)
	}
	if m.SkillAdherence != 90 {
		t.Fatalf("SkillAdherence: want 90 (judge replace), got %.1f", m.SkillAdherence)
	}
	if m.OutputQuality != 90 {
		t.Fatalf("OutputQuality: want 90 (judge replace), got %.1f", m.OutputQuality)
	}
	if m.JudgeStatus != "ok" {
		t.Fatalf("JudgeStatus=%q", m.JudgeStatus)
	}
	if m.DeterministicFail {
		t.Fatal("unexpected deterministic failure on a passing run")
	}
}

// Integration smoke test: assertion failure must keep the deterministic cap
// reachable even with a high judge score.
func TestIntegrationBugTriageDeterministicCap(t *testing.T) {
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

	// Final message missing several required substrings -> assertions fail.
	finalMsg := "I don't know."
	events := []model.Event{
		{Type: model.EventSkillInvocation, Name: "bug-triage"},
		{Type: model.EventAssistant, Text: finalMsg},
	}

	stub := judge.LLMJudge{
		Agent: model.AgentClaude,
		Run: func(_ context.Context, _ model.Agent, _ string) (string, error) {
			return `{"task_success":95,"skill_adherence":95,"output_quality":95}`, nil
		},
		Fallback: judge.NoopJudge{},
	}
	v, err := stub.Score(context.Background(), judge.Input{
		Case:   tc,
		Skill:  model.SkillInfo{Name: "bug-triage"},
		Events: events,
		Final:  finalMsg,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := metrics.ScoreWithVerdict(tc, events, v)
	if !m.DeterministicFail {
		t.Fatal("expected deterministic failure on missing assertions")
	}
	if m.Overall > 60 {
		t.Fatalf("deterministic cap not applied: overall=%.1f", m.Overall)
	}
}
