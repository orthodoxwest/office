#!/usr/bin/env python3
"""Draw the home frontispiece's pointed head as CSS clip-path polygons.

The head is a steep four-centred arch: short haunch arcs centred on the
springing line, so the head leaves the jambs without a kink, then long upper
arcs centred below it, meeting at the point. The phone's head rises 0.44 of
the card's width, the wider niche's 0.40 so a laptop still shows the hours;
the date sits where the phone's head has opened wide enough for it, and the
stylesheet's fitted date measure assumes this phone shape.

Every layer of the frontispiece (the day's ring, the stone, the panel, the
lining and its hairline) is the same arch offset by its own distance, as a
moulding's lines are: each arc keeps its centre and gains or loses that
distance from its radius, and the two upper arcs meet in a mitred point. So
the polygons are written once, in terms of three custom properties the
stylesheet sets per layer:

  --arch-w     the card's width (from container units: the card is an
               inline-size container, so its layers can measure it)
  --arch-lift  how far above the card each layer's box starts, room for an
               outer layer's point
  --arch-d     the layer's offset outward from the card's edge (negative
               inward); its box is the card's grown by it at the sides and
               foot
  --arch-t     a rule's width, for the lining's open band

The head scales with the card's width, so its points are a percentage of the
layer's width plus multiples of --arch-d across, and multiples of --arch-w
and --arch-d down.

The native apps draw the same heads with true arcs rather than polygons, so
they take each shape's figures (its arcs' centres and radii, in card widths)
and offset the arcs themselves: Arch.kt for Android, Arch.swift for iOS.

Plain Python, no dependencies; rewrites the marked blocks in style.css,
Arch.kt and Arch.swift. Run from the repository root.
"""
import math
import re
from pathlib import Path

CSS = Path("apps/office-web/static/style.css")
BEGIN = "/* genarch:begin"
END = "/* genarch:end */"
KOTLIN = Path("apps/android/app/src/main/kotlin/org/orthodoxwest/office/Arch.kt")
SWIFT = Path("apps/ios/Office/Arch.swift")
NATIVE_BEGIN = "// genarch:begin"
NATIVE_END = "// genarch:end"

# rise, haunch radius and the upper arc's centre across, all in card widths
# from the left springing. The haunches are generous (an upper arc no more
# than about four times their radius) and finely sampled, so the jamb eases
# into the curve as a Gothic arch's does rather than turning on a tight,
# faceted shoulder; the phone comes to a 144° point, the niche to 146°.
# Changing the phone's shape means refitting the date's max-width in
# style.css (".page-home .home h1").
# A taller phone has height to spare under the card, so its head rises
# further and comes to a sharper point: "tall" from 800px high, "taller"
# from 880px.
SHAPES = {
    "phone": dict(rise=0.44, haunch=0.30, centre=0.90, haunch_steps=7, upper_steps=10),
    "tall": dict(rise=0.56, haunch=0.40, centre=1.0, haunch_steps=8, upper_steps=10),
    "taller": dict(rise=0.64, haunch=0.40, centre=1.05, haunch_steps=8, upper_steps=11),
    "niche": dict(rise=0.40, haunch=0.24, centre=0.86, haunch_steps=8, upper_steps=16),
}
# Where each shape applies, in order (later ones win).
MEDIA = {
    "phone": None,
    "tall": "(max-width: 700px) and (min-height: 800px)",
    "taller": "(max-width: 700px) and (min-height: 880px)",
    "niche": "(min-width: 701px)",
}
# The deepest inward layer each shape draws (the lining's hairline), in card
# widths at the narrowest card: the samples next to the point must not cross
# the centre line at that depth.
DEEPEST = {"phone": 22 / 288, "tall": 22 / 288, "taller": 22 / 288, "niche": 40 / 600}


def solve(rise, haunch, centre, **_):
    """The upper arc's centre height and radius that make it tangent to the
    haunch and pass through the point."""
    def gap(cy):
        radius = haunch + math.hypot(centre - haunch, cy)
        return math.hypot(0.5 - centre, rise - cy) - radius

    lo, hi = -8.0, -1e-9
    for _ in range(200):
        mid = (lo + hi) / 2
        if gap(lo) * gap(mid) <= 0:
            hi = mid
        else:
            lo = mid
    cy = (lo + hi) / 2
    radius = haunch + math.hypot(centre - haunch, cy)
    return cy, radius


def left_side(shape):
    """Points up the left side of the head, each with its outward normal,
    from the springing to (not including) the point. y runs up."""
    rise, haunch, centre = shape["rise"], shape["haunch"], shape["centre"]
    cy, radius = solve(**shape)
    # The tangent point lies on the line through both centres.
    hx, hy = haunch - centre, 0 - cy
    length = math.hypot(hx, hy)
    tangent = (centre + hx * radius / length, cy + hy * radius / length)
    points = []
    start = math.pi
    turn = math.atan2(tangent[1], tangent[0] - haunch)
    for i in range(shape["haunch_steps"]):
        a = start + (turn - start) * i / shape["haunch_steps"]
        nx, ny = math.cos(a), math.sin(a)
        points.append((haunch + haunch * nx, haunch * ny, nx, ny))
    a0 = math.atan2(tangent[1] - cy, tangent[0] - centre)
    a1 = math.atan2(rise - cy, 0.5 - centre)
    for i in range(shape["upper_steps"]):
        a = a0 + (a1 - a0) * i / shape["upper_steps"]
        nx, ny = math.cos(a), math.sin(a)
        points.append((centre + radius * nx, cy + radius * ny, nx, ny))
    # The mitre: offset lines meet on the centre line, 1/cos of the
    # tangent's slope above the point.
    slope = math.atan2(abs(0.5 - centre), rise - cy)
    apex = (0.5, rise, 0.0, 1 / math.cos(slope))
    return points, apex


def num(v, digits=4):
    s = f"{v:.{digits}f}".rstrip("0").rstrip(".")
    return "0" if s in ("-0", "") else s


def term(coef, var):
    if abs(coef) < 5e-5:
        return ""
    sign = "-" if coef < 0 else "+"
    return f" {sign} {num(abs(coef))} * var({var})"


def point(rise, u, v, nx, ny, inner=False):
    """One polygon vertex. Across: the card's u is u of the layer's width
    less its two offsets, plus the offset itself, plus the normal's share of
    it. Down: the lift, the depth below the point's line, less the normal's
    share. An inner edge sits --arch-t further in."""
    across = f"calc({num(u * 100, 3)}%{term(1 - 2 * u + nx, '--arch-d')}"
    down = f"calc(var(--arch-lift) + {num(rise - v)} * var(--arch-w){term(-ny, '--arch-d')}"
    if inner:
        across += term(-nx, "--arch-t")
        down += term(ny, "--arch-t")
    return f"{across}) {down})"


def outline(shape, inner=False):
    """The head from the left springing over the point to the right one."""
    side, apex = left_side(shape)
    rise = shape["rise"]
    right = [(1 - u, v, -nx, ny) for (u, v, nx, ny) in reversed(side)]
    return [point(rise, *p, inner=inner) for p in side + [apex] + right]


def check(name, shape):
    side, _ = left_side(shape)
    u, v, nx, ny = side[-1]
    # Inward by the deepest layer, the last sample before the point must
    # stay left of the centre line.
    assert u - nx * DEEPEST[name] < 0.5, f"{name}: samples cross at the point"


def rules(name, shape):
    check(name, shape)
    head = outline(shape)
    foot_left, foot_right = "0 100%", "100% 100%"
    silhouette = ",\n    ".join([foot_left] + head + [foot_right])
    band = ",\n    ".join(
        [foot_left] + head + [foot_right, f"calc(100% - var(--arch-t)) 100%"]
        + list(reversed(outline(shape, inner=True)))
        + ["var(--arch-t) 100%"]
    )
    outside = ",\n    ".join(
        ["-10% -10%", "110% -10%", "110% 110%", "-10% 110%", "-10% -10%", foot_left]
        + head + [foot_right, foot_left]
    )
    return (
        f".page-home .home-hero {{\n  --arch-rise: {num(shape['rise'])};\n}}\n\n"
        ".page-home .home-arch::before,\n.page-home .home-arch::after,\n"
        ".page-home .home-arch-edge,\n.page-home .home-hero::before,\n"
        ".page-home .home-hero::after,\n.page-home .home-arch-fill {\n"
        f"  clip-path: polygon(\n    {silhouette});\n}}\n\n"
        ".page-home .home-arch-shade::before {\n"
        f"  clip-path: polygon(evenodd,\n    {outside});\n}}\n\n"
        ".page-home .home-lining::before,\n.page-home .home-lining::after {\n"
        f"  clip-path: polygon(\n    {band});\n}}\n"
    )


def indent(block):
    return "".join("  " + line if line.strip() else line for line in block.splitlines(True))


def point_angle(shape):
    cy, _ = solve(**shape)
    slope = math.degrees(math.atan2(abs(0.5 - shape["centre"]), shape["rise"] - cy))
    return 180 - 2 * slope


def figures(shape):
    """A shape's arcs for the native apps: the upper arcs' centre depth below
    the springing and their radius, with the shape's own figures."""
    cy, radius = solve(**shape)
    return dict(rise=shape["rise"], haunch=shape["haunch"], centre=shape["centre"], depth=-cy, radius=radius)


# Where the native apps use each shape: a phone's by the height it has for
# home, as the web's by its viewport's.
NATIVE_WHERE = {
    "phone": "a phone",
    "tall": "a phone from 800 high",
    "taller": "a phone from 880 high",
    "niche": "a wide screen's niche",
}


def kotlin():
    lines = []
    for name, shape in SHAPES.items():
        f = {k: num(v, 6) for k, v in figures(shape).items()}
        lines.append(f"/** The {name} head, for {NATIVE_WHERE[name]}: a {point_angle(shape):.0f}° point. */")
        lines.append(
            f"val {name.capitalize()}Arch = Arch(rise = {f['rise']}f, haunch = {f['haunch']}f, "
            f"centre = {f['centre']}f, depth = {f['depth']}f, radius = {f['radius']}f)"
        )
    return "\n".join(lines) + "\n"


def swift():
    lines = ["extension Arch {"]
    for name, shape in SHAPES.items():
        f = {k: num(v, 6) for k, v in figures(shape).items()}
        lines.append(f"    /// The {name} head, for {NATIVE_WHERE[name]}: a {point_angle(shape):.0f}° point.")
        lines.append(
            f"    static let {name} = Arch(rise: {f['rise']}, haunch: {f['haunch']}, "
            f"centre: {f['centre']}, depth: {f['depth']}, radius: {f['radius']})"
        )
    lines.append("}")
    return "\n".join(lines) + "\n"


def rewrite(path, begin, end, generated):
    text = path.read_text()
    pattern = re.compile(re.escape(begin) + r".*?" + re.escape(end), re.S)
    if not pattern.search(text):
        raise SystemExit(f"{path}: no genarch block to replace")
    path.write_text(pattern.sub(lambda _: generated, text, count=1))


def main():
    blocks = []
    for name, shape in SHAPES.items():
        media = MEDIA[name]
        block = rules(name, shape)
        blocks.append(block if media is None else f"@media {media} {{\n{indent(block)}}}\n")
    rewrite(CSS, BEGIN, END, (
        f"{BEGIN}: tools/genarch.py writes this block; edit the script, not the polygons. */\n"
        + "\n".join(blocks)
        + END
    ))
    native = f"{NATIVE_BEGIN}: tools/genarch.py writes this block; edit the script, not the figures.\n"
    rewrite(KOTLIN, NATIVE_BEGIN, NATIVE_END, native + kotlin() + NATIVE_END)
    rewrite(SWIFT, NATIVE_BEGIN, NATIVE_END, native + swift() + NATIVE_END)
    for name, shape in SHAPES.items():
        cy, radius = solve(**shape)
        print(f"{name}: point {point_angle(shape):.0f}°, upper radius {radius:.3f}, centre {cy:.3f} below")


if __name__ == "__main__":
    main()
