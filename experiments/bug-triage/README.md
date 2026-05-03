# bug-triage experiment

A skillbench experiment that probes whether an autoresearch loop can
improve a deliberately under-specified bug-triage skill.

## Skill under test

`skill/SKILL.md` — 7 lines. Has a name, a one-line description, and the
instruction "Read the bug report and decide a priority." Nothing about
output format, severity rubric, or factual grounding. Lots of room for
improvement.

## Cases

Four bug reports, one per priority tier:

| Case | Fixture | Expected priority | Why |
|---|---|---|---|
| `outage-payments` | `payment-gateway-down.md` | P0 | 100% checkout failure, $4,200/min revenue loss, recent deploy |
| `regression-search` | `search-pagination-broken.md` | P1 | Real revenue impact, real users affected, workaround exists |
| `intermittent-rate-limit` | `api-rate-limit-flaky.md` | P2 | Intermittent (2%), workaround exists, partner visibility issue |
| `cosmetic-admin` | `admin-sort-misalign.md` | P3 | Internal tool, cosmetic only, no functional impact |

Each case requires four labeled sections in the output: `Priority`,
`Rationale`, `Impact`, `Recommended Action`. Deterministic assertions
check the correct priority code (P0/P1/P2/P3) plus a domain keyword
characteristic of the reasoning.

## How to run

```bash
# Baseline
bin/skillbench suite --agent claude  --skill experiments/bug-triage/skill --cases experiments/bug-triage/cases.yaml
bin/skillbench suite --agent codex   --skill experiments/bug-triage/skill --cases experiments/bug-triage/cases.yaml
bin/skillbench suite --agent copilot --skill experiments/bug-triage/skill --cases experiments/bug-triage/cases.yaml

# Improvement loop (deterministic proposer, default)
bin/skillbench research --agent claude --skill experiments/bug-triage/skill --cases experiments/bug-triage/cases.yaml --max-trials 4 --min-improvement 5

# Improvement loop with LLM proposer
bin/skillbench research --agent claude --skill experiments/bug-triage/skill --cases experiments/bug-triage/cases.yaml \
  --proposer llm --proposer-agent claude --max-trials 4
```
