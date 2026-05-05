package research

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"skillbench/internal/model"
)

//go:embed prompts/proposer-system.md
var defaultProposerSystem string

// ProposerRun runs an LLM with the given prompt and returns its final text response.
// Wired by the caller (cmd/skillbench) to combine runner + adapter parsing.
type ProposerRun func(ctx context.Context, agent model.Agent, prompt string) (string, error)

// LLMProposer asks an LLM for a candidate skill. On any failure (runner error,
// empty output, parse failure, empty candidate) it falls back to Fallback.
type LLMProposer struct {
	Agent      model.Agent
	Run        ProposerRun
	SystemFile string
	Fallback   Proposer
}

func (p LLMProposer) Propose(ctx context.Context, in ProposerInput) (Candidate, error) {
	if p.Fallback == nil {
		return Candidate{}, fmt.Errorf("LLMProposer requires Fallback")
	}
	if p.Run == nil {
		return p.fallback(ctx, in, "no Run callback configured")
	}
	prompt, err := p.buildPrompt(in)
	if err != nil {
		return p.fallback(ctx, in, fmt.Sprintf("build prompt: %v", err))
	}
	text, err := p.Run(ctx, p.Agent, prompt)
	if err != nil {
		return p.fallback(ctx, in, fmt.Sprintf("run: %v", err))
	}
	if strings.TrimSpace(text) == "" {
		return p.fallback(ctx, in, "empty proposer response")
	}
	cand, err := parseProposerJSON(text)
	if err != nil {
		return p.fallback(ctx, in, fmt.Sprintf("parse: %v", err))
	}
	return cand, nil
}

func (p LLMProposer) fallback(ctx context.Context, in ProposerInput, reason string) (Candidate, error) {
	cand, err := p.Fallback.Propose(ctx, in)
	if err != nil {
		return cand, err
	}
	cand.Strategy = "llm-fallback:" + cand.Strategy
	cand.Notes = append(cand.Notes, "llm proposer fallback: "+reason)
	return cand, nil
}

func (p LLMProposer) buildPrompt(in ProposerInput) (string, error) {
	system := defaultProposerSystem
	if p.SystemFile != "" {
		b, err := os.ReadFile(p.SystemFile)
		if err != nil {
			return "", err
		}
		system = string(b)
	}
	caseJSON, err := json.MarshalIndent(in.Case, "", "  ")
	if err != nil {
		return "", err
	}
	metricsJSON, err := json.MarshalIndent(in.Baseline.Metrics, "", "  ")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(system)
	b.WriteString("\n\n## Current skill files\n\n")
	// Emit SKILL.md first, then any other files (sorted) so the prompt is
	// deterministic. Each file is fenced with its relative path as a header.
	paths := make([]string, 0, len(in.SkillFiles))
	for p := range in.SkillFiles {
		if p == "SKILL.md" {
			continue
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	if md, ok := in.SkillFiles["SKILL.md"]; ok {
		b.WriteString("### SKILL.md\n\n```markdown\n")
		b.WriteString(md)
		b.WriteString("\n```\n\n")
	} else if in.SkillContent != "" {
		b.WriteString("### SKILL.md\n\n```markdown\n")
		b.WriteString(in.SkillContent)
		b.WriteString("\n```\n\n")
	}
	for _, p := range paths {
		fmt.Fprintf(&b, "### %s\n\n```\n%s\n```\n\n", p, in.SkillFiles[p])
	}
	b.WriteString("## Case being trialed\n\n```json\n")
	b.Write(caseJSON)
	b.WriteString("\n```\n\n## Baseline metrics\n\n```json\n")
	b.Write(metricsJSON)
	b.WriteString("\n```\n")
	if in.Case.ProposerContext.IncludeFailedAssertions {
		if section := renderFailedAssertions(in.Baseline.Metrics.Assertions); section != "" {
			b.WriteString("\n## Failed assertions on baseline\n\n")
			b.WriteString(section)
		}
	}
	if len(in.History) > 0 {
		histJSON, err := json.MarshalIndent(in.History, "", "  ")
		if err != nil {
			return "", err
		}
		b.WriteString("\n## Prior trials (oldest first)\n\n```json\n")
		b.Write(histJSON)
		b.WriteString("\n```\n")
	}
	b.WriteString("\nRespond with the JSON object only.\n")
	return b.String(), nil
}

func renderFailedAssertions(results []model.AssertionResult) string {
	var b strings.Builder
	for _, a := range results {
		if a.Passed {
			continue
		}
		fmt.Fprintf(&b, "- `%s`", a.Name)
		if msg := strings.TrimSpace(a.Message); msg != "" {
			fmt.Fprintf(&b, " — %s", msg)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func parseProposerJSON(text string) (Candidate, error) {
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
		Strategy   string            `json:"strategy"`
		Hypothesis string            `json:"hypothesis"`
		Files      map[string]string `json:"files"`
		SkillMD    string            `json:"skill_md"` // legacy compat
	}
	dec := json.NewDecoder(strings.NewReader(s))
	if err := dec.Decode(&raw); err != nil {
		return Candidate{}, fmt.Errorf("decode: %w", err)
	}
	files := raw.Files
	if len(files) == 0 && raw.SkillMD != "" {
		files = map[string]string{"SKILL.md": raw.SkillMD}
	}
	if len(files) == 0 {
		return Candidate{}, fmt.Errorf("files is empty")
	}
	return Candidate{
		Files:      files,
		Strategy:   strings.TrimSpace(raw.Strategy),
		Hypothesis: strings.TrimSpace(raw.Hypothesis),
	}, nil
}
