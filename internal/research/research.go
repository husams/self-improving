package research

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	// ForceDirty bypasses the git-clean precondition on the skill dir.
	ForceDirty bool
	// Stdin/Stdout drive the end-of-run deploy prompt. Defaults to
	// os.Stdin / os.Stdout when nil.
	Stdin  io.Reader
	Stdout io.Writer
	// SkipDeployPrompt suppresses the interactive deploy gate (used by tests
	// that want to drive Deploy directly).
	SkipDeployPrompt bool
}

type Proposal struct {
	Path        string
	TrialID     string
	Improvement float64
}

type Result struct {
	ResearchDir  string
	BaselineDir  string
	Proposals    []Proposal
	Trials       []Trial
	DeployChoice string
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
	SnapshotPath   string    `json:"snapshot_path,omitempty"`
	ChangedFiles   []string  `json:"changed_files,omitempty"`
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
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	info, err := skills.Load(cfg.SkillPath)
	if err != nil {
		return Result{}, err
	}
	skillRoot := info.Path
	if !cfg.ForceDirty {
		if dirty, why, err := skillDirIsDirty(skillRoot); err != nil {
			// Not-a-repo or git-not-installed: treat as clean and continue.
			_ = err
		} else if dirty {
			return Result{}, fmt.Errorf("skill dir has uncommitted changes:\n%s\nuse --force-dirty to override", why)
		}
	}
	researchID := model.NewRunID(cfg.Agent, "research")
	researchDir := filepath.Join(".skillbench", "research", researchID)
	if err := os.MkdirAll(researchDir, 0o700); err != nil {
		return Result{}, err
	}
	baselineDir := filepath.Join(researchDir, "baseline-skill")
	if err := copyTree(skillRoot, baselineDir); err != nil {
		return Result{}, fmt.Errorf("backup baseline: %w", err)
	}
	trialsDir := filepath.Join(researchDir, "trials")
	if err := os.MkdirAll(trialsDir, 0o700); err != nil {
		return Result{}, err
	}

	var proposals []Proposal
	var trials []Trial
	// lastGoodSnapshot is the path to copy from when we need to roll back the
	// live skill dir. It starts as the baseline backup and advances to the
	// snapshot of the most recently accepted trial.
	lastGoodSnapshot := baselineDir

	for i := 0; i < cfg.MaxTrials; i++ {
		trialID := fmt.Sprintf("trial-%03d", i+1)
		tc := cfg.Cases[i%len(cfg.Cases)]

		// Re-read SKILL.md from the live (possibly mutated) skill dir so the
		// proposer sees the current cumulative state.
		liveInfo, err := skills.Load(skillRoot)
		if err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		baseline, err := cfg.Executor(ctx, cfg.Agent, skillRoot, tc)
		if err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		hist := trials
		if cfg.HistoryWindow > 0 && len(trials) > cfg.HistoryWindow {
			hist = trials[len(trials)-cfg.HistoryWindow:]
		}
		liveFiles, err := readSkillFiles(skillRoot)
		if err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		cand, err := cfg.Proposer.Propose(ctx, ProposerInput{
			SkillContent: liveInfo.Content,
			SkillFiles:   liveFiles,
			Case:         tc,
			Baseline:     baseline,
			History:      hist,
			TrialIndex:   i,
		})
		if err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		// Apply candidate files in place to the live skill dir.
		changed, err := applyFiles(skillRoot, cand.Files)
		if err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		candidate, err := cfg.Executor(ctx, cfg.Agent, skillRoot, tc)
		if err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		improvement := candidate.Metrics.Overall - baseline.Metrics.Overall
		decision, notes := decide(baseline, candidate, cfg.MinImprovement)

		// Snapshot current state regardless of decision.
		snapshotPath := filepath.Join(trialsDir, trialID, "skill")
		if err := copyTree(skillRoot, snapshotPath); err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}

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
			SnapshotPath:   snapshotPath,
			ChangedFiles:   changed,
			Notes:          append(notes, cand.Notes...),
			CreatedAt:      time.Now(),
		}
		if decision == "propose" {
			diff := summaryDiff(skillRoot, lastGoodSnapshot, cand.Files)
			path, err := store.WriteProposal(baseline.Skill.Name, trialID, diff, baseline.Metrics, candidate.Metrics)
			if err != nil {
				return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
			}
			trial.ProposalPath = path
			proposals = append(proposals, Proposal{Path: path, TrialID: trialID, Improvement: improvement})
			// Advance the rollback pointer: future discards revert to this snapshot.
			lastGoodSnapshot = snapshotPath
		} else {
			// Roll back the live skill dir to the last-good snapshot.
			if err := replaceTree(lastGoodSnapshot, skillRoot); err != nil {
				return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, fmt.Errorf("rollback: %w", err)
			}
		}
		if err := appendTrial(researchDir, trial); err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		trials = append(trials, trial)
	}
	if err := writeReport(researchDir, cfg, proposals); err != nil {
		return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
	}

	choice := ""
	if !cfg.SkipDeployPrompt {
		c, err := promptDeploy(cfg.Stdin, cfg.Stdout, trials)
		if err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		if err := Deploy(researchDir, skillRoot, c); err != nil {
			return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials}, err
		}
		choice = c
	}
	return Result{ResearchDir: researchDir, BaselineDir: baselineDir, Proposals: proposals, Trials: trials, DeployChoice: choice}, nil
}

// Deploy atomically replaces skillRoot with the contents of the chosen
// snapshot. choice may be "none" (leave dir at its current state),
// "baseline" (restore the original), or "trial-NNN" (deploy that trial's
// snapshot).
func Deploy(researchDir, skillRoot, choice string) error {
	choice = strings.TrimSpace(choice)
	switch {
	case choice == "" || choice == "none":
		return nil
	case choice == "baseline":
		src := filepath.Join(researchDir, "baseline-skill")
		return atomicReplace(src, skillRoot)
	case strings.HasPrefix(choice, "trial-"):
		src := filepath.Join(researchDir, "trials", choice, "skill")
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("snapshot %q not found: %w", choice, err)
		}
		return atomicReplace(src, skillRoot)
	default:
		return fmt.Errorf("unknown deploy choice %q (want none|baseline|trial-NNN)", choice)
	}
}

func promptDeploy(in io.Reader, out io.Writer, trials []Trial) (string, error) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Research complete. Trial summary (ranked by candidate score):")
	ranked := make([]Trial, len(trials))
	copy(ranked, trials)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].CandidateScore > ranked[j].CandidateScore
	})
	fmt.Fprintln(out, "trial\tdecision\tscore\timprovement\tstrategy")
	for _, t := range ranked {
		fmt.Fprintf(out, "%s\t%s\t%.1f\t%+.1f\t%s\n", t.TrialID, t.Decision, t.CandidateScore, t.Improvement, t.Strategy)
	}
	fmt.Fprintln(out, "")
	fmt.Fprint(out, "Deploy which? (none|baseline|trial-NNN) [none]: ")
	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		return "none", nil
	}
	choice := strings.TrimSpace(sc.Text())
	if choice == "" {
		choice = "none"
	}
	return choice, nil
}

// readSkillFiles reads the full skill tree as a relpath→contents map.
// Skips non-regular files (symlinks, devices). Caps individual file size at
// 256 KiB to keep proposer prompts bounded.
func readSkillFiles(root string) (map[string]string, error) {
	const maxFile = 256 * 1024
	out := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().Type() != 0 {
			return nil
		}
		if info.Size() > maxFile {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// applyFiles writes each path → contents into root, creating parent dirs as
// needed. Returns the list of relative paths actually changed (created or
// modified). Refuses paths that escape root.
func applyFiles(root string, files map[string]string) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	changed := make([]string, 0, len(files))
	for rel, content := range files {
		clean := filepath.Clean(filepath.FromSlash(rel))
		if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return nil, fmt.Errorf("invalid candidate path %q", rel)
		}
		dst := filepath.Join(rootAbs, clean)
		dstAbs, err := filepath.Abs(dst)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(dstAbs+string(filepath.Separator), rootAbs+string(filepath.Separator)) && dstAbs != rootAbs {
			return nil, fmt.Errorf("candidate path %q escapes skill dir", rel)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return nil, err
		}
		// Preserve existing file mode if any (so executable scripts stay +x).
		mode := os.FileMode(0o600)
		if fi, err := os.Stat(dst); err == nil {
			mode = fi.Mode().Perm()
		}
		// Skip writes when content is already identical (avoids spurious
		// "changed" entries).
		if existing, err := os.ReadFile(dst); err == nil && string(existing) == content {
			continue
		}
		if err := os.WriteFile(dst, []byte(content), mode); err != nil {
			return nil, err
		}
		changed = append(changed, filepath.ToSlash(clean))
	}
	sort.Strings(changed)
	return changed, nil
}

// replaceTree wipes dst and copies src over it. Used for rollbacks.
func replaceTree(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return copyTree(src, dst)
}

// atomicReplace stages src at dst.tmp and renames over dst. The rename is
// atomic on the same filesystem; the prior dst is removed first since
// os.Rename does not replace a non-empty directory.
func atomicReplace(src, dst string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".skillbench-deploy-*")
	if err != nil {
		return err
	}
	staged := filepath.Join(tmp, "stage")
	if err := copyTree(src, staged); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	// Move existing dst aside, swap in staged, then remove the old.
	backup := filepath.Join(tmp, "backup")
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, backup); err != nil {
			os.RemoveAll(tmp)
			return err
		}
	}
	if err := os.Rename(staged, dst); err != nil {
		// best effort: try to restore the backup
		if _, berr := os.Stat(backup); berr == nil {
			_ = os.Rename(backup, dst)
		}
		os.RemoveAll(tmp)
		return err
	}
	os.RemoveAll(tmp)
	return nil
}

// skillDirIsDirty reports whether `git status --porcelain <root>` lists any
// changes. Returns (false, "", err) when git is unavailable or root is not in
// a repo — callers treat that as "clean enough to proceed".
func skillDirIsDirty(root string) (bool, string, error) {
	cmd := exec.Command("git", "status", "--porcelain", "--", root)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return false, "", err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return false, "", nil
	}
	return true, s, nil
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

// summaryDiff emits a concatenated unified diff covering each path in `files`,
// comparing it against the same path under `lastGoodRoot` (treating missing
// files as empty). Multi-file proposals get one combined patch.
func summaryDiff(skillRoot, lastGoodRoot string, files map[string]string) string {
	if len(files) == 0 {
		return ""
	}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, rel := range keys {
		after := files[rel]
		var before string
		if b, err := os.ReadFile(filepath.Join(lastGoodRoot, filepath.FromSlash(rel))); err == nil {
			before = string(b)
		}
		if before == after {
			continue
		}
		labelBefore := rel
		labelAfter := rel + ".candidate"
		if d, err := unifiedDiff(labelBefore, labelAfter, before, after); err == nil {
			out.WriteString(d)
		} else {
			fmt.Fprintf(&out, "--- %s\n+++ %s\n@@\n-%s\n+%s\n", labelBefore, labelAfter, strings.TrimSpace(before), strings.TrimSpace(after))
		}
	}
	return out.String()
}

func unifiedDiff(labelBefore, labelAfter, before, after string) (string, error) {
	dir, err := os.MkdirTemp("", "skillbench-diff-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	bp := filepath.Join(dir, "before")
	ap := filepath.Join(dir, "after")
	if err := os.WriteFile(bp, []byte(before), 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(ap, []byte(after), 0o600); err != nil {
		return "", err
	}
	cmd := exec.Command("diff", "-u",
		"--label", labelBefore,
		"--label", labelAfter,
		bp, ap)
	out, err := cmd.Output()
	// `diff` exits 1 when files differ — that is success for our purposes.
	if err != nil {
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
			return "", err
		}
	}
	if len(out) == 0 {
		return fmt.Sprintf("--- %s\n+++ %s\n", labelBefore, labelAfter), nil
	}
	return string(out), nil
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
