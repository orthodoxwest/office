# Editing liturgical data

[Back to the README](README.md) · [Review workflow](REVIEWING.md)

Liturgical data lives in `data/`. The engine reads these at startup — no recompile needed when editing them.

```
data/
  feasts/          feast definitions (sanctoral, temporal, AWRV-specific)
  penitential.txt  fasting and abstinence discipline rules
  audit-ok.txt     feasts that intentionally use ordinary/common texts
  texts/
    psalms/        Coverdale Psalter, one file per psalm (Hebrew numbering)
    canticles/     Benedictus, Magnificat, Nunc Dimittis, etc.
    ordinary/      fixed prayers, hymns, versicles, Marian antiphons (per hour)
      session.txt  session opening/closing prayers (Aperi Domine, Sacrosanctae)
    proper/        feast-specific antiphons and collects (one file per feast)
    commons/       texts by category (apostle, martyr, confessor, etc.)
    seasonal/      season-specific overrides
    shared/        texts reused across several files (Marian texts, formulas)
    chant/         GABC chant scores (psalms/, canticles/, hymns/)
  office/          hour structure definitions
  review/          sign-offs, provenance attestations, assurance baseline
```

## File format

Feast metadata uses an INI-like format: `[section]` headers and `Key = value`
lines. Text corpus files use `[section]` headers followed by multiline text
bodies. Both support `#` comments and blank lines.

**Feast definition** (`data/feasts/sanctoral.txt`):

```ini
[st-andrew]
Name     = Saint Andrew, Apostle
Rank     = double-2nd-class
Color    = red
Category = apostle
Month    = 11
Day      = 30
```

Valid `Rank` values: `double-1st-class`, `double-2nd-class`, `greater-double`, `double`, `semi-double`, `privileged-feria`, `simple`, `commemoration`

Valid `Color` values: `white`, `red`, `green`, `violet`, `rose`, `black`

Valid `Category` values: `lord`, `blessed-virgin`, `angel`, `apostle`, `evangelist`, `martyr`, `martyrs`, `bishop-martyr`, `virgin-martyr`, `confessor-bishop`, `confessor-doctor`, `confessor`, `virgin`, `holy-woman`, `dedication`, `sunday`, `feria`

Optional keys: `HasOctave = true`, `HasVigil = true` (generate a preceding vigil), `IsVigil = true` with `VigilOf = feast-id` (this observance is an explicit vigil of the canonical feast), `IsApostolicCompanion = true` (the Peter/Paul companion retained at II Vespers), `ProperName = Andrew` (saint's given name, substituted for `N.` in common texts), `ProperID` (use another feast's proper texts), `DateRule` (for moveable feasts instead of `Month`/`Day`), `OnlyWith` (only kept on days where the named feast wins the day), `SkipRomanLeapShift = true` (keep a fixed late-February feast on its civil date in leap years), `Source` and `Notes` (documentation).

**Feast proper** (`data/texts/proper/st-andrew.txt`):

Each `[section]` key corresponds to a liturgical text slot. A feast file need only include the slots it actually has — the engine falls back to the common (by `Category`) or ordinary for anything omitted.

In prose sections such as collects, chapters, and prayers, a single newline is a soft source wrap and the web renderer lets the paragraph reflow. A blank line starts a new paragraph. Hymn and psalm renderers preserve their verse structure automatically.

```ini
[psalm-antiphon]
The Lord saw Peter and Andrew, * and He called them.

[benedictus-antiphon]
There followed the Lord two brethren, Peter and Andrew.

[magnificat-antiphon]
O Lord, Thou hast caused them that persecuted the just to be swallowed up in hell,
* but to the just Thou hast thyself shown the way on the tree of the cross.

[collect]
O Lord, we humbly beseech thy Majesty: that even as Thou didst give thy blessed
Apostle Andrew to thy Church to be a teacher and ruler on earth, so, now that he is
with thee, he may continually make intercession for us.
```

Common sections for feast propers (the [scaffold catalog](internal/scaffold/keys.go)
lists the supported core and optional keys):

| Section | Used in |
|---------|---------|
| `[psalm-antiphon]` | Single psalm antiphon at Lauds |
| `[psalm-antiphon-1]` … `[psalm-antiphon-5]` | One antiphon per psalm (add `-vespers` for Vespers variants, e.g. `[psalm-antiphon-4-vespers]`) |
| `[benedictus-antiphon]` | Benedictus antiphon at Lauds |
| `[magnificat-antiphon]` | Magnificat antiphon at (2nd) Vespers |
| `[magnificat-antiphon-first]` | Magnificat antiphon at 1st Vespers |
| `[collect]` | Collect at all hours |
| `[chapter-lauds]`, `[chapter-vespers]`, `[chapter-terce]`, … | Chapter, per hour |
| `[versicle-lauds]`, `[versicle-vespers]`, … | Versicle/response, per hour |
| `[short-responsory-lauds]`, `[short-responsory-vespers]`, … | Short responsory, per hour |
| `[hymn-lauds]`, `[hymn-vespers]` | Full hymn text (overrides common/ordinary) |
| `[commemoration-antiphon]` | Antiphon for commemoration |
| `[commemoration-versicle]` | Versicle/response for commemoration |
| `[commemoration-collect]` | Collect for commemoration (defaults to `[collect]`) |

A section whose body is a single `@use path/to/other/key` line reuses another
corpus entry verbatim (see [REVIEWING.md](REVIEWING.md)). `#` comment lines inside
sections — including `# SOURCE:` provenance annotations — are stripped by the
loader and never render.

A section whose body is a bare `@omit` says the element is **not said** in this
office. The resolver follows the slot's fallback chain, so leaving a section out
inherits a lower tier's text rather than suppressing it; `@omit` resolves like a text, ending the walk at the tier
that declares it, and the composers drop the element (and any section left with
no elements) instead of rendering it. Use it only where the books say an element
is omitted — the Triduum's chapter, short responsory, hymn and Vespers versicle
are the current cases. An omission carries no wording, so it is excluded from
the lint, provenance and zero-occurrence inventories; it is not permitted on a
psalm antiphon, since every psalm is sung under one.

**Common texts** (`data/texts/commons/confessor.txt`) — used when a feast has no proper of its own:

```ini
[collect]
O God, Who, year by year, dost gladden us by the solemn feast-day of thy blessed
confessor N., mercifully grant unto all who keep his birthday, grace to follow after
the pattern of his godly conversation.
```

`N.` is a placeholder substituted with the feast's `ProperName` field at runtime (e.g. `ProperName = Nicholas` → "blessed confessor Nicholas").

## Adding missing propers

Run `make scaffold-propers` first: it creates a proper file for every non-commemoration feast that lacks one, and appends any missing section keys as comments to sparse existing files. Live (uncommented) sections are never rewritten. Each commented key has a one-line explanation — uncomment the header and put text (or `@use …`) beneath it to activate.

```bash
make scaffold-propers                          # create/append scaffolds
./office scaffold propers -dry-run             # plan only
./office scaffold propers -check               # CI: fail if any feast still needs a scaffold
./office scaffold propers -feast st-ambrose    # one feast
./office scaffold propers -include-commemorations  # thin catalog for rank=commemoration
```

Then run `make audit` to see which feasts still need real proper *text*. For each feast listed, edit `data/texts/proper/{feast-id}.txt` (e.g. feast `[st-andrew]` → `data/texts/proper/st-andrew.txt`).

If a feast should intentionally fall back to ordinary/common texts (e.g. a feria or a minor feast without a unique proper), add its ID to `data/audit-ok.txt` to suppress the warning:

```
# data/audit-ok.txt
st-raphael-of-brooklyn *    # suppress all warnings for this feast
some-martyr commemoration-antiphon    # suppress only this one slot
```

## Psalm numbering

Psalm corpus keys use **Hebrew (Coverdale) numbering**. Hour definitions and
proper psalmody declarations reference those keys directly: Compline, for
example, uses `psalms/004`, `psalms/091`, and `psalms/134`. When copying a
reference from a book using Vulgate numbering, identify the matching corpus
psalm; the engine does not convert the number for you.

## Psalm text verification

Run `make verify-psalms` to compare every file in `data/texts/psalms/` with the [Church of England's official 1662 BCP Psalter](https://www.churchofengland.org/prayer-and-worship/worship-texts-and-resources/book-common-prayer/psalter). The check covers wording, punctuation, verse numbering, and chant separators; local `*` separators are retained as the project's representation. Historical readings are checked against the [official 1662 Book of Common Prayer PDF](https://www.churchofengland.org/sites/default/files/2019-10/the-book-of-common-prayer-1662.pdf) where the online transcription differs.
