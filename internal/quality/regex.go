package quality

import (
	"context"
	"fmt"
	"regexp"
)

// RegexCheck matches Pattern against the final assistant message. Binary:
// 100 on match, 0 otherwise. Compile errors translate to score 0 + note.
type RegexCheck struct {
	Pattern string
}

func (RegexCheck) Name() string { return "regex" }

func (r RegexCheck) Run(_ context.Context, in Input) Result {
	re, err := regexp.Compile(r.Pattern)
	if err != nil {
		return Result{Name: "regex", Score: 0, Passed: false, Note: fmt.Sprintf("compile %q: %v", r.Pattern, err)}
	}
	if re.MatchString(in.Final) {
		return Result{Name: "regex", Score: 100, Passed: true, Note: fmt.Sprintf("%q matched", r.Pattern)}
	}
	return Result{Name: "regex", Score: 0, Passed: false, Note: fmt.Sprintf("%q did not match", r.Pattern)}
}
