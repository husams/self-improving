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

## First-run results (2026-05-03)

### Baselines (per-case overall scores)

| Case | claude | codex | copilot |
|---|---|---|---|
| outage-payments (P0) | 96.3 | 56.3 | 56.3 |
| regression-search (P1) | 96.3 | 56.3 | 56.3 |
| intermittent-rate-limit (P2) | **60.0** | 56.3 | 56.3 |
| cosmetic-admin (P3) | 95.0 | 56.3 | 56.3 |

All four `final_contains` assertions (sections + priority + keyword) pass on
codex and copilot for every case. Their score caps at ~56.3 because the
`skill_use` metric requires a `Skill` tool invocation event — under
rendered-fallback exposure neither codex nor copilot fires one. Claude's
session DOES emit a `Skill` tool_use call, so it earns the full
`skill_use`/`adherence` weight.

The single weak Claude case (`intermittent-rate-limit`) is the model
classifying the bug as P1 instead of P2 — under-specified SKILL.md leaves
that judgement to model defaults.

### Research run with deterministic proposer

```
trial-001 outage-payments         95.0 → 96.3 (+1.3)  discard (below threshold)
trial-002 regression-search       95.0 → 95.0 (+0.0)  discard
trial-003 intermittent-rate-limit 60.0 → 96.3 (+36.3) PROPOSE via verification-checklist
trial-004 cosmetic-admin          95.0 → 96.3 (+1.3)  discard
```

The deterministic proposer's verification-checklist strategy fixed the weak
case end-to-end. Proposal filed at
`.skillbench/proposals/bug-triage/trial-003/`.

### Research run with LLM proposer (`--proposer=llm --proposer-agent=claude`)

```
trial-001 outage-payments         96.3 → 96.3 (+0.0)  discard
trial-002 regression-search       95.0 → 55.0 (-40.0) discard
trial-003 intermittent-rate-limit 96.3 → 56.3 (-40.0) discard
trial-004 cosmetic-admin          96.3 → 56.3 (-40.0) discard
```

The LLM proposer wrote four novel strategies
(`add-structured-output-template`, `add-efficiency-guidance`,
`add-investigation-emphasis`, `add-low-effort-fix-guidance`). On the runs
where it landed a wholesale rewrite, scores dropped by ~40 points — likely
because the rewritten SKILL.md no longer matched the trigger pattern that
makes Claude emit a `Skill` tool_use, so `skill_use`/`adherence` zeroed.
The keep/reset gate correctly discarded all four.

Lessons:

1. **Baseline variance is real**: `intermittent-rate-limit` scored 60.0 in the
   deterministic run and 96.3 in the LLM-proposer run, on the same SKILL.md.
   Multi-seed baselines or paired comparisons matter.
2. **LLM rewrites are higher-variance than deterministic templates**: a full
   rewrite may break something downstream (here, the Skill-tool trigger). The
   hardcoded "append a checklist" approach is conservative-but-robust.
3. **The keep/reset gate is doing its job**: every regression was discarded;
   only the +36.3 case was proposed.
