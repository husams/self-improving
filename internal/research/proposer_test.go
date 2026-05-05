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

func TestLLMProposerInjectsFailedAssertionsWhenOptedIn(t *testing.T) {
	var captured string
	run := func(_ context.Context, _ model.Agent, prompt string) (string, error) {
		captured = prompt
		return `{"strategy":"x","hypothesis":"y","files":{"SKILL.md":"z"}}`, nil
	}
	p := LLMProposer{Agent: model.AgentClaude, Run: run, Fallback: DeterministicProposer{}}
	tc := model.TestCase{ID: "case-1"}
	tc.ProposerContext.IncludeFailedAssertions = true
	baseline := model.NormalizedRun{
		Metrics: model.Metrics{
			Assertions: []model.AssertionResult{
				{Name: "final_contains:PNG Path", Passed: false, Message: "missing in output"},
				{Name: "final_contains:Theme", Passed: true},
				{Name: "tool_called:Bash", Passed: false},
			},
		},
	}
	if _, err := p.Propose(context.Background(), ProposerInput{
		SkillContent: "---\n",
		Case:         tc,
		Baseline:     baseline,
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(captured, "## Failed assertions on baseline") {
		t.Fatal("missing failed-assertions header")
	}
	if !strings.Contains(captured, "final_contains:PNG Path") || !strings.Contains(captured, "missing in output") {
		t.Fatalf("missing failed assertion detail in prompt:\n%s", captured)
	}
	if !strings.Contains(captured, "tool_called:Bash") {
		t.Fatal("missing second failed assertion")
	}
	if strings.Contains(captured, "final_contains:Theme") && strings.Contains(captured[strings.Index(captured, "## Failed assertions"):], "final_contains:Theme") {
		t.Fatal("passed assertion leaked into failed-assertions section")
	}
}

func TestLLMProposerOmitsFailedAssertionsByDefault(t *testing.T) {
	var captured string
	run := func(_ context.Context, _ model.Agent, prompt string) (string, error) {
		captured = prompt
		return `{"strategy":"x","hypothesis":"y","files":{"SKILL.md":"z"}}`, nil
	}
	p := LLMProposer{Agent: model.AgentClaude, Run: run, Fallback: DeterministicProposer{}}
	baseline := model.NormalizedRun{
		Metrics: model.Metrics{
			Assertions: []model.AssertionResult{{Name: "final_contains:X", Passed: false}},
		},
	}
	if _, err := p.Propose(context.Background(), ProposerInput{
		SkillContent: "---\n",
		Case:         model.TestCase{ID: "case-1"}, // ProposerContext zero value
		Baseline:     baseline,
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(captured, "Failed assertions on baseline") {
		t.Fatal("failed-assertions section appeared without opt-in")
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
