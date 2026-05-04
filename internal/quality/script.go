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
	trimmed := strings.TrimSpace(string(out))
	if len(trimmed) > 200 {
		trimmed = trimmed[:200]
	}
	if err != nil {
		// Exit code 2 is the skip sentinel (missing creds, env not configured).
		// Skipped results are excluded from the mean by Runner.Run.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {
			note := "skipped"
			if trimmed != "" {
				note = "skipped: " + trimmed
			}
			return Result{Name: "script", Score: 0, Passed: false, Skipped: true, Note: note}
		}
		note := fmt.Sprintf("%s exited with error: %v", s.Path, err)
		if trimmed != "" {
			note += "; output: " + trimmed
		}
		return Result{Name: "script", Score: 0, Passed: false, Note: note}
	}
	return Result{Name: "script", Score: 100, Passed: true, Note: fmt.Sprintf("%s exited 0", s.Path)}
}
