#!/usr/bin/env python3
"""Create a PNG file with embedded draw.io XML in the zTXt metadata chunk.

The generated PNG includes a simple visible raster preview for common
mxGraphModel boxes and connectors, while preserving editable draw.io XML for
BookStack's draw.io integration.
"""

import base64
import html
import re
import struct
import sys
import urllib.parse
import zlib
import xml.etree.ElementTree as ET


def _chunk(chunk_type, data):
    body = chunk_type + data
    crc = struct.pack('>I', zlib.crc32(body) & 0xFFFFFFFF)
    return struct.pack('>I', len(data)) + body + crc


def _hex_to_rgb(value, default):
    if not value:
        return default
    value = value.strip()
    if not re.fullmatch(r'#[0-9a-fA-F]{6}', value):
        return default
    return tuple(int(value[i:i + 2], 16) for i in (1, 3, 5))


def _style_value(style, key):
    for part in (style or '').split(';'):
        if '=' not in part:
            continue
        k, v = part.split('=', 1)
        if k == key:
            return v
    return None


def _decode_mxfile(xml_content):
    text = xml_content.strip()
    if text.startswith('<mxGraphModel'):
        return text
    if not text.startswith('<mxfile'):
        raise ValueError('XML must start with <mxfile> or <mxGraphModel>')
    root = ET.fromstring(text)
    diagram = root.find('diagram')
    if diagram is None or diagram.text is None:
        return text
    payload = diagram.text.strip()
    try:
        return urllib.parse.unquote(zlib.decompress(base64.b64decode(payload)).decode('utf-8'))
    except Exception:
        return text


def _parse_model(xml_content):
    model_xml = _decode_mxfile(xml_content)
    root = ET.fromstring(model_xml)
    cells = {}
    vertices = []
    edges = []
    for cell in root.findall('.//mxCell'):
        cid = cell.get('id')
        if not cid:
            continue
        geom = cell.find('mxGeometry')
        record = {'cell': cell, 'geom': geom}
        cells[cid] = record
        if cell.get('vertex') == '1' and geom is not None:
            vertices.append(record)
        elif cell.get('edge') == '1':
            edges.append(record)
    return vertices, edges, cells


def _canvas(width, height, color=(255, 255, 255)):
    return [[color for _ in range(width)] for _ in range(height)]


def _set_px(pixels, x, y, color):
    if 0 <= y < len(pixels) and 0 <= x < len(pixels[0]):
        pixels[y][x] = color


def _rect(pixels, x, y, w, h, fill, stroke, stroke_width=2):
    max_x = min(len(pixels[0]), x + w)
    max_y = min(len(pixels), y + h)
    for yy in range(max(0, y), max_y):
        for xx in range(max(0, x), max_x):
            if xx < x + stroke_width or xx >= x + w - stroke_width or yy < y + stroke_width or yy >= y + h - stroke_width:
                pixels[yy][xx] = stroke
            else:
                pixels[yy][xx] = fill


def _line(pixels, x1, y1, x2, y2, color, width=3):
    dx = abs(x2 - x1)
    dy = -abs(y2 - y1)
    sx = 1 if x1 < x2 else -1
    sy = 1 if y1 < y2 else -1
    err = dx + dy
    while True:
        for oy in range(-(width // 2), width // 2 + 1):
            for ox in range(-(width // 2), width // 2 + 1):
                _set_px(pixels, x1 + ox, y1 + oy, color)
        if x1 == x2 and y1 == y2:
            break
        e2 = 2 * err
        if e2 >= dy:
            err += dy
            x1 += sx
        if e2 <= dx:
            err += dx
            y1 += sy


def _arrow_head(pixels, x1, y1, x2, y2, color):
    if abs(x2 - x1) >= abs(y2 - y1):
        direction = 1 if x2 >= x1 else -1
        pts = [(x2, y2), (x2 - 10 * direction, y2 - 6), (x2 - 10 * direction, y2 + 6)]
    else:
        direction = 1 if y2 >= y1 else -1
        pts = [(x2, y2), (x2 - 6, y2 - 10 * direction), (x2 + 6, y2 - 10 * direction)]
    min_x = max(0, min(p[0] for p in pts))
    max_x = min(len(pixels[0]) - 1, max(p[0] for p in pts))
    min_y = max(0, min(p[1] for p in pts))
    max_y = min(len(pixels) - 1, max(p[1] for p in pts))
    def area(a, b, c):
        return abs((a[0] * (b[1] - c[1]) + b[0] * (c[1] - a[1]) + c[0] * (a[1] - b[1])) / 2.0)
    total = area(pts[0], pts[1], pts[2])
    for yy in range(min_y, max_y + 1):
        for xx in range(min_x, max_x + 1):
            p = (xx, yy)
            if abs((area(p, pts[1], pts[2]) + area(pts[0], p, pts[2]) + area(pts[0], pts[1], p)) - total) < 0.6:
                pixels[yy][xx] = color


def _draw_label_bar(pixels, x, y, w, h, color):
    # A small light text surrogate makes labels visible without font dependencies.
    bar_w = max(18, min(w - 16, w // 2))
    bar_h = 4
    bx = x + (w - bar_w) // 2
    by = y + (h - bar_h) // 2
    for yy in range(by, by + bar_h):
        for xx in range(bx, bx + bar_w):
            _set_px(pixels, xx, yy, color)


def render_preview_png(xml_content, width=800, height=600):
    pixels = _canvas(width, height)
    try:
        vertices, edges, cells = _parse_model(xml_content)
    except Exception:
        vertices, edges, cells = [], [], {}

    bounds = []
    for rec in vertices:
        geom = rec['geom']
        x = int(float(geom.get('x', '0')))
        y = int(float(geom.get('y', '0')))
        w = int(float(geom.get('width', '120')))
        h = int(float(geom.get('height', '60')))
        rec.update({'x': x, 'y': y, 'w': w, 'h': h})
        bounds.append((x + w, y + h))
    if bounds:
        needed_w = max(width, max(x for x, _ in bounds) + 40)
        needed_h = max(height, max(y for _, y in bounds) + 40)
        if needed_w != width or needed_h != height:
            width, height = needed_w, needed_h
            pixels = _canvas(width, height)

    for rec in edges:
        cell = rec['cell']
        source = cells.get(cell.get('source'))
        target = cells.get(cell.get('target'))
        if not source or not target or 'x' not in source or 'x' not in target:
            continue
        sx = source['x'] + source['w']
        sy = source['y'] + source['h'] // 2
        tx = target['x']
        ty = target['y'] + target['h'] // 2
        stroke = _hex_to_rgb(_style_value(cell.get('style'), 'strokeColor'), (80, 80, 80))
        mid = sx + max(12, (tx - sx) // 2)
        _line(pixels, sx, sy, mid, sy, stroke)
        _line(pixels, mid, sy, mid, ty, stroke)
        _line(pixels, mid, ty, tx, ty, stroke)
        if _style_value(cell.get('style'), 'endArrow') != 'none':
            _arrow_head(pixels, mid, ty, tx, ty, stroke)

    for rec in vertices:
        cell = rec['cell']
        style = cell.get('style') or ''
        fill = _hex_to_rgb(_style_value(style, 'fillColor'), (245, 245, 245))
        stroke = _hex_to_rgb(_style_value(style, 'strokeColor'), (102, 102, 102))
        font = _hex_to_rgb(_style_value(style, 'fontColor'), (0, 0, 0))
        _rect(pixels, rec['x'], rec['y'], rec['w'], rec['h'], fill, stroke)
        if html.unescape(cell.get('value') or '').strip():
            _draw_label_bar(pixels, rec['x'], rec['y'], rec['w'], rec['h'], font)

    raw = b''.join(b'\x00' + bytes(channel for px in row for channel in px) for row in pixels)
    signature = b'\x89PNG\r\n\x1a\n'
    ihdr = _chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 2, 0, 0, 0))
    idat = _chunk(b'IDAT', zlib.compress(raw))
    iend = _chunk(b'IEND', b'')
    return signature + ihdr + idat + iend


def create_ztxt_chunk(keyword, text_data):
    keyword_bytes = keyword.encode('latin-1')
    compressed = zlib.compress(text_data.encode('utf-8'))
    chunk_data = keyword_bytes + b'\x00\x00' + compressed
    return _chunk(b'zTXt', chunk_data)


def embed_drawio_in_png(xml_content, png_data=None, width=800, height=600):
    if png_data is None:
        png_data = render_preview_png(xml_content, width, height)

    encoded = base64.b64encode(
        zlib.compress(urllib.parse.quote(_decode_mxfile(xml_content), safe='').encode('utf-8'))
    ).decode('ascii')

    stripped = xml_content.strip()
    if stripped.startswith('<mxGraphModel'):
        store_xml = (
            '<mxfile host="Embedded" modified="2026-01-01T00:00:00.000Z" '
            'agent="Claude" version="1.0" type="embed">'
            f'<diagram id="d1" name="Page-1">{encoded}</diagram>'
            '</mxfile>'
        )
    elif stripped.startswith('<mxfile'):
        store_xml = stripped
    else:
        raise ValueError('XML must start with <mxfile> or <mxGraphModel>')

    ztxt_chunk = create_ztxt_chunk('mxGraphModel', store_xml)
    iend_pos = png_data.rfind(b'IEND') - 4
    return png_data[:iend_pos] + ztxt_chunk + png_data[iend_pos:]


def main():
    import argparse
    parser = argparse.ArgumentParser(description='Create PNG with embedded draw.io XML')
    parser.add_argument('input', help='Draw.io XML file path, or - for stdin')
    parser.add_argument('output', nargs='?', help='Output PNG file path')
    parser.add_argument('--base64', action='store_true', help='Output base64 to stdout')
    parser.add_argument('--width', type=int, default=800, help='PNG width (default: 800)')
    parser.add_argument('--height', type=int, default=600, help='PNG height (default: 600)')
    parser.add_argument('--png', help='Existing PNG file to embed XML into')
    args = parser.parse_args()

    if args.input == '-':
        xml_content = sys.stdin.read()
    else:
        with open(args.input, 'r', encoding='utf-8') as f:
            xml_content = f.read()

    png_data = None
    if args.png:
        with open(args.png, 'rb') as f:
            png_data = f.read()

    result = embed_drawio_in_png(xml_content, png_data, args.width, args.height)

    if args.base64:
        print(base64.b64encode(result).decode('ascii'))
    elif args.output:
        with open(args.output, 'wb') as f:
            f.write(result)
        print(f'Written to {args.output}', file=sys.stderr)
    else:
        parser.error('Either --base64 or output path required')


if __name__ == '__main__':
    main()
