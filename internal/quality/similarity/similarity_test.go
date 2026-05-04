package similarity

import (
	"context"
	"testing"
)

func TestROUGEIdentical(t *testing.T) {
	v := ROUGE_L("the quick brown fox", "the quick brown fox")
	if v != 100 {
		t.Fatalf("expected 100, got %.3f", v)
	}
}

func TestROUGEDisjoint(t *testing.T) {
	v := ROUGE_L("alpha beta gamma", "delta epsilon zeta")
	if v != 0 {
		t.Fatalf("expected 0, got %.3f", v)
	}
}

func TestROUGEPartial(t *testing.T) {
	// Both have len 4 tokens; LCS=2 ("the","fox") → P=R=0.5 → F=0.5 → 50.
	v := ROUGE_L("the brown fox jumped", "the lazy fox sat")
	if v < 40 || v > 60 {
		t.Fatalf("expected ~50, got %.3f", v)
	}
}

func TestROUGEEmpty(t *testing.T) {
	if v := ROUGE_L("", "x"); v != 0 {
		t.Fatalf("expected 0, got %.3f", v)
	}
	if v := ROUGE_L("x", ""); v != 0 {
		t.Fatalf("expected 0, got %.3f", v)
	}
}

func TestBLEUIdentical(t *testing.T) {
	s := "the quick brown fox jumped over the lazy dog"
	v := BLEU4(s, s)
	if v < 99.9 {
		t.Fatalf("expected ~100, got %.3f", v)
	}
}

func TestBLEUDisjointBelowIdentity(t *testing.T) {
	// Fully-disjoint short candidates score non-zero under +1 smoothing,
	// but must remain well below identity.
	v := BLEU4("alpha beta gamma delta", "epsilon zeta eta theta")
	if v >= 50 {
		t.Fatalf("expected well below identity, got %.3f", v)
	}
}

func TestBLEUBrevityPenalty(t *testing.T) {
	// Short candidate — BP should pull score below identity.
	v := BLEU4("the quick", "the quick brown fox jumped over")
	if v >= 100 {
		t.Fatalf("expected BP-reduced, got %.3f", v)
	}
}

func TestComputeFallsBackOnUnknown(t *testing.T) {
	sc := Compute(context.Background(), "wat", "the quick", "the quick", nil)
	if sc.Value != 100 {
		t.Fatalf("expected ROUGE fallback=100, got %.3f", sc.Value)
	}
	if len(sc.Notes) == 0 {
		t.Fatal("expected fallback note")
	}
}

func TestComputeEmbeddingFallsBackToROUGE(t *testing.T) {
	sc := Compute(context.Background(), MethodEmbedding, "the quick", "the quick", nil)
	if sc.Value != 100 {
		t.Fatalf("expected ROUGE fallback=100, got %.3f", sc.Value)
	}
}

func TestComputeLLMNilFallsBack(t *testing.T) {
	sc := Compute(context.Background(), MethodLLM, "the quick", "the quick", nil)
	if sc.Value != 100 {
		t.Fatalf("expected ROUGE fallback, got %.3f", sc.Value)
	}
}

func TestComputeLLMSuccess(t *testing.T) {
	llm := func(_ context.Context, _, _ string) (float64, []string, error) {
		return 73, []string{"semantic match"}, nil
	}
	sc := Compute(context.Background(), MethodLLM, "a", "b", llm)
	if sc.Value != 73 {
		t.Fatalf("expected 73, got %.3f", sc.Value)
	}
}
