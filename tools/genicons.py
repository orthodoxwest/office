#!/usr/bin/env python3
"""Render the app icon for the web, iOS and Android.

The icon is a mosaic clipeus after the apse of Sant'Apollinare in Classe
(c. 549): a gemmed gold cross in a starry blue medallion with a jewelled red
rim, set in a ground of gold tesserae. Each tessera is tilted a little, as the
Ravenna mosaicists set them, so the gold catches light unevenly; the stars
stand on rings as on the Galla Placidia vault.

One render feeds every platform. The ground and the medallion are separate
layers, so the medallion can sit inside each platform's mask: whole on iOS,
within the maskable safe circle on the web, within the 66dp safe zone of an
Android adaptive icon. Small and one-colour uses (favicon, Android's themed
icon and notification) get flat vectors of the same ring and cross.

Requires tools/requirements.txt. Seeded, so it regenerates byte for byte.
Run from the repository root.
"""
import argparse
import json
import math
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw
from scipy import ndimage as ndi

ROOT = Path(__file__).resolve().parents[1]
WEB = ROOT / "apps/office-web/static"
IOS = ROOT / "apps/ios/Office/Assets.xcassets/AppIcon.appiconset"
ANDROID = ROOT / "apps/android/app/src/main/res"

N = 1024                 # design canvas
C = N / 2
R_IN, R_OUT = 382, 452   # starry field, outer edge of the jewelled rim
SS = 4                   # mask supersampling
Y, X = np.mgrid[0:N, 0:N].astype(np.float32)
LIGHT = np.array([-0.45, -0.75, 0.85]) / np.linalg.norm([-0.45, -0.75, 0.85])

SKY = ["#1b2a66", "#22337a", "#16224f", "#2b3f8a", "#1e2d5c", "#262a62", "#1a3270"]
RIM = ["#8c2a22", "#7a2320", "#9a3426", "#6e1e1c", "#86301f"]
GEMS = ["#1f6a46", "#2a3f8f", "#8f1d2a"]


# ---------------------------------------------------------------- geometry
def qbez(p0, p1, p2, n):
    t = np.linspace(0, 1, n)[:, None]
    p0, p1, p2 = map(np.array, (p0, p1, p2))
    return list(map(tuple, (1 - t) ** 2 * p0 + 2 * (1 - t) * t * p1 + t ** 2 * p2))


def cross_poly(cx, cy, arms=(215, 205, 290, 205), a=36, b=70, straight=0.55, bulge=5, n=40):
    """Latin cross with gently flared terminals; arms are (top, right, bottom, left)."""
    pts = []
    for k, reach in enumerate(arms):
        stem = -a - (reach - a) * straight
        arm = [(-a, -a), (-a, stem)]
        arm += qbez((-a, stem), (-a, -reach + (reach - a) * 0.08), (-b, -reach), n)[1:]
        arm += qbez((-b, -reach), (0, -reach - bulge * 2), (b, -reach), n)[1:]
        arm += qbez((b, -reach), (a, -reach + (reach - a) * 0.08), (a, stem), n)[1:]
        arm += [(a, -a)]
        c, s = math.cos(k * math.pi / 2), math.sin(k * math.pi / 2)
        pts += [(cx + x * c - y * s, cy + x * s + y * c) for x, y in arm[:-1]]
    return pts


CROSS_Y = C - 22


def star_poly(x, y, r, long=1.0, short=0.62, valley=0.32):
    """The parish star: four long rays and four short diagonals."""
    pts = []
    for i in range(16):
        a = i * math.pi / 8 - math.pi / 2
        rr = long if i % 4 == 0 else short if i % 4 == 2 else valley
        pts.append((x + r * rr * math.cos(a), y + r * rr * math.sin(a)))
    return pts


def circle_poly(x, y, r, n=48):
    return [(x + r * math.cos(t), y + r * math.sin(t)) for t in np.linspace(0, 2 * math.pi, n, endpoint=False)]


def mask(polys):
    """Anti-aliased union of polygons, in canvas pixels."""
    im = Image.new("L", (N * SS, N * SS), 0)
    draw = ImageDraw.Draw(im)
    for p in polys:
        draw.polygon([(x * SS, y * SS) for x, y in p], fill=255)
    a = np.asarray(im, np.float32) / 255
    return a.reshape(N, SS, N, SS).mean(axis=(1, 3))


# ---------------------------------------------------------------- noise and light
def fbm(octaves, base, seed):
    rng = np.random.default_rng(seed)
    out = np.zeros((N, N), np.float32)
    amp, total = 1.0, 0.0
    for o in range(octaves):
        cells = base * 2 ** o
        z = ndi.zoom(rng.standard_normal((cells + 3, cells + 3)).astype(np.float32), N / cells, order=3)[:N, :N]
        out += amp * z
        total += amp
        amp *= 0.55
    out /= total
    return (out - out.mean()) / (out.std() + 1e-6)


WARP_U, WARP_V = fbm(3, 3, 91), fbm(3, 3, 92)


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


def rgb(hexs):
    return np.array([int(hexs[i:i + 2], 16) for i in (1, 3, 5)], np.float32) / 255


def fill(hexs):
    return np.ones((N, N, 3), np.float32) * rgb(hexs)


def over(base, top, alpha):
    a = alpha[..., None]
    return base * (1 - a) + top * a


def vignette():
    d = np.sqrt((X / N - 0.45) ** 2 + (Y / N - 0.4) ** 2)
    return np.clip(1 - 0.25 * d ** 2, 0, 1)[..., None]


# ---------------------------------------------------------------- tesserae
def grid_tiles(t, salt):
    """Rows of tesserae, offset by half a tile, drifting as hand-set rows do."""
    u = (X + WARP_U * t * 0.10) / t
    v = (Y + WARP_V * t * 0.09) / t
    row = np.floor(v)
    u = u + (row % 2) * 0.5 + (hash2(row, row * 0 + 3, salt) - 0.5) * 0.6
    col = np.floor(u)
    return u - col, v - row, col, row


def polar_tiles(t, salt):
    """Rings of tesserae round the centre, as a medallion is laid."""
    dx, dy = X - C + WARP_U * t * 0.07, Y - C + WARP_V * t * 0.07
    r = np.sqrt(dx ** 2 + dy ** 2)
    th = np.arctan2(dy, dx) + math.pi
    ring = np.floor(r / t)
    segments = np.maximum(np.round(2 * math.pi * (ring + 0.5)), 1)
    a = th / (2 * math.pi) * segments + hash2(ring, ring * 0 + 7, salt)
    seg = np.floor(a)
    return a - seg, r / t - ring, seg, ring


def tessellate(fu, fv, ia, ib, salt, tilt=0.22, grout=0.11):
    """Grout coverage, a tilted pillowed normal per tessera, and a random value per tessera."""
    edge = np.minimum(np.minimum(fu, 1 - fu), np.minimum(fv, 1 - fv))
    g = grout * (0.8 + 0.5 * hash2(ia, ib, salt + 11))
    is_grout = np.clip((g - edge) / 0.04 + 0.5, 0, 1)
    tx = (hash2(ia, ib, salt + 21) - 0.5) * 2 * tilt
    ty = (hash2(ia, ib, salt + 31) - 0.5) * 2 * tilt
    ex = np.where(fu < 0.5, -1, 1) * (1 - np.clip(np.minimum(fu, 1 - fu) / 0.16, 0, 1))
    ey = np.where(fv < 0.5, -1, 1) * (1 - np.clip(np.minimum(fv, 1 - fv) / 0.16, 0, 1))
    n = np.dstack([tx + 0.35 * ex, ty + 0.35 * ey, np.ones_like(tx)])
    n /= np.linalg.norm(n, axis=2, keepdims=True)
    return is_grout, n, hash2(ia, ib, salt + 41)


def gold(n, rnd):
    """Gold-leaf glass: a soft burnished reflection, a few glints, a few dull cubes."""
    v = np.array([0, 0, 1.0])
    r = 2 * (n * v).sum(2, keepdims=True) * n - v
    e = 0.62 + 0.30 * np.clip(-r[..., 1] * 0.8 - r[..., 0] * 0.5, -1, 1)
    e += 0.22 * np.exp(-((r[..., 0] + 0.35) ** 2 + (r[..., 1] + 0.45) ** 2) / 0.12)
    e += (rnd - 0.5) * 0.12 - 0.04
    stops = [0.0, 0.35, 0.6, 0.85, 1.05, 1.4]
    ramp = np.array([[70, 44, 14], [140, 100, 38], [196, 156, 74], [226, 192, 112], [242, 218, 150], [252, 240, 200]], np.float32) / 255
    col = np.dstack([np.interp(e, stops, ramp[:, c]) for c in range(3)]) * 0.92
    col *= (1 - 0.35 * np.clip((0.1 - rnd) / 0.1, 0, 1))[..., None]
    return col + (np.clip((rnd - 0.88) / 0.12, 0, 1) * 0.35)[..., None] * np.array([1.0, 0.92, 0.7])


def glass(palette, n, rnd):
    pal = np.stack([rgb(c) for c in palette])
    col = pal[np.floor(rnd * len(palette) * 0.999).astype(int)]
    col = col * (0.9 + 0.2 * hash2(rnd * 7e3, rnd * 1e3, 9))[..., None]
    return col * (0.8 + 0.3 * lambert(n))[..., None] + (spec(n, 60) * 0.35)[..., None]


def cabochon(m, colour, soft, strength, spec_power, spec_amount, lit=(0.55, 0.55)):
    n = normals(ndi.gaussian_filter(m, soft) * m, strength)
    return colour * (lit[0] + lit[1] * lambert(n))[..., None] + (spec(n, spec_power) * spec_amount)[..., None]


# ---------------------------------------------------------------- the icon
def ground(tile):
    g, n, rnd = tessellate(*grid_tiles(tile, 1), salt=1, tilt=0.14)
    return over(gold(n, rnd), fill("#3b3226"), g * 0.85) * vignette()


def medallion():
    """The clipeus as an RGB layer and its coverage."""
    r = np.sqrt((X - C) ** 2 + (Y - C) ** 2)
    disc = np.clip(R_IN - r + 0.5, 0, 1)
    rim = np.clip(R_OUT - r + 0.5, 0, 1) - disc
    cross = mask([cross_poly(C, CROSS_Y)])
    keep = ndi.binary_dilation(cross > 0.5, iterations=34)
    stars = []
    for radius, count, offset, size in ((185, 8, 0.5, 34), (300, 16, 0.25, 36)):
        for k in range(count):
            a = (k + offset) * 2 * math.pi / count
            x, y = C + radius * math.cos(a), C + radius * math.sin(a)
            if not keep[int(y), int(x)]:
                stars.append(star_poly(x, y, size))
    stars = mask(stars)

    sg, sn, sr = tessellate(*polar_tiles(12.5, 2), salt=2)
    cg, cn, cr = tessellate(*grid_tiles(13, 3), salt=3, tilt=0.16)
    rg, rn, rr = tessellate(*polar_tiles(11, 4), salt=4)

    sky = glass(SKY, sn, sr) * (0.75 + 0.35 * np.clip(1 - r / R_IN, 0, 1) ** 0.6)[..., None]
    sky = over(sky, gold(sn, sr), stars)
    sky = over(sky, fill("#141a2c"), sg * 0.8)
    col = over(fill("#3b3226"), sky, disc)
    col = over(col, over(glass(RIM, rn, rr), fill("#2c1a14"), rg * 0.8), rim)
    fillets = np.clip(np.clip(1 - np.abs(r - R_IN) / 6, 0, 1) + np.clip(1 - np.abs(r - R_OUT) / 6, 0, 1), 0, 1)
    col = over(col, gold(rn, rr), fillets * (1 - rg * 0.6))

    # Jewels and pearls alternate round the rim.
    jewels, pearls, colours = [], [], []
    for k in range(24):
        a = k * 2 * math.pi / 24 + math.pi / 24
        x, y = C + (R_IN + R_OUT) / 2 * math.cos(a), C + (R_IN + R_OUT) / 2 * math.sin(a)
        if k % 2 == 0:
            jewels.append(circle_poly(x, y, 15))
            colours.append(GEMS[(k // 2) % 3])
        else:
            pearls.append(circle_poly(x, y, 9, 40))
    jm = mask(jewels)
    tint = np.zeros((N, N, 3), np.float32)
    for poly, c in zip(jewels, colours):
        tint = over(tint, fill(c), mask([poly]))
    col = over(col, cabochon(jm, tint, 6, 60, 120, 0.55), jm)
    pm = mask(pearls)
    col = over(col, cabochon(pm, rgb("#e9dfcb"), 4, 60, 60, 0.25, (0.6, 0.45)), pm)

    # The cross, outlined in a row of dark tesserae as the mosaicists drew it.
    outline = np.clip(ndi.binary_dilation(cross > 0.5, iterations=9).astype(np.float32) - cross, 0, 1)
    dark = fill("#5a2a1c") * (0.82 + 0.3 * cr)[..., None] * (0.8 + 0.3 * lambert(cn))[..., None] + (spec(cn, 60) * 0.35)[..., None]
    col = over(col, over(dark, fill("#20160f"), cg * 0.8), ndi.gaussian_filter(outline, 0.7) * disc)
    col = over(col, over(gold(cn, cr), fill("#3b3226"), cg * 0.8), cross)
    gem = mask([circle_poly(C, CROSS_Y, 24, 64)])
    col = over(col, cabochon(gem, rgb("#a3202a"), 7, 70, 90, 0.9, (0.55, 0.5)), gem)

    coverage = np.clip(R_OUT + 5 - r + 0.5, 0, 1)
    return col * vignette(), coverage


# ---------------------------------------------------------------- composition
def resize(arr, size):
    """Lanczos resize of a float image, channel by channel."""
    if arr.ndim == 2:
        return np.asarray(Image.fromarray(arr.astype(np.float32), "F").resize((size, size), Image.LANCZOS))
    return np.dstack([resize(arr[..., c], size) for c in range(arr.shape[2])])


def placed(layer, alpha, scale):
    """The medallion scaled about the centre of the canvas (premultiplied, so its edge stays clean)."""
    size = int(round(N * scale / 2)) * 2
    pre = resize(np.dstack([layer * alpha[..., None], alpha]), size)
    out = np.zeros((N, N, 4), np.float32)
    o = (N - size) // 2
    out[o:o + size, o:o + size] = pre
    return out[..., :3], np.clip(out[..., 3], 0, 1)


def flatten(ground_rgb, pre_rgb, alpha):
    return ground_rgb * (1 - alpha[..., None]) + pre_rgb


def save(img, path, palette):
    """Mosaic is noise to PNG's filters; a 256-colour palette costs nothing visible and
    a fifth to a half of the bytes. iOS keeps full colour for its App Store artwork."""
    path.parent.mkdir(parents=True, exist_ok=True)
    if palette:
        method = Image.Quantize.FASTOCTREE if img.mode == "RGBA" else Image.Quantize.MEDIANCUT
        img = img.quantize(256, method=method, dither=Image.Dither.FLOYDSTEINBERG)
    img.save(path, optimize=True)
    print("wrote", path.relative_to(ROOT))


def save_rgb(arr, path, size, palette=True):
    img = Image.fromarray((np.clip(arr, 0, 1) * 255 + 0.5).astype(np.uint8), "RGB")
    save(img.resize((size, size), Image.LANCZOS), path, palette)


def save_rgba(pre_rgb, alpha, path, size, palette=True):
    pre = resize(np.dstack([pre_rgb, alpha]), size)
    a = np.clip(pre[..., 3], 0, 1)
    rgb_ = np.where(a[..., None] > 1e-4, pre[..., :3] / np.maximum(a[..., None], 1e-4), 0)
    img = np.dstack([np.clip(rgb_, 0, 1), a])
    save(Image.fromarray((img * 255 + 0.5).astype(np.uint8), "RGBA"), path, palette)


def tinted(pre_rgb, alpha):
    """Grayscale for iOS's tinted appearance: gold reads light, the sky dark."""
    lum = (pre_rgb * np.array([0.3, 0.59, 0.11])).sum(2)
    lum = np.clip((lum - 0.05 * alpha) / 0.5, 0, 1) ** 0.85
    return np.repeat(lum[..., None], 3, axis=2)


# ---------------------------------------------------------------- vectors
def path_data(points, scale, cx, cy, ox, oy):
    pts = [(ox + (x - cx) * scale, oy + (y - cy) * scale) for x, y in points]
    return "M" + " ".join(f"{x:.2f},{y:.2f}" for x, y in pts) + "Z"


def circle_path(cx, cy, r):
    return f"M{cx - r:.2f},{cy:.2f}a{r:.2f},{r:.2f} 0 1,0 {2 * r:.2f},0a{r:.2f},{r:.2f} 0 1,0 {-2 * r:.2f},0Z"


def bold_cross():
    """The cross with heavier limbs, for sizes where the mosaic's would vanish."""
    return cross_poly(C, CROSS_Y, a=52, b=92, n=10)


def favicon():
    s = 15.2 / R_OUT
    cross = path_data(bold_cross(), s, C, C, 16, 16)
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="15.2" fill="#d8bc74"/>
  <circle cx="16" cy="16" r="14.4" fill="#8c2a22"/>
  <circle cx="16" cy="16" r="{R_IN * s + 0.35:.2f}" fill="#d8bc74"/>
  <circle cx="16" cy="16" r="{R_IN * s:.2f}" fill="#1f2f6a"/>
  <path d="{cross}" fill="#e6c77e"/>
</svg>
'''


def vector_drawable(size_dp, viewport, paths, comment):
    body = "\n".join(
        f'    <path\n        android:fillColor="{colour}"\n'
        + ('        android:fillType="evenOdd"\n' if even_odd else "")
        + f'        android:pathData="{d}" />'
        for d, colour, even_odd in paths
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


def medallion_stars():
    """The medallion's stars, kept clear of the bold cross."""
    keep = ndi.binary_dilation(mask([bold_cross()]) > 0.5, iterations=40)
    stars = []
    for radius, count, offset, size in ((185, 8, 0.5, 40), (300, 16, 0.25, 40)):
        for k in range(count):
            a = (k + offset) * 2 * math.pi / count
            x, y = C + radius * math.cos(a), C + radius * math.sin(a)
            if not keep[int(y), int(x)]:
                stars.append(star_poly(x, y, size, 1.0, 0.5, 0.3))
    return stars


def themed_medallion(viewport, outer, thickness):
    """Ring, stars and cross: the medallion in one colour. The stars keep it from reading as a plus in a circle."""
    s = outer / R_OUT
    mid = viewport / 2
    ring = circle_path(mid, mid, outer) + circle_path(mid, mid, outer - thickness)
    stars = "".join(path_data(p, s, C, C, mid, mid) for p in medallion_stars())
    return [(ring, "#FFFFFFFF", True), (stars, "#FFFFFFFF", False),
            (path_data(bold_cross(), s, C, C, mid, mid), "#FFFFFFFF", False)]


def notification_cross(viewport):
    """The cross alone, filling the status-bar square: a ring round it would read as "add"."""
    pts = bold_cross()
    ys = [y for _, y in pts]
    s = (viewport - 3) / (max(ys) - min(ys))
    return [(path_data(pts, s, C, (max(ys) + min(ys)) / 2, viewport / 2, viewport / 2), "#FFFFFFFF", False)]


# ---------------------------------------------------------------- outputs
ANDROID_DENSITIES = {"mdpi": 108, "hdpi": 162, "xhdpi": 216, "xxhdpi": 324, "xxxhdpi": 432}
# An adaptive icon's layers are 108dp; any mask keeps a 66dp circle. The rim sits just inside it.
ANDROID_SCALE = (32.6 / 54) * (C / (R_OUT + 5))
# A maskable web icon keeps the circle of 40% radius.
MASKABLE_SCALE = 0.4 * N / (R_OUT + 5)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.parse_args()
    layer, alpha = medallion()
    wide = ground(16)

    whole_rgb, whole_a = placed(layer, alpha, 1.0)
    full = flatten(wide, whole_rgb, whole_a)

    # Web: a round icon on transparency for "any", the full tile for maskable and iOS home screens.
    for size in (192, 512):
        save_rgba(whole_rgb, whole_a, WEB / f"icons/icon-{size}.png", size)
    m_rgb, m_a = placed(layer, alpha, MASKABLE_SCALE)
    maskable = flatten(ground(16 * MASKABLE_SCALE), m_rgb, m_a)
    for size in (192, 512):
        save_rgb(maskable, WEB / f"icons/icon-maskable-{size}.png", size)
    save_rgb(full, WEB / "icons/apple-touch-icon.png", 180)
    (WEB / "favicon.svg").write_text(favicon())
    print("wrote", (WEB / "favicon.svg").relative_to(ROOT))

    # iOS: the full tile, the medallion alone for the dark appearance, and a grayscale for tinting.
    save_rgb(full, IOS / "icon-1024.png", 1024, palette=False)
    save_rgba(whole_rgb, whole_a, IOS / "icon-1024-dark.png", 1024, palette=False)
    save_rgb(tinted(whole_rgb, whole_a), IOS / "icon-1024-tinted.png", 1024, palette=False)
    images = [{"filename": "icon-1024.png", "idiom": "universal", "platform": "ios", "size": "1024x1024"}]
    for look in ("dark", "tinted"):
        images.append({"appearances": [{"appearance": "luminosity", "value": look}],
                       "filename": f"icon-1024-{look}.png", "idiom": "universal", "platform": "ios", "size": "1024x1024"})
    (IOS / "Contents.json").write_text(json.dumps({"images": images, "info": {"author": "xcode", "version": 1}}, indent=2) + "\n")
    print("wrote", (IOS / "Contents.json").relative_to(ROOT))

    # Android: adaptive layers at every density, and vectors for the themed icon and notifications.
    a_rgb, a_a = placed(layer, alpha, ANDROID_SCALE)
    a_ground = ground(16 * ANDROID_SCALE)
    for dpi, size in ANDROID_DENSITIES.items():
        save_rgba(a_rgb, a_a, ANDROID / f"mipmap-{dpi}/ic_launcher_foreground.png", size)
        save_rgb(a_ground, ANDROID / f"mipmap-{dpi}/ic_launcher_background.png", size)
    outer = (R_OUT + 5) * ANDROID_SCALE * 108 / N
    mono = vector_drawable(108, 108, themed_medallion(108, outer, outer * (R_OUT - R_IN) / R_OUT),
                           "The launcher medallion's ring, stars and cross, for themed icons.")
    (ANDROID / "drawable/ic_launcher_monochrome.xml").write_text(mono)
    note = vector_drawable(24, 24, notification_cross(24),
                           "The medallion's cross as a status-bar silhouette: a notification icon is drawn in one colour.")
    (ANDROID / "drawable/ic_notification.xml").write_text(note)
    print("wrote", (ANDROID / "drawable/ic_launcher_monochrome.xml").relative_to(ROOT))
    print("wrote", (ANDROID / "drawable/ic_notification.xml").relative_to(ROOT))


if __name__ == "__main__":
    main()
