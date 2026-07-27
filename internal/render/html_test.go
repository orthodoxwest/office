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
		{"Blessed be the Lord God of Israel", "Blessed be the Lord God of Israel"},
		{"That thy way may be known", "That thy way may be known"},
		{"", ""},
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

func TestRenderBlessingUsesVersicleLine(t *testing.T) {
	html := string(renderLiturgicalBlock("Blessing. May the Almighty and merciful Lord grant us a quiet night."))

	if !strings.Contains(html, `<span class="sigil">Blessing.</span>`) {
		t.Fatalf("expected Blessing. sigil: %s", html)
	}
	if !strings.Contains(html, `class="versicle-line"`) {
		t.Fatalf("expected Blessing on a versicle-line for the shared sigil column: %s", html)
	}
}
