package render

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestRenderBenediciteSpaceNumberedVerses(t *testing.T) {
	html := string(renderPsalmVerses("Song of the Three Children\n\n" +
		"O ALL ye Works of the Lord, bless ye the Lord: * praise him, and magnify him forever.\n" +
		"2 O ye Angels of the Lord, bless ye the Lord: * O ye Heavens, bless ye the Lord.\n" +
		"10 O let the Earth bless the Lord: * yea, let it praise him, and magnify him for ever.\n"))

	if !strings.Contains(html, `<p class="verse numbered"><span class="verse-num">2</span>`) {
		t.Fatalf("expected space-style verse 2 to use verse-num: %s", html)
	}
	if !strings.Contains(html, `<span class="verse-num">10</span>`) {
		t.Fatalf("expected space-style verse 10 to use verse-num: %s", html)
	}
	if strings.Contains(html, `>2 O ye Angels`) {
		t.Fatalf("verse number must not remain in the body text: %s", html)
	}
	// Drop-cap opening: single-letter O kept; multi-letter ALL softened.
	if !strings.Contains(html, `O All ye Works of the Lord`) {
		t.Fatalf("expected ALL-CAPS opening softened for drop cap: %s", html)
	}
	if strings.Contains(html, `O ALL ye`) {
		t.Fatalf("ALL-CAPS opening must not remain beside the drop cap: %s", html)
	}
}

func TestSoftenDropCapOpening(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"GOD be merciful unto us", "God be merciful unto us"},
		{"HAVE mercy upon me, O God", "Have mercy upon me, O God"},
		{"BLESSED are those", "Blessed are those"},
		{"WHEREWITHAL shall a young man", "Wherewithal shall a young man"},
		{"MY SOUL cleaveth to the dust", "My Soul cleaveth to the dust"},
		{"O GIVE thanks unto the Lord", "O Give thanks unto the Lord"},
		{"O ALL ye Works of the Lord", "O All ye Works of the Lord"},
		// Single-letter capitals are left alone (not title-cased to themselves via the multi-letter path).
		{"O God", "O God"},
		{"I will magnify thee", "I will magnify thee"},
		// Trailing punctuation is stripped from the letter run, then restored:
		// without the letterEnd > 0 loop, "GOD," would fail the all-caps check.
		{"GOD, be merciful", "God, be merciful"},
		{"BLESSED!", "Blessed!"},
		{"MY SOUL,", "My Soul,"},
		// Leading / trailing whitespace is preserved exactly (including when the
		// only content is spaces, and when softening stops after the run).
		{"  GOD be merciful", "  God be merciful"},
		{"GOD be merciful  ", "God be merciful  "},
		{"  GOD  ", "  God  "},
		{"\tGOD be", "\tGod be"},
		// A pure-punctuation token ends the opening run without rewriting it.
		{"... GOD be", "... GOD be"},
		{"— GOD be", "— GOD be"},
		// Already mixed / sentence case: unchanged.
		{"Blessed be the Lord God of Israel", "Blessed be the Lord God of Israel"},
		{"That thy way may be known", "That thy way may be known"},
		{"God be merciful", "God be merciful"},
		// A non-letter inside the letter run (not only trailing) fails all-caps.
		{"PSA6LM is", "PSA6LM is"},
		// Trailing digits are treated like trailing punctuation and kept after
		// the title-cased letter run (same path as "GOD,").
		{"PSALM67 is", "Psalm67 is"},
		{"", ""},
		{"   ", "   "},
	}
	for _, tt := range cases {
		if got := softenDropCapOpening(tt.in); got != tt.want {
			t.Errorf("softenDropCapOpening(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenderPsalmSoftensDropCapOpeningOnly(t *testing.T) {
	html := string(renderPsalmVerses("Psalm 67\n\n" +
		"GOD be merciful unto us, and bless us * and shew us the light of his countenance.\n" +
		"2. That thy way may be known upon earth * thy saving health among all nations.\n"))

	if !strings.Contains(html, `>God be merciful unto us`) {
		t.Fatalf("expected drop-cap verse opening softened: %s", html)
	}
	if strings.Contains(html, `>GOD be merciful`) {
		t.Fatalf("ALL-CAPS opening must not remain on the drop-cap verse: %s", html)
	}
	// Numbered verses keep their source capitalisation.
	if !strings.Contains(html, `That thy way may be known upon earth`) {
		t.Fatalf("expected numbered verse body preserved: %s", html)
	}
}

// Softening applies to the first verse of each .psalm-verses block — including
// the first verse after a mid-canticle section break — and never to later verses.
func TestRenderPsalmSoftensDropCapAfterSectionBreak(t *testing.T) {
	html := string(renderPsalmVerses("Benedicite\n\n" +
		"O ALL ye Works of the Lord, bless ye the Lord: * praise him forever.\n" +
		"2 O ye Angels of the Lord, bless ye the Lord: * O ye Heavens, bless ye the Lord.\n" +
		"[section: Part II]\n" +
		"O LET the Earth bless the Lord: * yea, let it praise him forever.\n" +
		"10 O ye Mountains and Hills, bless ye the Lord: * praise him forever.\n"))

	if !strings.Contains(html, `O All ye Works of the Lord`) {
		t.Fatalf("expected opening block drop-cap softened: %s", html)
	}
	if !strings.Contains(html, `O Let the Earth bless the Lord`) {
		t.Fatalf("expected post-section drop-cap softened: %s", html)
	}
	if strings.Contains(html, `O LET the Earth`) || strings.Contains(html, `O ALL ye Works`) {
		t.Fatalf("ALL-CAPS openings must not survive on drop-cap verses: %s", html)
	}
	// Numbered verse after a section is not a drop-cap verse.
	if !strings.Contains(html, `O ye Mountains and Hills`) {
		t.Fatalf("expected numbered post-section verse preserved: %s", html)
	}
}

func TestRenderSectionElementsMergesPsalmDoxologyIntoPsalmBlock(t *testing.T) {
	html := string(renderSectionElements([]models.OfficeElement{
		{Type: models.Psalm, Label: "Psalm 67", Text: "Psalm 67\n\n1. Be merciful unto us * and bless us."},
		{Type: models.PsalmDoxology, Text: "Glory be to the Father,\nas it was in the beginning."},
	}))

	if !strings.Contains(html, `<div class="psalm">`) {
		t.Fatalf("expected psalm wrapper in output: %s", html)
	}
	if !strings.Contains(html, `<div class="psalm"><h3 class="item-label">Psalm 67</h3>`) {
		t.Fatalf("expected psalm label inside wrapper: %s", html)
	}
	if !strings.Contains(html, `<div class="psalm-verses">`) {
		t.Fatalf("expected rendered psalm verses in output: %s", html)
	}
	if !strings.Contains(html, `<p class="gloria-patri">`) {
		t.Fatalf("expected gloria patri in output: %s", html)
	}
	if !strings.Contains(html, `</div><p class="gloria-patri">`) {
		t.Fatalf("expected gloria patri to be rendered immediately after the psalm verses inside the psalm block: %s", html)
	}
	if !strings.HasSuffix(html, `</p></div>`) {
		t.Fatalf("expected psalm block to close after the gloria patri: %s", html)
	}
}

func TestRenderCollectReflowsProseAndPreservesSemanticLines(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Collect,
		Text: "Almighty God, who hast brought us\nto the beginning of this day.\n\nV. O Lord, hear my prayer.\nR. And let my cry come unto thee.",
	}, "")

	if !strings.Contains(html, `<p class="plain-line">Almighty God, who hast brought us to the beginning of this day.</p>`) {
		t.Fatalf("expected source-wrapped prose to flow as one paragraph: %s", html)
	}
	if strings.Contains(html, `brought us<br>to`) {
		t.Fatalf("source wrapping must not produce a hard break: %s", html)
	}
	if !strings.Contains(html, `<span class="sigil">℣.</span>`) || !strings.Contains(html, `<span class="sigil">℟.</span>`) {
		t.Fatalf("expected versicle and response to remain semantic lines: %s", html)
	}
	if !strings.Contains(html, `<div class="liturgical-gap"></div>`) {
		t.Fatalf("expected a blank source line to retain paragraph spacing: %s", html)
	}
}

func TestRenderPrayerReflowsSourceLines(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Prayer,
		Text: "Thy kingdom come.\nThy will be done.",
	}, "")

	if !strings.Contains(html, `Thy kingdom come. Thy will be done.`) {
		t.Fatalf("expected prayer source lines to reflow: %s", html)
	}
}

func TestRenderSecretPrayerVoiceSpans(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Prayer,
		Text: "Our Father, who art in heaven.\nThy kingdom come.",
		Voice: []models.VoiceSpan{
			{Text: "Our Father", Spoken: true},
			{Text: ", who art in heaven.\nThy kingdom come.", Spoken: false},
		},
	}, "")

	if !strings.Contains(html, `<span class="spoken-text">Our Father</span>`) {
		t.Fatalf("expected spoken incipit: %s", html)
	}
	if !strings.Contains(html, `<span class="secret-text">, who art in heaven.</span>`) {
		t.Fatalf("expected silent body start: %s", html)
	}
	if !strings.Contains(html, `<span class="secret-text">Thy kingdom come.</span>`) {
		t.Fatalf("expected silent continuation after reflow: %s", html)
	}
	if !strings.Contains(html, `heaven.</span> <span class="secret-text">Thy kingdom`) {
		t.Fatalf("expected reflowed space between source lines: %s", html)
	}
}

func TestRenderPartlySecretPrayerVoiceSpans(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Prayer,
		Text: "Our Father, middle.\nAnd lead us not into temptation,\nBut deliver us from evil.",
		Voice: []models.VoiceSpan{
			{Text: "Our Father", Spoken: true},
			{Text: ", middle.\n", Spoken: false},
			{Text: "And lead us not into temptation,\nBut deliver us from evil.", Spoken: true},
		},
	}, "")

	if !strings.Contains(html, `<span class="spoken-text">Our Father</span><span class="secret-text">, middle.</span>`) {
		t.Fatalf("expected spoken incipit then silent middle: %s", html)
	}
	if !strings.Contains(html, `<span class="spoken-text">And lead us not into temptation,</span>`) {
		t.Fatalf("expected spoken tail start: %s", html)
	}
	if !strings.Contains(html, `<span class="spoken-text">But deliver us from evil.</span>`) {
		t.Fatalf("expected spoken tail continuation: %s", html)
	}
}

func TestRenderMarianAntiphonPreservesVerseAndReflowsPrayer(t *testing.T) {
	text := "[Ave Regina Caelorum]\n\nQueen of the heavens, we hail thee,\nHail thee, Lady of all the Angels;\n\nV. Vouchsafe that I may praise thee.\nR. Give me strength.\n\nLet us pray.\n\nGrant us, O merciful God, protection in our weakness:\nthat we may rise again from our sins."
	html := string(renderMarianAntiphon(text))

	if !strings.Contains(html, `<p class="chant-line">Queen of the heavens, we hail thee,</p><p class="chant-line">Hail thee, Lady of all the Angels;</p>`) {
		t.Fatalf("expected each opening Marian verse line as its own chant line: %s", html)
	}
	if !strings.Contains(html, `Grant us, O merciful God, protection in our weakness: that we may rise again from our sins.`) {
		t.Fatalf("expected the concluding Marian prayer to flow: %s", html)
	}
	if strings.Contains(html, `<br>`) {
		t.Fatalf("Marian antiphon should use per-line blocks, not hard breaks: %s", html)
	}
}

func TestRenderMarianAntiphonStylesIncipitMediant(t *testing.T) {
	text := "Mary we hail thee * Mother and Queen compassionate;\nMary our comfort, life, and hope, we hail thee."
	html := string(renderMarianAntiphon(text))

	if !strings.Contains(html, `<p class="chant-line">Mary we hail thee <span class="mediant">*</span> Mother and Queen compassionate;</p>`) {
		t.Fatalf("expected the incipit mediant styled like a psalm verse: %s", html)
	}
}

func TestRenderAntiphonStylesMediant(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Antiphon,
		Text: "The Lord said * to my Lord: Sit thou at my right hand.",
	}, "")

	if !strings.Contains(html, `The Lord said <span class="mediant">*</span> to my Lord: Sit thou at my right hand.`) {
		t.Fatalf("expected antiphon mediant styled like a psalm verse: %s", html)
	}
}

func TestRenderAnnouncedAntiphonPrintsIncipitOnly(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type:     models.Antiphon,
		Text:     "Do away, O Lord, * mine offenses.",
		Announce: true,
	}, "")

	if !strings.Contains(html, `class="antiphon antiphon-announce"`) {
		t.Fatalf("expected antiphon-announce class: %s", html)
	}
	if !strings.Contains(html, `Do away, O Lord, <span class="mediant">*</span>`) {
		t.Fatalf("expected announced form through the mediant: %s", html)
	}
	if strings.Contains(html, "mine offenses") {
		t.Fatalf("announced antiphon must not print the rest of the text: %s", html)
	}
}

func TestRenderResponseStylesMediant(t *testing.T) {
	html := string(renderLiturgicalBlock("R. Great is our Lord * and great is his power."))

	if !strings.Contains(html, `<span class="sigil-text">Great is our Lord <span class="mediant">*</span> and great is his power.</span>`) {
		t.Fatalf("expected response mediant styled like a psalm verse: %s", html)
	}
}

func TestRenderVersicleStylesMediant(t *testing.T) {
	html := string(renderLiturgicalBlock("V. Serve the Lord in fear: * and rejoice unto him with reverence."))

	if !strings.Contains(html, `<span class="sigil-text">Serve the Lord in fear: <span class="mediant">*</span> and rejoice unto him with reverence.</span>`) {
		t.Fatalf("expected versicle mediant styled like a psalm verse: %s", html)
	}
}

func TestRenderHymnStanzasPreservesVerseLines(t *testing.T) {
	html := string(renderHymnStanzas("Latin title\n\nFirst verse line,\nSecond verse line.\n\nAnother stanza."))

	if !strings.Contains(html, `<p class="hymn-stanza">First verse line,<br>Second verse line.</p>`) {
		t.Fatalf("expected hymn verse lines to remain hard-wrapped: %s", html)
	}
}

func TestRenderHymnMarksAmenCoda(t *testing.T) {
	html := string(renderHymnStanzas("Title\n\nFirst line,\nSecond line.\n\nAmen."))

	if !strings.Contains(html, `<p class="hymn-stanza hymn-amen">Amen.</p>`) {
		t.Fatalf("expected lone Amen stanza marked as coda: %s", html)
	}
	if !strings.Contains(html, `<p class="hymn-stanza">First line,<br>Second line.</p>`) {
		t.Fatalf("expected ordinary stanzas unmarked: %s", html)
	}
}

func TestIsHymnAmen(t *testing.T) {
	cases := []struct {
		stanza []string
		want   bool
	}{
		{[]string{"Amen."}, true},
		{[]string{"Amen"}, true},
		{[]string{"amen!"}, true},
		{[]string{"AMEN."}, true},
		{[]string{"  Amen.  "}, true},
		{[]string{"Amen.", "Again."}, false}, // multi-line stanza
		{[]string{"Amen, amen."}, false},
		{[]string{"So be it. Amen."}, false},
		{[]string{"First line,"}, false},
		{nil, false},
		{[]string{}, false},
	}
	for _, tt := range cases {
		if got := isHymnAmen(tt.stanza); got != tt.want {
			t.Errorf("isHymnAmen(%q) = %v, want %v", tt.stanza, got, tt.want)
		}
	}
}

func TestRenderHymnDoesNotMarkNonCodaAmen(t *testing.T) {
	// Amen glued to the last verse line is not a coda stanza — that is a data
	// problem fixed in the corpus; the renderer must not invent the class.
	html := string(renderHymnStanzas("Title\n\nLast line ends with Amen."))
	if strings.Contains(html, "hymn-amen") {
		t.Fatalf("inline Amen must not mark the stanza as coda: %s", html)
	}
}

func TestRenderBlessingUsesVersicleLine(t *testing.T) {
	html := string(renderLiturgicalBlock("Blessing. May the Almighty and merciful Lord grant us a quiet night."))

	// The spelled-out label carries sigil-word so CSS can widen its column —
	// and, via :has(), the columns of the ℣./℟. lines beside it. A plain
	// "sigil" here would set it in the narrow ℣./℟. gutter and overrun it.
	if !strings.Contains(html, `<span class="sigil sigil-word">Blessing.</span>`) {
		t.Fatalf("expected Blessing. sigil with the wide-column class: %s", html)
	}
	if !strings.Contains(html, `class="versicle-line"`) {
		t.Fatalf("expected Blessing on a versicle-line for the shared sigil column: %s", html)
	}
	if !strings.Contains(html, `<span class="sigil-text">May the Almighty and merciful Lord grant us a quiet night.</span>`) {
		t.Fatalf("expected blessing body in sigil-text: %s", html)
	}
}
