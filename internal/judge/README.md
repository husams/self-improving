# internal/judge

LLM-backed rubric scorer for skillbench. A `Judge` reads a finished case run
(test case, skill content, event trace, final assistant message) and returns
per-sub-metric scores aligned with the rubric questions authored on the test
case. The metrics package consumes these via `metrics.ScoreWithVerdict` to
blend or replace heuristic values without changing weights or caps.

## Implementations

- `NoopJudge` — returns `Verdict{Status: "skipped"}`. Default when no `--judge`
  flag is set; the heuristic-only `metrics.Score` path is preserved
  bit-for-bit.
- `LLMJudge` — calls a configured agent through a `JudgeRun` callback,
  parses strict JSON, and falls back to its `Fallback` (typically `NoopJudge`)
  on any failure (runner error, empty output, unparseable JSON, no scored
  keys). Mirrors `internal/research.LLMProposer`.

## Rubric-key → metric mapping

| `case.rubric` key | `Verdict` field   | `model.Metrics` slot it fills          |
| ----------------- | ----------------- | -------------------------------------- |
| `task_success`    | `TaskSuccess`     | `TaskSuccess` — blended 50/50 with assertion ratio when assertions exist; judge-only when none |
| `skill_adherence` | `SkillAdherence`  | `SkillAdherence` — replaces the heuristic                                  |
| `output_quality`  | `OutputQuality`   | `OutputQuality` — replaces the length heuristic                            |

A rubric key with empty / missing text is omitted from the prompt; the
corresponding `Verdict.*` pointer stays `nil`; the heuristic value is kept.

## Prompt contract

The judge LLM must respond with a strict JSON object:

```json
{
  "task_success": 88,
  "skill_adherence": 75,
  "output_quality": 90,
  "notes": ["P0 priority correct", "all four sections present"]
}
```

Rules enforced by the parser:

- No prose, no fences (fenced output is stripped before parse).
- Only the rubric keys that were asked for are expected; missing keys leave
  the corresponding pointer `nil`.
- Numeric values are clamped to `[0, 100]`.
- A response containing `notes` only and no scores → fallback.

The full system prompt lives at `prompts/judge-system.md` (embedded via
`go:embed`) and can be overridden per-call with `LLMJudge.SystemFile`.

## Caps preserved

The deterministic safety boundary at `metrics.go:92` (SafetyFailure → 40) and
`metrics.go:96` (DeterministicFail → 60) is unchanged. The judge can only
move a sub-metric within those bounds: a verdict of 95 across the board
combined with a failed `final_contains` assertion still produces an Overall
score capped at 60. See `TestScoreWithVerdictDeterministicCapStillApplies`.

## CLI flags

Available on `test`, `suite`, `research`, and `analyze`:

```
--judge none|llm           default: none (heuristic-only path; bit-identical to legacy)
--judge-agent <agent>      required when --judge=llm
--judge-system <path>      override the embedded prompt
```

`analyze` re-calls the judge each time it is invoked with `--judge=llm`; no
on-disk caching in v1.
