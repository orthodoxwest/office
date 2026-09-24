# Editing liturgical data

[Back to the README](README.md) · [Review workflow](REVIEWING.md)

Liturgical data lives in `data/` and is read at startup, so editing it needs no
recompile (restart the server to pick up changes).

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
    proper/        feast-specific texts (one file per feast)
    commons/       texts by category (apostle, martyr, confessor, etc.)
    seasonal/      season-specific overrides
    psalmody/      shared psalmody declarations (see below)
    shared/        texts reused across several files (Marian texts, formulas)
    chant/         GABC chant scores (psalms/, canticles/, hymns/)
  office/          hour structure definitions
  review/          provenance attestations, prescreen flags, assurance baseline
```

Feast files use `[section]` headers and `Key = value` lines; text files use
`[section]` headers followed by multiline bodies. Both allow `#` comments and
blank lines.

## Feast definitions

```ini
# data/feasts/sanctoral.txt
[st-andrew]
Name     = Saint Andrew, Apostle
Rank     = double-2nd-class
Color    = red
Category = apostle
Month    = 11
Day      = 30
```

- `Rank`: `double-1st-class`, `double-2nd-class`, `greater-double`, `double`,
  `semi-double`, `privileged-feria`, `simple`, `commemoration`
- `Color`: `white`, `red`, `green`, `violet`, `rose`, `black`
- `Category`: `lord`, `blessed-virgin`, `angel`, `apostle`, `evangelist`,
  `martyr`, `martyrs`, `bishop-martyr`, `virgin-martyr`, `confessor-bishop`,
  `confessor-doctor`, `confessor`, `virgin`, `holy-woman`, `dedication`,
  `sunday`, `feria`

Optional keys:

| Key | Meaning |
|---|---|
| `DateRule` | Moveable date, used instead of `Month`/`Day` |
| `HasOctave = true` | Generate an octave |
| `OctaveClass` | `privileged-first` (Easter, Pentecost), `privileged-second` (Epiphany, Corpus Christi), `privileged-third` (Nativity, Ascension, Sacred Heart) per General Rubrics VII.3; `simple` for simple octaves or explicit Simple octave days. Omitted means a common octave. Generated days inherit it; terminal days do not inherit weekday commemoration privilege. |
| `CommemorationClass` | One of the two ferial ordering exceptions in XIV.14: `epiphany-vigil`, `post-ascension-feria` |
| `HasVigil = true` | Generate a preceding vigil |
| `IsVigil = true` + `VigilOf = feast-id` | This observance is an explicit vigil of that feast |
| `CompanionOf = feast-id` | Peter/Paul companion kept at II Vespers and ordered with its parent |
| `ProperName` | Given name substituted for `N.` in common texts |
| `ProperID` | Use another feast's proper texts |
| `OnlyWith` | Keep only on days where the named feast wins |
| `SkipRomanLeapShift = true` | Keep a late-February feast on its civil date in leap years |
| `Source`, `Notes` | Documentation only |

`OctaveClass` and `CommemorationClass` describe liturgical classes, not
per-date overrides.

## Feast propers

`data/texts/proper/{feast-id}.txt` holds one `[section]` per text slot. Include
only the slots the feast actually has; anything omitted falls back to the common
(by `Category`), seasonal, or ordinary text.

```ini
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

In prose sections (collects, chapters, prayers) a single newline is a soft wrap
and the web renderer reflows the paragraph; a blank line starts a new one. Hymn
and psalm renderers keep their verse structure.

Common sections (the [scaffold catalog](internal/scaffold/keys.go) lists every
supported key):

| Section | Used in |
|---------|---------|
| `[psalm-antiphon]` | Single psalm antiphon at Lauds |
| `[psalm-antiphon-1]` … `[psalm-antiphon-5]` | One antiphon per psalm; add `-vespers` for Vespers variants (`[psalm-antiphon-4-vespers]`) |
| `[benedictus-antiphon]` | Lauds |
| `[magnificat-antiphon]` | (II) Vespers |
| `[magnificat-antiphon-first]` | I Vespers |
| `[collect]` | All hours |
| `[chapter-lauds]`, `[chapter-vespers]`, `[chapter-terce]`, … | Chapter, per hour |
| `[versicle-lauds]`, `[versicle-vespers]`, … | Versicle/response, per hour |
| `[short-responsory-lauds]`, `[short-responsory-vespers]`, … | Short responsory, per hour |
| `[hymn-lauds]`, `[hymn-vespers]` | Full hymn text |
| `[commemoration-antiphon]`, `[commemoration-versicle]` | When commemorated |
| `[commemoration-collect]` | When commemorated (defaults to `[collect]`) |

### Directives

- `@use path/to/other/key` as the whole body reuses another corpus entry verbatim.
- `@omit` as the whole body means the element is **not said**. Leaving a section
  out inherits a lower tier's text; `@omit` ends the fallback walk and the
  composer drops the element (and any section left empty). Use it only where
  the books say an element is omitted, such as the Triduum's chapter, short
  responsory, hymn, and Vespers versicle. It is not allowed on a psalm antiphon
  and is excluded from the lint, provenance, and zero-occurrence inventories.
- `#` lines, including `# SOURCE:` provenance notes, are stripped by the loader
  and never render.

### Psalmody declarations

`data/texts/psalmody/{lauds,vespers}.txt` define shared psalmody tables, each
line pairing an antiphon slot with a psalm or canticle key:

```ini
[dead]
psalm-antiphon-1 = psalms/051
psalm-antiphon-2 = psalms/065
psalm-antiphon-3 = psalms/063
psalm-antiphon-4 = canticles/isaiah-38
```

A proper selects one with `[vespers-psalmody]`, `[vespers-psalmody-first]`, or
`[lauds-psalmody]` (e.g. `@use psalmody/vespers/martyrs`). The bare markers
`festal` (Lauds) and `ferial` (Vespers) keep the built-in appointment.
A concrete Lauds table also needs a `[lauds-laudate-psalmody]` declaring the
final Laudate section. `ProperID` and Paschal redirects apply, and
`make validate` checks row syntax and references. A declaration controls only
the psalmody, not the rest of the office's structure.

## Commons

`data/texts/commons/{category}.txt` supplies texts for feasts without their own.
`N.` is replaced by the feast's `ProperName` (`ProperName = Nicholas` →
"blessed confessor Nicholas").

## Adding missing propers

`make scaffold-propers` creates a proper file for every non-commemoration feast
lacking one and appends missing section keys, commented out with a one-line
explanation, to sparse files. Live sections are never rewritten. Uncomment a
header and add text (or `@use …`) beneath it to activate it.

```bash
make scaffold-propers                              # create/append scaffolds
./office scaffold propers -dry-run                 # plan only
./office scaffold propers -check                   # CI: fail if any feast still needs a scaffold
./office scaffold propers -feast st-ambrose        # one feast
./office scaffold propers -include-commemorations  # thin catalog for rank=commemoration
```

Then `make audit` lists feasts still missing real proper text. If a feast
should intentionally use ordinary/common texts, add it to `data/audit-ok.txt`:

```
st-raphael-of-brooklyn *              # suppress all warnings for this feast
some-martyr commemoration-antiphon    # suppress only this slot
```

## Psalms

Psalm keys use **Hebrew (Coverdale) numbering**: Compline uses `psalms/004`,
`psalms/091`, and `psalms/134`. The engine does not convert Vulgate numbers, so
translate references from Vulgate-numbered books yourself.

`make verify-psalms` compares `data/texts/psalms/` with the
[Church of England's 1662 BCP Psalter](https://www.churchofengland.org/prayer-and-worship/worship-texts-and-resources/book-common-prayer/psalter):
wording, punctuation, verse numbering, and chant separators (local `*`
separators are kept). Where the online transcription differs, the
[1662 BCP PDF](https://www.churchofengland.org/sites/default/files/2019-10/the-book-of-common-prayer-1662.pdf)
decides.
