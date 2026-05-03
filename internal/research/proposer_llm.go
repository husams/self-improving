package research

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
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
	b.WriteString("\n\n## Current SKILL.md\n\n```markdown\n")
	b.WriteString(in.SkillContent)
	b.WriteString("\n```\n\n## Case being trialed\n\n```json\n")
	b.Write(caseJSON)
	b.WriteString("\n```\n\n## Baseline metrics\n\n```json\n")
	b.Write(metricsJSON)
	b.WriteString("\n```\n")
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
		Strategy   string `json:"strategy"`
		Hypothesis string `json:"hypothesis"`
		SkillMD    string `json:"skill_md"`
	}
	dec := json.NewDecoder(strings.NewReader(s))
	if err := dec.Decode(&raw); err != nil {
		return Candidate{}, fmt.Errorf("decode: %w", err)
	}
	if raw.SkillMD == "" {
		return Candidate{}, fmt.Errorf("skill_md is empty")
	}
	return Candidate{
		Content:    raw.SkillMD,
		Strategy:   strings.TrimSpace(raw.Strategy),
		Hypothesis: strings.TrimSpace(raw.Hypothesis),
	}, nil
}
