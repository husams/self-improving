package research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillbench/internal/model"
	"skillbench/internal/skills"
	"skillbench/internal/store"
)

type Executor func(context.Context, model.Agent, string, model.TestCase) (model.NormalizedRun, error)

type Config struct {
	Agent          model.Agent
	SkillPath      string
	Cases          []model.TestCase
	MaxTrials      int
	MinImprovement float64
	Executor       Executor
	Proposer       Proposer
	HistoryWindow  int
}

type Proposal struct {
	Path        string
	TrialID     string
	Improvement float64
}

type Result struct {
	ResearchDir string
	Proposals   []Proposal
	Trials      []Trial
}

type Trial struct {
	TrialID        string    `json:"trial_id"`
	CaseID         string    `json:"case_id"`
	Strategy       string    `json:"strategy"`
	Hypothesis     string    `json:"hypothesis"`
	BaselineRunID  string    `json:"baseline_run_id"`
	CandidateRunID string    `json:"candidate_run_id"`
	BaselineScore  float64   `json:"baseline_score"`
	CandidateScore float64   `json:"candidate_score"`
	Improvement    float64   `json:"improvement"`
	Decision       string    `json:"decision"`
	ProposalPath   string    `json:"proposal_path,omitempty"`
	Notes          []string  `json:"notes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type strategy struct {
	Name       string
	Hypothesis string
	Apply      func(content string, tc model.TestCase, baseline model.NormalizedRun) string
}

func Run(ctx context.Context, cfg Config) (Result, error) {
	if cfg.Executor == nil {
		return Result{}, fmt.Errorf("executor is required")
	}
	if len(cfg.Cases) == 0 {
		return Result{}, fmt.Errorf("at least one case is required")
	}
	if cfg.MaxTrials <= 0 {
		cfg.MaxTrials = 1
	}
	if cfg.MinImprovement <= 0 {
		cfg.MinImprovement = 5
	}
	if cfg.Proposer == nil {
		cfg.Proposer = DeterministicProposer{}
	}
	info, err := skills.Load(cfg.SkillPath)
	if err != nil {
		return Result{}, err
	}
	researchID := model.NewRunID(cfg.Agent, "research")
	researchDir := filepath.Join(".skillbench", "research", researchID)
	if err := os.MkdirAll(researchDir, 0o700); err != nil {
		return Result{}, err
	}
	var proposals []Proposal
	var trials []Trial
	for i := 0; i < cfg.MaxTrials; i++ {
		trialID := fmt.Sprintf("trial-%03d", i+1)
		tc := cfg.Cases[i%len(cfg.Cases)]
		baselinePath, cleanupBaseline, err := stageSkillCopy(cfg.SkillPath, info.Content)
		if err != nil {
			return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, err
		}
		baseline, err := cfg.Executor(ctx, cfg.Agent, baselinePath, tc)
		cleanupBaseline()
		if err != nil {
			return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, err
		}
		hist := trials
		if cfg.HistoryWindow > 0 && len(trials) > cfg.HistoryWindow {
			hist = trials[len(trials)-cfg.HistoryWindow:]
		}
		cand, err := cfg.Proposer.Propose(ctx, ProposerInput{
			SkillContent: info.Content,
			Case:         tc,
			Baseline:     baseline,
			History:      hist,
		})
		if err != nil {
			return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, err
		}
		candidateContent := cand.Content
		candidatePath, cleanupCandidate, err := stageSkillCopy(cfg.SkillPath, candidateContent)
		if err != nil {
			return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, err
		}
		candidate, err := cfg.Executor(ctx, cfg.Agent, candidatePath, tc)
		cleanupCandidate()
		if err != nil {
			return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, err
		}
		improvement := candidate.Metrics.Overall - baseline.Metrics.Overall
		decision, notes := decide(baseline, candidate, cfg.MinImprovement)
		trial := Trial{
			TrialID:        trialID,
			CaseID:         tc.ID,
			Strategy:       cand.Strategy,
			Hypothesis:     cand.Hypothesis,
			BaselineRunID:  baseline.RunID,
			CandidateRunID: candidate.RunID,
			BaselineScore:  baseline.Metrics.Overall,
			CandidateScore: candidate.Metrics.Overall,
			Improvement:    improvement,
			Decision:       decision,
			Notes:          append(notes, cand.Notes...),
			CreatedAt:      time.Now(),
		}
		if decision == "propose" {
			diff := appendDiff(info.SKILLPath, info.Content, candidateContent)
			path, err := store.WriteProposal(baseline.Skill.Name, trialID, diff, baseline.Metrics, candidate.Metrics)
			if err != nil {
				return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, err
			}
			trial.ProposalPath = path
			proposals = append(proposals, Proposal{Path: path, TrialID: trialID, Improvement: improvement})
		}
		if err := appendTrial(researchDir, trial); err != nil {
			return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, err
		}
		trials = append(trials, trial)
	}
	if err := writeReport(researchDir, cfg, proposals); err != nil {
		return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, err
	}
	return Result{ResearchDir: researchDir, Proposals: proposals, Trials: trials}, nil
}

func stageSkillCopy(skillPath, content string) (string, func(), error) {
	info, err := skills.Load(skillPath)
	if err != nil {
		return "", func() {}, err
	}
	tmp, err := os.MkdirTemp("", "skillbench-candidate-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	if err := copyTree(info.Path, tmp); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := os.WriteFile(filepath.Join(tmp, "SKILL.md"), []byte(content), 0o600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return tmp, cleanup, nil
}

func chooseStrategy(trial int, tc model.TestCase, baseline model.NormalizedRun) strategy {
	if baseline.Metrics.SkillUse < 100 && tc.ExpectedSkill != "" {
		return strategy{
			Name:       "trigger-and-skill-use",
			Hypothesis: "Making the trigger intent and visible skill workflow explicit will improve skill invocation and adherence.",
			Apply: func(content string, tc model.TestCase, baseline model.NormalizedRun) string {
				return augmentTrigger(content, tc) + guidanceSection("Skillbench Trigger Guidance", []string{
					fmt.Sprintf("When the user's task matches `%s`, explicitly follow this skill workflow.", tc.ExpectedSkill),
					"Make the skill workflow visible in the final response without over-explaining internals.",
				})
			},
		}
	}
	if baseline.Metrics.TaskSuccess < 100 || baseline.Metrics.DeterministicFail {
		return strategy{
			Name:       "verification-checklist",
			Hypothesis: "Adding an outcome verification checklist will reduce missed assertions and incomplete final answers.",
			Apply: func(content string, tc model.TestCase, baseline model.NormalizedRun) string {
				lines := []string{
					"Before the final response, verify each requested outcome against the user's prompt.",
					"State completed outcomes concretely and call out any remaining gap.",
				}
				for _, want := range tc.Assertions.FinalContains {
					lines = append(lines, fmt.Sprintf("Ensure the final answer includes the relevant result for `%s` when that result was produced.", want))
				}
				return content + guidanceSection("Skillbench Verification Checklist", lines)
			},
		}
	}
	if baseline.Metrics.ToolHealth < 100 {
		return strategy{
			Name:       "tool-health",
			Hypothesis: "Explicit tool error handling will reduce failed tool outcomes and improve recoverability.",
			Apply: func(content string, tc model.TestCase, baseline model.NormalizedRun) string {
				return content + guidanceSection("Skillbench Tool Hygiene", []string{
					"When a required tool fails, inspect the error once and retry simple recoverable failures once.",
					"If a tool remains unavailable, report the exact blocked step and proceed with the safest partial result.",
					"Do not expose secrets, tokens, or private credentials in commands or output.",
				})
			},
		}
	}
	if baseline.Metrics.OutputQuality < 80 {
		return strategy{
			Name:       "final-answer-quality",
			Hypothesis: "A tighter final-response contract will improve usefulness without adding tool risk.",
			Apply: func(content string, tc model.TestCase, baseline model.NormalizedRun) string {
				return content + guidanceSection("Skillbench Final Response Contract", []string{
					"Final responses should be concise and name the concrete work completed.",
					"Include evidence checked, output locations, and any residual risk only when relevant.",
				})
			},
		}
	}
	rotating := []strategy{
		{
			Name:       "minimal-checklist",
			Hypothesis: "A compact workflow checklist will make the skill more reliable without increasing complexity much.",
			Apply: func(content string, tc model.TestCase, baseline model.NormalizedRun) string {
				return content + guidanceSection("Skillbench Compact Workflow", []string{
					"Identify the requested outcome.",
					"Use the skill-specific workflow and tools.",
					"Validate the result before final response.",
				})
			},
		},
		{
			Name:       "safety-tightening",
			Hypothesis: "Explicit safety handling will preserve quality while reducing dangerous outputs.",
			Apply: func(content string, tc model.TestCase, baseline model.NormalizedRun) string {
				return content + guidanceSection("Skillbench Safety Guard", []string{
					"Never print secrets, API keys, access tokens, or private credentials.",
					"Prefer file-based secret references when a workflow requires credentials.",
				})
			},
		},
	}
	return rotating[trial%len(rotating)]
}

func guidanceSection(title string, lines []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n\n## %s\n\n", title)
	for _, line := range lines {
		fmt.Fprintf(&b, "- %s\n", line)
	}
	return b.String()
}

func augmentTrigger(content string, tc model.TestCase) string {
	insert := triggerText(tc)
	if strings.Contains(strings.ToLower(content), strings.ToLower(insert)) {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		desc := -1
		for i := 1; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "---" {
				end = i
				break
			}
			if strings.HasPrefix(trimmed, "description:") {
				desc = i
			}
		}
		if end > 0 {
			if desc > 0 {
				idx := strings.Index(lines[desc], ":")
				existing := ""
				if idx >= 0 {
					existing = strings.Trim(strings.TrimSpace(lines[desc][idx+1:]), `"'`)
				}
				combined := strings.TrimSpace(existing + " " + insert)
				lines[desc] = fmt.Sprintf("description: %q", combined)
			} else {
				lines = append(lines[:end], append([]string{fmt.Sprintf("description: %q", insert)}, lines[end:]...)...)
			}
			return strings.Join(lines, "\n")
		}
	}
	return fmt.Sprintf("---\ndescription: %q\n---\n\n%s", insert, content)
}

func triggerText(tc model.TestCase) string {
	if tc.ExpectedSkill != "" {
		return fmt.Sprintf("Use when the user request should trigger `%s` or asks for this workflow by intent rather than by exact name.", tc.ExpectedSkill)
	}
	return "Use when the user request matches this workflow by intent rather than by exact name."
}

func decide(baseline, candidate model.NormalizedRun, minImprovement float64) (string, []string) {
	var notes []string
	improvement := candidate.Metrics.Overall - baseline.Metrics.Overall
	if improvement < minImprovement {
		notes = append(notes, fmt.Sprintf("improvement %.1f below threshold %.1f", improvement, minImprovement))
		return "discard", notes
	}
	if candidate.Metrics.SafetyFailure {
		return "discard", []string{"candidate safety failure"}
	}
	if candidate.Metrics.DeterministicFail {
		return "discard", []string{"candidate deterministic assertion failure"}
	}
	if passedAssertions(candidate.Metrics) < passedAssertions(baseline.Metrics) {
		return "discard", []string{"candidate regressed deterministic assertions"}
	}
	return "propose", notes
}

func passedAssertions(m model.Metrics) int {
	var n int
	for _, ar := range m.Assertions {
		if ar.Passed {
			n++
		}
	}
	return n
}

func appendDiff(path, before, after string) string {
	if strings.HasPrefix(after, before) {
		added := strings.TrimPrefix(after, before)
		return fmt.Sprintf("--- %s\n+++ %s.candidate\n@@\n%s", path, path, prefixLines("+", strings.TrimLeft(added, "\n")))
	}
	return fmt.Sprintf("--- %s\n+++ %s.candidate\n@@\n-%s\n+%s\n", path, path, strings.TrimSpace(before), strings.TrimSpace(after))
}

func prefixLines(prefix, s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}

func appendTrial(dir string, trial Trial) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "ledger.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(trial)
}

func writeReport(dir string, cfg Config, proposals []Proposal) error {
	b, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		return err
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# Skillbench Research Run\n\n")
	fmt.Fprintf(&report, "- Agent: `%s`\n- Skill: `%s`\n- Max trials: `%d`\n- Min improvement: `%.1f`\n- Proposals: `%d`\n\n", cfg.Agent, cfg.SkillPath, cfg.MaxTrials, cfg.MinImprovement, len(proposals))
	fmt.Fprintf(&report, "## Trials\n\n")
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var t Trial
		if json.Unmarshal([]byte(line), &t) != nil {
			continue
		}
		fmt.Fprintf(&report, "- `%s` `%s`: %.1f -> %.1f (%+.1f), %s via `%s`\n", t.TrialID, t.CaseID, t.BaselineScore, t.CandidateScore, t.Improvement, t.Decision, t.Strategy)
	}
	return os.WriteFile(filepath.Join(dir, "report.md"), []byte(report.String()), 0o600)
}

func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		sp := filepath.Join(src, entry.Name())
		dp := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := copyTree(sp, dp); err != nil {
				return err
			}
			continue
		}
		if info.Mode().Type() != 0 {
			continue
		}
		if err := copyFile(sp, dp, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
