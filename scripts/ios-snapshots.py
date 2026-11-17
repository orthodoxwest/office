#!/usr/bin/env python3
"""The iOS app's checked-in screenshot baselines (apps/ios/snapshots/).

    ios-snapshots.py compare RAW BASELINES --updates DIR --diffs DIR [--summary FILE]
    ios-snapshots.py fetch BRANCH [--remote origin]
    ios-snapshots.py store RAW OUT

`compare` takes the simulator's full-size screenshots in RAW (apps/ios/screenshots.sh shoot),
brings each down to the device's points (a third of an iPhone's pixels, half an iPad's) and
compares it with its baseline. A pixel differs when any channel moves by more than
CHANNEL_TOLERANCE levels of 255, and a shot differs when more than PIXEL_RATIO of its pixels do
(the web's maxDiffPixelRatio, .web-tools/playwright.config.mjs). It writes:

- to --updates, the set the update-ios-snapshots label would commit: each baseline that still
  matches kept byte for byte, each changed or new shot as taken, none for a shot no longer
  taken, and the environment the shots were taken in;
- to --diffs, a picture of each changed shot: baseline, this run, and its differing pixels in red.

A blank shot (one colour below the status bar: a page not yet drawn) fails the run and is never
offered as a baseline; `fetch` refuses a set with one.

It exits 0 when every shot matches, when there are no baselines yet (with a notice) or when the
shots were not compared (another environment, with a warning); 1 when a shot changed, is new or is
gone; 2 when a shot is blank or none was taken. --output gets the state (matched, no-baselines,
not-compared, differ, blank or none), and --summary a section for each of them. Shots from another Xcode or runtime than the baselines'
(environment.json) are not compared: they differ in ways no page changed.

`fetch` copies the comparable set a CI run published to ci/ios-screenshots/BRANCH (its
snapshots/ folder) into apps/ios/snapshots/, for when the label cannot be used: on the pull
request that first adds the workflow, which pull_request_target reads from master.

`store` takes a store-size run's shots in RAW (a 6.9-inch iPhone and a 13-inch iPad) and writes
the App Store's sets to OUT, numbered in STORE_SHOTS' order: iphone-6.9/, iphone-6.3/ (the 6.9's
shots brought to 1206 x 2622, the size App Store Connect's 6.3-inch slot takes) and ipad-13/. Every
picture is RGB: App Store Connect refuses a screenshot with an alpha channel, calling its
dimensions invalid.
"""

import argparse
import io
import json
import pathlib
import re
import shutil
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
BASELINES = ROOT / "apps/ios/snapshots"
ENVIRONMENT = "environment.json"
# What a shot's page measured of its own layout (apps/ios/Office/ScreenshotReady.swift), beside it.
GEOMETRY = ".geometry.txt"
ENVIRONMENT_KEYS = ("xcode", "runtime", "phone", "ipad")
# What each of its values may hold. They are echoed into workflow commands (::warning::), so no
# newlines or markup; update-ios-snapshots.yml's commit job checks the same with jq.
ENVIRONMENT_VALUE = re.compile(r"[A-Za-z0-9 ._()-]{1,100}")

# A pixel differs when a channel moves by more than this many levels: antialiasing and the
# box filter's rounding move edges by a few, a changed glyph or colour by far more.
CHANNEL_TOLERANCE = 24
# A shot differs when more than this share of its pixels do: about 170 of an iPhone's 393 x 852
# points, less than one changed word.
PIXEL_RATIO = 0.0005

# A shot is blank, and never compared or kept, when below the status bar fewer than this share
# of its pixels stand more than BLANK_LEVELS from its commonest grey: the bare wall of a page not
# yet drawn (a page of text has several percent of ink, home its whole frontispiece).
BLANK_RATIO = 0.005
BLANK_LEVELS = 8
# Points at the top left out of that test: the status bar and the Dynamic Island.
STATUS_POINTS = 70

# Shots taken for review but not compared, with why. None yet: every page is shot at a fixed
# date and hour (-today, -date, -clock) and at rest.
EXCLUDED: dict[str, str] = {}


def scale_of(name):
    """The device's pixels to a point: 2 on an iPad, 3 on an iPhone."""
    return 2 if name.startswith("ipad-") else 3


def reduce(path):
    """The shot at the device's points, as RGB. Image.reduce is a box average, the same everywhere."""
    from PIL import Image

    with Image.open(path) as image:
        return image.convert("RGB").reduce(scale_of(path.stem))


def is_blank(image):
    """Whether the shot (at points) is a page not yet drawn: one colour below the status bar."""
    grey = image.convert("L").crop((0, min(STATUS_POINTS, image.height - 1), image.width, image.height))
    histogram = grey.histogram()
    common = max(range(256), key=histogram.__getitem__)
    near = sum(histogram[max(0, common - BLANK_LEVELS):common + BLANK_LEVELS + 1])
    total = grey.width * grey.height
    return (total - near) / total < BLANK_RATIO


def differing(a, b):
    """The mask of pixels where any channel of a and b differs by more than CHANNEL_TOLERANCE."""
    from PIL import ImageChops

    channels = ImageChops.difference(a, b).split()
    mask = channels[0]
    for c in channels[1:]:
        mask = ImageChops.lighter(mask, c)
    return mask.point(lambda v: 255 if v > CHANNEL_TOLERANCE else 0)


def diff_picture(baseline, actual, mask):
    """Baseline, this run, and this run faded with its differing pixels in red, side by side."""
    from PIL import Image

    w, h = max(baseline.width, actual.width), max(baseline.height, actual.height)
    sheet = Image.new("RGB", (w * 3 + 16, h), (255, 255, 255))
    sheet.paste(baseline, (0, 0))
    sheet.paste(actual, (w + 8, 0))
    faded = Image.blend(actual, Image.new("RGB", actual.size, (255, 255, 255)), 0.7)
    faded.paste(Image.new("RGB", actual.size, (220, 0, 0)), (0, 0), mask)
    sheet.paste(faded, (2 * w + 16, 0))
    return sheet


def read_environment(folder):
    path = folder / ENVIRONMENT
    if not path.is_file():
        return None
    env = json.loads(path.read_text())
    if (not isinstance(env, dict) or set(env) != set(ENVIRONMENT_KEYS)
            or not all(isinstance(v, str) and ENVIRONMENT_VALUE.fullmatch(v) for v in env.values())):
        raise SystemExit(f"{path}: expected exactly {', '.join(ENVIRONMENT_KEYS)}, each matching {ENVIRONMENT_VALUE.pattern}")
    return env


def write_summary(summary, heading, text):
    if summary:
        with open(summary, "a") as out:
            out.write(f"## iOS screenshots: {heading}\n\n{text}\n\n")


def compare(raw, baselines, updates, diffs, summary=None):
    """
    Compares the shots in raw with the baselines. Returns the exit status, the state
    (matched, differ, no-baselines, not-compared; blank or none, which exit 2) and the report's lines.
    """
    from PIL import Image

    updates.mkdir(parents=True, exist_ok=True)
    diffs.mkdir(parents=True, exist_ok=True)
    for old in [*updates.glob("*.png"), *updates.glob(f"*{GEOMETRY}"), *diffs.glob("*.png")]:
        old.unlink()
    shots = {p.stem: p for p in sorted(raw.glob("*.png")) if p.stem not in EXCLUDED}
    kept = {p.stem: p for p in sorted(baselines.glob("*.png"))} if baselines.is_dir() else {}
    taken_in = read_environment(raw)
    recorded = read_environment(baselines) if baselines.is_dir() else None
    if taken_in:
        (updates / ENVIRONMENT).write_text(json.dumps(taken_in, indent=2) + "\n")
    # What each page measured of itself goes with its shot, so the next baselines carry it too.
    for name in shots:
        if (raw / f"{name}{GEOMETRY}").is_file():
            shutil.copyfile(raw / f"{name}{GEOMETRY}", updates / f"{name}{GEOMETRY}")

    lines = []

    def take(name, image):
        image.save(updates / f"{name}.png", optimize=True)

    # A blank shot is the app's or the run's fault, never a page to keep or compare.
    blank = []
    for name, path in list(shots.items()):
        if is_blank(reduce(path)):
            blank.append(name)
            del shots[name]
    if blank:
        for name in blank:
            lines.append(f"::error title=Blank iOS screenshot::{name}.png is blank: its page was not drawn when it was shot")
        write_summary(summary, "blank", "These shots are blank (their page was not drawn when shot), so nothing was "
                      "compared or offered for checking in: " + ", ".join(f"`{n}`" for n in blank) + ".")
        return 2, "blank", lines

    if not shots:
        lines.append("::error::No screenshots to compare.")
        write_summary(summary, "none taken", "No screenshots were taken.")
        return 2, "none", lines
    if not kept:
        for name, path in shots.items():
            take(name, reduce(path))
        lines.append(
            f"::notice::No iOS baselines yet: {len(shots)} shots taken, nothing compared. "
            "Add the update-ios-snapshots label to a pull request to check them in."
        )
        write_summary(summary, "no baselines", f"There are no baselines in `apps/ios/snapshots/` yet, so the {len(shots)} shots "
                      "were not compared. The `ios-snapshot-candidates` artifact (and `snapshots/` on this branch's "
                      "`ci/ios-screenshots/` branch) holds the set to check in.")
        return 0, "no-baselines", lines
    if recorded and taken_in and recorded != taken_in:
        for name, path in shots.items():
            take(name, reduce(path))
        lines.append(
            "::warning::These shots were taken with "
            f"{taken_in['xcode']}, {taken_in['runtime']} ({taken_in['phone']}, {taken_in['ipad']}), "
            f"the baselines with {recorded['xcode']}, {recorded['runtime']} ({recorded['phone']}, {recorded['ipad']}); "
            "nothing compared. Add the update-ios-snapshots label to a pull request to take them again."
        )
        write_summary(summary, "not compared",
                      f"These shots were taken with {taken_in['xcode']}, {taken_in['runtime']} ({taken_in['phone']}, "
                      f"{taken_in['ipad']}), the baselines with {recorded['xcode']}, {recorded['runtime']} "
                      f"({recorded['phone']}, {recorded['ipad']}), which this runner lacks; nothing was compared. "
                      "The `update-ios-snapshots` label takes the baselines again on this image.")
        return 0, "not-compared", lines

    changed, new, gone, same = [], [], [], []
    for name, path in shots.items():
        actual = reduce(path)
        if name not in kept:
            new.append(name)
            take(name, actual)
            continue
        with Image.open(kept[name]) as image:
            baseline = image.convert("RGB")
        if baseline.size != actual.size:
            changed.append((name, f"size {baseline.width}x{baseline.height} -> {actual.width}x{actual.height}"))
            take(name, actual)
            diff_picture(baseline, actual, Image.new("L", actual.size, 255)).save(diffs / f"{name}.png", optimize=True)
            continue
        mask = differing(baseline, actual)
        count = mask.histogram()[255]
        ratio = count / (actual.width * actual.height)
        if ratio > PIXEL_RATIO:
            changed.append((name, f"{count} pixels ({ratio:.2%})"))
            take(name, actual)
            diff_picture(baseline, actual, mask).save(diffs / f"{name}.png", optimize=True)
        else:
            same.append(name)
            shutil.copyfile(kept[name], updates / f"{name}.png")
    gone = sorted(set(kept) - set(shots) - set(EXCLUDED))

    lines.append(f"{len(same)} of {len(shots)} iOS screenshots match their baselines.")
    for name, why in changed:
        lines.append(f"::error title=iOS screenshot changed::{name}.png differs from its baseline: {why}")
    for name in new:
        lines.append(f"::error title=New iOS screenshot::{name}.png has no baseline")
    for name in gone:
        lines.append(f"::error title=iOS screenshot gone::{name}.png has a baseline but was not taken")
    if summary:
        with open(summary, "a") as out:
            out.write(f"## iOS screenshots: {'changed' if changed or new or gone else 'matched'}\n\n")
            out.write(f"{len(same)} of {len(shots)} match their baselines (a pixel differs by more than "
                      f"{CHANNEL_TOLERANCE}/255 in a channel; a shot by more than {PIXEL_RATIO:.2%} of its pixels).\n\n")
            if changed or new or gone:
                out.write("| Shot | |\n| --- | --- |\n")
                for name, why in changed:
                    out.write(f"| `{name}` | changed: {why} |\n")
                for name in new:
                    out.write(f"| `{name}` | new, no baseline |\n")
                for name in gone:
                    out.write(f"| `{name}` | baseline, not taken |\n")
                out.write("\nThe `ios-snapshot-diffs` artifact has each changed shot beside its baseline. "
                          "If the change is intended, add the `update-ios-snapshots` label.\n")
                for name, _ in changed:
                    measured = geometry_report(name, baselines, raw)
                    if measured:
                        out.write(f"\n<details><summary>{name}: what the page measured</summary>\n\n{measured}\n</details>\n")
    if changed or new or gone:
        return 1, "differ", lines
    return 0, "matched", lines


def geometry_report(name, baselines, raw):
    """The baseline's and this run's measures of a shot's page (ScreenshotGeometry), where kept."""
    parts = []
    for label, folder in (("baseline", baselines), ("this run", raw)):
        path = folder / f"{name}{GEOMETRY}"
        if path.is_file():
            parts.append(f"{label}:\n\n```\n{path.read_text().rstrip()}\n```\n")
    return "\n".join(parts)


def fetch(branch, remote="origin", into=BASELINES):
    """Copies snapshots/ from ci/ios-screenshots/<branch> into the baselines."""
    ref = f"refs/heads/ci/ios-screenshots/{branch}"
    subprocess.run(["git", "fetch", "--quiet", remote, ref], cwd=ROOT, check=True)
    listing = subprocess.run(
        ["git", "ls-tree", "--name-only", "FETCH_HEAD", "snapshots/"],
        cwd=ROOT, check=True, capture_output=True, text=True,
    ).stdout.split()
    names = [n for n in listing if n.endswith((".png", GEOMETRY)) or n == f"snapshots/{ENVIRONMENT}"]
    if not any(n.endswith(".png") for n in names):
        raise SystemExit(f"{ref} has no snapshots/; has its iOS run finished?")
    blobs = {n: subprocess.run(["git", "show", f"FETCH_HEAD:{n}"], cwd=ROOT, check=True, capture_output=True).stdout
             for n in names}
    from PIL import Image

    blank = []
    for n, blob in blobs.items():
        if n.endswith(".png"):
            with Image.open(io.BytesIO(blob)) as image:
                if is_blank(image.convert("RGB")):
                    blank.append(n)
    if blank:
        raise SystemExit(f"{ref} has blank shots, which must not become baselines: {', '.join(blank)}")
    into.mkdir(parents=True, exist_ok=True)
    for old in [*into.glob("*.png"), *into.glob(f"*{GEOMETRY}")]:
        old.unlink()
    for name, blob in blobs.items():
        (into / pathlib.PurePosixPath(name).name).write_bytes(blob)
    print(f"{len(names)} files from {ref} into {into.relative_to(ROOT)}")


# The App Store's screenshots, in the order the listing shows them.
STORE_SHOTS = {
    "iphone": ["home-nave", "feast-nave", "compline-nave", "compline-apse", "ordo-nave", "year-nave", "reminders-nave"],
    "ipad": ["home-nave", "chant-nave", "feast-nave", "vespers-nave", "ordo-nave", "year-nave", "home-apse"],
}
STORE_SIZES = {"iphone-6.9": (1320, 2868), "iphone-6.3": (1206, 2622), "ipad-13": (2064, 2752)}


def store(raw, out):
    """The App Store's sets from a store-size run; see the module's docstring."""
    from PIL import Image

    for folder, (w, h) in STORE_SIZES.items():
        device = folder.split("-")[0]
        prefix = "ipad-" if device == "ipad" else ""
        (out / folder).mkdir(parents=True, exist_ok=True)
        for n, name in enumerate(STORE_SHOTS[device], 1):
            with Image.open(raw / f"{prefix}{name}.png") as image:
                shot = image.convert("RGB")
            if shot.size != (w, h):
                if folder == "iphone-6.3":
                    shot = shot.resize((w, h), Image.LANCZOS)
                else:
                    raise SystemExit(f"{prefix}{name}.png is {shot.size[0]} x {shot.size[1]}, not the {w} x {h} {folder} takes")
            shot.save(out / folder / f"{n}-{name}.png", optimize=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    sub = parser.add_subparsers(dest="command", required=True)
    c = sub.add_parser("compare")
    c.add_argument("raw", type=pathlib.Path)
    c.add_argument("baselines", type=pathlib.Path)
    c.add_argument("--updates", type=pathlib.Path, required=True)
    c.add_argument("--diffs", type=pathlib.Path, required=True)
    c.add_argument("--summary")
    c.add_argument("--output", help="a file to append state=<state> to, such as $GITHUB_OUTPUT")
    f = sub.add_parser("fetch")
    f.add_argument("branch")
    f.add_argument("--remote", default="origin")
    s = sub.add_parser("store")
    s.add_argument("raw", type=pathlib.Path)
    s.add_argument("out", type=pathlib.Path)
    args = parser.parse_args(argv)
    if args.command == "store":
        store(args.raw, args.out)
        return 0
    if args.command == "fetch":
        fetch(args.branch, args.remote)
        return 0
    status, state, lines = compare(args.raw, args.baselines, args.updates, args.diffs, args.summary)
    print("\n".join(lines))
    if args.output:
        with open(args.output, "a") as out:
            out.write(f"state={state}\n")
    return status


if __name__ == "__main__":
    sys.exit(main())
