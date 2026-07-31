package texts

import "strings"

// The corpus stores liturgical text as plain lines in a small, stable grammar
// that every renderer has to understand:
//
//	!Isaiah 55:1          a scripture reference
//	[section: Part II]    a canticle section break
//	V. O God, make speed  a versicle, response, or blessing sigil
//	1. Be merciful * unto us
//	                      a psalm verse: optional number, " * " mediant
//	Glory be to the Father, …
//	as it was in the beginning, …
//	                      the two-line Gloria Patri
//	[Ad Laudes]           a bracketed title artifact, dropped
//
// Parsing that grammar is knowledge about the data format, so it lives here
// beside the corpus loader rather than being reimplemented by each renderer.
// ParsePsalm and ParseBlock classify lines; deciding what markup a line turns
// into stays with the caller (HTML in internal/render, LaTeX in
// internal/output), which is the part that legitimately differs.

// PsalmKind classifies one parsed item of a psalm or canticle body.
type PsalmKind int

const (
	// PsalmVerse is a (possibly numbered) verse, split at its mediant.
	PsalmVerse PsalmKind = iota
	// PsalmSection is a canticle section break carrying a heading.
	PsalmSection
	// PsalmGloria is the two-line Gloria Patri. Second is empty when the
	// closing "as it was…" line is missing.
	PsalmGloria
)

// PsalmItem is one item of a parsed psalm or canticle body.
type PsalmItem struct {
	Kind PsalmKind

	// Heading is the section title (PsalmSection only).
	Heading string

	// Number is the verse number without its separator, empty when the verse
	// is unnumbered (PsalmVerse only).
	Number string

	// First and Second are the halves either side of the " * " mediant;
	// Second is empty when the line has no mediant. For PsalmGloria they are
	// the two whole lines, left unsplit — a Gloria is pointed as a pair.
	First  string
	Second string
}

// PsalmText is a parsed psalm or canticle: its scripture reference (from the
// title block above the first blank line) and its body items in order.
type PsalmText struct {
	ScriptureRef string
	Items        []PsalmItem
}

// ParsePsalm parses a psalm or canticle body. The title block — everything
// above the first blank line — is dropped except for a "!" scripture
// reference, which callers print above the verses.
//
// A "Glory be…" line opens a Gloria that the following "as it was…" line
// closes. If anything else intervenes, or the text ends first, the Gloria is
// emitted where it stands with an empty Second rather than swallowing the
// lines after it.
func ParsePsalm(text string) PsalmText {
	lines := strings.Split(text, "\n")

	var parsed PsalmText
	contentStart := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			contentStart = i + 1
			break
		}
		if strings.HasPrefix(trimmed, "!") {
			parsed.ScriptureRef = trimmed[1:]
		}
	}

	// gloria holds the index of an open Gloria item awaiting its second line.
	gloria := -1
	for _, line := range lines[contentStart:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if heading, ok := sectionHeading(line); ok {
			gloria = -1
			parsed.Items = append(parsed.Items, PsalmItem{Kind: PsalmSection, Heading: heading})
			continue
		}

		if gloria >= 0 && strings.HasPrefix(strings.ToLower(line), "as it was") {
			parsed.Items[gloria].Second = line
			gloria = -1
			continue
		}
		gloria = -1

		if strings.HasPrefix(line, "Glory be") {
			parsed.Items = append(parsed.Items, PsalmItem{Kind: PsalmGloria, First: line})
			gloria = len(parsed.Items) - 1
			continue
		}

		number, body, _ := splitLeadingVerseNumber(line)
		first, second, _ := strings.Cut(body, " * ")
		parsed.Items = append(parsed.Items, PsalmItem{
			Kind: PsalmVerse, Number: number, First: first, Second: second,
		})
	}

	return parsed
}

// sectionHeading recognises a "[section: Heading]" canticle break.
func sectionHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "[section:") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	return strings.TrimSpace(line[len("[section:") : len(line)-1]), true
}

// splitLeadingVerseNumber peels a leading verse number from a psalm or canticle
// line. Accepts the usual "2. Text" form and Benedicite-style "2 Text" (digits
// then a single space, no period). Numbers are at most 3 digits.
func splitLeadingVerseNumber(line string) (num, rest string, ok bool) {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i > 3 {
		return "", line, false
	}
	switch {
	case i+1 < len(line) && line[i] == '.' && line[i+1] == ' ':
		return line[:i], strings.TrimSpace(line[i+2:]), true
	case i < len(line) && line[i] == ' ':
		return line[:i], strings.TrimSpace(line[i+1:]), true
	default:
		return "", line, false
	}
}

// BlockKind classifies one parsed line of a liturgical block: collects,
// chapters, prayers, blessings, preces, and Marian antiphons.
type BlockKind int

const (
	// BlockProse is an ordinary text line. The corpus hard-wraps prose at
	// sense lines, so callers decide whether to keep or reflow the breaks.
	BlockProse BlockKind = iota
	// BlockGap is a blank separator line.
	BlockGap
	// BlockScriptureRef is a "!" reference line.
	BlockScriptureRef
	// BlockVersicle is a "V. " line.
	BlockVersicle
	// BlockResponse is an "R. " line.
	BlockResponse
	// BlockBlessing is a "Blessing. " line.
	BlockBlessing
	// BlockAll is an "All: " line, spoken by everyone together.
	BlockAll
)

// BlockLine is one parsed line of a liturgical block.
type BlockLine struct {
	Kind BlockKind

	// Text is the line's content with its sigil and surrounding space
	// removed; empty for BlockGap.
	Text string

	// Offset is the byte offset of Text within the string passed to
	// ParseBlock. Callers that partition a prayer into spoken and silent
	// spans use it to map each line back onto those spans.
	Offset int
}

// blockSigils maps a line prefix to the kind it introduces. Order matters
// only in that no prefix here is a prefix of another.
var blockSigils = []struct {
	prefix string
	kind   BlockKind
}{
	{"V. ", BlockVersicle},
	{"R. ", BlockResponse},
	{"Blessing. ", BlockBlessing},
	{"All: ", BlockAll},
}

// ParseBlock parses a liturgical block into classified lines. Bracketed title
// artifacts left over from the source books ("[Ad Laudes]") are dropped
// silently; they are not part of the prayed text.
func ParseBlock(text string) []BlockLine {
	var parsed []BlockLine

	offset := 0
	for raw := range strings.SplitSeq(text, "\n") {
		lineStart := offset
		offset += len(raw) + 1 // + the '\n' that Split consumed

		line := strings.TrimSpace(raw)
		if line == "" {
			parsed = append(parsed, BlockLine{Kind: BlockGap, Offset: lineStart})
			continue
		}

		// Offset of the trimmed content within the source.
		start := lineStart
		if pad := strings.Index(raw, line); pad > 0 {
			start += pad
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") &&
			strings.Contains(line[1:len(line)-1], " ") {
			continue
		}

		if strings.HasPrefix(line, "!") {
			parsed = append(parsed, BlockLine{Kind: BlockScriptureRef, Text: line[1:], Offset: start + 1})
			continue
		}

		matched := false
		for _, sigil := range blockSigils {
			if strings.HasPrefix(line, sigil.prefix) {
				parsed = append(parsed, BlockLine{
					Kind:   sigil.kind,
					Text:   line[len(sigil.prefix):],
					Offset: start + len(sigil.prefix),
				})
				matched = true
				break
			}
		}
		if matched {
			continue
		}

		parsed = append(parsed, BlockLine{Kind: BlockProse, Text: line, Offset: start})
	}

	return parsed
}

// Hymn is a parsed hymn: its Latin incipit, when the source carries one, and
// its stanzas, each a slice of lines.
type Hymn struct {
	Title   string
	Stanzas [][]string
}

// SplitHymnTitle peels a hymn's Latin incipit from its body. A title is a
// single line standing alone above a blank line ("Te lucis ante terminum");
// a multi-line first block is a stanza, not a title, and is left in the body.
//
// The office engine applies this when composing a hymn element, so a composed
// hymn usually arrives at a renderer with its title already in the element
// label and its body starting at the first stanza. Renderers must therefore
// parse with the same rule rather than assuming a title is still there —
// peeling a second time would eat the opening stanza.
func SplitHymnTitle(text string) (title, body string) {
	firstBlock, rest, found := strings.Cut(text, "\n\n")
	if !found {
		return "", text
	}
	firstBlock = strings.TrimSpace(firstBlock)
	if strings.ContainsRune(firstBlock, '\n') {
		return "", text
	}
	return firstBlock, strings.TrimSpace(rest)
}

// ParseHymn parses a hymn into its title and stanzas. Stanzas are separated by
// blank lines; each stanza keeps its own lines, since renderers break them
// differently.
func ParseHymn(text string) Hymn {
	title, body := SplitHymnTitle(strings.TrimSpace(text))
	parsed := Hymn{Title: title}

	var stanza []string
	flush := func() {
		if len(stanza) > 0 {
			parsed.Stanzas = append(parsed.Stanzas, stanza)
			stanza = nil
		}
	}
	for line := range strings.SplitSeq(body, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "" {
			flush()
		} else {
			stanza = append(stanza, trimmed)
		}
	}
	flush()

	return parsed
}
