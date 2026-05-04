package judge

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"skillbench/internal/model"
)

func sampleInput() Input {
	return Input{
		Case: model.TestCase{
			ID:     "demo",
			Prompt: "Triage this report.",
			Rubric: map[string]string{
				"task_success":    "Does the answer give priority P0?",
				"skill_adherence": "Does the answer cite concrete facts?",
				"output_quality":  "Does the rationale connect severity to action?",
			},
		},
		Skill:  model.SkillInfo{Name: "demo", Content: "---\nname: demo\n---\nbody"},
		Final:  "P0 priority — outage payments. cited foo, bar, baz.",
		Events: []model.Event{{Type: model.EventAssistant, Text: "P0"}},
	}
}

func TestLLMJudgeParsesValidJSON(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"task_success":88,"skill_adherence":75,"output_quality":90,"notes":["ok"]}`, nil
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: run, Fallback: NoopJudge{}}
	v, err := j.Score(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "ok" {
		t.Fatalf("status=%q", v.Status)
	}
	if v.TaskSuccess == nil || *v.TaskSuccess != 88 {
		t.Fatalf("task_success=%v", v.TaskSuccess)
	}
	if v.SkillAdherence == nil || *v.SkillAdherence != 75 {
		t.Fatalf("skill_adherence=%v", v.SkillAdherence)
	}
	if v.OutputQuality == nil || *v.OutputQuality != 90 {
		t.Fatalf("output_quality=%v", v.OutputQuality)
	}
}

func TestLLMJudgeStripsMarkdownFences(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return "```json\n{\"task_success\":50}\n```", nil
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: run, Fallback: NoopJudge{}}
	v, err := j.Score(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if v.TaskSuccess == nil || *v.TaskSuccess != 50 {
		t.Fatalf("task_success=%v", v.TaskSuccess)
	}
}

func TestLLMJudgePartialKeysLeaveOthersNil(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"output_quality":80,"notes":["only quality"]}`, nil
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: run, Fallback: NoopJudge{}}
	v, err := j.Score(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if v.TaskSuccess != nil || v.SkillAdherence != nil {
		t.Fatalf("expected nil partial fields, got %#v", v)
	}
	if v.OutputQuality == nil || *v.OutputQuality != 80 {
		t.Fatalf("output_quality=%v", v.OutputQuality)
	}
}

func TestLLMJudgeClampsOutOfRange(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"task_success":150,"skill_adherence":-20,"output_quality":50}`, nil
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: run, Fallback: NoopJudge{}}
	v, err := j.Score(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "ok" {
		t.Fatalf("status=%q want ok (clamp not fallback)", v.Status)
	}
	if v.TaskSuccess == nil || *v.TaskSuccess != 100 {
		t.Fatalf("task_success not clamped to 100: %v", v.TaskSuccess)
	}
	if v.SkillAdherence == nil || *v.SkillAdherence != 0 {
		t.Fatalf("skill_adherence not clamped to 0: %v", v.SkillAdherence)
	}
}

func TestLLMJudgeFallsBackOnRunError(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return "", fmt.Errorf("runner unavailable")
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: run, Fallback: NoopJudge{}}
	v, err := j.Score(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.Status, "fallback:") {
		t.Fatalf("status=%q want fallback prefix", v.Status)
	}
	if !strings.Contains(v.Status, "runner unavailable") {
		t.Fatalf("status missing reason: %q", v.Status)
	}
	if v.TaskSuccess != nil || v.SkillAdherence != nil || v.OutputQuality != nil {
		t.Fatalf("fallback should leave pointers nil: %#v", v)
	}
}

func TestLLMJudgeFallsBackOnGarbageJSON(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return "this is not json at all", nil
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: run, Fallback: NoopJudge{}}
	v, err := j.Score(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.Status, "fallback:") {
		t.Fatalf("status=%q want fallback prefix", v.Status)
	}
}

func TestLLMJudgeFallsBackOnEmptyResponse(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return "   \n  ", nil
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: run, Fallback: NoopJudge{}}
	v, err := j.Score(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.Status, "fallback:") {
		t.Fatalf("status=%q want fallback prefix", v.Status)
	}
}

func TestLLMJudgeFallsBackOnEmptyObject(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"notes":["nothing scored"]}`, nil
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: run, Fallback: NoopJudge{}}
	v, err := j.Score(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.Status, "fallback:") {
		t.Fatalf("status=%q want fallback prefix", v.Status)
	}
}

func TestParseVerdictJSONExtractsFromProse(t *testing.T) {
	text := "Here you go:\n\n{\"task_success\":80}\n\nThanks."
	v, err := parseVerdictJSON(text)
	if err != nil {
		t.Fatal(err)
	}
	if v.TaskSuccess == nil || *v.TaskSuccess != 80 {
		t.Fatalf("got %#v", v)
	}
}

func TestBuildPromptOmitsEmptyRubricKeys(t *testing.T) {
	in := sampleInput()
	in.Case.Rubric = map[string]string{
		"task_success":    "ask",
		"skill_adherence": "",
		// output_quality missing entirely
	}
	j := LLMJudge{Agent: model.AgentClaude, Run: func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"task_success":50}`, nil
	}, Fallback: NoopJudge{}}
	prompt, err := j.buildPrompt(in)
	if err != nil {
		t.Fatal(err)
	}
	// The rubric questions block is rendered as JSON between fenced blocks;
	// scope our assertions to that block, since the system prompt itself
	// names all three keys.
	const start = "## Rubric questions to answer"
	const end = "## Skill SKILL.md"
	si := strings.Index(prompt, start)
	ei := strings.Index(prompt, end)
	if si < 0 || ei < 0 || ei <= si {
		t.Fatalf("rubric block not found in prompt")
	}
	rubricBlock := prompt[si:ei]
	if !strings.Contains(rubricBlock, "task_success") {
		t.Fatal("rubric block missing task_success")
	}
	if strings.Contains(rubricBlock, "skill_adherence") {
		t.Fatal("rubric block should omit empty skill_adherence")
	}
	if strings.Contains(rubricBlock, "output_quality") {
		t.Fatal("rubric block should omit missing output_quality")
	}
}
