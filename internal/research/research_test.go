package research

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillbench/internal/metrics"
	"skillbench/internal/model"
	"skillbench/internal/skills"
)

// fixedProposer always returns the configured Files map.
type fixedProposer struct {
	files    map[string]string
	strategy string
}

func (p fixedProposer) Propose(_ context.Context, _ ProposerInput) (Candidate, error) {
	// Return a copy so the caller can't mutate our state.
	out := make(map[string]string, len(p.files))
	for k, v := range p.files {
		out[k] = v
	}
	return Candidate{Files: out, Strategy: p.strategy, Hypothesis: "test"}, nil
}

// scriptedProposer returns a different file map for each successive trial.
type scriptedProposer struct {
	steps []map[string]string
	calls int
}

func (p *scriptedProposer) Propose(_ context.Context, _ ProposerInput) (Candidate, error) {
	if p.calls >= len(p.steps) {
		return Candidate{Files: map[string]string{}, Strategy: "noop"}, nil
	}
	files := p.steps[p.calls]
	p.calls++
	return Candidate{Files: files, Strategy: "scripted", Hypothesis: "test"}, nil
}

// goodSkillExec scores higher when SKILL.md contains "good", lower otherwise.
// This lets tests deterministically drive accept vs. discard.
func goodSkillExec(ctx context.Context, agent model.Agent, skillPath string, tc model.TestCase) (model.NormalizedRun, error) {
	info, err := skills.Load(skillPath)
	if err != nil {
		return model.NormalizedRun{}, err
	}
	events := []model.Event{{Type: model.EventAssistant, Text: "ok response with enough length to score"}}
	if strings.Contains(info.Content, "good") {
		events = []model.Event{
			{Type: model.EventSkillInvocation, Name: "Skill", Text: "demo-skill"},
			{Type: model.EventAssistant, Text: "The dataset workflow completed successfully with the expected outcome."},
		}
	}
	run := model.NormalizedRun{
		RunID:  model.NewRunID(agent, tc.ID),
		Agent:  agent,
		Case:   tc,
		Skill:  info,
		Events: events,
	}
	run.Metrics = metrics.Score(tc, events)
	return run, nil
}

func setupSkillDir(t *testing.T) (workdir, skillDir, original string) {
	t.Helper()
	oldWd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })

	skillDir = filepath.Join(tmp, "demo-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	original = "---\nname: demo-skill\ndescription: Demo skill.\n---\n\n# Demo\n\nDo the task.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "helper.py"), []byte("# original helper\nprint('orig')\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return tmp, skillDir, original
}

func TestRunCreatesBaselineBackup(t *testing.T) {
	_, skillDir, original := setupSkillDir(t)
	tc := model.TestCase{ID: "demo", Prompt: "run", ExpectedSkill: "demo-skill", Assertions: model.Assertions{FinalContains: []string{"dataset"}}}
	prop := fixedProposer{files: map[string]string{"SKILL.md": "---\nname: demo-skill\n---\n\n# good\n"}, strategy: "make-good"}
	res, err := Run(context.Background(), Config{
		Agent:            model.AgentCodex,
		SkillPath:        skillDir,
		Cases:            []model.TestCase{tc},
		MaxTrials:        1,
		Executor:         goodSkillExec,
		Proposer:         prop,
		SkipDeployPrompt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	bSkillMD := filepath.Join(res.BaselineDir, "SKILL.md")
	b, err := os.ReadFile(bSkillMD)
	if err != nil {
		t.Fatalf("baseline SKILL.md missing: %v", err)
	}
	if string(b) != original {
		t.Fatal("baseline backup does not match original")
	}
	if _, err := os.Stat(filepath.Join(res.BaselineDir, "scripts", "helper.py")); err != nil {
		t.Fatalf("baseline scripts/helper.py missing: %v", err)
	}
}

func TestRunSnapshotsFullTreePerTrial(t *testing.T) {
	_, skillDir, _ := setupSkillDir(t)
	tc := model.TestCase{ID: "demo", Prompt: "run", ExpectedSkill: "demo-skill", Assertions: model.Assertions{FinalContains: []string{"dataset"}}}
	// Trial 1 mutates SKILL.md only; trial 2 mutates scripts/helper.py.
	prop := &scriptedProposer{steps: []map[string]string{
		{"SKILL.md": "---\nname: demo-skill\n---\n\n# good v1\n"},
		{"scripts/helper.py": "# v2 helper\nprint('v2')\n"},
	}}
	res, err := Run(context.Background(), Config{
		Agent: model.AgentCodex, SkillPath: skillDir, Cases: []model.TestCase{tc},
		MaxTrials: 2, Executor: goodSkillExec, Proposer: prop, SkipDeployPrompt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"trial-001", "trial-002"} {
		snap := filepath.Join(res.ResearchDir, "trials", name, "skill")
		if _, err := os.Stat(filepath.Join(snap, "SKILL.md")); err != nil {
			t.Fatalf("%s/SKILL.md missing: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(snap, "scripts", "helper.py")); err != nil {
			t.Fatalf("%s/scripts/helper.py missing: %v", name, err)
		}
	}
}

func TestRunCumulativeStateAcrossTrials(t *testing.T) {
	_, skillDir, _ := setupSkillDir(t)
	tc := model.TestCase{ID: "demo", Prompt: "run", ExpectedSkill: "demo-skill", Assertions: model.Assertions{FinalContains: []string{"dataset"}}}
	prop := &scriptedProposer{steps: []map[string]string{
		// Trial 1: improve SKILL.md (will be accepted because score jumps).
		{"SKILL.md": "---\nname: demo-skill\n---\n\n# good v1\n"},
		// Trial 2: only modify a script, keep the SKILL.md from trial 1.
		// If state is cumulative, the live skill dir going into trial 2
		// already has the v1 SKILL.md.
		{"scripts/helper.py": "# v2 helper\nprint('v2')\n"},
	}}
	res, err := Run(context.Background(), Config{
		Agent: model.AgentCodex, SkillPath: skillDir, Cases: []model.TestCase{tc},
		MaxTrials: 2, Executor: goodSkillExec, Proposer: prop, SkipDeployPrompt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// trial-002 snapshot must include both the v1 SKILL.md and the v2 helper.
	snap := filepath.Join(res.ResearchDir, "trials", "trial-002", "skill")
	md, _ := os.ReadFile(filepath.Join(snap, "SKILL.md"))
	if !strings.Contains(string(md), "good v1") {
		t.Fatalf("trial-002 SKILL.md should carry trial-001 edits, got: %q", md)
	}
	helper, _ := os.ReadFile(filepath.Join(snap, "scripts", "helper.py"))
	if !strings.Contains(string(helper), "v2") {
		t.Fatalf("trial-002 helper.py should reflect trial-002 edits, got: %q", helper)
	}
}

func TestDeployBaselineRestoresOriginal(t *testing.T) {
	_, skillDir, original := setupSkillDir(t)
	tc := model.TestCase{ID: "demo", Prompt: "run", ExpectedSkill: "demo-skill", Assertions: model.Assertions{FinalContains: []string{"dataset"}}}
	prop := fixedProposer{files: map[string]string{"SKILL.md": "---\nname: demo-skill\n---\n\n# good v1 mutated\n"}, strategy: "mutate"}
	res, err := Run(context.Background(), Config{
		Agent: model.AgentCodex, SkillPath: skillDir, Cases: []model.TestCase{tc},
		MaxTrials: 1, Executor: goodSkillExec, Proposer: prop, SkipDeployPrompt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// After Run, live skill dir is mutated.
	live, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if string(live) == original {
		t.Fatal("expected live skill to be mutated by accepted trial")
	}
	if err := Deploy(res.ResearchDir, skillDir, "baseline"); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("baseline deploy did not restore original; got: %q", restored)
	}
}

func TestDeployTrialCopiesSnapshot(t *testing.T) {
	_, skillDir, _ := setupSkillDir(t)
	tc := model.TestCase{ID: "demo", Prompt: "run", ExpectedSkill: "demo-skill", Assertions: model.Assertions{FinalContains: []string{"dataset"}}}
	prop := &scriptedProposer{steps: []map[string]string{
		{"SKILL.md": "---\nname: demo-skill\n---\n\n# good v1\n"},
		{"scripts/helper.py": "# v2 helper\nprint('v2')\n"},
	}}
	res, err := Run(context.Background(), Config{
		Agent: model.AgentCodex, SkillPath: skillDir, Cases: []model.TestCase{tc},
		MaxTrials: 2, Executor: goodSkillExec, Proposer: prop, SkipDeployPrompt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Replace live with garbage, then deploy trial-001 (which had only the
	// SKILL.md edit and original helper).
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Deploy(res.ResearchDir, skillDir, "trial-001"); err != nil {
		t.Fatal(err)
	}
	md, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if !strings.Contains(string(md), "good v1") {
		t.Fatalf("trial-001 deploy did not write expected SKILL.md; got: %q", md)
	}
	helper, _ := os.ReadFile(filepath.Join(skillDir, "scripts", "helper.py"))
	if !strings.Contains(string(helper), "orig") {
		t.Fatalf("trial-001 deploy should have left original helper.py; got: %q", helper)
	}
}

func TestRunDiscardsAndRollsBackToBaseline(t *testing.T) {
	_, skillDir, original := setupSkillDir(t)
	tc := model.TestCase{ID: "demo", Prompt: "Run it"}
	// Proposer makes a tiny non-improving edit; goodSkillExec returns the
	// same low score either way → decision = discard.
	prop := fixedProposer{files: map[string]string{"SKILL.md": original + "\n# noise\n"}, strategy: "noop"}
	_, err := Run(context.Background(), Config{
		Agent: model.AgentCodex, SkillPath: skillDir, Cases: []model.TestCase{tc},
		MaxTrials: 1, MinImprovement: 5, Executor: goodSkillExec, Proposer: prop, SkipDeployPrompt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if string(got) != original {
		t.Fatalf("after discard, live SKILL.md should match baseline; got: %q", got)
	}
}

func TestRunWritesProposalAndLedger(t *testing.T) {
	_, skillDir, _ := setupSkillDir(t)
	tc := model.TestCase{ID: "demo", Prompt: "run", ExpectedSkill: "demo-skill", Assertions: model.Assertions{FinalContains: []string{"dataset"}}}
	prop := fixedProposer{files: map[string]string{"SKILL.md": "---\nname: demo-skill\n---\n\n# good\n"}, strategy: "make-good"}
	res, err := Run(context.Background(), Config{
		Agent: model.AgentCodex, SkillPath: skillDir, Cases: []model.TestCase{tc},
		MaxTrials: 1, Executor: goodSkillExec, Proposer: prop, SkipDeployPrompt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) != 1 {
		t.Fatalf("proposals=%d, want 1", len(res.Proposals))
	}
	if _, err := os.Stat(filepath.Join(res.ResearchDir, "ledger.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(res.Proposals[0].Path, "diff.patch")); err != nil {
		t.Fatal(err)
	}
}
