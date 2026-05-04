package judge

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"skillbench/internal/model"
)

//go:embed prompts/judge-system.md
var defaultJudgeSystem string

// JudgeRun runs an LLM with the given prompt and returns its final text
// response. Wired by the caller (cmd/skillbench) to combine runner + adapter
// parsing — mirrors research.ProposerRun.
type JudgeRun func(ctx context.Context, agent model.Agent, prompt string) (string, error)

// LLMJudge calls a configured agent to score the rubric. On any failure
// (runner error, empty output, parse failure, missing fallback) it delegates
// to Fallback (typically NoopJudge) and tags the Verdict status.
type LLMJudge struct {
	Agent      model.Agent
	Run        JudgeRun
	SystemFile string // overrides the embedded prompts/judge-system.md if non-empty
	Fallback   Judge
}

func (j LLMJudge) Score(ctx context.Context, in Input) (Verdict, error) {
	if j.Fallback == nil {
		return Verdict{}, fmt.Errorf("LLMJudge requires Fallback")
	}
	if j.Run == nil {
		return j.fallback(ctx, in, "no Run callback configured")
	}
	prompt, err := j.buildPrompt(in)
	if err != nil {
		return j.fallback(ctx, in, fmt.Sprintf("build prompt: %v", err))
	}
	text, err := j.Run(ctx, j.Agent, prompt)
	if err != nil {
		return j.fallback(ctx, in, fmt.Sprintf("run: %v", err))
	}
	if strings.TrimSpace(text) == "" {
		return j.fallback(ctx, in, "empty judge response")
	}
	v, err := parseVerdictJSON(text)
	if err != nil {
		return j.fallback(ctx, in, fmt.Sprintf("parse: %v", err))
	}
	v.Status = "ok"
	return v, nil
}

func (j LLMJudge) fallback(ctx context.Context, in Input, reason string) (Verdict, error) {
	v, err := j.Fallback.Score(ctx, in)
	if err != nil {
		return v, err
	}
	v.Status = "fallback:" + reason
	v.Notes = append(v.Notes, "llm judge fallback: "+reason)
	return v, nil
}

func (j LLMJudge) buildPrompt(in Input) (string, error) {
	system := defaultJudgeSystem
	if j.SystemFile != "" {
		b, err := os.ReadFile(j.SystemFile)
		if err != nil {
			return "", err
		}
		system = string(b)
	}

	// Only include rubric keys the case actually authored (non-empty values).
	rubric := map[string]string{}
	for _, k := range []string{"task_success", "skill_adherence", "output_quality"} {
		if v := strings.TrimSpace(in.Case.Rubric[k]); v != "" {
			rubric[k] = v
		}
	}
	rubricJSON, err := json.MarshalIndent(rubric, "", "  ")
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(system)
	b.WriteString("\n\n## Case prompt\n\n")
	b.WriteString(in.Case.Prompt)
	b.WriteString("\n\n## Rubric questions to answer\n\n```json\n")
	b.Write(rubricJSON)
	b.WriteString("\n```\n\n## Skill SKILL.md\n\n```markdown\n")
	b.WriteString(in.Skill.Content)
	b.WriteString("\n```\n\n## Final assistant message\n\n")
	if strings.TrimSpace(in.Final) == "" {
		b.WriteString("(none)")
	} else {
		b.WriteString(in.Final)
	}
	b.WriteString("\n\n## Event trace (compact)\n\n")
	b.WriteString(compactTrace(in.Events))
	b.WriteString("\n\nRespond with the JSON object only.\n")
	return b.String(), nil
}

// compactTrace renders one line per non-elided event: Type|Name|Text[:500].
// Successful tool_result events are elided; errors and skill invocations are
// always kept.
func compactTrace(events []model.Event) string {
	var lines []string
	for _, ev := range events {
		// Elide successful tool_result events to keep the trace short.
		if ev.Type == model.EventToolResult {
			lower := strings.ToLower(ev.Text)
			if !strings.Contains(lower, "error") && !strings.Contains(lower, "fail") {
				continue
			}
		}
		text := ev.Text
		if len(text) > 500 {
			text = text[:500]
		}
		text = strings.ReplaceAll(text, "\n", " ")
		lines = append(lines, fmt.Sprintf("%s|%s|%s", ev.Type, ev.Name, text))
	}
	if len(lines) == 0 {
		return "(no events)"
	}
	return strings.Join(lines, "\n")
}

// parseVerdictJSON parses a strict JSON object emitted by the judge LLM.
// Out-of-range values are clamped to [0,100]. Missing keys leave the
// corresponding pointer fields nil.
func parseVerdictJSON(text string) (Verdict, error) {
	s := strings.TrimSpace(text)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl >= 0 {
			s = s[nl+1:]
		}
	}
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "```"))
	if !strings.HasPrefix(s, "{") {
		if idx := strings.Index(s, "{"); idx >= 0 {
			s = s[idx:]
		}
	}
	var raw struct {
		TaskSuccess    *float64 `json:"task_success"`
		SkillAdherence *float64 `json:"skill_adherence"`
		OutputQuality  *float64 `json:"output_quality"`
		Notes          []string `json:"notes"`
	}
	dec := json.NewDecoder(strings.NewReader(s))
	if err := dec.Decode(&raw); err != nil {
		return Verdict{}, fmt.Errorf("decode: %w", err)
	}
	if raw.TaskSuccess == nil && raw.SkillAdherence == nil && raw.OutputQuality == nil {
		return Verdict{}, fmt.Errorf("no rubric scores in response")
	}
	v := Verdict{Notes: raw.Notes}
	v.TaskSuccess = clamp(raw.TaskSuccess)
	v.SkillAdherence = clamp(raw.SkillAdherence)
	v.OutputQuality = clamp(raw.OutputQuality)
	return v, nil
}

func clamp(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return &v
}
