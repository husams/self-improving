package quality

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ScriptCheck runs a shell script (or executable) and scores 100 on exit 0,
// 0 otherwise. Errors (missing script, signal kill, timeout) are translated
// into score 0 + note; the run does not abort.
type ScriptCheck struct {
	Path           string
	TimeoutSeconds int
}

func (ScriptCheck) Name() string { return "script" }

func (s ScriptCheck) Run(ctx context.Context, _ Input) Result {
	timeout := time.Duration(s.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "sh", "-c", s.Path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		note := fmt.Sprintf("%s exited with error: %v", s.Path, err)
		if len(out) > 0 {
			trimmed := strings.TrimSpace(string(out))
			if len(trimmed) > 200 {
				trimmed = trimmed[:200]
			}
			note += "; output: " + trimmed
		}
		return Result{Name: "script", Score: 0, Passed: false, Note: note}
	}
	return Result{Name: "script", Score: 100, Passed: true, Note: fmt.Sprintf("%s exited 0", s.Path)}
}
