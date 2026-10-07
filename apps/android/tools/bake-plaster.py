#!/usr/bin/env python3
"""Bake the web's plaster wall, and the phone home panel's plaster, into opaque images for the apps.

The web composes apps/office-web/static/plaster.jpg (a grey field from
tools/genplaster) at paint time with background-blend-mode (style.css, the
"--plaster-layers" rule), bottom-up: the knee colour, darkened by the texture;
color-dodged by the dodge colour; screened with the tint; multiplied by the
base; then a veil of --bg sets the wall's strength. Android draws no CSS blend
stack, so the same arithmetic runs here once, in sRGB as CSS blends, and the
app lays the result cover-fitted behind its pages. Rerun after changing the
texture or any --plaster-* token:

    python3 apps/android/tools/bake-plaster.py

The phone home's panel (style.css, ".home-arch-fill" under 700px) lays the same grey texture as
soft light over the panel's ground (--arch-ground, the surface), then a veil of that ground
(--panel-plaster-veil: none by day, 45% on the Apse, whose night plaster read as cloud at full
strength). That is baked here too, as panel_<theme>.jpg, for the apps to clip to the arch.

Requires Pillow and numpy (tools/requirements.txt).
"""
from pathlib import Path

import numpy as np
from PIL import Image

ROOT = Path(__file__).resolve().parents[3]
SOURCE = ROOT / "apps/office-web/static/plaster.jpg"
OUT = ROOT / "apps/android/app/src/main/res/drawable-nodpi"

# The --plaster-* tokens and --bg of each theme (style.css :root and Apse).
THEMES = {
    "nave": dict(bg="#faf3e9", base="#fffdf7", tint="#e9d3c0", knee=(166, 166, 166), dodge=(89, 89, 89), strength=0.70),
    "apse": dict(bg="#121c28", base="#182332", tint="#7d949c", knee=(255, 255, 255), dodge=(0, 0, 0), strength=0.52),
}

# The panel's ground (--surface) and veil (--panel-plaster-veil) of each theme.
PANELS = {
    "nave": dict(ground="#f6eddf", veil=0.0),
    "apse": dict(ground="#172232", veil=0.45),
}


def rgb(v):
    if isinstance(v, str):
        v = tuple(int(v[i : i + 2], 16) for i in (1, 3, 5))
    return np.array(v, dtype=np.float64) / 255.0


def bake(texture, t):
    knee, dodge, tint, base, bg = (rgb(t[k]) for k in ("knee", "dodge", "tint", "base", "bg"))
    c = np.minimum(knee, texture)  # darken
    with np.errstate(divide="ignore", invalid="ignore"):
        c = np.where(dodge >= 1.0, np.where(c > 0, 1.0, 0.0), np.minimum(1.0, c / (1.0 - dodge)))  # color-dodge
    c = 1.0 - (1.0 - c) * (1.0 - tint)  # screen
    c = c * base  # multiply
    alpha = 1.0 - t["strength"]  # color-mix(bg, transparent strength)
    return bg * alpha + c * (1.0 - alpha)


def soft_light(backdrop, source):
    """CSS soft-light: the source (the texture) lightens or darkens the backdrop (the ground)."""
    cb, cs = backdrop, source
    d = np.where(cb <= 0.25, ((16.0 * cb - 12.0) * cb + 4.0) * cb, np.sqrt(cb))
    return np.where(cs <= 0.5, cb - (1.0 - 2.0 * cs) * cb * (1.0 - cb), cb + (2.0 * cs - 1.0) * (d - cb))


def bake_panel(texture, t):
    ground = rgb(t["ground"])
    c = soft_light(np.broadcast_to(ground, texture.shape), texture)
    return ground * t["veil"] + c * (1.0 - t["veil"])


def save(img, path, note):
    img.save(path, quality=90, optimize=True)
    mean = np.asarray(img, dtype=np.float64).reshape(-1, 3).mean(axis=0)
    print(f"{path.relative_to(ROOT)}: mean {'#%02x%02x%02x' % tuple(int(round(m)) for m in mean)} ({note})")


def to_image(field):
    return Image.fromarray(np.clip(np.round(field * 255.0), 0, 255).astype(np.uint8), "RGB")


def main():
    grey = np.asarray(Image.open(SOURCE).convert("L"), dtype=np.float64)[..., None] / 255.0
    texture = np.repeat(grey, 3, axis=2)
    for name, t in THEMES.items():
        save(to_image(bake(texture, t)), OUT / f"plaster_{name}.jpg", f"bg {t['bg']}")
    for name, t in PANELS.items():
        save(to_image(bake_panel(texture, t)), OUT / f"panel_{name}.jpg", f"ground {t['ground']}")


if __name__ == "__main__":
    main()
