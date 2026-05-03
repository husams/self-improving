package research

import (
	"context"

	"skillbench/internal/model"
)

type Proposer interface {
	Propose(ctx context.Context, in ProposerInput) (Candidate, error)
}

type ProposerInput struct {
	SkillContent string
	Case         model.TestCase
	Baseline     model.NormalizedRun
	History      []Trial
}

type Candidate struct {
	Content    string
	Strategy   string
	Hypothesis string
	Notes      []string
}
