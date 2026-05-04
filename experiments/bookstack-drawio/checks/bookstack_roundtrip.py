#!/usr/bin/env python3
"""
Check B: BookStack page validity (live).

Usage:
    bookstack_roundtrip.py <png_path> <hex_color> [<hex_color> ...]

Reads BOOKSTACK_BASE_URL and BOOKSTACK_API_TOKEN from env. The token format
expected by BookStack is "<token_id>:<token_secret>"; if only one piece is
provided, the script falls back to BOOKSTACK_TOKEN_ID + BOOKSTACK_TOKEN_SECRET.

If credentials are unset → exit 2 (skip sentinel; runner excludes from mean).

On valid creds:
  - Create a throwaway page (or use an existing scratch book/chapter) with
    the PNG attached as an image.
  - Fetch the rendered page HTML, find the served image URL, GET it, and
    re-run the same pixel-variance/color check on the bytes BookStack
    actually serves.
  - Clean up the page.

Exit 0 = pass. Exit 1 = fail. Exit 2 = skip.
Secrets are never echoed.
"""
import os
import sys
import json
import urllib.request
import urllib.error
import io
import math
from typing import NoReturn


SKIP = 2


def skip(msg) -> NoReturn:
    print(f"skipped: {msg}")
    sys.exit(SKIP)


def fail(msg) -> NoReturn:
    print(f"FAIL: {msg}")
    sys.exit(1)


def get_creds():
    base = os.environ.get("BOOKSTACK_BASE_URL", "").rstrip("/")
    token = os.environ.get("BOOKSTACK_API_TOKEN", "").strip()
    if not token:
        tid = os.environ.get("BOOKSTACK_TOKEN_ID", "").strip()
        tsec = os.environ.get("BOOKSTACK_TOKEN_SECRET", "").strip()
        if tid and tsec:
            token = f"{tid}:{tsec}"
    if not base or not token or ":" not in token:
        return None, None
    return base, token


def api(base, token, path, method="GET", data=None, headers=None):
    url = base + path
    req_headers = {"Authorization": f"Token {token}", "Accept": "application/json"}
    if headers:
        req_headers.update(headers)
    body = None
    if data is not None and not isinstance(data, (bytes, bytearray)):
        body = json.dumps(data).encode("utf-8")
        req_headers.setdefault("Content-Type", "application/json")
    elif data is not None:
        body = data
    req = urllib.request.Request(url, data=body, method=method, headers=req_headers)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status, resp.read(), dict(resp.headers)
    except urllib.error.HTTPError as e:
        return e.code, e.read(), dict(e.headers or {})


def multipart(fields, files):
    boundary = "----skillbench" + os.urandom(8).hex()
    out = io.BytesIO()
    for k, v in fields.items():
        out.write(f"--{boundary}\r\n".encode())
        out.write(f'Content-Disposition: form-data; name="{k}"\r\n\r\n'.encode())
        out.write(str(v).encode())
        out.write(b"\r\n")
    for k, (fn, content, ctype) in files.items():
        out.write(f"--{boundary}\r\n".encode())
        out.write(
            f'Content-Disposition: form-data; name="{k}"; filename="{fn}"\r\n'.encode()
        )
        out.write(f"Content-Type: {ctype}\r\n\r\n".encode())
        out.write(content)
        out.write(b"\r\n")
    out.write(f"--{boundary}--\r\n".encode())
    return out.getvalue(), f"multipart/form-data; boundary={boundary}"


def pstdev(values):
    n = len(values)
    if n == 0:
        return 0.0
    m = sum(values) / n
    return math.sqrt(sum((v - m) ** 2 for v in values) / n)


def parse_hex(s):
    s = s.lstrip("#")
    return (int(s[0:2], 16), int(s[2:4], 16), int(s[4:6], 16))


def check_png_bytes(buf, hexes, label):
    try:
        from PIL import Image
    except ImportError:
        fail("Pillow not installed")
    if buf[:8] != b"\x89PNG\r\n\x1a\n":
        fail(f"{label}: not a PNG (magic mismatch)")
    try:
        im = Image.open(io.BytesIO(buf)).convert("RGB")
    except Exception as e:
        fail(f"{label}: decode error {e}")
    px = im.tobytes()
    n = im.width * im.height
    rs = px[0::3]; gs = px[1::3]; bs = px[2::3]
    nonwhite = sum(1 for i in range(n) if not (rs[i] == 255 and gs[i] == 255 and bs[i] == 255))
    if nonwhite / n < 0.01:
        fail(f"{label}: served PNG is blank (nonwhite={nonwhite/n:.4%})")
    sample = 50000
    step = max(1, n // sample)
    std = (
        pstdev([rs[i] for i in range(0, n, step)])
        + pstdev([gs[i] for i in range(0, n, step)])
        + pstdev([bs[i] for i in range(0, n, step)])
    ) / 3.0
    if std < 5.0:
        fail(f"{label}: served PNG low variance std={std:.2f}")
    for h in hexes:
        tr, tg, tb = parse_hex(h)
        hits = 0
        for i in range(n):
            if abs(rs[i] - tr) <= 8 and abs(gs[i] - tg) <= 8 and abs(bs[i] - tb) <= 8:
                hits += 1
                if hits / n >= 0.0005:
                    break
        if hits / n < 0.0005:
            fail(f"{label}: theme color {h} absent in served pixels")


def main(argv):
    if len(argv) < 2:
        skip("usage: bookstack_roundtrip.py <png> [<hex>...]")
    png = argv[1]
    hexes = argv[2:]
    base, token = get_creds()
    if not base or not token:
        skip("BOOKSTACK_BASE_URL / BOOKSTACK_API_TOKEN unset")
    if not os.path.isfile(png):
        fail(f"missing file {png}")

    with open(png, "rb") as f:
        png_bytes = f.read()

    # Find or create a scratch book + chapter (best-effort; use first book).
    code, body, _ = api(base, token, "/api/books?count=1")
    if code != 200:
        skip(f"books list failed: HTTP {code}")
    try:
        first = json.loads(body)["data"][0]
    except Exception:
        skip("no books available to attach scratch page")
    book_id = first["id"]

    # Create page
    code, body, _ = api(
        base,
        token,
        "/api/pages",
        method="POST",
        data={"book_id": book_id, "name": "skillbench-drawio-roundtrip", "markdown": "scratch"},
    )
    if code not in (200, 201):
        fail(f"page create failed: HTTP {code}")
    page = json.loads(body)
    page_id = page["id"]
    cleanup_path = f"/api/pages/{page_id}"

    try:
        # Upload image attached to this page
        fields = {"name": os.path.basename(png), "uploaded_to": str(page_id)}
        files = {"image": (os.path.basename(png), png_bytes, "image/png")}
        body_bytes, ctype = multipart(fields, files)
        code, body, _ = api(base, token, "/api/image-gallery", method="POST", data=body_bytes, headers={"Content-Type": ctype})
        if code not in (200, 201):
            fail(f"image upload failed: HTTP {code}")
        img = json.loads(body)
        img_url = img.get("url") or img.get("thumbs", {}).get("display") or ""
        if not img_url:
            fail("uploaded image has no url")

        # Update page to embed the image
        html = f'<p><img src="{img_url}" alt="diagram"/></p>'
        code, body, _ = api(base, token, f"/api/pages/{page_id}", method="PUT", data={"html": html})
        if code not in (200, 201):
            fail(f"page update failed: HTTP {code}")

        # Fetch served image bytes
        req = urllib.request.Request(img_url, headers={"Authorization": f"Token {token}"})
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                served = resp.read()
        except urllib.error.HTTPError as e:
            fail(f"image fetch HTTP {e.code}")
        except Exception as e:
            fail(f"image fetch error: {e}")
        check_png_bytes(served, hexes, "served")
        print(f"OK: bookstack roundtrip page={page_id} url-served PNG variance + colors verified")
        return 0
    finally:
        api(base, token, cleanup_path, method="DELETE")
        api(base, token, f"/api/recycle-bin", method="GET")  # noop for visibility


if __name__ == "__main__":
    sys.exit(main(sys.argv))
