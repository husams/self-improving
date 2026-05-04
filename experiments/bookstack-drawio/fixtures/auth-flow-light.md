# Diagram request: Auth flow (light theme)

**Target page:** SSO Migration (page id 101) — do NOT call the BookStack API.
**Output file:** write the generated PNG to `/tmp/skillbench-drawio-auth-flow-light.png`.
**Theme:** light. Use the blue palette `fillColor=#dae8fc;strokeColor=#6c8ebf;`
on every node. White background.

## Required components (exactly 3, all must appear with these labels)

1. `Client` — entry point, leftmost
2. `Auth Service` — middle
3. `Token Store` — rightmost (cylinder shape)

## Required connections

- `Client` -> `Auth Service` (labelled or unlabelled, orthogonal)
- `Auth Service` -> `Token Store`

The PNG must be a real PNG with embedded mxfile/mxGraphModel XML in a zTXt
chunk so it is editable in draw.io. Report the absolute path of the PNG and
echo the embedded XML in your final answer so it can be verified.
