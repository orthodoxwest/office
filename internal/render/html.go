package render

import (
	"html/template"
	"strings"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
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
			// Diurnal convention: the Latin incipit identifies the psalm
			// beside its number. It shares the label line so it costs no
			// vertical space on a phone; the separator is hidden from
			// assistive technology, which reads label then incipit.
			sb.WriteString(`<h3 class="item-label">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			if elem.Incipit != "" {
				sb.WriteString(`<span class="label-sep" aria-hidden="true"> · </span>`)
				sb.WriteString(`<span class="psalm-incipit" lang="la">`)
				sb.WriteString(template.HTMLEscapeString(elem.Incipit))
				sb.WriteString(`</span>`)
			}
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

// renderPsalmVerses renders a parsed psalm or canticle as structured HTML.
//
// Scripture references are emitted as a <p class="scripture-ref"> BEFORE the
// psalm-verses div, so that the first verse is always the first child of
// .psalm-verses and receives the drop cap.
func renderPsalmVerses(text string) template.HTML {
	psalm := texts.ParsePsalm(text)
	var sb strings.Builder

	if psalm.ScriptureRef != "" {
		sb.WriteString(`<p class="scripture-ref">`)
		sb.WriteString(template.HTMLEscapeString(psalm.ScriptureRef))
		sb.WriteString(`</p>`)
	}

	sb.WriteString(`<div class="psalm-verses">`)

	for _, item := range psalm.Items {
		switch item.Kind {
		case texts.PsalmSection:
			sb.WriteString(`</div>`)
			sb.WriteString(`<p class="canticle-section">`)
			sb.WriteString(template.HTMLEscapeString(item.Heading))
			sb.WriteString(`</p>`)
			sb.WriteString(`<div class="psalm-verses">`)

		case texts.PsalmGloria:
			// Pointed as a pair: the break falls between the two lines.
			sb.WriteString(`<p class="verse">`)
			sb.WriteString(template.HTMLEscapeString(item.First))
			if item.Second != "" {
				sb.WriteString(` <span class="mediant">*</span> `)
				sb.WriteString(template.HTMLEscapeString(item.Second))
			}
			sb.WriteString(`</p>`)

		default:
			if item.Number != "" {
				sb.WriteString(`<p class="verse numbered"><span class="verse-num">`)
				sb.WriteString(template.HTMLEscapeString(item.Number))
				sb.WriteString(`</span><span class="verse-body">`)
			} else {
				sb.WriteString(`<p class="verse">`)
			}
			sb.WriteString(escCross(item.First))
			if item.Second != "" {
				sb.WriteString(` <span class="mediant">*</span> `)
				sb.WriteString(escCross(item.Second))
			}
			if item.Number != "" {
				sb.WriteString(`</span>`)
			}
			sb.WriteString(`</p>`)
		}
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
	var sb strings.Builder
	sb.WriteString(`<div class="liturgical-block">`)

	var proseLines []texts.BlockLine
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
			emitVoicedHTML(&sb, l.Text, l.Offset, spokenAt)
		}
		sb.WriteString(`</p>`)
		proseLines = nil
		proseBlocks++
	}

	sigilLine := func(class, sigil string, line texts.BlockLine) {
		flushProse()
		emitGap()
		sb.WriteString(`<p class="` + class + `"><span class="sigil">` + sigil + `</span><span class="sigil-text">`)
		emitVoicedHTML(&sb, line.Text, line.Offset, spokenAt)
		sb.WriteString(`</span></p>`)
	}

	for _, line := range texts.ParseBlock(text) {
		switch line.Kind {
		case texts.BlockGap:
			flushProse()
			pendingGap = true
		case texts.BlockScriptureRef:
			flushProse()
			emitGap()
			sb.WriteString(`<p class="scripture-ref">`)
			sb.WriteString(template.HTMLEscapeString(line.Text))
			sb.WriteString(`</p>`)
		case texts.BlockVersicle:
			sigilLine("versicle-line", "℣.", line)
		case texts.BlockResponse:
			sigilLine("response-line", "℟.", line)
		case texts.BlockBlessing:
			sigilLine("versicle-line", "Blessing.", line)
		default:
			proseLines = append(proseLines, line)
		}
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

	sigilLine := func(class, sigil, text string) {
		flushProse()
		emitGap()
		sb.WriteString(`<p class="` + class + `"><span class="sigil">` + sigil + `</span><span class="sigil-text">`)
		sb.WriteString(chantLineHTML(text))
		sb.WriteString(`</span></p>`)
	}

	for _, line := range texts.ParseBlock(text) {
		switch line.Kind {
		case texts.BlockGap:
			flushProse()
			pendingGap = true
		case texts.BlockScriptureRef:
			flushProse()
			emitGap()
			sb.WriteString(`<p class="scripture-ref">`)
			sb.WriteString(template.HTMLEscapeString(line.Text))
			sb.WriteString(`</p>`)
		case texts.BlockVersicle:
			sigilLine("versicle-line", "℣.", line.Text)
		case texts.BlockResponse:
			sigilLine("response-line", "℟.", line.Text)
		case texts.BlockBlessing:
			sigilLine("versicle-line", "Blessing.", line.Text)
		default:
			proseLines = append(proseLines, line.Text)
		}
	}

	flushProse()
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

// renderHymnStanzas renders a hymn as structured HTML, one <p> per stanza.
func renderHymnStanzas(text string) template.HTML {
	hymn := texts.ParseHymn(text)
	var sb strings.Builder

	sb.WriteString(`<div class="hymn-verses">`)
	if hymn.Title != "" {
		sb.WriteString(`<p class="hymn-latin">`)
		sb.WriteString(template.HTMLEscapeString(hymn.Title))
		sb.WriteString(`</p>`)
	}

	for _, stanza := range hymn.Stanzas {
		sb.WriteString(`<p class="hymn-stanza">`)
		for i, line := range stanza {
			if i > 0 {
				sb.WriteString(`<br>`)
			}
			sb.WriteString(escCross(line))
		}
		sb.WriteString(`</p>`)
	}

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
