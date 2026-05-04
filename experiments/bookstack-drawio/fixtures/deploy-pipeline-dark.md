# Diagram request: Deploy pipeline (dark theme)

**Target page:** CI/CD Overview (page id 202) — do NOT call the BookStack API.
**Output file:** write the generated PNG to
`/tmp/skillbench-drawio-deploy-pipeline-dark.png`.
**Theme:** dark. Use dark fills with light font on every node:
`fillColor=#1f1f1f;strokeColor=#cccccc;fontColor=#ffffff;`.

## Required components (exactly 4, all must appear with these labels)

1. `Build`
2. `Test`
3. `Stage`
4. `Prod`

## Required connections

A linear pipeline: `Build` -> `Test` -> `Stage` -> `Prod`, all orthogonal edges.

The PNG must be a real PNG with embedded draw.io mxGraphModel XML (editable).
Report the absolute path of the PNG and echo the embedded XML in your final
answer.
