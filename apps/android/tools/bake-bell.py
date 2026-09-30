#!/usr/bin/env python3
"""Bake the reminder bell: one soft stroke of a tubular bell, rung out and faded.

The recording is "Tubular Bells 2/TB_hit_E4_v2_1.wav" from the Versilian Community
Sample Library (https://github.com/sgossner/VCSL, revision c1ea7bcc), which its
authors dedicate to the public domain under CC0 1.0. Of the library's bells it is the
cleanest soft stroke: a noise floor near -80 dB, no second sound in its ring, and a
middle pitch a phone's speaker carries. Folded to mono, faded from 4.5 s to silence
at 7 s, and peaked at -1 dBFS like the phone's own sounds. Android takes it as Ogg
Vorbis; iOS plays notification sounds only from linear PCM (or its telephone codecs),
so the iPhone's copy is 16-bit PCM in a Core Audio file:

    GIT_LFS_SKIP_SMUDGE=1 git clone --depth 1 --filter=blob:none --no-checkout \\
        https://github.com/sgossner/VCSL /tmp/vcsl
    git -C /tmp/vcsl checkout HEAD -- "Idiophones/Struck Idiophones/Tubular Bells 2/TB_hit_E4_v2_1.wav"
    python3 apps/android/tools/bake-bell.py /tmp/vcsl

Requires numpy and soundfile.
"""
import sys
from pathlib import Path

import numpy as np
import soundfile as sf

ROOT = Path(__file__).resolve().parents[3]
SOURCE = "Idiophones/Struck Idiophones/Tubular Bells 2/TB_hit_E4_v2_1.wav"
OUT = ROOT / "apps/android/app/src/main/res/raw/bell.ogg"
IOS_OUT = ROOT / "apps/ios/Office/Resources/bell.caf"

FADE_FROM, END = 4.5, 7.0
PEAK_DB = -1.0


def main() -> None:
    if len(sys.argv) != 2:
        sys.exit("usage: bake-bell.py VCSL_CHECKOUT")
    x, rate = sf.read(Path(sys.argv[1]) / SOURCE, always_2d=True)
    x = x.mean(axis=1)[: int(END * rate)]
    # A millisecond's rise so the stroke starts from silence, and a raised-cosine fade.
    x[: rate // 1000] *= np.linspace(0, 1, rate // 1000)
    t = np.arange(len(x)) / rate
    fade = np.clip((t - FADE_FROM) / (END - FADE_FROM), 0, 1)
    x *= 0.5 * (1 + np.cos(np.pi * fade))
    x *= 10 ** (PEAK_DB / 20) / np.abs(x).max()
    for out, format, subtype in ((OUT, "OGG", "VORBIS"), (IOS_OUT, "CAF", "PCM_16")):
        out.parent.mkdir(parents=True, exist_ok=True)
        sf.write(out, x, rate, format=format, subtype=subtype)
        print(f"{out.relative_to(ROOT)}: {len(x) / rate:.1f} s, {out.stat().st_size // 1024} KB")


if __name__ == "__main__":
    main()
