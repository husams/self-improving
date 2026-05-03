# skillbench program

This is the operating manual for an LLM agent driving the `skillbench`
autoresearch loop in **branch-advance mode**: the LLM edits SKILL.md
in place on a `skillbench/<tag>` branch, runs `skillbench suite` as the
per-trial harness, and uses git for keep/reset. Live SKILL.md is modified
on the branch — operators must merge consciously.

For the alternative — the LLM running *inside* `skillbench research` as a
proposer with proposals-only output — see `skillbench research --proposer=llm`
in the README; no `program.md` needed for that mode.

## Setup

Work with the user to:

1. **Pick an experiment**: a directory under `experiments/` containing
   `skill/SKILL.md`, `cases.yaml`, and `fixtures/`.
   `experiments/basic-doc-reading/` is the reference example.
2. **Pick an adapter**: `claude`, `codex`, or `copilot`. Verify the chosen
   CLI is on `$PATH`.
3. **Agree on a run tag**: propose a tag based on today's date and adapter,
   e.g. `mar5-claude`. The branch `skillbench/<tag>` must not already exist.
4. **Create the branch**: `git checkout -b skillbench/<tag>` from `main`.
   Refuse to run if `git status` is not clean.
5. **Read the in-scope files**:
   - `README.md` and `docs/skill-evaluation-autoresearch-flow.md` — repo context.
   - `experiments/<exp>/skill/SKILL.md` — the file you propose edits to.
   - `experiments/<exp>/cases.yaml` and `experiments/<exp>/fixtures/*` —
     the fixed evaluation. Do not modify.
   - `internal/metrics/metrics.go` — how scores are computed.
6. **Verify the harness**: `go build -o bin/skillbench ./cmd/skillbench`.
7. **Establish the baseline**:

   ```
   bin/skillbench suite \
     --agent <agent> \
     --skill experiments/<exp>/skill \
     --cases experiments/<exp>/cases.yaml > run.log 2>&1
   ```

   Record per-case scores. The first ledger row is `commit=<HEAD>`,
   `status=baseline`, `description=baseline`.
8. **Confirm and go**.

## What you CAN do

- Edit `experiments/<exp>/skill/SKILL.md` only.
- Read any file under the repo to inform your proposal.
- Run `bin/skillbench suite ...` and `bin/skillbench analyze ...`.
- Read `.skillbench/runs/<run-id>/report.md` and `metrics.json`.
- Read `.skillbench/research/<id>/ledger.jsonl` from prior runs.

## What you CANNOT do

- Modify `cases.yaml`, anything under `fixtures/`, `internal/`, `cmd/`,
  or `tests/`.
- Add new dependencies (no `go.mod` edits).
- Edit a SKILL.md outside the chosen experiment directory.
- Merge `skillbench/<tag>` into `main` — that is the human's call.
- Print, commit, or expose any token, key, or credential.

## The goal

Maximize per-case `metrics.Overall` (0–100) on the chosen experiment's
cases without regressing any single case below its baseline.

Score weights live in `internal/metrics/metrics.go`:

- task_success 30
- skill_use 20
- skill_adherence 20
- output_quality 15
- tool_health 10
- efficiency 5

Safety failures cap the score at 40. Failed deterministic assertions cap
it at 60.

**Simplicity criterion**: a small score gain that bloats SKILL.md is not
worth it. SKILL.md should stay ≤ 500 lines. Removing content while holding
score is a clear win.

## The trial loop

LOOP FOREVER (or until the user interrupts):

1. **Inspect**: read the latest ledger entry and current SKILL.md.
2. **Hypothesize**: name one specific change. Examples:
   "trigger description is too narrow; broadening to intent-based phrasing
   should raise skill_use" or "verification step is missing; final answer
   omits assertion targets". Avoid bundling.
3. **Edit**: make the smallest edit to SKILL.md that tests the hypothesis.
4. **Commit**:

   ```
   git add experiments/<exp>/skill/SKILL.md
   git commit -m "trial-NNN: <hypothesis>"
   ```

5. **Run**:

   ```
   bin/skillbench suite \
     --agent <agent> \
     --skill experiments/<exp>/skill \
     --cases experiments/<exp>/cases.yaml > run.log 2>&1
   ```

6. **Read result**: each line in `run.log` is `<case-id>\t<run-id>\t<score>`.
   If empty or the command exited non-zero, the run failed; `tail -n 50 run.log`
   and decide fix-and-rerun (typo, missing CLI) vs revert (broken hypothesis).
7. **Decide**:
   - **Keep** if every case score ≥ its baseline AND at least one case
     improved by ≥ 5 points. The new commit becomes the new baseline.
   - **Discard** otherwise: `git reset --hard HEAD~1`. The ledger entry is
     still recorded.
8. **Log** to `results.tsv` (tab-separated, do not commit it):

   ```
   trial_id	commit	avg_score	per_case	status	description
   trial-007	a1b2c3d	74.5	doc1=80,doc2=72,doc3=72	keep	tighten trigger
   ```

   `status` ∈ {`baseline`, `keep`, `discard`, `crash`}.

9. **Continue**.

## Output format

`bin/skillbench suite ...` prints one line per case:

```
<case-id>\t<run-id>\t<overall-score>
```

For deeper inspection read `.skillbench/runs/<run-id>/report.md` and
`.skillbench/runs/<run-id>/metrics.json`.

## Crashes

- Adapter CLI not found / token missing → tell the user, stop. Don't fix.
- Timeout (case `timeout_seconds`) → log `status=crash`,
  `git reset --hard HEAD~1`, treat as a failed hypothesis, continue.
- SKILL.md syntactic issue (frontmatter broken) → fix once; if it crashes
  again, revert and skip.

## NEVER STOP

Do not pause to ask whether to continue. The user may be asleep. Stop only
when:

- The user interrupts.
- 5 consecutive `discard` decisions — read the ledger, name the pattern,
  try a more radical change (restructure the SKILL, swap the trigger model,
  simplify by deletion).
- 100 trials on this branch — stop and ask for review.

## Reflection (every ~10 trials)

Read your own ledger. Ask:

- Which strategies repeatedly worked? (e.g. trigger broadening,
  explicit checklist)
- Which repeatedly failed?
- Did `metrics.Overall` rise while a sub-metric (e.g. efficiency) regressed?
- Did SKILL.md grow without proportionate score gain?

If you see a pattern, name it in your next hypothesis instead of trying
random edits.
