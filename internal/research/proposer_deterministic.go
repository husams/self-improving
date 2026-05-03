package research

import "context"

type DeterministicProposer struct{}

func (DeterministicProposer) Propose(_ context.Context, in ProposerInput) (Candidate, error) {
	s := chooseStrategy(in.TrialIndex, in.Case, in.Baseline)
	return Candidate{
		Content:    s.Apply(in.SkillContent, in.Case, in.Baseline),
		Strategy:   s.Name,
		Hypothesis: s.Hypothesis,
	}, nil
}
