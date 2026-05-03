package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"skillbench/internal/metrics"
	"skillbench/internal/model"
	"skillbench/internal/runner"
)

func JSONEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

func WriteNewRun(run model.NormalizedRun, raw runner.Result) (string, error) {
	dir := filepath.Join(".skillbench", "runs", run.RunID)
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "raw", "stdout.jsonl"), raw.Stdout, 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "raw", "stderr.log"), redact(raw.Stderr), 0o600); err != nil {
		return "", err
	}
	return dir, nil
}

func WriteRun(dir string, run model.NormalizedRun) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "run.json"), run); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "metrics.json"), run.Metrics); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(dir, "events.jsonl"), run.Events); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report.md"), []byte(report(run)), 0o600)
}

func ReadRun(path string) (model.NormalizedRun, error) {
	var run model.NormalizedRun
	b, err := os.ReadFile(filepath.Join(path, "run.json"))
	if err != nil {
		return run, err
	}
	return run, json.Unmarshal(b, &run)
}

func WriteProposal(skillName, trialID, diff string, baseline, candidate model.Metrics) (string, error) {
	dir := filepath.Join(".skillbench", "proposals", skillName, trialID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "diff.patch"), []byte(diff), 0o600); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(dir, "metrics.json"), map[string]any{"baseline": baseline, "candidate": candidate}); err != nil {
		return "", err
	}
	decision := fmt.Sprintf("# Proposal %s\n\nBaseline: %s\n\nCandidate: %s\n\nDecision: proposal-only; human review required.\n", trialID, metrics.Summary(baseline), metrics.Summary(candidate))
	if err := os.WriteFile(filepath.Join(dir, "decision.md"), []byte(decision), 0o600); err != nil {
		return "", err
	}
	return dir, os.WriteFile(filepath.Join(dir, "report.md"), []byte(decision), 0o600)
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := JSONEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeJSONL(path string, events []model.Event) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := JSONEncoder(f)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			return err
		}
	}
	return nil
}

func report(run model.NormalizedRun) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Skillbench Run %s\n\n", run.RunID)
	fmt.Fprintf(&b, "- Agent: `%s`\n- Skill: `%s`\n- Exposure: `%s`\n- Score: `%.1f`\n- Exit code: `%d`\n\n", run.Agent, run.Skill.Name, run.Exposure, run.Metrics.Overall, run.ExitCode)
	fmt.Fprintf(&b, "## Metrics\n\n%s\n\n", metrics.Summary(run.Metrics))
	fmt.Fprintf(&b, "## Assertions\n\n")
	for _, ar := range run.Metrics.Assertions {
		status := "fail"
		if ar.Passed {
			status = "pass"
		}
		fmt.Fprintf(&b, "- `%s`: %s\n", ar.Name, status)
	}
	return b.String()
}

func redact(b []byte) []byte {
	s := string(b)
	for _, marker := range []string{"PRIVATE-TOKEN:", "Authorization:", "Bearer "} {
		if idx := strings.Index(strings.ToLower(s), strings.ToLower(marker)); idx >= 0 {
			end := strings.IndexByte(s[idx:], '\n')
			if end < 0 {
				end = len(s) - idx
			}
			s = s[:idx+len(marker)] + " [redacted]" + s[idx+end:]
		}
	}
	return []byte(s)
}
