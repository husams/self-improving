package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"skillbench/internal/model"
)

type Spec struct {
	Agent   model.Agent
	Prompt  string
	WorkDir string
	RunID   string
	Timeout time.Duration
}

type Result struct {
	Stdout         []byte
	Stderr         []byte
	ExitCode       int
	StartedAt      time.Time
	FinishedAt     time.Time
	RawSessionPath string
}

type Runner interface {
	Run(context.Context, Spec) (Result, error)
}

type CLI struct{}

func NewCLI() CLI {
	return CLI{}
}

func (CLI) Run(ctx context.Context, spec Spec) (Result, error) {
	if spec.Timeout <= 0 {
		spec.Timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()
	name, args, err := command(spec)
	if err != nil {
		return Result{}, err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = spec.WorkDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	err = cmd.Run()
	finished := time.Now()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), StartedAt: started, FinishedAt: finished}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() == context.DeadlineExceeded {
		return res, fmt.Errorf("%s timed out after %s", spec.Agent, spec.Timeout)
	}
	if err != nil {
		// Return the result for analysis; non-zero exits are represented in events.
		return res, nil
	}
	return res, nil
}

func command(spec Spec) (string, []string, error) {
	switch spec.Agent {
	case model.AgentCodex:
		return "codex", []string{"exec", "--json", "--sandbox", "read-only", "--skip-git-repo-check", "-C", spec.WorkDir, spec.Prompt}, nil
	case model.AgentClaude:
		return "claude", []string{"-p", spec.Prompt, "--output-format", "stream-json", "--verbose", "--permission-mode", "bypassPermissions"}, nil
	case model.AgentCopilot:
		return "copilot", []string{"-p", spec.Prompt, "--output-format", "json", "--stream", "off", "--log-level", "error", "--no-custom-instructions"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported agent %q", spec.Agent)
	}
}
