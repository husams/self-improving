#!/usr/bin/env python3
"""
Check A: PNG visual validity (offline).

Usage:
    pixel_check.py <png_path> <hex_color> [<hex_color> ...]

Asserts:
  - File is a valid PNG (magic bytes + Pillow decode).
  - >= 1.0% non-white pixels AND mean per-channel stdev >= 5
    (a blank 800x600 white canvas fails by miles: 0% / std=0).
  - Each requested theme hex color appears within +-8/channel of
    >= 0.05% of pixels in the rendered image (not just the embedded XML).

Exit 0 = pass. Exit 1 = fail (prints reason). Skipping is not applicable
to A — the file is local; if it's missing that's a real failure.
"""
import os
import sys
import math

NONWHITE_MIN_FRAC = 0.01
STDEV_MIN = 5.0
COLOR_TOL = 8
COLOR_MIN_FRAC = 0.0005


def parse_hex(s: str):
    s = s.lstrip("#").strip()
    if len(s) != 6:
        raise ValueError(f"bad hex: {s!r}")
    return (int(s[0:2], 16), int(s[2:4], 16), int(s[4:6], 16))


def pstdev(values):
    n = len(values)
    if n == 0:
        return 0.0
    m = sum(values) / n
    return math.sqrt(sum((v - m) ** 2 for v in values) / n)


def main(argv):
    if len(argv) < 2:
        print("usage: pixel_check.py <png> [<hex> ...]", file=sys.stderr)
        return 1
    path = argv[1]
    targets = [parse_hex(h) for h in argv[2:]]

    if not os.path.isfile(path):
        print(f"FAIL: missing file {path}")
        return 1
    with open(path, "rb") as f:
        magic = f.read(8)
    if magic[:8] != b"\x89PNG\r\n\x1a\n":
        print(f"FAIL: not a PNG (magic={magic!r})")
        return 1

    try:
        from PIL import Image
    except ImportError:
        print("FAIL: Pillow not installed (pip install pillow)")
        return 1

    try:
        im = Image.open(path).convert("RGB")
    except Exception as e:
        print(f"FAIL: PIL decode {path}: {e}")
        return 1

    px = im.tobytes()
    n = im.width * im.height
    if n == 0:
        print("FAIL: zero-sized image")
        return 1
    rs = px[0::3]
    gs = px[1::3]
    bs = px[2::3]

    # non-white fraction
    nonwhite = 0
    for i in range(n):
        if not (rs[i] == 255 and gs[i] == 255 and bs[i] == 255):
            nonwhite += 1
    nw_frac = nonwhite / n
    if nw_frac < NONWHITE_MIN_FRAC:
        print(f"FAIL: blank PNG: nonwhite={nw_frac:.4%} < {NONWHITE_MIN_FRAC:.2%} ({path})")
        return 1

    # rgb stdev (sample if huge)
    sample = 50000
    step = max(1, n // sample)
    sr = [rs[i] for i in range(0, n, step)]
    sg = [gs[i] for i in range(0, n, step)]
    sb = [bs[i] for i in range(0, n, step)]
    std = (pstdev(sr) + pstdev(sg) + pstdev(sb)) / 3.0
    if std < STDEV_MIN:
        print(f"FAIL: low variance: mean stdev={std:.2f} < {STDEV_MIN} ({path})")
        return 1

    # color presence
    missing = []
    for hexcol, (tr, tg, tb) in zip(argv[2:], targets):
        hits = 0
        for i in range(n):
            if abs(rs[i] - tr) <= COLOR_TOL and abs(gs[i] - tg) <= COLOR_TOL and abs(bs[i] - tb) <= COLOR_TOL:
                hits += 1
                if hits / n >= COLOR_MIN_FRAC:
                    break
        frac = hits / n
        if frac < COLOR_MIN_FRAC:
            missing.append(f"{hexcol}({frac:.4%})")
    if missing:
        print(f"FAIL: theme colors absent in pixels: {', '.join(missing)} ({path})")
        return 1

    print(f"OK: {path} nonwhite={nw_frac:.2%} std={std:.1f} colors={argv[2:]}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
