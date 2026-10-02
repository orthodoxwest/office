#!/usr/bin/env python3
"""Draw the painted ornaments the web pages mask their colours through.

Each file is a shape only (fill = alpha); style.css paints a colour token
through it, so the seasons and themes stay in tokens. Plain Python, no
dependencies. Seeded, so it regenerates byte for byte. Run from the
repository root.

  ornaments/consecration.svg  the consecration cross in its compass circle:
                              the hour's end, the home niche's crown, the
                              header's brand mark
  ornaments/cross.svg         the same cross without its circle: section
                              breaks and the ordo's first-class feasts
  ornaments/sun.svg           a disc with alternating long and short rays
                              (the Lauds headpiece)
  ornaments/moon.svg          a crescent (the Vespers and Compline headpieces)
  ornaments/vault.svg         one seamless tile of the Apse vault: parish
                              eight-ray stars in three sizes, set by hand
                              rather than on a lattice, each catching its own
                              share of light
"""
import argparse
import math
import random
from pathlib import Path

OUT = Path("apps/office-web/static/ornaments")


def fmt(points):
    return "M" + " ".join(f"{x:.2f},{y:.2f}" for x, y in points) + "Z"


def rotate(points, quarter_turns, c):
    a = quarter_turns * math.pi / 2
    ca, sa = math.cos(a), math.sin(a)
    return [(c + (x - c) * ca - (y - c) * sa, c + (x - c) * sa + (y - c) * ca) for x, y in points]


def flared_arm(c, reach, end_half_angle, root, steps=14):
    """One arm pointing up from the centre: concave sides spreading to an arc."""
    he = math.radians(end_half_angle)
    side = []
    for i in range(steps + 1):
        t = i / steps
        r = root + (reach - root) * t
        w = root + (reach * math.sin(he) - root) * t ** 2.2
        side.append((w, r))
    left = [(c - w, c - r) for w, r in side]
    arc = [(c + reach * math.sin(a), c - reach * math.cos(a))
           for a in (-he + 2 * he * i / steps for i in range(1, steps))]
    right = [(c + w, c - r) for w, r in reversed(side)]
    return left + arc + right


def cross_paths(c, reach, end_half_angle, root, centre):
    arms = [fmt(rotate(flared_arm(c, reach, end_half_angle, root), k, c)) for k in range(4)]
    box = [(c - centre, c - centre), (c + centre, c - centre), (c + centre, c + centre), (c - centre, c + centre)]
    return "".join(arms) + fmt(box)


def circle(c, r):
    return f"M{c - r:.2f},{c:.2f}a{r:.2f},{r:.2f} 0 1,0 {2 * r:.2f},0a{r:.2f},{r:.2f} 0 1,0 {-2 * r:.2f},0Z"


def svg(view, body):
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{view}">{body}</svg>\n'


def consecration():
    c = 20
    ring = f'<path fill-rule="evenodd" d="{circle(c, 19.3)}{circle(c, 18.1)}"/>'
    return svg("0 0 40 40", ring + f'<path d="{cross_paths(c, 16.6, 13, 1.25, 1.3)}"/>')


def cross():
    return svg("0 0 20 20", f'<path d="{cross_paths(10, 9.4, 24, 1.5, 1.6)}"/>')


def sun():
    c, r, long_r, short_r, base = 12, 5.2, 11.6, 9.4, 1.45
    parts = [f"M{c + r:.2f},{c:.2f}A{r},{r} 0 1,0 {c - r:.2f},{c:.2f}A{r},{r} 0 1,0 {c + r:.2f},{c:.2f}Z"]
    for i in range(12):
        a = i * math.pi / 6 - math.pi / 2
        tip = long_r if i % 2 == 0 else short_r
        ux, uy = math.cos(a), math.sin(a)
        nx, ny = -uy, ux
        r0 = r + 1.2
        parts.append(fmt([(c + ux * r0 + nx * base, c + uy * r0 + ny * base),
                          (c + ux * tip, c + uy * tip),
                          (c + ux * r0 - nx * base, c + uy * r0 - ny * base)]))
    return svg("0 0 24 24", f'<path d="{"".join(parts)}"/>')


def moon():
    """A crescent: the outer disc less an offset inner one, as two arcs."""
    (cx, cy, big), (ix, iy, small) = (12.0, 12.0, 9.6), (16.2, 10.2, 7.9)
    d = math.hypot(ix - cx, iy - cy)
    a = (big * big - small * small + d * d) / (2 * d)
    h = math.sqrt(big * big - a * a)
    px, py = cx + a * (ix - cx) / d, cy + a * (iy - cy) / d
    p1 = (px + h * (iy - cy) / d, py - h * (ix - cx) / d)
    p2 = (px - h * (iy - cy) / d, py + h * (ix - cx) / d)
    path = (f"M{p1[0]:.2f} {p1[1]:.2f}A{big} {big} 0 1 0 {p2[0]:.2f} {p2[1]:.2f}"
            f"A{small} {small} 0 0 1 {p1[0]:.2f} {p1[1]:.2f}Z")
    return svg("0 0 24 24", f'<path d="{path}"/>')


def star(rng, x, y, r, rot, irregular):
    """The parish star: four long rays, four short diagonals, each a little uneven."""
    points = []
    for i in range(16):
        a = rot + i * math.pi / 8 - math.pi / 2
        rr = 1.0 if i % 4 == 0 else 0.5 if i % 4 == 2 else 0.17
        rr *= 1 + rng.uniform(-irregular, irregular)
        points.append((x + r * rr * math.cos(a), y + r * rr * math.sin(a)))
    return fmt(points)


def vault(seed=1120, tile=528, count=40, attempts=40000):
    rng = random.Random(seed)
    placed = []
    for _ in range(attempts):
        if len(placed) >= count:
            break
        x, y = rng.uniform(0, tile), rng.uniform(0, tile)
        size = rng.choices([4.5, 6.5, 9], weights=[0.5, 0.32, 0.18])[0]
        clear = True
        for px, py, ps in placed:
            dx = min(abs(x - px), tile - abs(x - px))
            dy = min(abs(y - py), tile - abs(y - py))
            if dx * dx + dy * dy < (46 + 3 * (size + ps)) ** 2:
                clear = False
                break
        if clear:
            placed.append((x, y, size))
    parts = []
    for x, y, size in placed:
        opacity = rng.uniform(0.45, 0.85)
        rot, irregular = rng.gauss(0, 0.06), 0.1
        shape = None
        # A star crossing the tile's edge is drawn again on the far side, so the repeat is seamless.
        for ox in (-tile, 0, tile):
            for oy in (-tile, 0, tile):
                cx, cy = x + ox, y + oy
                if -20 < cx < tile + 20 and -20 < cy < tile + 20:
                    if shape is None:
                        state = rng.getstate()
                    else:
                        rng.setstate(state)
                    shape = star(rng, cx, cy, size, rot, irregular)
                    parts.append(f'<path d="{shape}" fill-opacity="{opacity:.2f}"/>')
    return svg(f"0 0 {tile} {tile}", "".join(parts)), len(placed)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--out", type=Path, default=OUT)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    tile, stars = vault()
    for name, body in [("consecration.svg", consecration()), ("cross.svg", cross()),
                       ("sun.svg", sun()), ("moon.svg", moon()), ("vault.svg", tile)]:
        path = args.out / name
        path.write_text(body)
        print("wrote", path, f"({stars} stars)" if name == "vault.svg" else "")


if __name__ == "__main__":
    main()
