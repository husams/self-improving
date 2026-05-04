package quality

import (
	"context"
	"testing"
)

func TestSimilarityCheckRawScore(t *testing.T) {
	c := SimilarityCheck{Reference: "the quick brown fox", Method: "rouge"}
	res := c.Run(context.Background(), Input{Final: "the quick brown fox"})
	if !res.Passed || res.Score != 100 {
		t.Fatalf("expected pass 100, got %+v", res)
	}
}

func TestSimilarityCheckThresholdPasses(t *testing.T) {
	c := SimilarityCheck{Reference: "the quick brown fox", Method: "rouge", Threshold: 0.75}
	res := c.Run(context.Background(), Input{Final: "the quick brown fox"})
	if !res.Passed || res.Score != 100 {
		t.Fatalf("expected gated pass=100, got %+v", res)
	}
}

func TestSimilarityCheckThresholdFails(t *testing.T) {
	c := SimilarityCheck{Reference: "the quick brown fox", Method: "rouge", Threshold: 0.95}
	res := c.Run(context.Background(), Input{Final: "completely unrelated text here"})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected gated fail=0, got %+v", res)
	}
}

func TestSimilarityCheckEmptyReference(t *testing.T) {
	c := SimilarityCheck{Reference: "", Method: "rouge"}
	res := c.Run(context.Background(), Input{Final: "anything"})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected fail, got %+v", res)
	}
}
