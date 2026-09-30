#!/usr/bin/env python3
"""Render the PWA cross icons, and the iOS app's. Requires Pillow (tools/requirements.txt)."""
import argparse
from pathlib import Path
from PIL import Image

# The iOS app icon: one 1024-pixel image, which Xcode scales for every place it is shown.
IOS_ICON = Path(__file__).resolve().parents[1] / "apps/ios/Office/Assets.xcassets/AppIcon.appiconset/icon-1024.png"


def draw_icon(size):
    image = Image.new("RGB", (size, size), "#121c28")
    scale, offset = size * 0.6 / 32, size * 0.2
    for x, y, w, h in [(13.5, 4, 5, 24), (5.5, 10, 21, 5)]:
        box = tuple(int(offset + n * scale + 0.5) for n in (x, y, x+w, y+h))
        image.paste("#d8bc74", box)
    return image


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=Path("apps/office-web/static/icons"))
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    for name, size in [("icon-192.png", 192), ("icon-512.png", 512), ("apple-touch-icon.png", 180)]:
        path = args.out / name
        draw_icon(size).save(path)
        print("wrote", path)
    IOS_ICON.parent.mkdir(parents=True, exist_ok=True)
    draw_icon(1024).save(IOS_ICON)
    print("wrote", IOS_ICON)


if __name__ == "__main__":
    main()
