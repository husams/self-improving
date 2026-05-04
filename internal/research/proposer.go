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
	// SkillFiles is the full skill tree as a map of relative path → contents.
	// Includes SKILL.md plus any other files (scripts/, references/, etc.).
	SkillFiles map[string]string
	Case       model.TestCase
	Baseline   model.NormalizedRun
	History    []Trial
	TrialIndex int
}

// Candidate is the proposer's output: a set of whole-file replacements keyed
// by skill-relative paths. An empty/nil Files map means "no change".
type Candidate struct {
	Files      map[string]string
	Strategy   string
	Hypothesis string
	Notes      []string
}
