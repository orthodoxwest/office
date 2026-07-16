#!/usr/bin/env python3
"""Machine-diff the app's output against a printed archdiocesan ordo.

The parish ordo PDFs (see ../resources/) are the authority for what the app
should produce. This script parses `pdftotext -layout` output of an ordo and
compares it, per day, against the app's `ordo` and `rubrics` output.

Usage:
    pdftotext -layout ../resources/2026-ordo.pdf /tmp/2026-ordo.txt
    ./office ordo 2026 > /tmp/our-ordo.txt
    ./office rubrics 2026 > /tmp/our-rubrics.tsv

    scripts/ordo-compare.py calendar /tmp/2026-ordo.txt /tmp/our-ordo.txt
    scripts/ordo-compare.py rubrics  /tmp/2026-ordo.txt /tmp/our-rubrics.tsv
    scripts/ordo-compare.py commemorations /tmp/2026-ordo.txt /tmp/our-rubrics.tsv
    scripts/ordo-compare.py antiphons /tmp/2026-ordo.txt /tmp/our-rubrics.tsv
    scripts/ordo-compare.py colors   /tmp/2026-ordo.txt /tmp/our-ordo.txt
    scripts/ordo-compare.py vespers  /tmp/2026-ordo.txt /tmp/our-ordo.txt
    scripts/ordo-compare.py moveable ../resources 2017 2018 2019 2021 2022 2023 2024 2025 2026

Caveats learned the hard way (2026 sweep):
  - The PDFs are Word exports; day headers usually parse cleanly but titles
    sometimes wrap or land a line above/below the day number.
  - The Epiphany proclamation (Jan 6) announces all moveable feasts; the
    Holy Saturday block mentions Easter. Anchor matching must be scoped to
    day-title blocks and month floors.
  - The 2026 PDF misprints the Vigil of the Nativity as "23 Thu" (Dec 24).
  - `additional-sunday-rubrics.pdf` year table row for May 5 disagrees with
    the applied 2024 ordo; trust the prose rule and the applied ordos.
"""

import datetime
import re
import subprocess
import sys

MONTHS = {m: i + 1 for i, m in enumerate(
    ["JANUARY", "FEBRUARY", "MARCH", "APRIL", "MAY", "JUNE", "JULY",
     "AUGUST", "SEPTEMBER", "OCTOBER", "NOVEMBER", "DECEMBER"])}
MONTH_NAMES = {m: i + 1 for i, m in enumerate(
    ["January", "February", "March", "April", "May", "June", "July",
     "August", "September", "October", "November", "December"])}
DAY_RE = re.compile(r"^\s{0,10}(\d{1,2})\s+(Sun|Mon|Tue|Wed|Thu|Fri|Sat)(?:\s+(.*?))?\s*$")
SECT_RE = re.compile(r"^\s*(Matins|Lauds|Mass|Hours|Vespers|Prime|Compline|N\.B\.)\b")
RANK_RE = re.compile(r"\s{2,}((?:D1|D2|Gd|Sd|D|M|S|F|V|Pr|C)\.?\d?)\s*$")


def pdf_days(path):
    """Segment a pdftotext ordo into {(month, day): {section: text}}.

    Also returns per-day title lines under the '' key.
    """
    cur_m = 0
    last_day = 0
    key = None
    sect = None
    in_title = False
    days = {}
    for ln in open(path, errors="replace"):
        s = ln.strip()
        if s in MONTHS:
            m = MONTHS[s]
            # month running heads repeat; only advance forward
            if m == cur_m + 1 or (cur_m == 0 and m == 1):
                cur_m, last_day = m, 0
            continue
        md = DAY_RE.match(ln)
        if md and cur_m:
            d = int(md.group(1))
            # tolerate one skipped day (PDF typos, e.g. 2026 Dec 24)
            if d in (last_day + 1, last_day + 2):
                last_day = d
                key = (cur_m, d)
                days[key] = {"": (md.group(3) or "").strip()}
                sect = None
                in_title = True
                continue
        if key is None:
            continue
        ms = SECT_RE.match(ln)
        if ms:
            sect = ms.group(1)
            in_title = False
        if in_title:
            days[key][""] += "\n" + s
        elif sect:
            days[key][sect] = days[key].get(sect, "") + " " + s
    return days


def our_ordo_days(path):
    """Parse `office ordo` output into {(month, day): {...}}."""
    cur_m = 0
    key = None
    days = {}
    for ln in open(path):
        s = ln.strip()
        if s in MONTHS:
            cur_m = MONTHS[s]
            continue
        md = re.match(r"^ *(\d{1,2})  (\w{3})\s+(.*?)(?:\s*\[(\w+)\])?\s+([wrgvb])\s*$", ln)
        if md and cur_m:
            key = (cur_m, int(md.group(1)))
            days[key] = {"title": md.group(3).strip(), "rank": md.group(4) or "",
                         "color": md.group(5), "coms": [], "vespers": None,
                         "vcolor": md.group(5)}
            continue
        # feria lines carry no [rank]/color suffix
        md = re.match(r"^ *(\d{1,2})  (\w{3})\s+(\S.*?)\s*$", ln)
        if md and cur_m and not s.startswith(("Com:", "Vespers:")):
            key = (cur_m, int(md.group(1)))
            days[key] = {"title": md.group(3).strip(), "rank": "", "color": None,
                         "coms": [], "vespers": None, "vcolor": None}
            continue
        if key and s.startswith("Com."):
            days[key]["coms"].append(s[4:].strip())
        # Vespers stanza line: "Vespers <color> · <owner> · Mag. ... · <suff>".
        # Owner (I fol./II prec.) is absent when Vespers is not designated.
        mv = re.match(r"^\s*Vespers\s+([wrgvbp])\b", ln)
        if mv and key:
            days[key]["vcolor"] = mv.group(1)
            mo = re.search(r"(I fol\.|II prec\.)", ln)
            if mo:
                days[key]["vespers"] = {"I fol.": "fol", "II prec.": "prec"}[mo.group(1)]
    return days


STOP = {"of", "the", "our", "in", "and", "a", "an", "st", "ss", "holy", "day",
        "after", "within", "lord", "jesus", "christ", "bvm", "blessed",
        "virgin", "mary"}


def tokens(t):
    t = t.lower().replace("æ", "ae")
    return {w for w in re.findall(r"[a-z0-9]+", t) if w not in STOP}


def similar(a, b):
    ta, tb = tokens(a), tokens(b)
    if not ta or not tb:
        return 0.0
    return len(ta & tb) / min(len(ta), len(tb))


def is_ferial(title):
    return title.lower().startswith(
        ("feria", "monday", "tuesday", "wednesday", "thursday", "friday",
         "saturday", "ember")) or title == ""


def cmd_calendar(pdf_path, ours_path):
    pdf = pdf_days(pdf_path)
    ours = our_ordo_days(ours_path)
    n = 0
    for k in sorted(set(pdf) & set(ours)):
        if not pdf[k].get(""):  # day whose title section didn't parse from the PDF
            continue
        p_title = pdf[k][""].splitlines()[0]
        p_title = RANK_RE.sub("", re.sub(r"^[L§†‡\s]+", "", p_title)).strip()
        o_title = ours[k]["title"]
        if similar(p_title, o_title) < 0.5 and not (is_ferial(p_title) and is_ferial(o_title)):
            n += 1
            print(f"{k[0]:02d}-{k[1]:02d}  ours: {o_title}")
            print(f"        pdf: {p_title}")
    print(f"-- {n} headline mismatches")


def read_rubrics(path):
    ours = {}
    for ln in open(path):
        p = ln.rstrip("\n").split("\t")
        if p[0] == "date":
            continue
        dt = datetime.date.fromisoformat(p[0])
        lauds_comms = split_our_commemorations(p[4])
        vespers_comms = split_our_commemorations(p[8])
        ours[(dt.month, dt.day)] = {
            "cel": p[1], "l_suff": p[3] == "true", "l_comm": bool(lauds_comms),
            "l_comms": lauds_comms,
            "h_preces": p[5] == "true", "v_suff": p[7] == "true",
            "v_comm": bool(vespers_comms), "v_comms": vespers_comms,
            "ben": p[9] if len(p) > 9 else "", "mag": p[10] if len(p) > 10 else ""}
    return ours


def split_our_commemorations(value):
    """Split the semicolon-delimited commemoration names in `office rubrics`."""
    return [name.strip() for name in value.split(";") if name.strip()]


def pdf_commemorations(section):
    """Extract commemoration names from one printed-or-do office section.

    The PDF writes the first item as ``Comm. Name (...)`` and commonly omits
    the repeated ``Comm.`` after an ampersand. Parenthesized antiphon, collect,
    and page references delimit the names more reliably than punctuation does.
    Return None when the section states neither Comm. nor No Comm., so an
    unparseable/missing section is not counted as agreement.
    """
    if section is None:
        return None
    starts = list(re.finditer(r"(?<!No )\bComm\.\s*", section))
    if not starts:
        return [] if re.search(r"\bNo Comm\.", section) else None

    block = section[starts[0].end():]
    block = re.split(r"\s+/\s*(?:No )?(?:Comm\.|Suff\.)", block, maxsplit=1)[0]
    # The ordo's parenthetical references are occasionally unbalanced after
    # text extraction. A closing page-reference parenthesis followed by "&"
    # is nevertheless a stable item boundary.
    parts = re.split(r"\)\s*&\s*(?=(?:Comm\.\s*)?[A-Z])", block)
    names = []
    for part in parts:
        name = part.split("(", 1)[0]
        # Subsequent items often begin "Comm."; Ash Wednesday can even be
        # printed as the redundant "Comm. Comm. Walburga".
        name = re.sub(r"^(?:Comm\.\s*)+", "", name)
        name = re.sub(r"^[\s&;/]+", "", name).strip()
        name = re.sub(r"\s+only$", "", name, flags=re.I).strip()
        # Holy Cross commemorations are reported through a separate rubrics
        # flag and are not present in the TSV's feast-name column.
        if name and name.upper() != "HC":
            names.append(name)
    return names


COMM_STOP = STOP | {
    "saint", "saints", "bishop", "confessor",
    "doctor", "virgin", "priest", "pope", "abbot", "king", "emperor",
    "apostle", "apostles", "evangelist", "companions", "rome",
}

BVM_RE = r"b\s*\.?\s*(?:v\s*\.?\s*m|m\s*\.?\s*v)\b\.?"


def commemoration_tokens(name):
    """Normalize a printed/app commemoration name for fuzzy set matching."""
    text = name.lower().replace("æ", "ae").replace("&c.", " companions ")
    text = re.sub(r"([a-z])-\s+([a-z])", r"\1\2", text)
    text = text.replace("&", " and ").replace("pope", "bishop")
    # Historical ordos abbreviate the Saturday Office as BVM, BMV, B.V.M.,
    # or B.M.V. A bare acronym denotes the Saturday Office; within a named
    # Marian feast it is only a title abbreviation and must not make every
    # Marian feast look like the Saturday Office.
    generic_bvm = bool(re.fullmatch(r"(?:the\s+)?" + BVM_RE, text.strip()))
    text = re.sub(r"\b" + BVM_RE, " blessed virgin mary ", text)
    text = re.sub(r"\bdorothea\b", "dorothy", text)
    text = re.sub(r"\bcommemoration of\b", " ", text)
    text = re.sub(r"\b(?:ss?|st)\.?(?=\s)", " saint ", text)
    text = re.sub(r"\bsun\.?(?=\s|$)", " sunday ", text)
    text = re.sub(r"\bfer\.?(?=\s|$)", " feria ", text)
    text = re.sub(r"\boct\.?(?=\s|$)", " octave ", text)
    words = re.findall(r"[a-z0-9]+", text)
    normalized = set()
    for word in words:
        if word in COMM_STOP:
            continue
        # Printed ordos sometimes pluralize a surname or collective title.
        if len(word) > 4 and word.endswith("s") and not word.endswith("ss"):
            word = word[:-1]
        normalized.add(word)
    # A generic "Comm. Fer." must match the descriptive weekday feria emitted
    # by the engine; preserve the weekday too so two named ferias stay distinct.
    if re.match(r"^(monday|tuesday|wednesday|thursday|friday|saturday)\b", text):
        normalized.add("feria")
    if generic_bvm:
        normalized.update(("saturday", "office"))
    return normalized


def commemoration_similarity(a, b):
    ta, tb = commemoration_tokens(a), commemoration_tokens(b)
    if not ta or not tb:
        return 0.0
    return len(ta & tb) / min(len(ta), len(tb))


def match_commemorations(pdf_names, our_names):
    """Return (missing_from_app, extra_in_app) after greedy fuzzy matching."""
    candidates = []
    for pi, pdf_name in enumerate(pdf_names):
        for oi, our_name in enumerate(our_names):
            score = commemoration_similarity(pdf_name, our_name)
            if score >= 0.6:
                candidates.append((score, pi, oi))
    used_pdf, used_ours = set(), set()
    for _, pi, oi in sorted(candidates, reverse=True):
        if pi not in used_pdf and oi not in used_ours:
            used_pdf.add(pi)
            used_ours.add(oi)
    return ([name for i, name in enumerate(pdf_names) if i not in used_pdf],
            [name for i, name in enumerate(our_names) if i not in used_ours])


def flag(text, yes, no):
    if text is None:
        return None
    if re.search(no, text):
        return False
    if re.search(yes, text):
        return True
    return None


def cmd_rubrics(pdf_path, tsv_path):
    pdf = pdf_days(pdf_path)
    ours = read_rubrics(tsv_path)
    fields = [
        ("h_preces", "Hours preces", "Hours", r"Preces", r"No [Pp]re-?\s*ces"),
        ("l_suff", "Lauds suffrage", "Lauds", r"Suff\.", r"No Suff"),
        ("v_suff", "Vespers suffrage", "Vespers", r"Suff\.", r"No Suff"),
        ("l_comm", "Lauds comm", "Lauds", r"Comm\.", r"No Comm\."),
        ("v_comm", "Vespers comm", "Vespers", r"Comm\.", r"No Comm\.(?!\s*HC)"),
    ]
    for f, label, sect, yes, no in fields:
        bad = []
        for k in sorted(ours):
            pv = flag(pdf.get(k, {}).get(sect), yes, no)
            if pv is not None and pv != ours[k][f]:
                bad.append((k, ours[k][f], pv, ours[k]["cel"]))
        print(f"== {label}: {len(bad)} mismatches ==")
        for (m, d), ov, pv, cel in bad[:10]:
            print(f"   {m:02d}-{d:02d}  ours={ov} pdf={pv}  ({cel[:48]})")
        if len(bad) > 10:
            print(f"   ... and {len(bad) - 10} more")


def cmd_commemorations(pdf_path, tsv_path):
    """Compare normalized commemoration names, not merely presence/absence."""
    pdf = pdf_days(pdf_path)
    ours = read_rubrics(tsv_path)
    for field, sect, label in (
            ("l_comms", "Lauds", "Lauds"),
            ("v_comms", "Vespers", "Vespers")):
        compared = 0
        bad = []
        missing_total = extra_total = 0
        for key in sorted(ours):
            pdf_names = pdf_commemorations(pdf.get(key, {}).get(sect))
            if pdf_names is None:
                continue
            compared += 1
            missing, extra = match_commemorations(pdf_names, ours[key][field])
            if missing or extra:
                bad.append((key, missing, extra))
                missing_total += len(missing)
                extra_total += len(extra)
        print(f"== {label} commemorations: {len(bad)}/{compared} dates differ; "
              f"{missing_total} missing; {extra_total} extra ==")
        for (month, day), missing, extra in bad[:20]:
            details = []
            if missing:
                details.append("missing: " + "; ".join(missing))
            if extra:
                details.append("extra: " + "; ".join(extra))
            print(f"   {month:02d}-{day:02d}  " + " | ".join(details))
        if len(bad) > 20:
            print(f"   ... and {len(bad) - 20} more dates")


def _incipit_words(text):
    # æ→ae and z→s fold the books' orthographic variants (Elisabeth/Elizabeth,
    # Sion/Zion) that the ordo quotes interchangeably.
    norm = text.lower().replace("æ", "ae").replace("z", "s")
    return re.sub(r"[^a-z0-9 ]", " ", norm).split()


def incipit_matches(incipit, full):
    wi = _incipit_words(incipit)
    wf = _incipit_words(full)
    if not wi or not wf:
        return None
    # The printed ordo quotes the same antiphon with and without a leading
    # "O" interjection (e.g. "King of glory" vs "O King of glory"); align by
    # dropping the lone leading O from whichever side has it.
    if wi[0] == "o" and wf[0] != "o":
        wi = wi[1:] or wi
    elif wf[0] == "o" and wi[0] != "o":
        wf = wf[1:] or wf
    if not wi or not wf:
        return None
    n = min(len(wi), len(wf), 4)
    hits = sum(1 for a, b in zip(wi[:n], wf[:n])
               if a == b or a.startswith(b) or b.startswith(a))
    # short incipits must match fully; longer ones tolerate one hyphenation slip
    return hits >= (n if n < 3 else n - 1)


def cmd_antiphons(pdf_path, tsv_path):
    pdf = pdf_days(pdf_path)
    ours = read_rubrics(tsv_path)
    for field, sect, pat, label in (
            ("ben", "Lauds", r"Ben\.?\s*Ant\.?\s*[“\"]([^”\"]+)", "Benedictus"),
            ("mag", "Vespers", r"Mag\.?\s*Ant\.?\s*[“\"]([^”\"]+)", "Magnificat")):
        n = tot = 0
        samples = []
        for k in sorted(ours):
            m = re.search(pat, pdf.get(k, {}).get(sect, ""))
            if not m or not ours[k][field]:
                continue
            tot += 1
            if not incipit_matches(m.group(1), ours[k][field]):
                n += 1
                if len(samples) < 10:
                    samples.append(f"   {k[0]:02d}-{k[1]:02d} pdf=\"{m.group(1)[:36]}\""
                                   f" ours=\"{ours[k][field][:36]}\"")
        print(f"== {label} antiphon: {n}/{tot} mismatches ==")
        print("\n".join(samples))


def cmd_colors(pdf_path, ours_path):
    pdf = pdf_days(pdf_path)
    ours = our_ordo_days(ours_path)
    n = tot = 0
    for k in sorted(set(pdf) & set(ours)):
        for sect, field in (("Lauds", "color"), ("Vespers", "vcolor")):
            # section text begins with the section keyword itself
            m = re.match(sect + r"\s+([WRGVB])\b", pdf[k].get(sect, "").strip())
            ov = ours[k][field]
            if m and ov:
                tot += 1
                if m.group(1).lower() != ov:
                    n += 1
                    print(f"   {k[0]:02d}-{k[1]:02d} {sect}: pdf={m.group(1).lower()} ours={ov}")
    print(f"-- {n}/{tot} color mismatches")


def cmd_vespers(pdf_path, ours_path):
    pdf = pdf_days(pdf_path)
    ours = our_ordo_days(ours_path)
    agree = dis = pdf_only = ours_only = 0
    for k in sorted(set(pdf) | set(ours)):
        pv = None
        v = pdf.get(k, {}).get("Vespers", "")
        if re.search(r"I of fol", v):
            pv = "fol"
        elif re.search(r"II of prec", v):
            pv = "prec"
        ov = ours.get(k, {}).get("vespers")
        if pv and ov:
            if pv == ov:
                agree += 1
            else:
                dis += 1
                print(f"   {k[0]:02d}-{k[1]:02d}: pdf={pv} ours={ov}")
        elif pv:
            pdf_only += 1
        elif ov:
            ours_only += 1
    print(f"-- agree {agree}, disagree {dis}, pdf-only {pdf_only}, ours-only {ours_only}")


MOVEABLE_TABLE = {
    "ash-wednesday": r"Ash Wednesday",
    "pentecost": r"(?:Whitsunday|Pentecost)",
    "corpus-christi": r"Corpus Christi",
    "advent1": r"Advent Sunday",
}


def cmd_moveable(resources_dir, years):
    import tempfile
    for y in years:
        txt = subprocess.run(
            ["pdftotext", "-layout", f"{resources_dir}/{y}-ordo.pdf", "-"],
            capture_output=True, text=True).stdout[:8000]
        found = {}
        for k, pat in MOVEABLE_TABLE.items():
            m = re.search(pat + r"[ .\t]*?(\d{1,2}) ("
                          + "|".join(MONTH_NAMES) + ")", txt)
            if m:
                found[k] = (MONTH_NAMES[m.group(2)], int(m.group(1)))
                continue
            m = re.search(pat + r"[ .\t]*?(" + "|".join(MONTH_NAMES) + r") (\d{1,2})", txt)
            if m:
                found[k] = (MONTH_NAMES[m.group(1)], int(m.group(2)))
        out = subprocess.run(["./office", "ordo", str(y)], capture_output=True, text=True).stdout
        ours = {}
        pats = {"ash-wednesday": r"ash wednesday", "pentecost": r"pentecost$|whitsun",
                "corpus-christi": r"corpus christi", "advent1": r"i sunday of advent"}
        cur_m = 0
        for ln in out.splitlines():
            s = ln.strip()
            if s in MONTHS:
                cur_m = MONTHS[s]
                continue
            md = re.match(r"^ *(\d{1,2})  \w{3}\s+(.*)$", ln)
            if md and cur_m and not re.search(r"vigil|octave|within|eve", md.group(2), re.I):
                for k, rx in pats.items():
                    if k not in ours and re.search(rx, md.group(2), re.I):
                        ours[k] = (cur_m, int(md.group(1)))
        diffs = [f"{k}: pdf={found[k]} ours={ours.get(k)}"
                 for k in found if ours.get(k) != found[k]]
        print(f"{y} {'DIFF ' + '; '.join(diffs) if diffs else 'OK'}")


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)
    cmd = sys.argv[1]
    if cmd == "calendar":
        cmd_calendar(sys.argv[2], sys.argv[3])
    elif cmd == "rubrics":
        cmd_rubrics(sys.argv[2], sys.argv[3])
    elif cmd in ("commemorations", "comms"):
        cmd_commemorations(sys.argv[2], sys.argv[3])
    elif cmd == "antiphons":
        cmd_antiphons(sys.argv[2], sys.argv[3])
    elif cmd == "colors":
        cmd_colors(sys.argv[2], sys.argv[3])
    elif cmd == "vespers":
        cmd_vespers(sys.argv[2], sys.argv[3])
    elif cmd == "moveable":
        cmd_moveable(sys.argv[2], sys.argv[3:])
    else:
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
