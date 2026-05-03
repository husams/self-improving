---
name: Skill Evaluation Autoresearch Flow
status: stable
last_updated: 2026-05-03
---

# Skill Evaluation Autoresearch Flow

Implementation note for `skillbench`, the local Go CLI in this project that evaluates Agent Skills across Codex, Claude Code, and GitHub Copilot, then writes proposal-only improvement artifacts.

## Goal

Create a reusable harness for testing and improving skills:

1. Run a fresh CLI test against one agent and one skill.
2. Capture full logs.
3. Normalize messages, responses, tool calls, tool results, and skill invocation evidence.
4. Score the run with deterministic assertions plus heuristic/rubric-style metrics.
5. Use an Autoresearch-style loop to compare baseline vs candidate skill variants.
6. Write proposals only; never modify live skills automatically.

## Current Implementation

`skillbench` is implemented as a Go module with:

- `cmd/skillbench` - CLI entrypoint.
- `internal/runner` - runs `codex exec --json`, `claude -p --output-format stream-json`, and `copilot -p --output-format json`.
- `internal/adapters` - parses Codex, Claude, and Copilot JSONL logs into normalized events.
- `internal/skills` - loads `SKILL.md`, computes fingerprints, and chooses native vs rendered-fallback exposure.
- `internal/metrics` - scores task success, skill use, adherence, output quality, tool health, and efficiency.
- `internal/research` - creates temporary candidate skills and writes proposal artifacts only when a candidate beats baseline.
- `internal/store` - writes `.skillbench/runs/<run-id>/` and `.skillbench/proposals/<skill>/<trial-id>/`.

## CLI Surface

```bash
skillbench test <codex|claude|copilot> -p "<prompt>" --skill <path-to-skill> --case <case.yaml>
skillbench suite --agent <agent> --skill <path> --cases <cases.yaml>
skillbench extract --agent <agent> --latest
skillbench extract --agent <agent> --session <path-or-id>
skillbench analyze --run .skillbench/runs/<run-id>
skillbench research --agent <agent> --skill <path> --cases <cases.yaml> --max-trials 20 --min-improvement 5
```

## Metrics

Default weighted score:

- task success: 30%
- skill use: 20%
- skill adherence: 20%
- output quality: 15%
- tool health: 10%
- efficiency: 5%

Safety failure caps score at 40. Failed deterministic assertions cap score at 60.

## Notes

- Codex and Claude can expose native skill evidence when the tested skill is in their native skill directories.
- Copilot does not expose the same Agent Skills mechanism, so `skillbench` uses rendered `SKILL.md` fallback for Copilot.
- VS Code Copilot logs are treated as diagnostic fallback; `~/.copilot/session-state/*/events.jsonl` is the primary Copilot transcript source.
- Raw logs stay local under `.skillbench/runs` with restrictive permissions.

## Autoresearch Loop

The research command now follows the local Autoresearch pattern:

1. Stage the baseline skill in a temporary directory.
2. Run the selected case and score the baseline.
3. Choose a candidate strategy from the baseline failure signal.
4. Stage a temporary candidate skill copy with one focused change.
5. Run and score the candidate on the same case.
6. Append the trial to `.skillbench/research/<run-id>/ledger.jsonl`.
7. Write a proposal only if the candidate improves by at least `--min-improvement`, has no safety failure, has no deterministic assertion failure, and does not regress passed assertions.

The candidate strategies cover trigger/skill-use guidance, verification checklists, tool hygiene, final-answer quality, compact workflow structure, and safety tightening. Live skills are never modified.
