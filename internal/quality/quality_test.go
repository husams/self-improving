package quality

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"skillbench/internal/model"
)

func TestBuildEmpty(t *testing.T) {
	r, err := Build(model.TestCase{}, BuildOptions{})
	if err != nil {
		t.Fatalf("Build(empty): %v", err)
	}
	if r != nil {
		t.Fatalf("expected nil runner for empty quality_checks, got %+v", r)
	}
}

func TestBuildRejectsEmptySpec(t *testing.T) {
	tc := model.TestCase{QualityChecks: []model.QualityCheckSpec{{}}}
	if _, err := Build(tc, BuildOptions{}); err == nil {
		t.Fatal("expected error for empty spec")
	}
}

func TestBuildRejectsMultipleKinds(t *testing.T) {
	tc := model.TestCase{QualityChecks: []model.QualityCheckSpec{{Script: "true", FileExists: "x"}}}
	if _, err := Build(tc, BuildOptions{}); err == nil {
		t.Fatal("expected error for multi-kind spec")
	}
}

func TestRegexCheckHappy(t *testing.T) {
	c := RegexCheck{Pattern: "^Priority: P0"}
	res := c.Run(context.Background(), Input{Final: "Priority: P0\nrest"})
	if !res.Passed || res.Score != 100 {
		t.Fatalf("expected pass 100, got %+v", res)
	}
}

func TestRegexCheckMiss(t *testing.T) {
	c := RegexCheck{Pattern: "P0"}
	res := c.Run(context.Background(), Input{Final: "P3 something"})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected fail 0, got %+v", res)
	}
}

func TestRegexCheckBadPatternIsZero(t *testing.T) {
	c := RegexCheck{Pattern: "[invalid"}
	res := c.Run(context.Background(), Input{Final: "x"})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected error→0, got %+v", res)
	}
}

func TestFileExistsHappy(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := FileExistsCheck{Path: p}.Run(context.Background(), Input{})
	if !res.Passed || res.Score != 100 {
		t.Fatalf("expected pass, got %+v", res)
	}
}

func TestFileExistsMiss(t *testing.T) {
	res := FileExistsCheck{Path: "/nonexistent/path/xyz"}.Run(context.Background(), Input{})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected fail, got %+v", res)
	}
}

func TestScriptCheckExitZero(t *testing.T) {
	res := ScriptCheck{Path: "true", TimeoutSeconds: 5}.Run(context.Background(), Input{})
	if !res.Passed || res.Score != 100 {
		t.Fatalf("expected pass, got %+v", res)
	}
}

func TestScriptCheckExitNonZero(t *testing.T) {
	res := ScriptCheck{Path: "false", TimeoutSeconds: 5}.Run(context.Background(), Input{})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected fail, got %+v", res)
	}
}

func TestScriptCheckMissingDoesNotCrash(t *testing.T) {
	res := ScriptCheck{Path: "/definitely/not/a/real/script-xyzzy", TimeoutSeconds: 5}.Run(context.Background(), Input{})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected error→0, got %+v", res)
	}
	if res.Note == "" {
		t.Fatal("expected note with error context")
	}
}

func TestRunnerRunMeans(t *testing.T) {
	tc := model.TestCase{QualityChecks: []model.QualityCheckSpec{
		{Script: "true"},
		{Script: "false"},
	}}
	r, err := Build(tc, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sig := r.Run(context.Background(), Input{})
	if sig == nil {
		t.Fatal("nil signal")
	}
	if sig.Score != 50 {
		t.Fatalf("expected mean=50, got %.2f", sig.Score)
	}
	if len(sig.Results) != 2 || len(sig.Notes) != 2 {
		t.Fatalf("expected 2 results+notes, got %+v", sig)
	}
}

func TestRunnerRunNilOnEmpty(t *testing.T) {
	var r *Runner
	if got := r.Run(context.Background(), Input{}); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
