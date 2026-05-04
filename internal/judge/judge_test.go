package judge

import (
	"context"
	"testing"
)

func TestNoopJudgeReturnsSkipped(t *testing.T) {
	v, err := NoopJudge{}.Score(context.Background(), Input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.TaskSuccess != nil || v.SkillAdherence != nil || v.OutputQuality != nil {
		t.Fatalf("expected nil pointer fields, got %#v", v)
	}
	if v.Status != "skipped" {
		t.Fatalf("status=%q want skipped", v.Status)
	}
}

func TestFloatHelper(t *testing.T) {
	p := Float(42)
	if p == nil || *p != 42 {
		t.Fatalf("Float(42) = %v", p)
	}
}
