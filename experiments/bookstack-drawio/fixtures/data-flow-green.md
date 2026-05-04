# Diagram request: Data flow (green color scheme)

**Target page:** Analytics Pipeline (page id 303) — do NOT call the BookStack API.
**Output file:** write the generated PNG to
`/tmp/skillbench-drawio-data-flow-green.png`.
**Theme:** green color scheme. Use the green palette
`fillColor=#d5e8d4;strokeColor=#82b366;` on every node.

## Required components (exactly 5, all must appear with these labels)

1. `Source` — parallelogram shape
2. `Ingest`
3. `Transform`
4. `Warehouse` — cylinder shape
5. `Dashboard`

## Required connections

`Source` -> `Ingest` -> `Transform` -> `Warehouse` -> `Dashboard`, orthogonal edges.

The PNG must be a real PNG with editable embedded mxGraphModel XML. Report
the absolute path and echo the embedded XML in your final answer.
