# bookstack-drawio experiment

Skillbench experiment for the `bookstack-drawio` skill. Tests two things:

1. **Valid + visible + correct theme** — output is a real PNG with embedded
   draw.io `mxGraphModel` XML (editable in draw.io) and uses the requested
   theme colors (light blue / dark / green).
2. **Correct sections** — diagram contains exactly the components and
   connections the prompt specifies.

Offline by design: cases instruct the agent NOT to call BookStack. The
agent writes the PNG to `/tmp/skillbench-drawio-<id>.png` and echoes the
embedded XML in its final answer, so deterministic `final_contains`
assertions can verify both criteria without any API.

## Skill under test

`skill/SKILL.md` — copy of the real `~/.claude/skills/bookstack-drawio`
skill (with `scripts/drawio_png.py`).

## Cases

| Case | Theme | Components |
|---|---|---|
| `auth-flow-light` | light blue (`#dae8fc`/`#6c8ebf`) | Client, Auth Service, Token Store |
| `deploy-pipeline-dark` | dark (`#1f1f1f`/`#cccccc`/`#ffffff`) | Build, Test, Stage, Prod |
| `data-flow-green` | green (`#d5e8d4`/`#82b366`) | Source, Ingest, Transform, Warehouse, Dashboard |

Each case requires four labeled sections: `PNG Path`, `Embedded XML`,
`Components`, `Theme`. Assertions check:
- `mxGraphModel` literal in output (proves XML embedded, PNG is editable)
- every required component label appears
- requested theme hex codes appear

## Pass criteria

- All `final_contains` strings present (covers "correct section
  generated" — labels, components, theme hex echoed in the reply).
- `quality_checks`:
  - regex matches on `mxGraphModel|mxfile` and theme hex literal.
  - ROUGE similarity >= 0.4 against the reference summary.
  - **Check A — `checks/pixel_check.py` (offline, deterministic):**
    opens the PNG the skill wrote at `/tmp/skillbench-drawio-<id>.png`,
    asserts valid PNG magic + Pillow decode, >= 1% non-white pixels,
    mean per-channel RGB stdev >= 5, and >= 0.05% of pixels within
    +-8/channel of each requested theme hex. A blank 800x600 white
    canvas fails by design.
  - **Check B — `checks/bookstack_roundtrip.py` (live):** uploads the
    PNG to BookStack, embeds it in a throwaway page, fetches the
    served image bytes back, and re-runs the same pixel/colour check
    on what BookStack actually serves. Reads `BOOKSTACK_BASE_URL` and
    `BOOKSTACK_API_TOKEN` from env (same vars the bookstack MCP uses).
    If creds are unset → exit 2 (skip sentinel; harness excludes the
    check from the mean rather than counting it as a pass).

Mapping to the user's two requirements:

| User requirement | Checks |
|---|---|
| "valid and visible with correct theme" | Check A (offline pixel + colour) + Check B (served image pixel + colour) |
| "correct section generated" | `final_contains` + regex + ROUGE similarity |

The skip semantics for Check B require a `Skipped` field on
`quality.Result`; skipped checks are excluded from the deterministic
mean. See `internal/quality/script.go` (exit code 2 sentinel) and
`internal/quality/quality.go` (`Runner.Run` aggregation).

## How to run

```bash
bin/skillbench suite --agent claude --skill experiments/bookstack-drawio/skill --cases experiments/bookstack-drawio/cases.yaml

bin/skillbench research --agent claude --skill experiments/bookstack-drawio/skill --cases experiments/bookstack-drawio/cases.yaml --max-trials 4 --min-improvement 5
```
