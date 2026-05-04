package research

import "context"

type DeterministicProposer struct{}

func (DeterministicProposer) Propose(_ context.Context, in ProposerInput) (Candidate, error) {
	s := chooseStrategy(in.TrialIndex, in.Case, in.Baseline)
	return Candidate{
		Files:      map[string]string{"SKILL.md": s.Apply(in.SkillContent, in.Case, in.Baseline)},
		Strategy:   s.Name,
		Hypothesis: s.Hypothesis,
	}, nil
}
