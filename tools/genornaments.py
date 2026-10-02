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
  ornaments/powder.svg        one cell of the Nave wall's powdering: a six-petal
                              rosette stencilled on a quincunx lattice (96px
                              across, 84px between rows, alternate rows set
                              half a cell over), as the margins of English
                              parish walls were powdered in the fourteenth and
                              fifteenth centuries; the Nave's counterpart to
                              the vault
  ornaments/quatrefoil.svg    the Gothic quatrefoil (corner knops of painted
                              frames)
  ornaments/tailpiece.svg     a quatrefoil between two painted rules that thin
                              toward their ends: the footer's tailpiece, closing
                              each page as the headpiece's cross opens it
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


def disc(cx, cy, r, steps=20):
    return fmt([(cx + r * math.cos(2 * math.pi * i / steps), cy + r * math.sin(2 * math.pi * i / steps))
                for i in range(steps)])


def quatrefoil_paths(cx, cy, r):
    """Four lobes about a centre, as cut in tracery and painted in borders.
    The lobes stand at 0.46r from the centre with radius 0.5r, so the cusps
    between them fall well inside the lobes' tips."""
    parts = [disc(cx, cy, 0.5 * r)]
    for k in range(4):
        a = k * math.pi / 2
        parts.append(disc(cx + 0.46 * r * math.cos(a), cy + 0.46 * r * math.sin(a), 0.5 * r))
    return "".join(parts)


def quatrefoil():
    return svg("0 0 20 20", f'<path d="{quatrefoil_paths(10, 10, 9.6)}"/>')


def tailpiece():
    """Two painted rules, each thinning as the brush lifts, meeting a quatrefoil."""
    w, h, c = 120, 14, 7
    q = quatrefoil_paths(w / 2, c, 6.2)
    left = fmt([(5, c - 0.3), (50.5, c - 0.75), (50.5, c + 0.75), (5, c + 0.3)])
    right = fmt([(w - 5, c - 0.3), (w - 50.5, c - 0.75), (w - 50.5, c + 0.75), (w - 5, c + 0.3)])
    return svg(f"0 0 {w} {h}", f'<path d="{q}{left}{right}"/>')



def rosette(cx, cy, r, rot, steps=10):
    """A six-petal rosette, the stencil's flower: petals as ellipses about a
    small centre, each petal's long axis on its own ray."""
    parts = [disc(cx, cy, 0.2 * r, 12)]
    for k in range(6):
        a = rot + k * math.pi / 3
        px, py = cx + 0.56 * r * math.cos(a), cy + 0.56 * r * math.sin(a)
        ca, sa = math.cos(a), math.sin(a)
        pts = []
        for i in range(steps):
            t = 2 * math.pi * i / steps
            u, v = 0.44 * r * math.cos(t), 0.27 * r * math.sin(t)
            pts.append((px + u * ca - v * sa, py + u * sa + v * ca))
        parts.append(fmt(pts))
    return "".join(parts)


def powder(cell_w=96, cell_h=84, radius=7.0):
    """Rosettes powdered over the limewash on a quincunx: one rosette to a
    cell, alternate rows set half a cell over, so the tile is one cell wide
    and two rows tall. Powdering was ordered, not sprinkled."""
    tile_w, tile_h = cell_w, 2 * cell_h
    rows = [(cell_w / 4, cell_h / 2), (3 * cell_w / 4, 3 * cell_h / 2)]
    parts = [f'<path d="{rosette(x, y, radius, 0)}"/>' for x, y in rows]
    return svg(f"0 0 {tile_w} {tile_h}", "".join(parts)), len(rows)


def vault(seed=1120, tile=528, count=64, attempts=60000):
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
            if dx * dx + dy * dy < (26 + 3 * (size + ps)) ** 2:
                clear = False
                break
        if clear:
            placed.append((x, y, size))
    parts = []
    for x, y, size in placed:
        opacity = rng.uniform(0.6, 0.95)
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
    field, rosettes = powder()
    counts = {"vault.svg": f"({stars} stars)", "powder.svg": f"({rosettes} rosettes a cell)"}
    for name, body in [("consecration.svg", consecration()), ("cross.svg", cross()),
                       ("sun.svg", sun()), ("moon.svg", moon()), ("vault.svg", tile),
                       ("powder.svg", field), ("quatrefoil.svg", quatrefoil()),
                       ("tailpiece.svg", tailpiece())]:
        path = args.out / name
        path.write_text(body)
        print("wrote", path, counts.get(name, ""))


if __name__ == "__main__":
    main()
