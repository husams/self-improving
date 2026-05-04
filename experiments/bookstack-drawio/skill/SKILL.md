---
name: bookstack-drawio
description: Create and embed editable draw.io diagrams in BookStack pages. Generates PNG files with embedded draw.io XML, uploads them via BookStack API, and inserts them as editable diagrams in pages. Use when adding architecture diagrams, flowcharts, or any draw.io diagrams to BookStack documentation. Examples: <example>Context: User wants to add an architecture diagram to a BookStack page. user: "Add a draw.io diagram showing the auth flow to the SSO Migration page" assistant: "I will generate the draw.io XML, create an embedded PNG, upload it to BookStack, and update the page." <commentary>Use this skill for any draw.io diagram creation in BookStack.</commentary></example> <example>Context: User has a draw.io PNG file to embed. user: "Upload this diagram.png to BookStack page 95" assistant: "I will upload the PNG as a drawio image and embed it in the page." <commentary>Use this skill for uploading existing draw.io PNGs to BookStack.</commentary></example>
tools: Bash
---

Embeds editable draw.io diagrams in BookStack pages via the API. Diagrams are stored as PNG files with draw.io XML in the `zTXt` metadata chunk, making them both viewable and editable.

## Prerequisites

- BookStack MCP tools (`bookstack_images_create`, `bookstack_pages_update`)
- Python 3 (for PNG generation)
- Helper script: `~/.claude/skills/bookstack-drawio/scripts/drawio_png.py`

## Quick Reference

### Step 1: Create draw.io XML

Write standard mxGraphModel XML. Cells `id="0"` and `id="1"` are always required as root cells.

```xml
<mxGraphModel>
  <root>
    <mxCell id="0"/>
    <mxCell id="1" parent="0"/>
    <mxCell id="2" value="Service A" style="rounded=1;whiteSpace=wrap;" vertex="1" parent="1">
      <mxGeometry x="100" y="100" width="120" height="60" as="geometry"/>
    </mxCell>
  </root>
</mxGraphModel>
```

### Step 2: Generate PNG with embedded XML

```bash
# From XML file
python3 ~/.claude/skills/bookstack-drawio/scripts/drawio_png.py diagram.xml --base64 > /tmp/diagram_b64.txt

# From inline XML via stdin
cat <<'XML' | python3 ~/.claude/skills/bookstack-drawio/scripts/drawio_png.py - --base64 > /tmp/diagram_b64.txt
<mxGraphModel>
  <root>
    <mxCell id="0"/><mxCell id="1" parent="0"/>
    <mxCell id="2" value="Hello" style="rounded=1;" vertex="1" parent="1">
      <mxGeometry x="20" y="20" width="100" height="50" as="geometry"/>
    </mxCell>
  </root>
</mxGraphModel>
XML
```

### Step 3: Upload to BookStack as draw.io image

Use `bookstack_images_create` MCP tool:

```
bookstack_images_create(
  name: "Diagram Name",
  type: "drawio",              # CRITICAL: must be "drawio"
  image: <base64 PNG string>,
  uploaded_to: <page_id>
)
```

The response returns `id` and `url` needed for embedding.

### Step 4: Embed in page HTML

Use `bookstack_pages_update` MCP tool. The `drawio-diagram` attribute with the image ID makes it editable:

```
bookstack_pages_update(
  id: <page_id>,
  html: '<p>Architecture:</p><div drawio-diagram="IMAGE_ID"><img src="IMAGE_URL"/></div>'
)
```

## Common Diagram Styles

### Box styles
| Style | Description |
|-------|-------------|
| `rounded=1;whiteSpace=wrap;` | Rounded rectangle |
| `shape=cylinder3;whiteSpace=wrap;size=15;` | Database cylinder |
| `ellipse;whiteSpace=wrap;` | Ellipse/circle |
| `rhombus;whiteSpace=wrap;` | Diamond (decision) |
| `shape=hexagon;perimeter=hexagonPerimeter2;` | Hexagon |
| `shape=parallelogram;perimeter=parallelogramPerimeter;` | Parallelogram |

### Edge (arrow) styles
| Style | Description |
|-------|-------------|
| `edgeStyle=orthogonalEdgeStyle;` | Right-angle connectors |
| `edgeStyle=entityRelationEdgeStyle;` | Entity relation style |
| `dashed=1;` | Dashed line |
| `endArrow=none;` | No arrowhead |

### Colors
Use `fillColor=#HEX;strokeColor=#HEX;fontColor=#HEX;` in styles.

Common palette:
- Blue: `fillColor=#dae8fc;strokeColor=#6c8ebf;`
- Green: `fillColor=#d5e8d4;strokeColor=#82b366;`
- Orange: `fillColor=#ffe6cc;strokeColor=#d6b656;`
- Red: `fillColor=#f8cecc;strokeColor=#b85450;`
- Purple: `fillColor=#e1d5e7;strokeColor=#9673a6;`
- Grey: `fillColor=#f5f5f5;strokeColor=#666666;`

## Edge Connections

Connect cells using `source` and `target` attributes:

```xml
<mxCell id="3" style="edgeStyle=orthogonalEdgeStyle;" edge="1" source="2" target="4" parent="1">
  <mxGeometry relative="1" as="geometry"/>
</mxCell>
```

## Upload Existing draw.io PNG

For PNGs already exported from draw.io (with embedded XML):

```bash
# Base64 encode existing file
base64 -i diagram.png | tr -d '\n' > /tmp/diagram_b64.txt
```

Then upload with `bookstack_images_create(type: "drawio", ...)`.

## Full Example: Architecture Diagram

```bash
cat <<'XML' | python3 ~/.claude/skills/bookstack-drawio/scripts/drawio_png.py - --base64 > /tmp/arch_b64.txt
<mxGraphModel dx="1000" dy="600" grid="1" gridSize="10">
  <root>
    <mxCell id="0"/>
    <mxCell id="1" parent="0"/>
    <mxCell id="2" value="Client" style="rounded=1;whiteSpace=wrap;fillColor=#dae8fc;strokeColor=#6c8ebf;" vertex="1" parent="1">
      <mxGeometry x="50" y="200" width="120" height="60" as="geometry"/>
    </mxCell>
    <mxCell id="3" value="API Gateway" style="rounded=1;whiteSpace=wrap;fillColor=#d5e8d4;strokeColor=#82b366;" vertex="1" parent="1">
      <mxGeometry x="250" y="200" width="120" height="60" as="geometry"/>
    </mxCell>
    <mxCell id="4" value="Database" style="shape=cylinder3;whiteSpace=wrap;size=15;fillColor=#ffe6cc;strokeColor=#d6b656;" vertex="1" parent="1">
      <mxGeometry x="450" y="190" width="100" height="80" as="geometry"/>
    </mxCell>
    <mxCell id="5" style="edgeStyle=orthogonalEdgeStyle;" edge="1" source="2" target="3" parent="1">
      <mxGeometry relative="1" as="geometry"/>
    </mxCell>
    <mxCell id="6" style="edgeStyle=orthogonalEdgeStyle;" edge="1" source="3" target="4" parent="1">
      <mxGeometry relative="1" as="geometry"/>
    </mxCell>
  </root>
</mxGraphModel>
XML
```

Then upload the base64 content and embed in page HTML.

## Key Rules

1. **`type: "drawio"` is required** when uploading -- without it, BookStack treats the image as a static gallery image
2. **`drawio-diagram="IMAGE_ID"` attribute on the wrapper `<div>` is essential** -- this enables double-click editing in BookStack
3. **Image `src` must be the full absolute URL** returned by the images API
4. **Cells id="0" and id="1" are always required** as the root base cells in mxGraphModel XML
5. **Use `--base64` flag** with the helper script for direct use with BookStack MCP tools
