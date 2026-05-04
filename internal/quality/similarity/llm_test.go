package similarity

import (
	"context"
	"errors"
	"testing"
)

func TestLLMRunnerCannedSuccess(t *testing.T) {
	pr := func(_ context.Context, _ string) (string, error) {
		return `{"similarity": 78, "notes": ["close paraphrase"]}`, nil
	}
	r := BuildLLMRunner("", pr)
	v, notes, err := r(context.Background(), "cand", "ref")
	if err != nil {
		t.Fatal(err)
	}
	if v != 78 {
		t.Fatalf("v=%.1f", v)
	}
	if len(notes) != 1 || notes[0] != "close paraphrase" {
		t.Fatalf("notes=%v", notes)
	}
}

func TestLLMRunnerCodeFenceTolerated(t *testing.T) {
	pr := func(_ context.Context, _ string) (string, error) {
		return "```json\n{\"similarity\":42}\n```", nil
	}
	r := BuildLLMRunner("", pr)
	v, _, err := r(context.Background(), "c", "r")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if v != 42 {
		t.Fatalf("v=%.1f", v)
	}
}

func TestLLMRunnerOutOfRangeClampedSilently(t *testing.T) {
	pr := func(_ context.Context, _ string) (string, error) {
		return `{"similarity": 250}`, nil
	}
	r := BuildLLMRunner("", pr)
	v, _, err := r(context.Background(), "c", "r")
	if err != nil {
		t.Fatal(err)
	}
	if v != 100 {
		t.Fatalf("expected clamp to 100, got %.1f", v)
	}
	pr2 := func(_ context.Context, _ string) (string, error) {
		return `{"similarity": -10}`, nil
	}
	r2 := BuildLLMRunner("", pr2)
	v2, _, err := r2(context.Background(), "c", "r")
	if err != nil {
		t.Fatal(err)
	}
	if v2 != 0 {
		t.Fatalf("expected clamp to 0, got %.1f", v2)
	}
}

// Malformed JSON → error returned by runner; dispatcher (computeLLM) falls back to ROUGE.
func TestLLMRunnerMalformedFallsBackViaDispatcher(t *testing.T) {
	pr := func(_ context.Context, _ string) (string, error) {
		return "not json at all", nil
	}
	r := BuildLLMRunner("", pr)
	if _, _, err := r(context.Background(), "c", "r"); err == nil {
		t.Fatal("expected parse error")
	}
	// Dispatcher path: confirm ROUGE fallback.
	sc := Compute(context.Background(), MethodLLM, "the same words", "the same words", r)
	if sc.Value != 100 {
		t.Fatalf("expected ROUGE-fallback=100, got %.1f", sc.Value)
	}
	if len(sc.Notes) == 0 {
		t.Fatal("expected fallback note")
	}
}

func TestLLMRunnerRunErrorFallsBackViaDispatcher(t *testing.T) {
	pr := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("boom")
	}
	r := BuildLLMRunner("", pr)
	sc := Compute(context.Background(), MethodLLM, "alpha beta", "alpha beta", r)
	if sc.Value != 100 {
		t.Fatalf("expected ROUGE-fallback=100, got %.1f", sc.Value)
	}
}

func TestLLMRunnerEmptyResponseErrors(t *testing.T) {
	pr := func(_ context.Context, _ string) (string, error) {
		return "   ", nil
	}
	r := BuildLLMRunner("", pr)
	if _, _, err := r(context.Background(), "c", "r"); err == nil {
		t.Fatal("expected error on empty response")
	}
}

func TestLLMRunnerMissingSimilarityField(t *testing.T) {
	pr := func(_ context.Context, _ string) (string, error) {
		return `{"notes":["nothing useful"]}`, nil
	}
	r := BuildLLMRunner("", pr)
	if _, _, err := r(context.Background(), "c", "r"); err == nil {
		t.Fatal("expected error on missing field")
	}
}
