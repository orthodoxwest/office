#!/usr/bin/env python3
"""Derive the grey limewash texture from the parish nave photograph.

Requires tools/requirements.txt. The portrait field serves phones; generate the
wide field with --width 1600 --aspect 1.6 --soften 2 --out PATH. Keep source
photographs outside the repository. Assets are regenerated only for design edits.
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


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--src", type=Path, default=Path("../resources/design/parish/nave-wall-plaster.jpg"))
    parser.add_argument("--out", type=Path, default=Path("apps/office-web/static/plaster.jpg"))
    for name, default, kind in [("width", 800, int), ("radius", 0.06, float), ("gain", 5, float),
                                ("crop", 0.06, float), ("aspect", 0, float), ("limit", 2.5, float),
                                ("soften", 1, int), ("quality", 72, int)]:
        parser.add_argument("--" + name, type=kind, default=default)
    args = parser.parse_args()
    if not (args.width > 0 and 0 <= args.crop < 0.5 and args.radius >= 0 and args.aspect >= 0
            and args.limit > 0 and args.soften >= 0 and 1 <= args.quality <= 95):
        parser.error("invalid texture dimensions or processing parameters")
    with Image.open(args.src) as photo:
        result = texture(photo, args.width, args.radius, args.gain, args.crop, args.aspect, args.limit, args.soften)
    result.save(args.out, quality=args.quality)
    print("wrote", args.out)


if __name__ == "__main__":
    main()
