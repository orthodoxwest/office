package render

import (
	"html/template"
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// escCross replaces the ✠ cross character with a styled HTML span.
func escCross(s string) string {
	return strings.ReplaceAll(template.HTMLEscapeString(s), "✠", `<span class="cross">✠</span>`)
}

func renderSectionElements(elems []models.OfficeElement) template.HTML {
	var sb strings.Builder

	for i := 0; i < len(elems); i++ {
		elem := elems[i]
		var doxologyText string
		if i+1 < len(elems) && (elem.Type == models.Psalm || elem.Type == models.Canticle) && elems[i+1].Type == models.PsalmDoxology {
			doxologyText = elems[i+1].Text
			i++
		}
		sb.WriteString(renderOfficeElement(elem, doxologyText))
	}

	return template.HTML(sb.String())
}

func renderOfficeElement(elem models.OfficeElement, doxologyText string) string {
	var sb strings.Builder

	switch elem.Type {
	case models.Heading:
		sb.WriteString(`<h2 class="section-heading">`)
		sb.WriteString(template.HTMLEscapeString(elem.Text))
		sb.WriteString(`</h2>`)
	case models.Rubric:
		sb.WriteString(`<p class="rubric">`)
		sb.WriteString(template.HTMLEscapeString(elem.Text))
		sb.WriteString(`</p>`)
	case models.Antiphon:
		if elem.Label != "" {
			sb.WriteString(`<div class="marian-antiphon"><h3 class="item-label">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			sb.WriteString(`</h3>`)
			sb.WriteString(string(renderMarianAntiphon(elem.Text)))
			sb.WriteString(`</div>`)
		} else {
			sb.WriteString(`<p class="antiphon"><em>Ant.</em> `)
			sb.WriteString(chantLineHTML(elem.Text))
			sb.WriteString(`</p>`)
		}
	case models.Psalm, models.Canticle:
		className := string(elem.Type)
		sb.WriteString(`<div class="`)
		sb.WriteString(className)
		sb.WriteString(`">`)
		if elem.Label != "" {
			sb.WriteString(`<h3 class="item-label">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			sb.WriteString(`</h3>`)
		}
		sb.WriteString(string(renderPsalmVerses(elem.Text)))
		if doxologyText != "" {
			sb.WriteString(string(renderGloriaPatri(doxologyText)))
		}
		sb.WriteString(`</div>`)
	case models.Hymn:
		sb.WriteString(`<div class="hymn"><h2 class="section-heading">Hymn</h2>`)
		if elem.Label != "" {
			sb.WriteString(`<p class="hymn-title">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			sb.WriteString(`</p>`)
		}
		sb.WriteString(string(renderHymnStanzas(elem.Text)))
		sb.WriteString(`</div>`)
	case models.Versicle, models.Response, models.Blessing, models.Doxology:
		sb.WriteString(string(renderLiturgicalBlock(elem.Text)))
	case models.Collect, models.Prayer:
		if len(elem.Voice) > 0 {
			sb.WriteString(string(renderVoiceLiturgicalBlock(elem.Voice, flowProseLines)))
		} else {
			sb.WriteString(string(renderFlowingLiturgicalBlock(elem.Text)))
		}
	case models.Chapter:
		sb.WriteString(`<div class="chapter"><h2 class="section-heading">Chapter</h2>`)
		if elem.Label != "" {
			sb.WriteString(`<p class="chapter-ref">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			sb.WriteString(`</p>`)
		}
		sb.WriteString(string(renderFlowingLiturgicalBlock(elem.Text)))
		sb.WriteString(`</div>`)
	case models.Preces:
		sb.WriteString(`<div class="preces">`)
		sb.WriteString(string(renderLiturgicalBlock(elem.Text)))
		sb.WriteString(`</div>`)
	case models.PsalmDoxology:
		sb.WriteString(string(renderGloriaPatri(elem.Text)))
	default:
		sb.WriteString(`<p class="element">`)
		sb.WriteString(template.HTMLEscapeString(elem.Text))
		sb.WriteString(`</p>`)
	}

	return sb.String()
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

// renderPsalmVerses parses a raw psalm or canticle text (title/scripture-ref line,
// numbered verses with " * " mediants, Gloria Patri) into structured HTML.
//
// Scripture references (lines beginning with "!") are emitted as a
// <p class="scripture-ref"> BEFORE the psalm-verses div, so that the first
// verse is always the first child of .psalm-verses and receives the drop cap.
func renderPsalmVerses(text string) template.HTML {
	lines := strings.Split(text, "\n")
	var sb strings.Builder

	scriptureRef := ""
	contentStart := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			contentStart = i + 1
			break
		}
		if strings.HasPrefix(trimmed, "!") {
			scriptureRef = trimmed[1:]
		}
	}

	if scriptureRef != "" {
		sb.WriteString(`<p class="scripture-ref">`)
		sb.WriteString(template.HTMLEscapeString(scriptureRef))
		sb.WriteString(`</p>`)
	}

	sb.WriteString(`<div class="psalm-verses">`)

	gloriaOpen := false
	for _, line := range lines[contentStart:] {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[section:") && strings.HasSuffix(line, "]") {
			heading := strings.TrimSpace(line[9 : len(line)-1])
			sb.WriteString(`</div>`)
			sb.WriteString(`<p class="canticle-section">`)
			sb.WriteString(template.HTMLEscapeString(heading))
			sb.WriteString(`</p>`)
			sb.WriteString(`<div class="psalm-verses">`)
			continue
		}

		if strings.HasPrefix(line, "Glory be") {
			sb.WriteString(`<p class="verse">`)
			sb.WriteString(template.HTMLEscapeString(line))
			gloriaOpen = true
			continue
		}
		if (strings.HasPrefix(line, "as it was") || strings.HasPrefix(line, "As it was")) && gloriaOpen {
			sb.WriteString(` <span class="mediant">*</span> `)
			sb.WriteString(template.HTMLEscapeString(line))
			sb.WriteString(`</p>`)
			gloriaOpen = false
			continue
		}

		verseNum, verseText, _ := splitLeadingVerseNumber(line)

		parts := strings.SplitN(verseText, " * ", 2)

		if verseNum != "" {
			sb.WriteString(`<p class="verse numbered"><span class="verse-num">`)
			sb.WriteString(template.HTMLEscapeString(verseNum))
			sb.WriteString(`</span><span class="verse-body">`)
			sb.WriteString(escCross(parts[0]))
			if len(parts) == 2 {
				sb.WriteString(` <span class="mediant">*</span> `)
				sb.WriteString(escCross(parts[1]))
			}
			sb.WriteString(`</span></p>`)
		} else {
			sb.WriteString(`<p class="verse">`)
			sb.WriteString(escCross(parts[0]))
			if len(parts) == 2 {
				sb.WriteString(` <span class="mediant">*</span> `)
				sb.WriteString(escCross(parts[1]))
			}
			sb.WriteString(`</p>`)
		}
	}

	if gloriaOpen {
		sb.WriteString(`</p>`)
	}
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

type proseLineMode uint8

const (
	preserveProseLines proseLineMode = iota
	flowProseLines
	preserveFirstProseBlock
)

// renderLiturgicalBlock renders multi-line liturgical text while preserving prose
// line breaks, as required by blessings, doxologies, and preces.
func renderLiturgicalBlock(text string) template.HTML {
	return renderLiturgicalBlockWithMode(text, preserveProseLines)
}

// renderVoiceLiturgicalBlock renders a prayer partitioned into spoken/silent spans.
// Span texts are concatenated in order; presentation follows the same prose-line
// rules as renderLiturgicalBlockWithMode, with each run of spoken or silent text
// wrapped in a CSS class.
func renderVoiceLiturgicalBlock(spans []models.VoiceSpan, mode proseLineMode) template.HTML {
	var full strings.Builder
	for _, sp := range spans {
		full.WriteString(sp.Text)
	}
	text := full.String()
	spokenAt := make([]bool, len(text))
	off := 0
	for _, sp := range spans {
		for i := 0; i < len(sp.Text); i++ {
			spokenAt[off+i] = sp.Spoken
		}
		off += len(sp.Text)
	}
	return renderLiturgicalBlockWithVoice(text, spokenAt, mode)
}

// emitVoicedHTML writes text[0:] as one or more <span class="spoken-text|secret-text">
// runs according to spokenAt[offset:offset+len(text)].
func emitVoicedHTML(sb *strings.Builder, text string, offset int, spokenAt []bool) {
	if text == "" {
		return
	}
	i := 0
	for i < len(text) {
		sp := spokenAt[offset+i]
		j := i + 1
		for j < len(text) && spokenAt[offset+j] == sp {
			j++
		}
		class := "secret-text"
		if sp {
			class = "spoken-text"
		}
		sb.WriteString(`<span class="`)
		sb.WriteString(class)
		sb.WriteString(`">`)
		sb.WriteString(escCross(text[i:j]))
		sb.WriteString(`</span>`)
		i = j
	}
}

func renderLiturgicalBlockWithVoice(text string, spokenAt []bool, mode proseLineMode) template.HTML {
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	sb.WriteString(`<div class="liturgical-block">`)

	type proseLine struct {
		text   string
		offset int
	}
	var proseLines []proseLine
	proseBlocks := 0
	pendingGap := false
	// Byte offset of the start of the current line within text (accounting for '\n').
	lineOffset := 0

	emitGap := func() {
		if pendingGap {
			sb.WriteString(`<div class="liturgical-gap"></div>`)
			pendingGap = false
		}
	}

	flushProse := func() {
		if len(proseLines) == 0 {
			return
		}
		emitGap()
		sb.WriteString(`<p class="plain-line">`)
		preserveLines := mode == preserveProseLines || (mode == preserveFirstProseBlock && proseBlocks == 0)
		for i, l := range proseLines {
			if i > 0 {
				if preserveLines {
					sb.WriteString(`<br>`)
				} else {
					sb.WriteByte(' ')
				}
			}
			emitVoicedHTML(&sb, l.text, l.offset, spokenAt)
		}
		sb.WriteString(`</p>`)
		proseLines = nil
		proseBlocks++
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Offset of trimmed content within text (line may have leading spaces).
		contentOffset := lineOffset
		if pad := strings.Index(line, trimmed); pad > 0 && trimmed != "" {
			contentOffset = lineOffset + pad
		}

		if trimmed == "" {
			flushProse()
			pendingGap = true
			lineOffset += len(line) + 1
			continue
		}

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") &&
			strings.Contains(trimmed[1:len(trimmed)-1], " ") {
			lineOffset += len(line) + 1
			continue
		}

		if strings.HasPrefix(trimmed, "!") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="scripture-ref">`)
			sb.WriteString(template.HTMLEscapeString(trimmed[1:]))
			sb.WriteString(`</p>`)
			lineOffset += len(line) + 1
			continue
		}

		if strings.HasPrefix(trimmed, "V. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="versicle-line"><span class="sigil">℣.</span><span class="sigil-text">`)
			emitVoicedHTML(&sb, trimmed[3:], contentOffset+3, spokenAt)
			sb.WriteString(`</span></p>`)
			lineOffset += len(line) + 1
			continue
		}

		if strings.HasPrefix(trimmed, "R. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="response-line"><span class="sigil">℟.</span><span class="sigil-text">`)
			emitVoicedHTML(&sb, trimmed[3:], contentOffset+3, spokenAt)
			sb.WriteString(`</span></p>`)
			lineOffset += len(line) + 1
			continue
		}

		if strings.HasPrefix(trimmed, "Blessing. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="versicle-line"><span class="sigil">Blessing.</span><span class="sigil-text">`)
			emitVoicedHTML(&sb, trimmed[10:], contentOffset+10, spokenAt)
			sb.WriteString(`</span></p>`)
			lineOffset += len(line) + 1
			continue
		}

		proseLines = append(proseLines, proseLine{text: trimmed, offset: contentOffset})
		lineOffset += len(line) + 1
	}

	flushProse()
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

// renderFlowingLiturgicalBlock renders collects, chapters, and prayers with soft
// source wrapping, while retaining semantic lines such as versicles and responses.
func renderFlowingLiturgicalBlock(text string) template.HTML {
	return renderLiturgicalBlockWithMode(text, flowProseLines)
}

// renderMarianAntiphon preserves the verse lines in the antiphon's opening prose
// block while allowing its versicles and concluding prayer to flow normally.
func renderMarianAntiphon(text string) template.HTML {
	return renderLiturgicalBlockWithMode(text, preserveFirstProseBlock)
}

// chantLineHTML renders a line of liturgical text, styling a " * " pointing
// mediant as the muted glyph used in psalm verses (antiphons, responsories,
// versicles, plain prose in liturgical blocks, Marian chant lines, etc.).
func chantLineHTML(line string) string {
	before, after, found := strings.Cut(line, " * ")
	if !found {
		return escCross(line)
	}
	return escCross(before) + ` <span class="mediant">*</span> ` + escCross(after)
}

func renderLiturgicalBlockWithMode(text string, mode proseLineMode) template.HTML {
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	sb.WriteString(`<div class="liturgical-block">`)

	var proseLines []string
	proseBlocks := 0
	pendingGap := false

	emitGap := func() {
		if pendingGap {
			sb.WriteString(`<div class="liturgical-gap"></div>`)
			pendingGap = false
		}
	}

	flushProse := func() {
		if len(proseLines) == 0 {
			return
		}
		emitGap()

		// The sung Marian antiphon (the first preserved block) renders each
		// source line as its own chant line, so a hanging indent can tuck a
		// wrapped remainder under its own line while every line stays a
		// discrete reference point for chanting. Other preserved blocks
		// (preces, blessings, doxologies) keep the <br>-joined paragraph.
		if mode == preserveFirstProseBlock && proseBlocks == 0 {
			for _, l := range proseLines {
				sb.WriteString(`<p class="chant-line">`)
				sb.WriteString(chantLineHTML(l))
				sb.WriteString(`</p>`)
			}
			proseLines = nil
			proseBlocks++
			return
		}

		sb.WriteString(`<p class="plain-line">`)
		preserveLines := mode == preserveProseLines
		for i, l := range proseLines {
			if i > 0 {
				if preserveLines {
					sb.WriteString(`<br>`)
				} else {
					sb.WriteByte(' ')
				}
			}
			sb.WriteString(chantLineHTML(l))
		}
		sb.WriteString(`</p>`)
		proseLines = nil
		proseBlocks++
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			flushProse()
			pendingGap = true
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") &&
			strings.Contains(line[1:len(line)-1], " ") {
			continue
		}

		if strings.HasPrefix(line, "!") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="scripture-ref">`)
			sb.WriteString(template.HTMLEscapeString(line[1:]))
			sb.WriteString(`</p>`)
			continue
		}

		if strings.HasPrefix(line, "V. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="versicle-line"><span class="sigil">℣.</span><span class="sigil-text">`)
			sb.WriteString(chantLineHTML(line[3:]))
			sb.WriteString(`</span></p>`)
			continue
		}

		if strings.HasPrefix(line, "R. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="response-line"><span class="sigil">℟.</span><span class="sigil-text">`)
			sb.WriteString(chantLineHTML(line[3:]))
			sb.WriteString(`</span></p>`)
			continue
		}

		if strings.HasPrefix(line, "Blessing. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="versicle-line"><span class="sigil">Blessing.</span><span class="sigil-text">`)
			sb.WriteString(chantLineHTML(line[10:]))
			sb.WriteString(`</span></p>`)
			continue
		}

		proseLines = append(proseLines, line)
	}

	flushProse()
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

// renderHymnStanzas parses a hymn text into structured HTML with per-stanza <p> elements.
func renderHymnStanzas(text string) template.HTML {
	var sb strings.Builder

	title := ""
	body := strings.TrimSpace(text)
	if firstBlock, rest, found := strings.Cut(body, "\n\n"); found {
		if !strings.ContainsRune(strings.TrimSpace(firstBlock), '\n') {
			title = strings.TrimSpace(firstBlock)
			body = strings.TrimSpace(rest)
		}
	}

	sb.WriteString(`<div class="hymn-verses">`)
	if title != "" {
		sb.WriteString(`<p class="hymn-latin">`)
		sb.WriteString(template.HTMLEscapeString(title))
		sb.WriteString(`</p>`)
	}

	var stanza []string
	emitStanza := func() {
		if len(stanza) == 0 {
			return
		}
		sb.WriteString(`<p class="hymn-stanza">`)
		for i, l := range stanza {
			if i > 0 {
				sb.WriteString(`<br>`)
			}
			sb.WriteString(escCross(l))
		}
		sb.WriteString(`</p>`)
		stanza = nil
	}
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "" {
			emitStanza()
		} else {
			stanza = append(stanza, trimmed)
		}
	}
	emitStanza()

	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

// renderGloriaPatri renders the two-line Gloria Patri text with a * mediant break.
func renderGloriaPatri(text string) template.HTML {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var line1, line2 string
	if len(lines) >= 2 {
		line1 = strings.TrimSpace(lines[0])
		line2 = strings.TrimSpace(lines[1])
	} else {
		line1 = strings.TrimSpace(text)
	}
	var sb strings.Builder
	sb.WriteString(`<p class="gloria-patri">`)
	sb.WriteString(template.HTMLEscapeString(line1))
	if line2 != "" {
		sb.WriteString(` <span class="mediant">*</span> `)
		sb.WriteString(template.HTMLEscapeString(line2))
	}
	sb.WriteString(`</p>`)
	return template.HTML(sb.String())
}
