#!/usr/bin/env python3
"""Derive the grey limewash texture from the parish nave photograph.

Requires tools/requirements.txt. The portrait field serves phones; generate the
wide field with --width 1600 --aspect 1.6 --soften 2 --out PATH. Keep source
photographs outside the repository. Assets are regenerated only for design edits.

Wide hours lay a softened copy of the wall behind the prayer. Derive it from a
generated field, not the photograph, so it keeps that field's clouds exactly:
--soft-of apps/office-web/static/plaster-wide.jpg --quality 90 --out
.../plaster-wide-soft.jpg, and likewise plaster.jpg -> plaster-soft.jpg. The
smooth gradients need the higher quality; the files stay under 3 KB.

The Apse vault's gold leaf varies star by star through a small tileable alpha
mask: --leaf --out apps/office-web/static/leaf.png. Seeded, so it regenerates
byte for byte.
"""
import argparse
from pathlib import Path
import numpy as np
from PIL import Image


def blur(values, radius):
    if radius <= 0:
        return values.copy()
    # Three separable box passes, with reflected edge pixels.
    for _ in range(3):
        for axis in (1, 0):
            padding = [(0, 0), (0, 0)]
            padding[axis] = (radius, radius)
            padded = np.pad(values, padding, mode="symmetric")
            sums = np.cumsum(padded, axis=axis)
            zeros = [(0, 0), (0, 0)]
            zeros[axis] = (1, 0)
            sums = np.pad(sums, zeros)
            values = (np.take(sums, range(2*radius+1, sums.shape[axis]), axis=axis)
                      - np.take(sums, range(sums.shape[axis]-2*radius-1), axis=axis)) / (2*radius+1)
    return values


def texture(photo, width, radius, gain, crop, aspect, limit, soften):
    w, h = photo.size
    cx, cy = int(w * crop), int(h * crop)
    x, y, w, h = cx, cy, w - 2*cx, h - 2*cy
    if aspect > 0:
        if int(h * aspect) < w:
            nw = max(1, int(h * aspect))
            x, w = x + (w - nw)//2, nw
        else:
            nh = max(1, int(w / aspect))
            y, h = y + (h - nh)//2, nh
    pixels = np.asarray(photo.convert("RGB").crop((x, y, x+w, y+h)))
    height = max(1, int(width * h / w + 0.5))
    if width > w or height > h:
        raise ValueError("output must not be larger than the cropped photograph")
    srgb = np.arange(256) / 255
    linear = np.where(srgb <= 0.04045, srgb / 12.92, ((srgb + 0.055) / 1.055)**2.4)
    luminance = linear[pixels] @ np.array([0.2126, 0.7152, 0.0722])
    bins = (np.arange(h)[:, None] * height // h) * width + np.arange(w)[None, :] * width // w
    counts = np.bincount(bins.ravel(), minlength=width*height)
    lum = (np.bincount(bins.ravel(), weights=luminance.ravel(), minlength=width*height) / counts).reshape(height, width)
    lum = blur(lum, soften)
    mean = blur(lum, int(radius * width + 0.5))
    detail = lum / np.maximum(mean, 1e-3)
    mu, sd = detail.mean(), detail.std()
    print(f"detail mean {mu:.4f} sd {sd:.4f}")
    threshold = limit * sd
    deviation = threshold * np.tanh((detail - mu) / threshold) if sd > 1e-12 else np.zeros_like(detail)
    grey = np.floor(255 * np.clip(0.5 + deviation * gain * 0.5, 0, 1) + 0.5).astype(np.uint8)
    return Image.fromarray(grey)


def soft_field(field, radius, scale):
    """A low-frequency copy of a generated field: trowel marks and mottle
    blurred away, the broad clouds kept in place. Its mean is the field's, so
    it composes to the same average colour. Stored small, since nothing is
    left at fine scale for the browser's smooth upscaling to lose."""
    values = blur(np.asarray(field.convert("L"), dtype=float), radius)
    w, h = field.size
    grey = Image.fromarray(np.floor(np.clip(values, 0, 255) + 0.5).astype(np.uint8))
    return grey.resize((max(1, round(w / scale)), max(1, round(h / scale))), Image.BOX)


def leaf_field(size=64, feature=5.5, seed=11, levels=64):
    """Smooth periodic noise as an alpha mask: each star of the vault catches
    its own share of light (mean 85%, clipped to 55-100%). Band-limited in the
    frequency domain, so the tile repeats seamlessly; stored small because the
    browser's smooth upscaling (16x) carries nothing finer than a star."""
    rng = np.random.default_rng(seed)
    white = rng.standard_normal((size, size))
    fx = np.fft.fftfreq(size)[:, None]
    fy = np.fft.fftfreq(size)[None, :]
    field = np.real(np.fft.ifft2(np.fft.fft2(white) * np.exp(-(fx ** 2 + fy ** 2) * feature ** 2 / 2)))
    field = (field - field.mean()) / field.std()
    alpha = np.clip(0.85 + 0.12 * field, 0.55, 1.0)
    alpha = np.round(alpha * (levels - 1)) / (levels - 1)
    pixels = np.zeros((size, size, 2), np.uint8)
    pixels[..., 1] = np.round(alpha * 255).astype(np.uint8)
    return Image.fromarray(pixels, "LA")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--src", type=Path, default=Path("../resources/design/parish/nave-wall-plaster.jpg"))
    parser.add_argument("--out", type=Path, default=Path("apps/office-web/static/plaster.jpg"))
    for name, default, kind in [("width", 800, int), ("radius", 0.06, float), ("gain", 5, float),
                                ("crop", 0.06, float), ("aspect", 0, float), ("limit", 2.5, float),
                                ("soften", 1, int), ("quality", 72, int)]:
        parser.add_argument("--" + name, type=kind, default=default)
    parser.add_argument("--leaf", action="store_true", help="write the vault's gold-leaf variation mask")
    parser.add_argument("--soft-of", type=Path, help="derive the softened field from this generated field")
    parser.add_argument("--soft-radius", type=int, default=20, help="blur radius in field pixels")
    parser.add_argument("--soft-scale", type=int, default=8, help="downscale factor for the stored copy")
    args = parser.parse_args()
    if args.leaf:
        leaf_field().save(args.out, optimize=True)
        print("wrote", args.out)
        return
    if args.soft_of:
        if args.soft_radius < 1 or args.soft_scale < 1:
            parser.error("invalid soft field parameters")
        with Image.open(args.soft_of) as field:
            result = soft_field(field, args.soft_radius, args.soft_scale)
        result.save(args.out, quality=args.quality)
        print("wrote", args.out, result.size)
        return
    if not (args.width > 0 and 0 <= args.crop < 0.5 and args.radius >= 0 and args.aspect >= 0
            and args.limit > 0 and args.soften >= 0 and 1 <= args.quality <= 95):
        parser.error("invalid texture dimensions or processing parameters")
    with Image.open(args.src) as photo:
        result = texture(photo, args.width, args.radius, args.gain, args.crop, args.aspect, args.limit, args.soften)
    result.save(args.out, quality=args.quality)
    print("wrote", args.out)


if __name__ == "__main__":
    main()
