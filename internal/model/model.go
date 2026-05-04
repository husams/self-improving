package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Agent string

const (
	AgentCodex   Agent = "codex"
	AgentClaude  Agent = "claude"
	AgentCopilot Agent = "copilot"
)

type Exposure string

const (
	ExposureNative           Exposure = "native"
	ExposureRenderedFallback Exposure = "rendered_fallback"
)

type EventType string

const (
	EventUserMessage     EventType = "user_message"
	EventAssistant       EventType = "assistant_message"
	EventToolCall        EventType = "tool_call"
	EventToolResult      EventType = "tool_result"
	EventSkillInvocation EventType = "skill_invocation"
	EventError           EventType = "error"
	EventSystem          EventType = "system"
)

type TestCase struct {
	ID               string            `json:"id" yaml:"id"`
	Prompt           string            `json:"prompt" yaml:"prompt"`
	ExpectedSkill    string            `json:"expected_skill" yaml:"expected_skill"`
	FixturePath      string            `json:"fixture_path,omitempty" yaml:"fixture_path"`
	WorkspaceFixture string            `json:"workspace_fixture" yaml:"workspace_fixture"`
	TimeoutSeconds   int               `json:"timeout_seconds" yaml:"timeout_seconds"`
	Assertions       Assertions        `json:"assertions" yaml:"assertions"`
	Rubric           map[string]string `json:"rubric" yaml:"rubric"`
}

type Assertions struct {
	FinalContains []string `json:"final_contains" yaml:"final_contains"`
	ToolCalled    []string `json:"tool_called" yaml:"tool_called"`
	ToolNotCalled []string `json:"tool_not_called" yaml:"tool_not_called"`
	FilesCreated  []string `json:"files_created" yaml:"files_created"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SKILLPath   string `json:"skill_path"`
	Fingerprint string `json:"fingerprint"`
	Content     string `json:"-"`
}

type Event struct {
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
	Role      string          `json:"role,omitempty"`
	Name      string          `json:"name,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Text      string          `json:"text,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
}

type AssertionResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

type Metrics struct {
	Overall           float64           `json:"overall"`
	TaskSuccess       float64           `json:"task_success"`
	SkillUse          float64           `json:"skill_use"`
	SkillAdherence    float64           `json:"skill_adherence"`
	OutputQuality     float64           `json:"output_quality"`
	ToolHealth        float64           `json:"tool_health"`
	Efficiency        float64           `json:"efficiency"`
	SafetyFailure     bool              `json:"safety_failure"`
	DeterministicFail bool              `json:"deterministic_fail"`
	Assertions        []AssertionResult `json:"assertions"`
	Notes             []string          `json:"notes,omitempty"`
	JudgeStatus       string            `json:"judge_status,omitempty"`
}

type NormalizedRun struct {
	RunID       string    `json:"run_id"`
	Agent       Agent     `json:"agent"`
	Case        TestCase  `json:"case"`
	Skill       SkillInfo `json:"skill"`
	Exposure    Exposure  `json:"exposure"`
	Events      []Event   `json:"events"`
	Metrics     Metrics   `json:"metrics"`
	RawLogPaths []string  `json:"raw_log_paths"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	ExitCode    int       `json:"exit_code"`
}

type casesFile struct {
	Cases []TestCase `yaml:"cases"`
}

func LoadCases(path string) ([]TestCase, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapped casesFile
	if err := yaml.Unmarshal(b, &wrapped); err == nil && len(wrapped.Cases) > 0 {
		return wrapped.Cases, nil
	}
	var list []TestCase
	if err := yaml.Unmarshal(b, &list); err == nil && len(list) > 0 {
		return list, nil
	}
	var single TestCase
	if err := yaml.Unmarshal(b, &single); err != nil {
		return nil, err
	}
	if single.Prompt == "" && single.ID == "" {
		return nil, fmt.Errorf("no test cases found in %s", path)
	}
	return []TestCase{single}, nil
}

func NewRunID(agent Agent, caseID string) string {
	safe := strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(caseID)
	if safe == "" {
		safe = "case"
	}
	return fmt.Sprintf("%s-%s-%s", agent, time.Now().UTC().Format("20060102T150405.000000000"), safe)
}

func Fingerprint(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

func SkillNameFromPath(path string) string {
	base := filepath.Base(path)
	if base == "SKILL.md" {
		return filepath.Base(filepath.Dir(path))
	}
	return base
}
