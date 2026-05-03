# skillbench

`skillbench` is a local Go CLI for evaluating Agent Skills across Codex, Claude Code, and GitHub Copilot. It runs fresh CLI sessions, captures raw logs, normalizes messages/tools/skill invocation evidence, scores runs with hybrid metrics, and writes proposal artifacts without modifying live skills.

## Commands

```bash
go run ./cmd/skillbench --help
go run ./cmd/skillbench test codex -p "Use the skill" --skill ~/.codex/skills/cognee-memory --case case.yaml
go run ./cmd/skillbench suite --agent claude --skill ~/.claude/skills/my-skill --cases cases.yaml
go run ./cmd/skillbench extract --agent copilot --latest
go run ./cmd/skillbench analyze --run .skillbench/runs/<run-id>
go run ./cmd/skillbench research --agent codex --skill ~/.codex/skills/cognee-memory --cases cases.yaml --max-trials 20 --min-improvement 5
go run ./cmd/skillbench research --agent claude --skill ... --cases ... --proposer llm --proposer-agent claude
```

Artifacts are written under `.skillbench/runs` and `.skillbench/proposals`.

## Autoresearch loop

`skillbench research` runs an Autoresearch-style improvement loop:

1. Stage a temporary baseline copy of the skill.
2. Run the selected test case against that baseline.
3. Generate one temporary candidate skill using the current failure signal.
4. Run the same case against the candidate.
5. Compare weighted scores and deterministic assertion results.
6. Append a trial record to `.skillbench/research/<run-id>/ledger.jsonl`.
7. Write a proposal under `.skillbench/proposals/<skill>/<trial-id>/` only when the candidate clears the improvement gate.

The live skill directory is never modified.

The proposer is pluggable: `--proposer=deterministic` (default) uses six
hardcoded mutation strategies; `--proposer=llm` calls the configured agent
and parses a strict JSON response (`{strategy, hypothesis, skill_md}`),
falling back to the deterministic proposer on any failure.

## Branch-advance mode (operating manual)

For an unattended LLM agent driving the loop directly — editing SKILL.md
in place on a `skillbench/<tag>` branch, using `skillbench suite` as the
per-trial harness, and using git for keep/reset — see
[`program.md`](./program.md). This mode does modify the live SKILL.md on
the branch, so operators must merge consciously.
