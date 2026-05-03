package adapters

import (
	"os"
	"testing"

	"skillbench/internal/model"
)

func TestParseCodex(t *testing.T) {
	b, _ := os.ReadFile("../../tests/fixtures/codex.jsonl")
	events, err := Parse(model.AgentCodex, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events=%d", len(events))
	}
	if events[1].Type != model.EventToolCall || events[1].Name != "cognee" {
		t.Fatalf("unexpected tool event: %#v", events[1])
	}
}

func TestParseClaudeSkill(t *testing.T) {
	b, _ := os.ReadFile("../../tests/fixtures/claude.jsonl")
	events, err := Parse(model.AgentClaude, b)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, ev := range events {
		if ev.Type == model.EventSkillInvocation {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected skill invocation in %#v", events)
	}
}

func TestParseCopilot(t *testing.T) {
	b, _ := os.ReadFile("../../tests/fixtures/copilot.jsonl")
	events, err := Parse(model.AgentCopilot, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("events=%d", len(events))
	}
	if events[1].Type != model.EventToolCall {
		t.Fatalf("expected tool call, got %#v", events[1])
	}
}
