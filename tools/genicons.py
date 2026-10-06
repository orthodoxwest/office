#!/usr/bin/env python3
"""Render the app icon for the web, iOS and Android.

The icon is the consecration cross in its compass ring, the mark that ends
every hour in the app, laid in matte gold leaf on the blue-green limewash of
the parish apse. The geometry is genornaments' (flared arms, one ring), at the
proportions the icon needs to survive 48px: the ring's outer edge at 0.78 of
the tile's radius, nothing but plain wall outside it. A second rendering, for
iOS's light appearance only, scribes the same mark in red ochre on the Nave's
rose limewash.

One scene feeds every platform. It is rendered on a 1536 canvas that is an
Android adaptive layer (108dp); the central 1024 is the 72dp a launcher shows
and is the iOS, web and Play Store tile. Small one-colour uses (favicon,
Android's themed icon and notification) are flat vectors of the same ring and
cross.

Requires tools/requirements.txt. Seeded, so it regenerates byte for byte.
Run from the repository root; it checks its own output (ring width, contrast
and overall darkness at launcher size) and fails if the icon stops reading.
"""
import argparse
import json
import math
import sys
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw
from scipy import ndimage as ndi

sys.path.insert(0, str(Path(__file__).resolve().parent))
import genornaments as orn  # noqa: E402

ROOT = Path(__file__).resolve().parents[1]
WEB = ROOT / "apps/office-web/static"
IOS = ROOT / "apps/ios/Office/Assets.xcassets/AppIcon.appiconset"
ANDROID = ROOT / "apps/android/app/src/main/res"

N = 1536                       # the canvas: an adaptive icon's 108dp layer
C = N / 2
TILE = 1024                    # the visible 72dp, cropped from the centre
K = N / TILE                   # noise cell counts scale with the canvas, so the wall's clouds keep their size
SS = 4                         # mask supersampling
Y, X = np.mgrid[0:N, 0:N].astype(np.float32)
R = np.sqrt((X - C) ** 2 + (Y - C) ** 2)
LIGHT = np.array([-0.55, -0.65, 0.75]) / np.linalg.norm([-0.55, -0.65, 0.75])

# The mark, in pixels of the 1024 tile (ring outer 400 = 0.78 R, so every launcher mask keeps it with room):
# ring 352..400; arms reach 286 (gap 66 to the ring, 3px at 48) from a root 68 wide, flaring to 17.5 degrees.
RING_OUT, RING_W = 400, 48
RING_IN = RING_OUT - RING_W
REACH, ROOT_HALF, CENTRE_HALF, HALF_ANGLE = 286, 34, 34, 17.5

# Colours.
APSE_LO, APSE_HI = "#223b47", "#2c4b57"          # the parish apse's blue-green limewash
LEAF = (0.90, 0.75, 0.44)                        # matte leaf, #d6b062 at rest
NAVE_BASE, NAVE_TINT = "#efe1d0", "#dcc6b0"      # the Nave's rose-buff limewash
OCHRE_DARK, OCHRE_LIGHT = "#733a2a", "#8f4838"   # red ochre, dark where the brush pooled
GILT_THREAD = "#c9a24e"                          # the mordant-gilt thread on the groove's wall


# ---------------------------------------------------------------- helpers
def rgb(h):
    return np.array([int(h[i:i + 2], 16) for i in (1, 3, 5)], np.float32) / 255


def fill(h):
    return np.ones((N, N, 3), np.float32) * rgb(h)


def lin(c):
    c = np.asarray(c, np.float32)
    return np.where(c <= 0.04045, c / 12.92, ((c + 0.055) / 1.055) ** 2.4)


def srgb(c):
    c = np.clip(np.asarray(c, np.float32), 0, 1)
    return np.where(c <= 0.0031308, c * 12.92, 1.055 * c ** (1 / 2.4) - 0.055)


def luma(img):
    c = lin(img)
    return 0.2126 * c[..., 0] + 0.7152 * c[..., 1] + 0.0722 * c[..., 2]


def lerp(a, b, t):
    t = np.asarray(t, np.float32)
    if t.ndim == 2:
        t = t[..., None]
    return a * (1 - t) + b * t


def over(base, top, alpha):
    return lerp(base, top, np.clip(alpha, 0, 1))


_fbm_cache = {}


def fbm(octaves, base, seed, falloff=0.55):
    """Fractal noise, zero mean and unit variance; `base` is the cell count across a 1024 tile."""
    key = (octaves, base, seed, falloff)
    if key not in _fbm_cache:
        rng = np.random.default_rng(seed)
        out = np.zeros((N, N), np.float32)
        amp, total = 1.0, 0.0
        for o in range(octaves):
            cells = int(round(base * K)) * 2 ** o
            z = ndi.zoom(rng.standard_normal((cells + 3, cells + 3)).astype(np.float32), N / cells, order=3)[:N, :N]
            out += amp * z
            total += amp
            amp *= falloff
        out /= total
        _fbm_cache[key] = (out - out.mean()) / (out.std() + 1e-6)
    return _fbm_cache[key]


def hash2(a, b, salt):
    h = (a.astype(np.int64) * 73856093) ^ (b.astype(np.int64) * 19349663) ^ (salt * 83492791)
    h = (h ^ (h >> 13)) * 1274126177
    h = h ^ (h >> 16)
    return ((h & 0xFFFFFF) / float(0xFFFFFF)).astype(np.float32)


def normals(h, strength):
    gy, gx = np.gradient(h * strength)
    n = np.dstack([-gx, -gy, np.ones_like(h)])
    return n / np.linalg.norm(n, axis=2, keepdims=True)


def lambert(n):
    return np.clip((n * LIGHT).sum(2), 0, 1)


def spec(n, power):
    half = LIGHT + np.array([0, 0, 1.0])
    half /= np.linalg.norm(half)
    return np.clip((n * half).sum(2), 0, 1) ** power


def sdf(m):
    """Signed distance (px) from the mask's edge; positive outside."""
    inside = m > 0.5
    return ndi.distance_transform_edt(~inside) - ndi.distance_transform_edt(inside)


# ---------------------------------------------------------------- the mark
def mask(polys):
    im = Image.new("L", (N * SS, N * SS), 0)
    d = ImageDraw.Draw(im)
    for p in polys:
        d.polygon([(x * SS, y * SS) for x, y in p], fill=255)
    a = np.asarray(im, np.float32) / 255
    return a.reshape(N, SS, N, SS).mean(axis=(1, 3))


def ring_mask(r_in, r_out):
    return np.clip(r_out - R + 0.5, 0, 1) * np.clip(R - r_in + 0.5, 0, 1)


def cross_polys(c=C):
    arms = [orn.rotate(orn.flared_arm(c, REACH, HALF_ANGLE, ROOT_HALF, steps=40), k, c) for k in range(4)]
    box = [(c - CENTRE_HALF, c - CENTRE_HALF), (c + CENTRE_HALF, c - CENTRE_HALF),
           (c + CENTRE_HALF, c + CENTRE_HALF), (c - CENTRE_HALF, c + CENTRE_HALF)]
    return arms + [box]


_marks = {}


def marks(ring_w=RING_W):
    if ring_w not in _marks:
        _marks[ring_w] = (ring_mask(RING_OUT - ring_w, RING_OUT), mask(cross_polys()))
    return _marks[ring_w]


def chipped(m, seed, frac, depth, rmin=0):
    """Flake chips along the leaf's stop edge: small bites of a few px where the leaf did not take."""
    rng = np.random.default_rng(seed)
    band = np.clip(m - ndi.binary_erosion(m > 0.5, iterations=depth).astype(np.float32), 0, 1)
    cells = int(150 * K)
    z = ndi.zoom(rng.standard_normal((cells, cells)).astype(np.float32), N / cells, order=3)[:N, :N]
    z = ndi.gaussian_filter(z, 1.5)
    thr = np.quantile(z[band > 0.5], 1 - frac)
    chips = (z > thr) & (band > 0.5) & (R > rmin)
    d = ndi.distance_transform_edt(~chips)
    return m * np.clip(d - 0.5, 0, 1)


# ---------------------------------------------------------------- the apse by night
def limewash(c_lo, c_hi, seed, sponge=0.06, cloud=0.8, fine=0.012):
    """Sponge-applied limewash: broad clouds of thicker and thinner wash between two tints, the sponge's
    dabs as a mid-scale mottle, and a whisper of grain that vanishes under 96px."""
    clouds = fbm(3, 2, seed, 0.6)
    dabs = ndi.gaussian_filter(fbm(3, 10, seed + 1, 0.6), 1.5)
    grain = fbm(2, 90, seed + 2, 0.5)
    t = np.clip(0.5 + 0.18 * clouds * cloud + 0.22 * dabs, 0, 1)
    lo, hi = lin(rgb(c_lo)), lin(rgb(c_hi))
    col = lo * (1 - t)[..., None] + hi * t[..., None]
    col = col * (1 + sponge * dabs + fine * grain)[..., None]
    return srgb(col)


def gold_leaf(seed, cell=92, angle=6.0, base=LEAF, spread=0.09, lap=0.055, burnish=0.06):
    """Hand-laid matte leaf: squares a shade apart, a lap of double thickness where sheets overlap,
    soft burnish streaks, no gradient and no crumple."""
    a = math.radians(angle)
    u = ((X - C) * math.cos(a) - (Y - C) * math.sin(a)) / cell
    v = ((X - C) * math.sin(a) + (Y - C) * math.cos(a)) / cell
    iu, iv = np.floor(u), np.floor(v)
    fu, fv = u - iu, v - iv
    tone = (hash2(iu, iv, seed) - 0.5) * 2 * spread
    lapw = 7.0 / cell
    lapm = np.clip((lapw - fu) / (lapw * 0.4) + 0.5, 0, 1) + np.clip((lapw - fv) / (lapw * 0.4) + 0.5, 0, 1)
    lapm = np.clip(lapm, 0, 1)
    streak = fbm(3, 3, seed + 7, 0.5)
    s2 = ndi.gaussian_filter(streak, (2, 24))
    s2 = (s2 - s2.mean()) / (s2.std() + 1e-6)
    tooth = fbm(2, 64, seed + 9)
    sheen = ndi.gaussian_filter(fbm(2, 2, seed + 5), 8)
    sheen = (sheen - sheen.mean()) / (sheen.std() + 1e-6)
    per = (hash2(iu, iv, seed + 3) - 0.5) * 0.06   # each sheet was burnished separately
    e = 1 + tone + lap * lapm + burnish * 0.5 * s2 * (1 + 4 * per) + 0.05 * sheen + per + 0.012 * tooth
    col = lin(np.array(base, np.float32))[None, None, :] * e[..., None]
    return srgb(col)


def relief(m, soft=1.0, strength=2.0, amount=0.10, spec_amount=0.03, spec_power=40):
    """Light the raised mark only, from the upper left: the gesso build-up under the leaf."""
    h = ndi.gaussian_filter(m, soft)
    n = normals(h, strength)
    flat = normals(np.zeros_like(m), 1)
    f = (1 - amount) + amount * lambert(n) / max(lambert(flat).max(), 1e-6)
    return f + spec(n, spec_power) * spec_amount - spec(flat, spec_power).max() * spec_amount


def apse():
    """Matte gold leaf on the apse's blue-green sponge limewash. Returns (scene, ground)."""
    ring, cross = marks()
    ring = chipped(ring, 81, 0.04, 5)
    cross = chipped(cross, 82, 0.035, 5, rmin=0.6 * REACH)
    vault = limewash(APSE_LO, APSE_HI, seed=5)
    leaf_ring = gold_leaf(seed=3, cell=92, angle=22)
    leaf_cross = gold_leaf(seed=11, cell=92, angle=-5)
    m_all = np.clip(ring + cross, 0, 1)
    lit = relief(m_all)
    gold = over(leaf_ring, leaf_cross, cross)
    gold = srgb(lin(gold) * lit[..., None])
    return np.clip(over(vault, gold, m_all), 0, 1), vault


# ---------------------------------------------------------------- the nave by day
def plaster_field():
    """The site's own limewash clouds (static/plaster.jpg), zero mean and unit variance."""
    im = Image.open(WEB / "plaster.jpg").convert("L")
    side = min(im.size)
    box = ((im.width - side) // 2, (im.height - side) // 2)
    im = im.crop((box[0], box[1], box[0] + side, box[1] + side)).resize((N, N), Image.LANCZOS)
    p = np.asarray(im, np.float32)
    return (p - p.mean()) / p.std()


def aniso(cells_y, cells_x, seed):
    rng = np.random.default_rng(seed)
    cells_y, cells_x = int(round(cells_y * K)), int(round(cells_x * K))
    z = rng.standard_normal((cells_y + 3, cells_x + 3)).astype(np.float32)
    out = ndi.zoom(z, (N / cells_y, N / cells_x), order=3)[:N, :N]
    return (out - out.mean()) / (out.std() + 1e-6)


def streaks(seed=41):
    """The brush's direction: along the arms and round the ring."""
    vert = 0.7 * aniso(5, 72, seed) + 0.3 * aniso(12, 160, seed + 1)
    horiz = 0.7 * aniso(72, 5, seed + 2) + 0.3 * aniso(160, 12, seed + 3)
    rng = np.random.default_rng(seed + 4)
    gy, gx = int(64 * K), int(28 * K)
    grid = ndi.zoom(rng.standard_normal((gy + 3, gx)).astype(np.float32), (N / gy, N / gx), order=3)[:N, :N]
    grid = (grid - grid.mean()) / grid.std()
    th = (np.arctan2(Y - C, X - C) / (2 * math.pi) % 1.0) * (N - 1)
    rr = np.clip((R + 18 * fbm(2, 6, seed + 5)) / C * (N - 1), 0, N - 1)
    ring = ndi.map_coordinates(grid, [rr, th], order=1, mode="wrap")
    arm = np.where(np.abs(X - C) < np.abs(Y - C), vert, horiz)
    return np.where(R > RING_OUT - RING_W / 2 - RING_W * 1.5, ring, arm)


def brushed(m, strokes, bleed=2.2, bleed_seed=23, load_seed=31, halo=0.0, streak_weight=0.13):
    """Coverage of a pigment brushed onto lime: the edge wanders a little as it bleeds into the wash,
    and the loading is uneven where the brush ran dry. Returns (coverage, loading)."""
    d = sdf(m)
    wander = fbm(2, 12, bleed_seed, 0.5) * bleed + 0.45 * fbm(2, 160, bleed_seed + 3, 0.5)
    edge = np.clip(0.5 - (d + wander) / 2.0, 0, 1)
    feather = halo * np.clip(1 - np.maximum(d + wander, 0) / 5, 0, 1) * np.clip(fbm(3, 24, bleed_seed + 1) * 0.6 + 0.5, 0, 1)
    cov = np.clip(edge + feather * (1 - edge), 0, 1)
    load = fbm(4, 7, load_seed, 0.5)
    load = np.clip(0.5 + 0.26 * load + streak_weight * strokes + 0.06 * fbm(3, 40, load_seed + 1), 0, 1)
    return cov, load


def nave():
    """The scribed wall by day: the mark in red ochre on rose-buff limewash, the compass's groove round the
    ring catching the light, a mordant-gilt thread on the groove's inner wall. Returns (scene, ground)."""
    field = ndi.gaussian_filter(plaster_field(), 2.5)
    field = (field - field.mean()) / field.std()
    trowel = fbm(4, 10, 17, 0.5)
    soft = ndi.gaussian_filter(trowel, 3)
    clouds = np.clip(-field * 0.11, 0, 1)
    wall = lerp(fill(NAVE_BASE), fill(NAVE_TINT), clouds) * (1 + 0.012 * soft)[..., None]
    h = field * 0.8 + 0.4 * soft
    strokes = streaks()

    ring, cross = marks(RING_W + 4)              # the ochre fills the groove a hair past the scribe
    m = np.clip(ring + cross, 0, 1)
    cov, load = brushed(m, strokes, halo=0.10)
    pig = lerp(fill(OCHRE_DARK), fill(OCHRE_LIGHT), load)
    pig = pig * (1 + 0.05 * field + 0.02 * strokes)[..., None]
    d = sdf(m)
    pig = pig * (1 - np.clip(1 - np.abs(d + 2.5) / 4, 0, 1) * 0.12)[..., None]   # darker where it pooled
    col = over(wall, pig, cov * 0.96)

    # the compass's shallow V groove, lit from the upper left; the ring only, nothing on the tile
    d_ring = np.abs(R - (RING_OUT - RING_W / 2))
    groove = -np.clip(1 - d_ring / (RING_W / 2 + 6), 0, 1) ** 1.3 * 6.0 + h * 0.5
    n = normals(ndi.gaussian_filter(groove, 1.2), 1.6)
    shade = 0.86 + 0.24 * lambert(n)
    where = np.clip(1 - np.maximum(d_ring - RING_W / 2 - 6, 0) / 10, 0, 1)
    col = col * lerp(np.ones_like(col), shade[..., None] * np.ones_like(col), where)
    # the pigment's own thickness on the cross
    cov_x, _ = brushed(cross, strokes)
    n2 = normals(ndi.gaussian_filter(cov_x, 1.5) * 3.0 + h * 0.5, 1.0)
    lit = ndi.binary_dilation(cross > 0.5, iterations=4).astype(np.float32)
    col = col * lerp(np.ones_like(col), (0.84 + 0.16 * lambert(n2))[..., None] * np.ones_like(col), lit)
    # the gilt thread, 3px, on the groove's inner wall
    lip = ring_mask(RING_IN + 1.5, RING_IN + 4.5)
    gold = fill(GILT_THREAD) * (0.94 + 0.08 * lambert(n))[..., None]
    return np.clip(over(col, gold, lip * 0.95), 0, 1), wall


def tinted(scene, ground):
    """Grayscale for iOS's tinted appearance: the mark light, the wall dark, nothing else."""
    ring, cross = marks()
    m = np.clip(ring + cross, 0, 1)
    lum = luma(scene)
    lum = np.where(m > 0.02, 0.72 + 0.28 * np.clip((lum - 0.35) / 0.35, 0, 1), 0.10)
    return np.repeat(srgb(lum)[..., None], 3, axis=2)


# ---------------------------------------------------------------- outputs
def tile(arr):
    """The central 1024 of the canvas: what a launcher shows."""
    o = (N - TILE) // 2
    return arr[o:o + TILE, o:o + TILE]


def to_image(arr):
    return Image.fromarray((np.clip(arr, 0, 1) * 255 + 0.5).astype(np.uint8), "RGB")


def save(img, path, palette):
    """Limewash is noise to PNG's filters; a 256-colour palette costs nothing visible and halves the bytes.
    iOS keeps full colour for its App Store artwork."""
    path.parent.mkdir(parents=True, exist_ok=True)
    if palette:
        img = img.quantize(256, method=Image.Quantize.MEDIANCUT, dither=Image.Dither.FLOYDSTEINBERG)
    img.save(path, optimize=True)
    print("wrote", path.relative_to(ROOT))


def save_rgb(arr, path, size, palette=True):
    save(to_image(arr).resize((size, size), Image.LANCZOS), path, palette)


# ---------------------------------------------------------------- vectors
def path_data(points, scale, mid):
    pts = [(mid + (x - C) * scale, mid + (y - C) * scale) for x, y in points]
    return "M" + " ".join(f"{x:.2f},{y:.2f}" for x, y in pts) + "Z"


def circle_path(mid, r):
    return f"M{mid - r:.2f},{mid:.2f}a{r:.2f},{r:.2f} 0 1,0 {2 * r:.2f},0a{r:.2f},{r:.2f} 0 1,0 {-2 * r:.2f},0Z"


def mark_paths(viewport, outer, ring_w=None):
    """The ring (even-odd) and the cross, scaled so the ring's outer edge sits at `outer` from the centre."""
    s = outer / RING_OUT
    mid = viewport / 2
    w = ring_w if ring_w is not None else RING_W * s
    ring = circle_path(mid, outer) + circle_path(mid, outer - w)
    cross = "".join(path_data(p, s, mid) for p in cross_polys())
    return ring, cross


def favicon():
    ring, cross = mark_paths(32, 15.0, ring_w=2.0)
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="{APSE_LO}"/>
  <path fill-rule="evenodd" d="{ring}" fill="#d6b062"/>
  <path d="{cross}" fill="#d6b062"/>
</svg>
'''


def vector_drawable(size_dp, viewport, paths, comment):
    body = "\n".join(
        f'    <path\n        android:fillColor="#FFFFFFFF"\n'
        + ('        android:fillType="evenOdd"\n' if even_odd else "")
        + f'        android:pathData="{d}" />'
        for d, even_odd in paths
    )
    return f'''<?xml version="1.0" encoding="utf-8"?>
<!-- {comment} Generated by tools/genicons.py. -->
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="{size_dp}dp"
    android:height="{size_dp}dp"
    android:viewportWidth="{viewport}"
    android:viewportHeight="{viewport}">
{body}
</vector>
'''


# ---------------------------------------------------------------- checks
def contrast(a, b):
    hi, lo = max(a, b), min(a, b)
    return (hi + 0.05) / (lo + 0.05)


def check(name, scene, dark):
    """Measure the tile as a launcher shows it, after Lanczos to 48px: the ring must still be 2px wide, ring and
    cross at least 4.5:1 against the wall, and (by night) the tile dark enough to sit beside flat icons."""
    t = tile(scene)
    small = np.asarray(to_image(t).resize((48, 48), Image.LANCZOS), np.float32) / 255
    y48 = luma(small)
    yy, xx = np.mgrid[0:48, 0:48]
    r = np.hypot(yy - 23.5, xx - 23.5) / 24
    ang = np.arctan2(yy - 23.5, xx - 23.5)
    off_axis = np.abs(np.sin(2 * ang)) > 0.85
    ground = float(np.median(y48[(r > 0.46) & (r < 0.62) & off_axis]))
    cross = float(np.median(y48[r < 0.07]))
    prof = [float(np.median(y48[(r >= i / 24) & (r < (i + 1) / 24) & off_axis])) if ((r >= i / 24) & (r < (i + 1) / 24) & off_axis).any() else ground for i in range(24)]
    ring = max(prof[14:22], key=lambda v: abs(v - ground))
    width = sum(abs(v - ground) > 0.5 * abs(ring - ground) for v in prof[14:22])
    mean = float(luma(t).mean())
    report = (f"{name}: ring {width}px @48, ring {contrast(ring, ground):.1f}:1, cross {contrast(cross, ground):.1f}:1, "
              f"mean Y {mean:.3f}")
    fails = []
    if width < 2:
        fails.append("ring under 2px at 48")
    if contrast(ring, ground) < 4.5 or contrast(cross, ground) < 4.5:
        fails.append("contrast under 4.5:1 at 48")
    if dark and not 0.12 <= mean <= 0.22:
        fails.append("mean luminance outside 0.12..0.22")
    print(report + ("" if not fails else "  FAIL: " + "; ".join(fails)))
    return not fails


# ---------------------------------------------------------------- main
ANDROID_DENSITIES = {"mdpi": 108, "hdpi": 162, "xhdpi": 216, "xxhdpi": 324, "xxxhdpi": 432}


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--out", type=Path, help="also write the 1024 tiles here (e.g. the Play Store's 512 is "
                        "icon-512.png scaled from it); nothing else changes")
    args = parser.parse_args()

    night, vault = apse()
    day, _ = nave()
    ok = check("apse", night, dark=True) & check("nave", day, dark=False)
    if not ok:
        sys.exit("the icon no longer reads at launcher size; see above")
    night_tile, day_tile = tile(night), tile(day)

    # Web: the tile for "any" and "maskable" alike (the ring sits inside the maskable safe circle).
    for size in (192, 512):
        save_rgb(night_tile, WEB / f"icons/icon-{size}.png", size)
        save_rgb(night_tile, WEB / f"icons/icon-maskable-{size}.png", size)
    save_rgb(night_tile, WEB / "icons/apple-touch-icon.png", 180)
    (WEB / "favicon.svg").write_text(favicon())
    print("wrote", (WEB / "favicon.svg").relative_to(ROOT))

    # iOS: the nave by day for the light appearance, the apse for dark, and a grayscale for tinting.
    save_rgb(day_tile, IOS / "icon-1024.png", TILE, palette=False)
    save_rgb(night_tile, IOS / "icon-1024-dark.png", TILE, palette=False)
    save_rgb(tile(tinted(night, vault)), IOS / "icon-1024-tinted.png", TILE, palette=False)
    images = [{"filename": "icon-1024.png", "idiom": "universal", "platform": "ios", "size": "1024x1024"}]
    for look in ("dark", "tinted"):
        images.append({"appearances": [{"appearance": "luminosity", "value": look}],
                       "filename": f"icon-1024-{look}.png", "idiom": "universal", "platform": "ios", "size": "1024x1024"})
    (IOS / "Contents.json").write_text(json.dumps({"images": images, "info": {"author": "xcode", "version": 1}}, indent=2) + "\n")
    print("wrote", (IOS / "Contents.json").relative_to(ROOT))

    # Android: the whole 108dp canvas as the foreground, the bare wall behind it for launchers that
    # parallax the layers, and vectors for the themed icon and notifications.
    for dpi, size in ANDROID_DENSITIES.items():
        save_rgb(night, ANDROID / f"mipmap-{dpi}/ic_launcher_foreground.png", size)
        save_rgb(vault, ANDROID / f"mipmap-{dpi}/ic_launcher_background.png", size)
    ring, cross = mark_paths(108, RING_OUT * 72 / TILE)
    mono = vector_drawable(108, 108, [(ring, True), (cross, False)],
                           "The launcher's ring and cross, for themed icons.")
    (ANDROID / "drawable/ic_launcher_monochrome.xml").write_text(mono)
    ring, cross = mark_paths(24, 11.5, ring_w=1.6)
    note = vector_drawable(24, 24, [(ring, True), (cross, False)],
                           "The launcher's ring and cross as a status-bar silhouette: a notification icon is drawn in one colour.")
    (ANDROID / "drawable/ic_notification.xml").write_text(note)
    print("wrote", (ANDROID / "drawable/ic_launcher_monochrome.xml").relative_to(ROOT))
    print("wrote", (ANDROID / "drawable/ic_notification.xml").relative_to(ROOT))

    if args.out:
        args.out.mkdir(parents=True, exist_ok=True)
        to_image(night_tile).save(args.out / "apse-1024.png")
        to_image(day_tile).save(args.out / "nave-1024.png")
        to_image(night_tile).resize((512, 512), Image.LANCZOS).save(args.out / "play-store-512.png")
        print("wrote", args.out)


if __name__ == "__main__":
    main()
