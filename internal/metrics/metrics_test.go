package metrics

import (
	"testing"

	"skillbench/internal/model"
)

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
