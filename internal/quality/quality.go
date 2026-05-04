// Package quality runs deterministic quality_checks declared on a TestCase
// and produces a 0-100 signal that the metrics package blends into
// OutputQuality. It mirrors the interface + multi-impl + fallback discipline
// already proven by internal/judge, but the checks here are deterministic
// (script, file_exists, json_schema, regex) plus a similarity backend.
//
// Dependency direction: internal/metrics MUST NOT import this package. The
// caller (cmd/skillbench) computes a *QualitySignal and passes it into
// metrics.ScoreWithVerdictAndQuality.
package quality

import (
	"context"
	"fmt"
	"strings"

	"skillbench/internal/model"
)

// Check is a single deterministic quality check. Implementations return a
// 0-100 score. An error returned from Run (the Check's own runtime) is NOT
// fatal to the suite: Runner.Run translates errors into score=0/passed=false
// with a note, mirroring the judge fallback discipline at judge/llm.go:58.
type Check interface {
	// Name is a short identifier used in notes (e.g. "script", "regex").
	Name() string
	// Run produces a Result for the given case + final assistant message.
	Run(ctx context.Context, in Input) Result
}

// Input is what every Check sees. The final assistant message is precomputed
// by the caller (mirrors judge.Input.Final).
type Input struct {
	Case  model.TestCase
	Final string
}

// Result is the outcome of one Check.
type Result struct {
	Name    string  // check kind, e.g. "script", "regex", "similarity"
	Score   float64 // 0-100
	Passed  bool
	Skipped bool   // true → excluded from the mean (e.g. missing creds)
	Note    string // short human-readable summary; surfaced via metrics.Notes
}

// QualitySignal is the aggregate produced by Runner.Run. The metrics layer
// blends Score 50/50 with the judge's OutputQuality when both are present.
type QualitySignal struct {
	Score   float64  // mean of per-check scores, 0-100
	Notes   []string // per-check notes, "<name>: <note>"
	Results []Result
}

// Runner executes a slice of Checks. Build it from a TestCase via Build.
type Runner struct {
	Checks []Check
}

// Build decodes tc.QualityChecks into a Runner. Unknown / empty specs return
// an error so cases.yaml typos surface fast.
func Build(tc model.TestCase, opts BuildOptions) (*Runner, error) {
	if len(tc.QualityChecks) == 0 {
		return nil, nil
	}
	r := &Runner{}
	for i, spec := range tc.QualityChecks {
		c, err := buildCheck(spec, opts)
		if err != nil {
			return nil, fmt.Errorf("quality_checks[%d]: %w", i, err)
		}
		r.Checks = append(r.Checks, c)
	}
	return r, nil
}

// BuildOptions configures runtime knobs (script timeout, similarity backend).
// Slice 6 wires CLI flags into this struct; Slice 1 only needs ScriptTimeout.
type BuildOptions struct {
	ScriptTimeout    int // seconds; 0 = use default (30)
	SimilarityMethod string
	SimilarityRunner SimilarityRunner // optional LLM backend; nil = rouge fallback
}

// SimilarityRunner is the function signature for an LLM-backed similarity
// backend. Slice 5 fills this in; Slice 3's rouge/bleu paths don't need it.
type SimilarityRunner func(ctx context.Context, candidate, reference string) (float64, []string, error)

// Run executes every check and returns the aggregate. nil receiver returns nil.
func (r *Runner) Run(ctx context.Context, in Input) *QualitySignal {
	if r == nil || len(r.Checks) == 0 {
		return nil
	}
	sig := &QualitySignal{}
	var sum float64
	var counted int
	for _, c := range r.Checks {
		res := c.Run(ctx, in)
		if res.Name == "" {
			res.Name = c.Name()
		}
		sig.Results = append(sig.Results, res)
		if !res.Skipped {
			sum += res.Score
			counted++
		}
		if note := strings.TrimSpace(res.Note); note != "" {
			sig.Notes = append(sig.Notes, fmt.Sprintf("%s: %s", res.Name, note))
		} else {
			sig.Notes = append(sig.Notes, fmt.Sprintf("%s: %s", res.Name, passedTag(res)))
		}
	}
	if counted == 0 {
		// All checks skipped: no signal.
		return nil
	}
	sig.Score = sum / float64(counted)
	return sig
}

func passedTag(r Result) string {
	if r.Skipped {
		return "skipped"
	}
	if r.Passed {
		return "pass"
	}
	return "fail"
}
