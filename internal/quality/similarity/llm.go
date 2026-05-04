package similarity

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed prompts/similarity-system.md
var defaultSimilaritySystem string

// computeLLM dispatches MethodLLM through the configured LLMRunner. On any
// error from the runner it falls back to ROUGE with a note — mirroring the
// LLMJudge.fallback discipline at internal/judge/llm.go:58. nil runners also
// fall back so cases that author method:llm don't crash when the agent is
// unconfigured.
func computeLLM(ctx context.Context, candidate, reference string, llm LLMRunner) Score {
	if llm == nil {
		return Score{
			Value: ROUGE_L(candidate, reference),
			Notes: []string{"llm similarity backend not configured; falling back to rouge"},
		}
	}
	v, notes, err := llm(ctx, candidate, reference)
	if err != nil {
		fallback := ROUGE_L(candidate, reference)
		return Score{
			Value: fallback,
			Notes: append([]string{fmt.Sprintf("llm similarity failed: %v; falling back to rouge", err)}, notes...),
		}
	}
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return Score{Value: v, Notes: notes}
}

// PromptRunner is the LLM execution callback (mirrors judge.JudgeRun): given a
// rendered prompt, return the agent's final assistant text. The cmd/skillbench
// layer wires this to the real CLI runner.
type PromptRunner func(ctx context.Context, prompt string) (string, error)

// BuildLLMRunner constructs an LLMRunner from a PromptRunner + optional system
// prompt override. The returned runner renders the prompt, calls run, parses
// strict JSON, clamps to [0,100], and surfaces "notes" through.
//
// Callers (cmd/skillbench Slice 6) get a similarity-method-llm path that
// reuses the same parse/fallback discipline as the LLM judge, with the
// additional contract documented in prompts/similarity-system.md.
func BuildLLMRunner(systemFile string, run PromptRunner) LLMRunner {
	return func(ctx context.Context, candidate, reference string) (float64, []string, error) {
		if run == nil {
			return 0, nil, fmt.Errorf("no PromptRunner configured")
		}
		system := defaultSimilaritySystem
		if systemFile != "" {
			b, err := os.ReadFile(systemFile)
			if err != nil {
				return 0, nil, fmt.Errorf("read system prompt %s: %w", systemFile, err)
			}
			system = string(b)
		}
		prompt := buildPrompt(system, candidate, reference)
		text, err := run(ctx, prompt)
		if err != nil {
			return 0, nil, fmt.Errorf("run: %w", err)
		}
		if strings.TrimSpace(text) == "" {
			return 0, nil, fmt.Errorf("empty similarity response")
		}
		score, notes, err := parseSimilarityJSON(text)
		if err != nil {
			return 0, nil, fmt.Errorf("parse: %w", err)
		}
		return score, notes, nil
	}
}

func buildPrompt(system, candidate, reference string) string {
	var b strings.Builder
	b.WriteString(system)
	b.WriteString("\n\n## Reference answer\n\n")
	if strings.TrimSpace(reference) == "" {
		b.WriteString("(empty)")
	} else {
		b.WriteString(reference)
	}
	b.WriteString("\n\n## Candidate answer\n\n")
	if strings.TrimSpace(candidate) == "" {
		b.WriteString("(empty)")
	} else {
		b.WriteString(candidate)
	}
	b.WriteString("\n\nRespond with the JSON object only.\n")
	return b.String()
}

// parseSimilarityJSON parses a strict JSON object {similarity, notes}. Out-of-
// range similarity values are clamped to [0,100] silently — mirrors the judge
// clamp policy at internal/judge/llm.go:166-184.
func parseSimilarityJSON(text string) (float64, []string, error) {
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
		Similarity *float64 `json:"similarity"`
		Notes      []string `json:"notes"`
	}
	dec := json.NewDecoder(strings.NewReader(s))
	if err := dec.Decode(&raw); err != nil {
		return 0, nil, fmt.Errorf("decode: %w", err)
	}
	if raw.Similarity == nil {
		return 0, nil, fmt.Errorf("missing similarity field")
	}
	v := *raw.Similarity
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v, raw.Notes, nil
}
