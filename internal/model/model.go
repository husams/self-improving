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
	QualityChecks    []QualityCheckSpec `json:"quality_checks,omitempty" yaml:"quality_checks,omitempty"`
	ProposerContext  ProposerContext   `json:"proposer_context,omitempty" yaml:"proposer_context,omitempty"`
	ScoreWeights     *ScoreWeights     `json:"score_weights,omitempty" yaml:"score_weights,omitempty"`
}

// ScoreWeights configures the dimension weights used to blend the overall
// score. Any field left unset falls back to the package default. Weights are
// normalized to sum to 1.0 before blending, so authors can write whole numbers
// (e.g. output_quality: 3 to triple its influence) and still get a 0–100
// overall.
type ScoreWeights struct {
	TaskSuccess    *float64 `json:"task_success,omitempty" yaml:"task_success,omitempty"`
	SkillUse       *float64 `json:"skill_use,omitempty" yaml:"skill_use,omitempty"`
	SkillAdherence *float64 `json:"skill_adherence,omitempty" yaml:"skill_adherence,omitempty"`
	OutputQuality  *float64 `json:"output_quality,omitempty" yaml:"output_quality,omitempty"`
	ToolHealth     *float64 `json:"tool_health,omitempty" yaml:"tool_health,omitempty"`
	Efficiency     *float64 `json:"efficiency,omitempty" yaml:"efficiency,omitempty"`
}

// ProposerContext lets the case author opt in to extra signals that the
// research-loop LLM proposer receives when crafting candidate edits. The
// default zero value preserves the original behavior (case + baseline metrics
// only); experiments enable the fields they want fed back.
type ProposerContext struct {
	// IncludeFailedAssertions injects a list of failed final_contains /
	// tool_called / etc. assertions from the baseline run so the proposer
	// can target the actual gap, not guess from the score alone.
	IncludeFailedAssertions bool `json:"include_failed_assertions,omitempty" yaml:"include_failed_assertions,omitempty"`
}

// QualityCheckSpec is the YAML/JSON shape for a single deterministic quality
// check. Exactly one of the optional fields should be populated; validation
// happens at decode time in internal/quality.
type QualityCheckSpec struct {
	Script     string                 `json:"script,omitempty" yaml:"script,omitempty"`
	FileExists string                 `json:"file_exists,omitempty" yaml:"file_exists,omitempty"`
	JSONSchema *JSONSchemaCheckSpec   `json:"json_schema,omitempty" yaml:"json_schema,omitempty"`
	Regex      string                 `json:"regex,omitempty" yaml:"regex,omitempty"`
	Similarity *SimilarityCheckSpec   `json:"similarity,omitempty" yaml:"similarity,omitempty"`
}

// JSONSchemaCheckSpec validates a JSON file against a JSON Schema document.
type JSONSchemaCheckSpec struct {
	File   string `json:"file" yaml:"file"`
	Schema string `json:"schema" yaml:"schema"`
}

// SimilarityCheckSpec scores the final assistant message against a reference
// answer using the configured method (rouge | bleu | llm | embedding).
type SimilarityCheckSpec struct {
	Reference string  `json:"reference" yaml:"reference"`
	Method    string  `json:"method,omitempty" yaml:"method,omitempty"`
	Threshold float64 `json:"threshold,omitempty" yaml:"threshold,omitempty"`
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
