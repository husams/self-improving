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

func TestRunWritesProposalAndDoesNotMutateOriginalSkill(t *testing.T) {
	oldWd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	skillDir := filepath.Join(tmp, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: demo-skill\ndescription: Demo skill.\n---\n\n# Demo\n\nDo the task.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	tc := model.TestCase{
		ID:            "demo",
		Prompt:        "Run the demo workflow",
		ExpectedSkill: "demo-skill",
		Assertions: model.Assertions{
			FinalContains: []string{"dataset"},
		},
	}

	exec := func(ctx context.Context, agent model.Agent, skillPath string, tc model.TestCase) (model.NormalizedRun, error) {
		info, err := skills.Load(skillPath)
		if err != nil {
			return model.NormalizedRun{}, err
		}
		events := []model.Event{{Type: model.EventAssistant, Text: "I did something."}}
		if strings.Contains(info.Content, "Skillbench Trigger Guidance") {
			events = []model.Event{
				{Type: model.EventSkillInvocation, Name: "Skill", Text: "demo-skill"},
				{Type: model.EventAssistant, Text: "The dataset workflow completed successfully."},
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

	result, err := Run(context.Background(), Config{
		Agent:          model.AgentCodex,
		SkillPath:      skillDir,
		Cases:          []model.TestCase{tc},
		MaxTrials:      1,
		MinImprovement: 5,
		Executor:       exec,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Proposals) != 1 {
		t.Fatalf("proposals=%d, want 1; trials=%#v", len(result.Proposals), result.Trials)
	}
	if _, err := os.Stat(filepath.Join(result.ResearchDir, "ledger.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(result.Proposals[0].Path, "diff.patch")); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatal("original skill was mutated")
	}
}

func TestRunDiscardsWhenCandidateDoesNotImprove(t *testing.T) {
	oldWd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	skillDir := filepath.Join(tmp, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo-skill\n---\n\n# Demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tc := model.TestCase{ID: "demo", Prompt: "Run it"}
	exec := func(ctx context.Context, agent model.Agent, skillPath string, tc model.TestCase) (model.NormalizedRun, error) {
		info, err := skills.Load(skillPath)
		if err != nil {
			return model.NormalizedRun{}, err
		}
		events := []model.Event{{Type: model.EventAssistant, Text: "same adequate response with enough length to score"}}
		run := model.NormalizedRun{RunID: model.NewRunID(agent, tc.ID), Agent: agent, Case: tc, Skill: info, Events: events}
		run.Metrics = metrics.Score(tc, events)
		return run, nil
	}
	result, err := Run(context.Background(), Config{Agent: model.AgentCodex, SkillPath: skillDir, Cases: []model.TestCase{tc}, MaxTrials: 1, MinImprovement: 5, Executor: exec})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Proposals) != 0 {
		t.Fatalf("proposals=%d, want 0", len(result.Proposals))
	}
	if len(result.Trials) != 1 || result.Trials[0].Decision != "discard" {
		t.Fatalf("unexpected trials: %#v", result.Trials)
	}
}
