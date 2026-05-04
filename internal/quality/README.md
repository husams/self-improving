# internal/quality — deterministic OutputQuality channel

The `quality` package runs deterministic per-case checks declared under
`quality_checks:` in `cases.yaml` and produces a 0–100 signal that the
metrics layer blends into `OutputQuality`. It complements the LLM judge
without replacing it: when both signals are present, `OutputQuality` is
the 50/50 blend; when only one is present, that signal flows through
unblended.

The dependency direction is one-way: `cmd/skillbench` builds a
`quality.Runner`, runs it, and passes the `*QualitySignal` into
`metrics.ScoreWithVerdictAndQuality`. `internal/metrics` does **not**
import `internal/quality`.

See the design doc: `~/workspace/wiki/pages/planning/skillbench-deterministic-quality.md`.

## Check matrix

| Kind          | YAML field                                                           | Score |
|---------------|----------------------------------------------------------------------|-------|
| `script`      | `script: ./check.sh`                                                 | 100 if exit 0 else 0 |
| `file_exists` | `file_exists: out/result.json`                                       | 100 if `os.Stat` succeeds else 0 |
| `json_schema` | `json_schema: {file: doc.json, schema: schema.json}`                 | 100 if doc validates else 0 |
| `regex`       | `regex: "^Priority: P0"`                                             | 100 if pattern matches the final assistant message else 0 |
| `similarity`  | `similarity: {reference: "...", method: rouge\|bleu\|llm, threshold: 0.75}` | raw 0–100 (or 100/0 if `threshold` set) |

A `QualityCheckSpec` must have **exactly one** of these fields populated;
empty or multi-kind specs are rejected at `quality.Build` time so YAML
typos surface fast.

## Scoring rules

- Each check returns `(score float64, passed bool, note string)`.
- Binary checks: pass = 100, fail = 0.
- Similarity without `threshold`: raw 0–100. With `threshold`: passed iff
  `score/100 >= threshold` → 100 / 0.
- A check error (missing script, unreadable file, malformed schema,
  unparseable regex) → score 0, `passed=false`, note appended; **the run
  does not crash**. This mirrors the judge fallback discipline at
  `internal/judge/llm.go:58-66`.
- `script` checks may **skip** by exiting with code `2`. A skipped
  result has `Skipped=true`, is **excluded from the mean**, and is
  surfaced in notes as `script: skipped: <stdout>`. Use this for
  preconditions you don't control (e.g. missing live-API creds) so a
  missing env var doesn't silently inflate or deflate the score.
- `deterministic_quality = mean(scores over non-skipped checks)`. Empty
  `quality_checks` OR all checks skipped → no signal (caller passes `nil`).

## Resolution order for OutputQuality

(`metrics.ScoreWithVerdictAndQuality`)

| `quality_checks` | judge `OutputQuality` | Result                               |
|------------------|-----------------------|--------------------------------------|
| Yes              | Yes                   | `0.5*det + 0.5*judge`                |
| Yes              | No                    | `det` alone                          |
| No               | Yes                   | judge alone (current)                |
| No               | No                    | length heuristic (BIT-IDENTICAL legacy) |

Critical asymmetry vs the judge wire-in: `quality_checks` does **not**
require a `rubric["output_quality"]` key. Its presence in YAML is itself
the authored signal (Open Q9 in the design doc, signed off 2026-05-04).

## Caps and safety boundary — unchanged

- `SafetyFailure` cap (Overall→40) and `DeterministicFail` cap
  (Overall→60) are unchanged.
- The deterministic-quality channel scores **quality**, not correctness.
  It does NOT set `m.DeterministicFail`. That flag remains driven by
  `tc.Assertions` (`final_contains` / `tool_called` / `tool_not_called`
  / `files_created`).

## Similarity backends

`internal/quality/similarity/` provides four backends:

- `rouge` — pure-Go ROUGE-L F1. **Default.** No external deps; cheap;
  deterministic; weak on paraphrase.
- `bleu` — pure-Go BLEU-4 with NIST method-1 smoothing.
- `llm` — strict-JSON contract `{"similarity": <0-100>, "notes": [...]}`,
  parsed with the same fence/clamp tolerance as the judge. Errors fall
  back to ROUGE with a note. See `prompts/similarity-system.md`.
- `embedding` — **deferred to v2.** Selecting it falls back to ROUGE.

## CLI flags (test, suite, research, analyze)

```
--quality on|off                                     (default on)
--quality-similarity-method rouge|bleu|llm|embedding (default rouge)
--quality-similarity-agent <agent>                   (required when method=llm)
--quality-similarity-system <path>                   (overrides embedded prompt)
--quality-script-timeout 30                          (per-script seconds)
```

`--quality off` → no quality runner is built; the metrics layer falls
through to its bit-identical legacy path. Existing `cases.yaml` files
without `quality_checks` round-trip unchanged.

## Trust model for `script` checks

Scripts run with `sh -c "<value>"` in the current working directory using
the user's environment. There is no sandbox. `--quality-script-timeout`
is a foot-gun mitigator, not a security boundary. Treat `quality_checks`
declarations the same way you'd treat any code committed alongside the
suite: only run cases from sources you trust.
