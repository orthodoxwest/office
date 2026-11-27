#!/usr/bin/env python3
"""Home's hand-made materials, as masks: how paint sits on the wall.

Every output is an alpha mask (white with varying alpha), so colour stays a
token on all three fronts, as tools/genornaments.py's masks do.

  pigment.png   tileable 256px: how thinly earth pigment sits on limewash
                (alpha 0.62-1, with pinholes where the plaster's tooth shows).
                Masks the lining and the painted cross.
  tablet.png    the invitation's ground: a field of thin wash, cloudy, its
                edge a little uneven and thinning over about 4 CSS px (drawn at
                3x) before it reaches the ruled frame, so it reads as a wash in
                a frame, not a label with an edge. Stretched to the tablet.
  gild.png      tileable 128px: gold leaf's faint squares and wrinkles, white
                where lighter and black where darker, laid over the gilt
                cross's gradient on a first-class day.

Written to apps/office-web/static/ and, for the apps, pigment and tablet to
apps/android/app/src/main/res/drawable-nodpi/ (which the iOS project shares).
Requires tools/requirements.txt. Seeded, so a rebuild is byte-stable. Run from
the repository root.
"""
from pathlib import Path

import numpy as np
from PIL import Image
from scipy.ndimage import gaussian_filter, gaussian_filter1d

WEB = Path("apps/office-web/static")
NATIVE = Path("apps/android/app/src/main/res/drawable-nodpi")
rng = np.random.default_rng(1105)


def tile_noise(h, w, sx, sy=None):
    sy = sx if sy is None else sy
    n = gaussian_filter(rng.standard_normal((h, w)), (sy, sx), mode="wrap")
    return (n - n.mean()) / (n.std() + 1e-9)


def alpha_image(a):
    a = np.clip(a, 0, 1)
    im = np.zeros(a.shape + (4,), np.uint8)
    im[..., :3] = 255
    im[..., 3] = (a * 255 + 0.5).astype(np.uint8)
    return Image.fromarray(im, "RGBA")


def smooth01(x):
    x = np.clip(x, 0, 1)
    return x * x * (3 - 2 * x)


def pigment(n=256):
    """Blotch, grain and sparse pinholes where the plaster's tooth shows through."""
    a = 0.86 + 0.07 * tile_noise(n, n, 14) + 0.035 * tile_noise(n, n, 3) + 0.02 * tile_noise(n, n, 0.8)
    holes = tile_noise(n, n, 1.6)
    a -= 0.22 * smooth01((holes - 1.9) / 0.8)
    return np.clip(a, 0.62, 1.0)


def wash(w, h, edge_wobble, feather, lo=0.62):
    """A field of thin limewash: cloudy, as the wall's own wash is, with an edge the brush left a
    little uneven. No bristle lines: on a phone they read as scanlines, and limewash keeps none."""
    y = np.arange(h)[:, None]
    x = np.arange(w)[None, :]
    c1 = gaussian_filter(rng.standard_normal((h, w)), (16, 34))
    c1 /= c1.std() + 1e-9
    c2 = gaussian_filter(rng.standard_normal((h, w)), (5, 9))
    c2 /= c2.std() + 1e-9
    c3 = gaussian_filter(rng.standard_normal((h, w)), (1.2, 2.4))
    c3 /= c3.std() + 1e-9
    body = 0.84 + 0.085 * c1 + 0.04 * c2 + 0.015 * c3

    def edge(n):
        e = gaussian_filter1d(rng.standard_normal(n), 30)
        e = e / (np.abs(e).max() + 1e-9)
        f = gaussian_filter1d(rng.standard_normal(n), 5)
        f = f / (np.abs(f).max() + 1e-9)
        return edge_wobble * (1 + 0.75 * e + 0.25 * f) + 0.5

    t, b, l, r = edge(w), edge(w), edge(h), edge(h)
    a = (smooth01((y - t[None, :]) / feather) * smooth01((h - 1 - y - b[None, :]) / feather)
         * smooth01((x - l[:, None]) / feather) * smooth01((w - 1 - x - r[:, None]) / feather))
    return np.clip(body, lo, 1) * a


def gild(n=128):
    """The squares a gilder lays overlap by a hair: faint seams and a wrinkle or two, as a
    soft-light overlay (white where lighter, black where darker, alpha by distance from mid)."""
    a = 0.5 + 0.10 * tile_noise(n, n, 5) + 0.05 * tile_noise(n, n, 1.0)
    g = np.zeros((n, n))
    g[:, 41] = g[:, 107] = 1
    g[23, :] = g[89, :] = 1
    a = np.clip(a + 0.16 * gaussian_filter(g, 0.7, mode="wrap") / 0.4, 0, 1)
    im = np.zeros(a.shape + (4,), np.uint8)
    im[..., :3] = np.where(a[..., None] > 0.5, 255, 0)
    im[..., 3] = (np.abs(a - 0.5) * 2 * 255 * 0.55).astype(np.uint8)
    return Image.fromarray(im, "RGBA")


def main():
    images = {
        "pigment.png": alpha_image(pigment()),
        "tablet.png": alpha_image(wash(978, 171, edge_wobble=5.0, feather=13.0)),
        "gild.png": gild(),
    }
    for name, im in images.items():
        im.save(WEB / name, optimize=True)
        print("wrote", WEB / name, (WEB / name).stat().st_size)
    for name in ("pigment.png", "tablet.png"):
        images[name].save(NATIVE / name, optimize=True)
        print("wrote", NATIVE / name)


if __name__ == "__main__":
    main()
