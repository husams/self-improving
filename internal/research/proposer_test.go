package research

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"skillbench/internal/model"
)

func TestDeterministicProposerReturnsCandidate(t *testing.T) {
	tc := model.TestCase{ID: "demo", Prompt: "Run the demo", ExpectedSkill: "demo-skill"}
	baseline := model.NormalizedRun{
		Metrics: model.Metrics{Overall: 50, SkillUse: 0},
	}
	cand, err := DeterministicProposer{}.Propose(context.Background(), ProposerInput{
		SkillContent: "---\nname: demo\n---\n# demo\n",
		Case:         tc,
		Baseline:     baseline,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cand.Content == "" {
		t.Fatal("empty content")
	}
	if cand.Strategy == "" {
		t.Fatal("empty strategy")
	}
}

func TestLLMProposerParsesValidJSON(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"strategy":"trigger-broaden","hypothesis":"broaden trigger","skill_md":"---\nname: demo\n---\nbody"}`, nil
	}
	p := LLMProposer{Agent: model.AgentClaude, Run: run, Fallback: DeterministicProposer{}}
	cand, err := p.Propose(context.Background(), ProposerInput{
		SkillContent: "---\nname: demo\n---\n",
		Case:         model.TestCase{ID: "x"},
		Baseline:     model.NormalizedRun{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cand.Strategy != "trigger-broaden" {
		t.Fatalf("strategy=%q", cand.Strategy)
	}
	if !strings.Contains(cand.Content, "body") {
		t.Fatalf("content=%q", cand.Content)
	}
}

func TestLLMProposerStripsMarkdownFences(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return "```json\n{\"strategy\":\"x\",\"hypothesis\":\"y\",\"skill_md\":\"z\"}\n```", nil
	}
	p := LLMProposer{Agent: model.AgentClaude, Run: run, Fallback: DeterministicProposer{}}
	cand, err := p.Propose(context.Background(), ProposerInput{
		SkillContent: "---\n",
		Case:         model.TestCase{ID: "x"},
		Baseline:     model.NormalizedRun{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cand.Strategy != "x" {
		t.Fatalf("strategy=%q", cand.Strategy)
	}
	if cand.Content != "z" {
		t.Fatalf("content=%q", cand.Content)
	}
}

func TestLLMProposerFallsBackOnRunError(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return "", fmt.Errorf("runner unavailable")
	}
	p := LLMProposer{Agent: model.AgentClaude, Run: run, Fallback: DeterministicProposer{}}
	cand, err := p.Propose(context.Background(), ProposerInput{
		SkillContent: "---\nname: demo\n---\n",
		Case:         model.TestCase{ID: "x"},
		Baseline:     model.NormalizedRun{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cand.Strategy, "llm-fallback:") {
		t.Fatalf("strategy=%q want llm-fallback prefix", cand.Strategy)
	}
	if cand.Content == "" {
		t.Fatal("empty content from fallback")
	}
	if len(cand.Notes) == 0 || !strings.Contains(cand.Notes[0], "runner unavailable") {
		t.Fatalf("notes=%v", cand.Notes)
	}
}

func TestLLMProposerFallsBackOnGarbageJSON(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return "this is not json at all", nil
	}
	p := LLMProposer{Agent: model.AgentClaude, Run: run, Fallback: DeterministicProposer{}}
	cand, err := p.Propose(context.Background(), ProposerInput{
		SkillContent: "---\nname: demo\n---\n",
		Case:         model.TestCase{ID: "x"},
		Baseline:     model.NormalizedRun{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cand.Strategy, "llm-fallback:") {
		t.Fatalf("strategy=%q want llm-fallback prefix", cand.Strategy)
	}
}

func TestLLMProposerFallsBackOnEmptySkillMD(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"strategy":"x","hypothesis":"y","skill_md":""}`, nil
	}
	p := LLMProposer{Agent: model.AgentClaude, Run: run, Fallback: DeterministicProposer{}}
	cand, err := p.Propose(context.Background(), ProposerInput{
		SkillContent: "---\nname: demo\n---\n",
		Case:         model.TestCase{ID: "x"},
		Baseline:     model.NormalizedRun{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cand.Strategy, "llm-fallback:") {
		t.Fatalf("strategy=%q want llm-fallback prefix", cand.Strategy)
	}
}

func TestParseProposerJSONExtractsJSONFromProse(t *testing.T) {
	text := `Here you go:

{"strategy":"a","hypothesis":"b","skill_md":"c"}

Hope that helps.`
	cand, err := parseProposerJSON(text)
	if err != nil {
		t.Fatal(err)
	}
	if cand.Strategy != "a" || cand.Content != "c" {
		t.Fatalf("got %#v", cand)
	}
}
