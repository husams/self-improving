package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"skillbench/internal/adapters"
	"skillbench/internal/metrics"
	"skillbench/internal/model"
	"skillbench/internal/research"
	"skillbench/internal/runner"
	"skillbench/internal/skills"
	"skillbench/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "skillbench:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "test":
		return runTest(args[1:])
	case "suite":
		return runSuite(args[1:])
	case "extract":
		return runExtract(args[1:])
	case "analyze":
		return runAnalyze(args[1:])
	case "research":
		return runResearch(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Println(`skillbench - evaluate and improve Agent Skills

Commands:
  test <codex|claude|copilot> -p <prompt> --skill <path> --case <case.yaml>
  suite --agent <agent> --skill <path> --cases <cases.yaml>
  extract --agent <agent> --latest | --session <path-or-id>
  analyze --run .skillbench/runs/<run-id>
  research --agent <agent> --skill <path> --cases <cases.yaml> --max-trials 20 --min-improvement 5`)
}

func runTest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing agent")
	}
	agent := model.Agent(args[0])
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	prompt := fs.String("p", "", "prompt")
	skillPath := fs.String("skill", "", "path to skill directory or SKILL.md")
	casePath := fs.String("case", "", "case yaml")
	timeout := fs.Int("timeout", 0, "override timeout seconds")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	tc, err := loadCase(*casePath, *prompt)
	if err != nil {
		return err
	}
	if *timeout > 0 {
		tc.TimeoutSeconds = *timeout
	}
	run, err := executeCase(context.Background(), agent, *skillPath, tc)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", run.RunID)
	fmt.Printf("score: %.1f\n", run.Metrics.Overall)
	fmt.Printf("report: %s\n", filepath.Join(".skillbench", "runs", run.RunID, "report.md"))
	return nil
}

func runSuite(args []string) error {
	fs := flag.NewFlagSet("suite", flag.ContinueOnError)
	agent := fs.String("agent", "", "agent")
	skillPath := fs.String("skill", "", "path to skill")
	casesPath := fs.String("cases", "", "cases yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cases, err := loadCases(*casesPath)
	if err != nil {
		return err
	}
	var failed int
	for _, tc := range cases {
		run, err := executeCase(context.Background(), model.Agent(*agent), *skillPath, tc)
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "%s: %v\n", tc.ID, err)
			continue
		}
		fmt.Printf("%s\t%s\t%.1f\n", tc.ID, run.RunID, run.Metrics.Overall)
	}
	if failed > 0 {
		return fmt.Errorf("%d case(s) failed", failed)
	}
	return nil
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	agent := fs.String("agent", "", "agent")
	latest := fs.Bool("latest", false, "extract latest session")
	session := fs.String("session", "", "session path or id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *agent == "" {
		return fmt.Errorf("--agent is required")
	}
	if !*latest && *session == "" {
		return fmt.Errorf("use --latest or --session")
	}
	path := *session
	var err error
	if *latest {
		path, err = adapters.LatestSessionPath(model.Agent(*agent))
		if err != nil {
			return err
		}
	}
	events, err := adapters.ParseSessionFile(model.Agent(*agent), path)
	if err != nil {
		return err
	}
	enc := store.JSONEncoder(os.Stdout)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			return err
		}
	}
	return nil
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	runPath := fs.String("run", "", "run directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runPath == "" {
		return fmt.Errorf("--run is required")
	}
	run, err := store.ReadRun(*runPath)
	if err != nil {
		return err
	}
	run.Metrics = metrics.Score(run.Case, run.Events)
	if err := store.WriteRun(*runPath, run); err != nil {
		return err
	}
	fmt.Printf("score: %.1f\n", run.Metrics.Overall)
	fmt.Printf("report: %s\n", filepath.Join(*runPath, "report.md"))
	return nil
}

func runResearch(args []string) error {
	fs := flag.NewFlagSet("research", flag.ContinueOnError)
	agent := fs.String("agent", "", "agent")
	skillPath := fs.String("skill", "", "path to skill")
	casesPath := fs.String("cases", "", "cases yaml")
	maxTrials := fs.Int("max-trials", 20, "max trials")
	minImprovement := fs.Float64("min-improvement", 5, "minimum score improvement required for a proposal")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cases, err := loadCases(*casesPath)
	if err != nil {
		return err
	}
	cfg := research.Config{
		Agent:          model.Agent(*agent),
		SkillPath:      *skillPath,
		Cases:          cases,
		MaxTrials:      *maxTrials,
		MinImprovement: *minImprovement,
		Executor:       executeCase,
	}
	result, err := research.Run(context.Background(), cfg)
	if err != nil {
		return err
	}
	fmt.Printf("research: %s\n", result.ResearchDir)
	fmt.Printf("trials: %d\n", len(result.Trials))
	fmt.Printf("proposals: %d\n", len(result.Proposals))
	for _, p := range result.Proposals {
		fmt.Println(p.Path)
	}
	return nil
}

func executeCase(ctx context.Context, agent model.Agent, skillPath string, tc model.TestCase) (model.NormalizedRun, error) {
	if agent == "" {
		return model.NormalizedRun{}, fmt.Errorf("agent is required")
	}
	info, err := skills.Load(skillPath)
	if err != nil {
		return model.NormalizedRun{}, err
	}
	exposure := skills.ExposureFor(agent, info)
	prompt, err := promptWithFixture(tc)
	if err != nil {
		return model.NormalizedRun{}, err
	}
	if exposure == model.ExposureRenderedFallback {
		prompt = skills.RenderPrompt(info, prompt)
	}
	timeout := time.Duration(tc.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	runID := model.NewRunID(agent, tc.ID)
	rr, err := runner.NewCLI().Run(ctx, runner.Spec{
		Agent:   agent,
		Prompt:  prompt,
		WorkDir: ".",
		RunID:   runID,
		Timeout: timeout,
	})
	if err != nil {
		return model.NormalizedRun{}, err
	}
	events, err := adapters.Parse(agent, rr.Stdout)
	if err != nil {
		return model.NormalizedRun{}, err
	}
	if len(events) == 0 && rr.RawSessionPath != "" {
		events, _ = adapters.ParseSessionFile(agent, rr.RawSessionPath)
	}
	if rr.ExitCode != 0 {
		events = append(events, model.Event{Type: model.EventError, Text: strings.TrimSpace(string(rr.Stderr)), Timestamp: time.Now()})
	}
	nrun := model.NormalizedRun{
		RunID:       runID,
		Agent:       agent,
		Case:        tc,
		Skill:       info,
		Exposure:    exposure,
		Events:      events,
		RawLogPaths: []string{},
		StartedAt:   rr.StartedAt,
		FinishedAt:  rr.FinishedAt,
		ExitCode:    rr.ExitCode,
	}
	nrun.Metrics = metrics.Score(tc, events)
	dir, err := store.WriteNewRun(nrun, rr)
	if err != nil {
		return model.NormalizedRun{}, err
	}
	nrun.RawLogPaths = []string{filepath.Join(dir, "raw", "stdout.jsonl"), filepath.Join(dir, "raw", "stderr.log")}
	return nrun, store.WriteRun(dir, nrun)
}

func loadCase(path, promptOverride string) (model.TestCase, error) {
	var tc model.TestCase
	if path != "" {
		cases, err := loadCases(path)
		if err != nil {
			return tc, err
		}
		if len(cases) == 0 {
			return tc, fmt.Errorf("no cases in %s", path)
		}
		tc = cases[0]
	}
	if promptOverride != "" {
		tc.Prompt = promptOverride
	}
	if tc.ID == "" {
		tc.ID = "ad-hoc-" + strconv.FormatInt(time.Now().Unix(), 10)
	}
	if tc.Prompt == "" {
		return tc, fmt.Errorf("prompt is required")
	}
	return tc, nil
}

func loadCases(path string) ([]model.TestCase, error) {
	if path == "" {
		return nil, fmt.Errorf("cases path is required")
	}
	cases, err := model.LoadCases(path)
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Dir(path)
	for i := range cases {
		cases[i].FixturePath = resolveCasePath(baseDir, cases[i].FixturePath)
		cases[i].WorkspaceFixture = resolveCasePath(baseDir, cases[i].WorkspaceFixture)
	}
	return cases, nil
}

func resolveCasePath(baseDir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

func promptWithFixture(tc model.TestCase) (string, error) {
	fixturePath := tc.FixturePath
	if fixturePath == "" {
		fixturePath = tc.WorkspaceFixture
	}
	if fixturePath == "" {
		return tc.Prompt, nil
	}
	b, err := os.ReadFile(fixturePath)
	if err != nil {
		return "", fmt.Errorf("read fixture %s: %w", fixturePath, err)
	}
	return fmt.Sprintf("%s\n\nFixture document (%s):\n\n<fixture>\n%s\n</fixture>", tc.Prompt, filepath.Base(fixturePath), strings.TrimSpace(string(b))), nil
}
