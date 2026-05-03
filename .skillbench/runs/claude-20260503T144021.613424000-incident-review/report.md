# Skillbench Run claude-20260503T144021.613424000-incident-review

- Agent: `claude`
- Skill: `doc-reading-reviewer`
- Exposure: `rendered_fallback`
- Score: `60.0`
- Exit code: `0`

## Metrics

overall 60.0 task 66.7 skill 100.0 adherence 100.0 quality 80.0 tools 100.0 efficiency 60.0

## Assertions

- `final_contains:Timeline`: pass
- `final_contains:Root Cause`: pass
- `final_contains:Customer Impact`: pass
- `final_contains:Follow-up Checks`: pass
- `final_contains:09:42`: fail
- `final_contains:idempotency`: fail
