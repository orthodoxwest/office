#!/usr/bin/env python3
"""The apse sky's gold-leaf stars: one seeded field for each phone head.

By night a phone's home sets the parish apse's painted sky inside the head of
its arch, and these are its stars. A real ceiling does not reshuffle its stars
each morning, so the field is fixed: a scatter with a floor on closeness (no
lattice) inside the app's own arch (tools/genarch.py's shapes), kept clear of
the leaf fillet and the day's hairline. Every front draws the same list and
leaves out the stars that fall in the painter's reserve round the day's words
and the cross, and those at or below the frieze.

The figures are measured from the client's photograph of the vault
(`--measure PHOTO` prints them again; the photograph stays outside the
repository, in ../resources/design/parish/): in a 684x804px piece of it there
are 46 stars, the four-point ones about 14.5px across, the closest pairs about
35px apart. By eye about three in five are four-point: a long-armed cross with
thin tapered rays, the upright longer than the bar, none plumb; the rest have
a small boss, four long rays and four short. The median four-point star is set
8 CSS px across, which makes the measured piece about one phone sky. The field
is set at twice the measured density, because the day's words take the middle
of the head as a reserve and at the vault's own density a feria kept 13 stars.

Coordinates are in card widths from the card's top left (the point of the head
is at y = 0); radii and the outline's offsets are CSS px (dp, pt). Writes

  apps/office-web/static/ornaments/stars.json   {shape: [[x, y, r, leaf, [dx, dy, ...]], ...]}
  apps/android/.../Stars.kt, apps/ios/Office/Stars.swift   the same, as one string per shape

Requires tools/requirements.txt. Seeded, so it regenerates byte for byte.
Run from the repository root.
"""
import argparse
import json
import math
import sys
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw

sys.path.insert(0, str(Path(__file__).resolve().parent))
import genarch  # noqa: E402

WEB = Path("apps/office-web/static/ornaments/stars.json")
KOTLIN = Path("apps/android/app/src/main/kotlin/org/orthodoxwest/office/Stars.kt")
SWIFT = Path("apps/ios/Office/Stars.swift")

# Measured from apse-sky-close.jpg (--measure): the piece's size, its stars, the median four-point
# star's diameter and the tenth percentile of the nearest-neighbour distance, in photo pixels.
CROP = (684, 804)
STARS = 46
FOUR_POINT_DIAMETER = 14.5
NEAREST_P10 = 35.389
W = 358.0            # the card at 390px; positions are stored in widths
DEPTH = 1.75         # how far down the field runs, in widths (the frieze cuts it)
INSET = 19.0         # clear of the card's edge: the stone 5, the fillet, the hairline at 11, air
K = 8.0 / FOUR_POINT_DIAMETER
DENSITY = 2.0 * STARS / (CROP[0] * CROP[1] * K * K)     # stars per CSS px²
GAP = NEAREST_P10 * K * 1.1                            # none closer than the vault's closest
# The ray count over-reads the small stars as four-point (0.83); by eye it is about three in five.
FOUR = 0.6
SEEDS = {"phone": 1117, "tall": 1118, "taller": 1119}


def outline(shape, w, h, d):
    """The head's silhouette offset by d (negative inward), y down from the point's line."""
    side, apex = genarch.left_side(shape)
    rise = shape["rise"]
    pts = [(-d, h + d)]
    for (u, v, nx, ny) in side:
        pts.append((u * w + nx * d, (rise - v) * w - ny * d))
    pts.append((0.5 * w, -apex[3] * d))
    for (u, v, nx, ny) in reversed(side):
        pts.append(((1 - u) * w - nx * d, (rise - v) * w - ny * d))
    pts.append((w + d, h + d))
    return pts


def glyph(kind, r, rot, rng):
    """A star's closed outline, as offsets in CSS px. Four-point: a long-armed cross, the upright
    longer than the bar and the foot a little longer than the head. Eight-point: a small boss,
    four long rays and four short between."""
    if kind == 4:
        lens, half, n = [0.88, 0.62, 1.0, 0.62], max(0.2 * r, 0.8), 4
    else:
        lens, half, n = [1.0, 0.56, 0.86, 0.56, 1.0, 0.56, 0.86, 0.56], max(0.15 * r, 0.8), 8
    out = []
    for i in range(n):
        a = rot + i * 2 * math.pi / n
        length = r * lens[i] * rng.uniform(0.92, 1.08)
        a += rng.uniform(-0.05, 0.05)
        # Round the star: the notch between rays, then the ray's tip.
        an = a - math.pi / n
        notch = half * (1.15 if kind == 4 else 1.9)
        out.append((notch * math.sin(an), -notch * math.cos(an)))
        out.append((length * math.sin(a), -length * math.cos(a)))
    return out


def field(name):
    shape = genarch.SHAPES[name]
    rng = np.random.default_rng(SEEDS[name])
    H, R = DEPTH * W, 4
    im = Image.new("L", (int(W * R), int(H * R)), 0)
    ImageDraw.Draw(im).polygon([(x * R, y * R) for x, y in outline(shape, W, H + 50, -INSET)], fill=255)
    inside = np.asarray(im) > 0
    want = int(round(DENSITY * inside.sum() / (R * R)))
    pts = []
    # Scattered, not packed: random, with only a floor on closeness.
    while len(pts) < want:
        x, y = rng.uniform(0, W), rng.uniform(0, H)
        if not inside[min(int(y * R), inside.shape[0] - 1), min(int(x * R), inside.shape[1] - 1)]:
            continue
        if all((x - px) ** 2 + (y - py) ** 2 > GAP * GAP for px, py in pts):
            pts.append((x, y))
    stars = []
    for (x, y) in pts:
        kind = 4 if rng.uniform() < FOUR else 8
        # Radii from the photograph's diameters (p10..p90 of each kind), in CSS px.
        r = float(rng.uniform(3.7, 5.2) if kind == 4 else rng.choice([5.6, 6.6, 8.0], p=[0.45, 0.4, 0.15]))
        rot = rng.normal(0, 0.16)      # set by hand: none is plumb
        poly = [round(v, 1) for p in glyph(kind, r, rot, rng) for v in p]
        leaf = round(float(rng.uniform(0.62, 1.0)), 2)
        stars.append([round(x / W, 4), round(y / W, 4), round(r, 2), leaf, poly])
    return stars


def num(v):
    s = f"{v:.4f}".rstrip("0").rstrip(".")
    return "0" if s in ("-0", "") else s


def flat(stars):
    """One string per shape for the native apps: each star `x y r leaf dx dy dx dy ...`, stars
    parted by `;`. A string, not an array literal, keeps the generated source small to compile."""
    return ";".join(" ".join(num(v) for v in [x, y, r, leaf, *poly]) for x, y, r, leaf, poly in stars)


def kotlin(data):
    lines = [
        "package org.orthodoxwest.office",
        "",
        "// tools/genstars.py writes this file; edit the script, not the figures.",
        "",
        "/**",
        " * The apse sky's stars for each phone head, as the web's `ornaments/stars.json`: each star",
        " * `x y r leaf` (x and y in card widths from the card's top left, r in dp) then its outline's",
        " * offsets in dp, stars parted by `;`. Parsed by [skyStars].",
        " */",
        "internal object StarFields {",
    ]
    for name, stars in data.items():
        lines.append(f'    const val {name.upper()} = "{flat(stars)}"')
    lines.append("}")
    return "\n".join(lines) + "\n"


def swift(data):
    lines = [
        "// tools/genstars.py writes this file; edit the script, not the figures.",
        "",
        "/// The apse sky's stars for each phone head, as the web's `ornaments/stars.json`: each star",
        "/// `x y r leaf` (x and y in card widths from the card's top left, r in points) then its",
        "/// outline's offsets in points, stars parted by `;`. Parsed by `SkyStar.parse`.",
        "enum StarFields {",
    ]
    for name, stars in data.items():
        lines.append(f'    static let {name} = "{flat(stars)}"')
    lines.append("}")
    return "\n".join(lines) + "\n"


def measure(photo_path):
    """Count and size the stars in the clean piece of the vault left of the lamp's chain."""
    from scipy import ndimage
    im = Image.open(photo_path).convert("RGB")
    k = max(im.size) / 1400   # the box was chosen on a 1400px thumbnail
    box = tuple(int(v * k) for v in (350, 312, 585, 588))
    a = np.asarray(im.crop(box)).astype(np.float32)
    gold = (a[..., 0] > a[..., 2] + 28) & (a[..., 0] > 105)
    lab, _ = ndimage.label(ndimage.binary_dilation(gold, iterations=1))
    found = []
    for i, sl in enumerate(ndimage.find_objects(lab), 1):
        m = lab[sl] == i
        h, w = m.shape
        if m.sum() < 25 or max(h, w) > 90:
            continue
        cy, cx = ndimage.center_of_mass(m)
        d = max(h, w)
        r = 0.62 * d / 2
        th = np.linspace(0, 2 * np.pi, 180, endpoint=False)
        ys = np.clip((cy + r * np.sin(th)).round().astype(int), 0, h - 1)
        xs = np.clip((cx + r * np.cos(th)).round().astype(int), 0, w - 1)
        ring = m[ys, xs]
        rays = int(((ring.astype(int) - np.roll(ring, 1).astype(int)) == 1).sum())
        found.append((sl[1].start + cx, sl[0].start + cy, d, rays))
    pts = np.array([[x, y] for x, y, _, _ in found])
    ds = np.array([d for _, _, d, _ in found])
    four = np.array([rays <= 5 for *_, rays in found])
    nn = np.sort(np.hypot(*(pts[:, None] - pts[None]).transpose(2, 0, 1)) + np.eye(len(pts)) * 1e9, axis=1)[:, 0]
    print(f"CROP = {a.shape[1], a.shape[0]}\nSTARS = {len(found)}\n"
          f"FOUR_POINT_DIAMETER = {float(np.median(ds[four]))}\nNEAREST_P10 = {float(np.percentile(nn, 10)):.3f}")


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--measure", type=Path, help="print the figures measured from this photograph of the vault")
    args = parser.parse_args()
    if args.measure:
        measure(args.measure)
        return
    data = {name: field(name) for name in SEEDS}
    WEB.write_text(json.dumps(data, separators=(",", ":")) + "\n")
    KOTLIN.write_text(kotlin(data))
    SWIFT.write_text(swift(data))
    for name, stars in data.items():
        print(f"{name}: {len(stars)} stars")
    print(f"{WEB}: {WEB.stat().st_size} bytes")


if __name__ == "__main__":
    main()
