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
	if len(cand.Files) == 0 {
		t.Fatal("empty files")
	}
	if cand.Files["SKILL.md"] == "" {
		t.Fatal("empty SKILL.md content")
	}
	if cand.Strategy == "" {
		t.Fatal("empty strategy")
	}
}

func TestLLMProposerParsesValidJSON(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"strategy":"trigger-broaden","hypothesis":"broaden trigger","files":{"SKILL.md":"---\nname: demo\n---\nbody","scripts/foo.py":"print('hi')"}}`, nil
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
	if !strings.Contains(cand.Files["SKILL.md"], "body") {
		t.Fatalf("SKILL.md=%q", cand.Files["SKILL.md"])
	}
	if cand.Files["scripts/foo.py"] != "print('hi')" {
		t.Fatalf("scripts/foo.py=%q", cand.Files["scripts/foo.py"])
	}
}

func TestLLMProposerStripsMarkdownFences(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return "```json\n{\"strategy\":\"x\",\"hypothesis\":\"y\",\"files\":{\"SKILL.md\":\"z\"}}\n```", nil
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
	if cand.Files["SKILL.md"] != "z" {
		t.Fatalf("SKILL.md=%q", cand.Files["SKILL.md"])
	}
}

func TestLLMProposerAcceptsLegacySkillMDField(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"strategy":"x","hypothesis":"y","skill_md":"legacy body"}`, nil
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
	if cand.Files["SKILL.md"] != "legacy body" {
		t.Fatalf("SKILL.md=%q", cand.Files["SKILL.md"])
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
	if len(cand.Files) == 0 {
		t.Fatal("empty files from fallback")
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

func TestLLMProposerFallsBackOnEmptyFiles(t *testing.T) {
	run := func(_ context.Context, _ model.Agent, _ string) (string, error) {
		return `{"strategy":"x","hypothesis":"y","files":{}}`, nil
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

{"strategy":"a","hypothesis":"b","files":{"SKILL.md":"c"}}

Hope that helps.`
	cand, err := parseProposerJSON(text)
	if err != nil {
		t.Fatal(err)
	}
	if cand.Strategy != "a" || cand.Files["SKILL.md"] != "c" {
		t.Fatalf("got %#v", cand)
	}
}
