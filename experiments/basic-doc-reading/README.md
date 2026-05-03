# Basic Doc Reading Skill Experiment

This is a complete local experiment for `skillbench research`. It tests whether
an agent can use a simple document-reading skill, follow a requested output
shape, preserve factual details from fixtures, and produce useful next actions.

The source skill is intentionally minimal. The research loop should write
proposal-only improvements under `.skillbench/proposals/` when a candidate skill
scores better than the baseline. The original skill in this directory should not
change during the experiment.

## Layout

- `skill/SKILL.md`: baseline skill under test
- `cases.yaml`: three test cases for the experiment
- `fixtures/`: source documents used by the cases

## Run One Trial

From the project root:

```bash
skillbench research \
  --agent claude \
  --skill experiments/basic-doc-reading/skill \
  --cases experiments/basic-doc-reading/cases.yaml \
  --max-trials 3 \
  --min-improvement 5
```

You can switch the agent:

```bash
skillbench research --agent codex --skill experiments/basic-doc-reading/skill --cases experiments/basic-doc-reading/cases.yaml --max-trials 3
skillbench research --agent copilot --skill experiments/basic-doc-reading/skill --cases experiments/basic-doc-reading/cases.yaml --max-trials 3
```

## Review Results

After a run, check:

- `.skillbench/research/<run-id>/report.md`
- `.skillbench/research/<run-id>/ledger.jsonl`
- `.skillbench/proposals/doc-reading-reviewer/<trial-id>/decision.md`
- `.skillbench/proposals/doc-reading-reviewer/<trial-id>/diff.patch`

## Expected Outcome

A strong candidate should improve:

- task success: returns the requested sections
- skill adherence: follows a repeatable reading workflow
- output quality: cites fixture facts and avoids unsupported claims
- efficiency: stays concise and avoids unnecessary tool churn

