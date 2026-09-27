#!/usr/bin/env python3
"""Read-only comparison of the Coverdale psalter with the official 1662 BCP witness.

Use --source-dir for cached official HTML, or --cache-dir to save fetched pages
outside version control. Chant divisions are compared structurally; ordinary
punctuation differences remain findings. This tool never writes the corpus.
"""
import argparse
from collections import Counter
from dataclasses import dataclass
import html
from pathlib import Path
import re
import sys
from urllib.request import Request, urlopen

INDEX = "https://www.churchofengland.org/prayer-and-worship/worship-texts-and-resources/book-common-prayer/psalter"
TYPOGRAPHY = str.maketrans({"’": "'", "‘": "'", "＇": "'", "“": '"', "”": '"', "–": "-", "—": "-", "‑": "-", "\u00a0": " "})


@dataclass
class Verse:
    number: int
    explicit: bool
    text: str


def spaces(text):
    return " ".join(text.split())


def words(text):
    return spaces("".join(c if c.isalnum() else " " for c in text.lower().translate(TYPOGRAPHY)))


def comparable(text):
    return re.sub(r" ([,.;:!?])", r"\1", spaces(text.lower().translate(TYPOGRAPHY)))


def clean_html(text):
    return spaces(html.unescape(re.sub(r"<[^>]*>", "", text)))


def parse_source(body):
    corpus = {}
    current, next_verse, first = 0, 1, True
    for cls, inner in re.findall(r'<p\s+class="([^"]*)"[^>]*>(.*?)</p>', body, re.I | re.S):
        if "vlitemheading" in cls:
            title = clean_html(inner)
            heading = re.match(r"Psalm\s+(\d+)\b", title, re.I)
            current = int(heading[1]) if heading else 0
            start = re.match(r"Psalm\s+\d+\.(\d+)-\d+", title, re.I)
            next_verse, first = int(start[1]) if start else 1, True
            if current:
                corpus.setdefault(current, [])
            continue
        if not current or "vlpsalm" not in cls:
            continue
        for part in re.split(r"<br\s*/?>", inner, flags=re.I):
            text = clean_html(part)
            if not text or text.startswith(("Text from", "is reproduced by permission")):
                continue
            label = re.match(r'^\s*<span\s+class="vlversenumber"[^>]*>\s*(\d+)\s*</span>', part, re.I | re.S)
            label = label or re.match(r"^\s*(\d+)\s+", text)
            if label:
                number = int(label[1])
                text = text.removeprefix(label[1]).strip()
            elif not first and corpus[current]:
                corpus[current][-1].text = spaces(corpus[current][-1].text + " " + text)
                continue
            else:
                number = next_verse
            corpus[current].append(Verse(number, True, historical_source(current, number, text)))
            next_verse, first = number + 1, False
    return corpus


def merge_verses(prior, incoming, source):
    by_number = {v.number: v for v in prior}
    for verse in incoming:
        if verse.number in by_number:
            if words(by_number[verse.number].text) != words(verse.text):
                raise ValueError(f"Psalm verse {verse.number} appears with conflicting text in {source}")
        else:
            by_number[verse.number] = verse
    return [by_number[n] for n in sorted(by_number)]


def fetch(url):
    request = Request(url, headers={"User-Agent": "orthodoxwest-office-psalm-verifier/1.0"})
    with urlopen(request, timeout=60) as response:
        return response.read().decode("utf-8")


def load_source(source_dir, cache_dir):
    if source_dir:
        pages = {p.stem: p.read_text() for p in sorted(Path(source_dir).glob("*.html"))}
    else:
        prefix = INDEX.removeprefix("https://www.churchofengland.org")
        slugs = sorted(set(re.findall(r'href="' + re.escape(prefix) + r'/(psalm[^"/]+)"', fetch(INDEX))))
        if len(slugs) < 40:
            raise ValueError(f"Psalter index yielded only {len(slugs)} source pages")
        pages = {}
        for slug in slugs:
            pages[slug] = fetch(INDEX + "/" + slug)
            if cache_dir:
                Path(cache_dir).mkdir(parents=True, exist_ok=True)
                (Path(cache_dir) / (slug + ".html")).write_text(pages[slug])
    corpus = {}
    for name, body in pages.items():
        for psalm, verses in parse_source(body).items():
            corpus[psalm] = merge_verses(corpus.get(psalm, []), verses, name)
    for psalm in range(1, 151):
        if not corpus.get(psalm):
            raise ValueError(f"official source did not yield Psalm {psalm}")
    return corpus


def parse_local(body):
    lines = body.splitlines()
    header = re.fullmatch(r"Psalm\s+(\d+)(?::(\d+)-(\d+))?([a-z]?)\s*", lines[0].strip() if lines else "", re.I)
    if not header:
        raise ValueError("invalid Psalm header")
    base, start, end = int(header[1]), int(header[2] or 1), int(header[3] or 0)
    verses = []
    for line in map(str.strip, lines[1:]):
        if not line or line.startswith(("Glory be to the Father", "as it was in the beginning")):
            continue
        label = re.match(r"(\d+)\.\s+(.*)", line)
        verses.append(Verse(int(label[1]) if label else 0, bool(label), spaces(label[2] if label else line)))
    return base, start, end, verses


def compare_verse(got, want):
    left, right = got.split("*"), want.split(":", 1)
    if len(left) != 2 or len(right) != 2:
        return "pointing"
    if [words(p) for p in left] != [words(p) for p in right]:
        return "wording"
    if [comparable(p) for p in left] != [comparable(p) for p in right]:
        return "punctuation"
    return None


def compare_file(body, source):
    base, start, end, local = parse_local(body)
    expected = source.get(base, [])
    findings = []
    def add(number, kind, detail):
        findings.append((base, number, kind, detail))
    if not expected:
        add(0, "source", "source Psalm is missing")
        return findings
    if end and end - start + 1 != len(local):
        add(start, "verse-number", f"header declares {start}-{end} but file has {len(local)} verses")
    # Unnumbered split files (e.g. 148b) are aligned by their complete wording.
    local_words = [words(v.text) for v in local]
    begin = next((i for i in range(len(expected) - len(local) + 1) if local and
                  local_words == [words(v.text) for v in expected[i:i+len(local)]]), start - 1)
    if begin < 0 or begin >= len(expected):
        add(start, "missing-verse", "could not locate local verses in official Psalm")
        return findings
    for i, got in enumerate(local):
        if begin + i >= len(expected):
            add(got.number, "extra-verse", "local file has text beyond official Psalm")
            continue
        want = expected[begin+i]
        if got.explicit and got.number != want.number:
            add(want.number, "verse-number", f"local label {got.number}; source verse {want.number}")
        if not got.explicit and i > 0 and base != 119:
            add(want.number, "verse-number", "verse is missing its numeric label")
        if kind := compare_verse(got.text, want.text):
            add(want.number, kind, f"local={got.text!r}; source={want.text!r}")
    return findings


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data-dir", "-data-dir", default="data/texts/psalms")
    parser.add_argument("--source-dir", "-source-dir")
    parser.add_argument("--cache-dir", "-cache-dir")
    args = parser.parse_args()
    source = load_source(args.source_dir, args.cache_dir)
    files = sorted(Path(args.data_dir).glob("*.txt"))
    if not files:
        raise ValueError("no local psalms found")
    findings = [(path.name, *finding) for path in files for finding in compare_file(path.read_text(), source)]
    print(f"Verified {len(files)} local files against {len(source)} source Psalms")
    if not findings:
        print("PASS: wording, punctuation, chant separators, and verse numbering match")
        return 0
    for name, psalm, number, kind, detail in sorted(findings, key=lambda f: (f[0], f[2], f[3])):
        print(f"{name} Psalm {psalm}:{number} [{kind}] {detail}")
    counts = Counter(f[3] for f in findings)
    print(f"FAIL: {len(findings)} mismatch(es)" + "".join(f" {k}={counts[k]}" for k in
          ("wording", "punctuation", "pointing", "verse-number", "missing-verse", "extra-verse", "source") if counts[k]))
    return 1


# The official HTML modernizes a few readings. These mappings were checked
# against the printed official 1662 PDF; retain that historical witness.
def historical_source(psalm, number, text):
    match (psalm, number):
        case (2, 2):
            text = text.removesuffix(":") + "."
        case (2, 5):
            text = text.removesuffix(":") + "."
        case (2, 7):
            text = text.replace("I will preach the law : whereof the Lord hath said unto me,", "I will preach the law, whereof the Lord hath said unto me :", 1)
        case (2, 8):
            text = text.replace("the nations for", "the heathen for")
            text = text.replace("inheritance, :", "inheritance :")
        case (2, 12):
            text = text.replace("yea but a little", "yea, but a little")
            text = text.replace("right way, if", "right way : if", 1)
            text = text.replace("yea, but a little) blessed", "yea, but a little,) blessed", 1)
        case (3, 1):
            text = text.replace("trouble me!", "trouble me")
        case (3, 7):
            text = text.replace("cheek-bone", "cheekbone")
        case (4, 6):
            text = text.replace("show us any good", "shew us any good")
        case (5, 1):
            text = text.removesuffix(".")
        case (5, 2):
            text = text.replace("my King and my God", "my King, and my God")
        case (4, 3):
            text = text.replace("Lord he will", "Lord, he will")
        case (7, 14):
            text = text.removesuffix(".")
        case (7, 4):
            text = text.removesuffix(";") + ";)"
        case (21, 13):
            text = text.replace("so will we sing", "so we will sing")
        case (23, 6):
            text = text.replace("Surely thy loving-kindness", "But thy loving-kindness", 1)
        case (26, 6):
            text = text.removesuffix(";") + "."
        case (26, 9):
            text = text.removesuffix(";") + "."
        case (35, 8):
            text = text.replace("un-awares", "unawares")
        case (40, 6):
            text = text.removesuffix(".") + ":"
        case (52, 4):
            text = text.replace("more than goodness", "more then goodness")
        case (55, 18):
            text = text.replace("noon-day", "noonday")
        case (59, 8):
            text = text.replace("But thou, O Lord", "But thou. O Lord", 1)
        case (68, 1):
            text = text.replace("scattered let them", "scattered : let them", 1)
        case (69, 28):
            text = text.replace("an-other", "another")
        case (78, 48):
            text = text.replace("mulberry trees", "mulberry-trees")
        case (78, 50):
            text = text.replace("displeasure, and trouble", "displeasure and trouble")
        case (86, 9):
            text = text.replace("thou hast made", "thou hadst made")
        case (89, 49):
            text = text.rstrip(".;") + ";"
        case (90, 4):
            text = text.replace("yester-day", "yesterday")
        case (102, 3):
            text = text.replace("fire-brand", "firebrand")
        case (102, 4):
            text = text.replace("withered like grass", "withered liked grass")
        case (105, 5):
            text = text.removesuffix(",") + "."
        case (106, 37):
            text = text.replace("whom they offered", "whom they had offered")
        case (109, 30):
            text = text.replace("from unrighteous judges", "from the unrighteous judges")
        case (119, 65):
            text = text.replace("O Lord thou", "O Lord, thou", 1)
        case (124, 2):
            text = text.replace("when they were", "when thy were")
        case (140, 7):
            text = text.replace("day of battle", "day of the battle")
        case (95, 8):
            text = text.removesuffix(";") + "."
    return text


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError) as error:
        sys.exit(f"verify-psalms: {error}")
