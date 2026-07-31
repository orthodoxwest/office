package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/orthodoxwest/office/internal/models"
)

func TestEscapeTeX(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`hello`, `hello`},
		{`a & b`, `a \& b`},
		{`100%`, `100\%`},
		{`$10`, `\$10`},
		{`#1`, `\#1`},
		{`a_b`, `a\_b`},
		{`{x}`, `\{x\}`},
		{`~tilde`, `\textasciitilde{}tilde`},
		{`a^b`, `a\textasciicircum{}b`},
		{`back\slash`, `back\textbackslash{}slash`},
		// Multiple specials
		{`a & b % c`, `a \& b \% c`},
	}
	for _, tt := range tests {
		got := escapeTeX(tt.in)
		if got != tt.want {
			t.Errorf("escapeTeX(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTexLine(t *testing.T) {
	// Cross replacement after escaping
	got := texLine("May the Lord ✠ bless us.")
	want := `May the Lord \crux{} bless us.`
	if got != want {
		t.Errorf("texLine cross: got %q, want %q", got, want)
	}

	// Both cross and ampersand
	got = texLine("God & ✠")
	want = `God \& \crux{}`
	if got != want {
		t.Errorf("texLine cross+amp: got %q, want %q", got, want)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Benedictus", "benedictus"},
		{"Nunc Dimittis", "nunc-dimittis"},
		{"EB Garamond", "eb-garamond"},
		{"hello-world", "hello-world"},
		{"  spaces  ", "spaces"},
	}
	for _, tt := range tests {
		got := slugify(tt.in)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLabelToSlug(t *testing.T) {
	tests := []struct {
		label    string
		elemType models.ElementType
		want     string
	}{
		{"Psalm 67", models.Psalm, "067"},
		{"Psalm 4", models.Psalm, "004"},
		{"Psalm 118", models.Psalm, "118"},
		{"Psalm 118 (Aleph)", models.Psalm, "118"},
		{"Benedictus", models.Canticle, "benedictus"},
		{"Magnificat", models.Canticle, "magnificat"},
		{"Nunc Dimittis", models.Canticle, "nunc-dimittis"},
		{"Aeterne Rerum Conditor", models.Hymn, "aeterne-rerum-conditor"},
	}
	for _, tt := range tests {
		got := labelToSlug(tt.label, tt.elemType)
		if got != tt.want {
			t.Errorf("labelToSlug(%q, %v) = %q, want %q", tt.label, tt.elemType, got, tt.want)
		}
	}
}

func TestFormatPsalmTeX(t *testing.T) {
	psalmText := `Psalm 67

GOD be merciful unto us, and bless us * and shew us the light of his countenance, and be merciful unto us:
2. That thy way may be known upon earth * thy saving health among all nations.
3. Let the people praise thee, O God * yea, let all the people praise thee.
Glory be to the Father, and to the Son, and to the Holy Ghost;
as it was in the beginning, is now, and ever shall be, world without end. Amen.
`

	got := formatPsalmTeX(psalmText, "", "Psalm 67", models.Psalm, false)

	if !strings.Contains(got, `\begin{psalmverses}`) {
		t.Error("expected psalmverses environment")
	}
	if !strings.Contains(got, `\psalmverse{}`) {
		t.Error("expected unnumbered first verse")
	}
	if !strings.Contains(got, `\psalmverse{2}`) {
		t.Error("expected numbered verse 2")
	}
	if !strings.Contains(got, `\mediant{}`) {
		t.Error("expected mediant marker")
	}
	if !strings.Contains(got, `\gloriapatri{`) {
		t.Error("expected gloriapatri command")
	}
	if !strings.Contains(got, `\end{psalmverses}`) {
		t.Error("expected end of psalmverses environment")
	}
}

func TestFormatLiturgicalBlockTeX(t *testing.T) {
	text := `V. O God, ✠ make speed to save us.
R. O Lord, make haste to help us.

Glory be to the Father, and to the Son, and to the Holy Ghost;
as it was in the beginning, is now, and ever shall be, world without end. Amen.`

	got := formatLiturgicalBlockTeX(text)

	if !strings.Contains(got, `\Vbar{}`) {
		t.Error("expected \\Vbar{}")
	}
	if !strings.Contains(got, `\Rbar{}`) {
		t.Error("expected \\Rbar{}")
	}
	if !strings.Contains(got, `\crux{}`) {
		t.Error("expected \\crux{} for cross character")
	}
}

func TestSemanticOfficeElementsTeX(t *testing.T) {
	acclamation := texElement(models.OfficeElement{Type: models.OpeningAcclamation, Text: "Alleluia"}, "", false)
	if !strings.Contains(acclamation, `\acclamation{Alleluia}`) || strings.Contains(acclamation, `\ant{`) {
		t.Fatalf("opening acclamation must not use antiphon macro:\n%s", acclamation)
	}
	short := texElement(models.OfficeElement{Type: models.ShortResponsory, Text: "R. The Lord hath set his love upon me.\nV. He shall deliver me.\nR. The Lord hath set his love upon me."}, "", false)
	if !strings.Contains(short, `\shortresponse{T}{he} Lord`) || strings.Count(short, `\Rbar{}`) != 1 {
		t.Fatalf("short responsory must drop only its first response sigil:\n%s", short)
	}
	dialogue := formatLiturgicalBlockTeX("V. Kyrie, eleison.\nR. Christe, eleison.\nAll: Kyrie, eleison.")
	if !strings.Contains(dialogue, `\textsc{All:}`) {
		t.Fatalf("expected All speaker mark:\n%s", dialogue)
	}
	prayer := texElement(models.OfficeElement{Type: models.CorporateLordPrayer, Voice: []models.VoiceSpan{
		{Text: "Our Father.\nAnd lead us not into temptation,\n", Spoken: true, Role: models.VoiceOfficiant},
		{Text: "But deliver us from evil. Amen.", Spoken: true, Role: models.VoiceResponse},
	}}, "", false)
	if !strings.Contains(prayer, `\dropcap{O}{ur} Father.`) || !strings.Contains(prayer, `\Rbar{}But deliver us from evil. Amen.`) {
		t.Fatalf("expected corporate response with Amen:\n%s", prayer)
	}
}

func TestTeXTypographyParity(t *testing.T) {
	preamble := texPreamble(&models.OfficeHour{})
	for _, want := range []string{
		`\definecolor{ornamentgold}`,            // gilded drop caps
		`\newcommand{\crux}{{\color{rubricred}`, // rubric-red crosses
		`\newcommand{\rubric}[1]{{\color{rubricred}\small\normalfont #1}}`,
		`\newcommand{\rubricprayed}[1]{{\color{black}\normalfont #1}}`,
		`\newcommand{\scriptureref}[1]{{\color{rubricred}\small\normalfont #1}}`,
		`\newcommand{\dropcap}[2]{\lettrine`,
		`\newcommand{\shortresponse}[2]{\dropcap{#1}{#2}\par}`,
	} {
		if !strings.Contains(preamble, want) {
			t.Errorf("preamble is missing %q", want)
		}
	}
	if strings.Contains(preamble, `\newcommand{\rubric}[1]{{\color{rubricred}\small\itshape`) {
		t.Error("rubrics must be roman, not italic")
	}
	psalmLabel := preamble[strings.Index(preamble, `\newcommand{\psalmlabel}`):strings.Index(preamble, `% Psalm mediant marker`)]
	if strings.Contains(psalmLabel, `{\small\scshape #1}`) || strings.Contains(psalmLabel, `{\small\itshape\enspace`) {
		t.Error("psalm/canticle labels must be body-sized rather than small")
	}

	rubric := formatRubricTeX(models.OfficeElement{
		Type:        models.Rubric,
		Text:        "Our Father is said secretly.",
		RubricSpans: []models.RubricSpan{{Text: "Our Father", Prayed: true}, {Text: " is said secretly."}},
	})
	if !strings.Contains(rubric, `\rubric{\rubricprayed{Our Father} is said secretly.}`) {
		t.Fatalf("quoted prayer must remain black/roman inside a rubric:\n%s", rubric)
	}

	for _, got := range []string{
		formatPsalmTeX("!Luke 1:68-79\n\nBlessed be the Lord.", "", "Psalm 1", models.Psalm, false),
		formatLiturgicalBlockTeX("!Rom. 13:12-13\nThe night is far spent."),
		formatShortResponsoryTeX("!Luke 1:68-79\nR. Blessed be the Lord."),
	} {
		if !strings.Contains(got, `\scriptureref{`) || strings.Contains(got, `\small\itshape`) {
			t.Fatalf("scripture reference must be small red roman:\n%s", got)
		}
	}
}

func TestTeXDropCapsAreSemanticAndExcludeSecretPrayer(t *testing.T) {
	hymn := formatHymnTeX("Latin title\n\nO Framer of the earth and sky,\nRuler of all things high and low.", "", "", false)
	if !strings.Contains(hymn, `\dropcap{O}{} Framer of the earth and sky,`) {
		t.Fatalf("first English hymn stanza needs a drop cap:\n%s", hymn)
	}
	collect := texElement(models.OfficeElement{Type: models.Collect, Text: "Almighty God, who hast brought us to the beginning of this day."}, "", false)
	if !strings.Contains(collect, `\dropcap{A}{lmighty} God`) {
		t.Fatalf("collect needs a drop cap:\n%s", collect)
	}
	marian := formatMultilineAntiphonTeX(models.OfficeElement{Type: models.Antiphon, Text: "Hail, holy Queen, Mother of mercy,\nour life and our hope."})
	if !strings.Contains(marian, `\dropcap{H}{ail,} holy Queen`) {
		t.Fatalf("Marian antiphon needs a drop cap:\n%s", marian)
	}
	corporate := formatCorporateLordPrayerTeX(models.OfficeElement{Type: models.CorporateLordPrayer, Voice: []models.VoiceSpan{
		{Text: "Our Father, who art in heaven.", Role: models.VoiceOfficiant},
		{Text: "But deliver us from evil. Amen.", Role: models.VoiceResponse},
	}})
	if !strings.Contains(corporate, `\dropcap{O}{ur} Father`) {
		t.Fatalf("corporate Lord's Prayer officiant needs a drop cap:\n%s", corporate)
	}
	secret := texElement(models.OfficeElement{Type: models.Prayer, Text: "Our Father, who art in heaven."}, "", false)
	if strings.Contains(secret, `\dropcap`) {
		t.Fatalf("private/secret prayer must not get a drop cap:\n%s", secret)
	}
	fallback := formatCorporateLordPrayerTeX(models.OfficeElement{Type: models.CorporateLordPrayer, Text: "Our Father, who art in heaven."})
	if strings.Contains(fallback, `\dropcap`) {
		t.Fatalf("invalid corporate prayer must not get a drop cap:\n%s", fallback)
	}
}

func TestTeXDropCapSplittingKeepsOnlyTheFirstWordBoxed(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		text                              string
		initial, firstWordRest, tail, tex string
	}{
		{"empty", "", "", "", "", ""},
		{"single rune", "O", "O", "", "", `\dropcap{O}{}`},
		{"ordinary word", "Almighty God", "A", "lmighty", " God", `\dropcap{A}{lmighty} God`},
		{"standalone initial", "O God", "O", "", " God", `\dropcap{O}{} God`},
		{"unicode whitespace", "O\u00a0God", "O", "", "\u00a0God", `\dropcap{O}{} God`},
		{"mediant tail", "God * save us", "G", "od", " * save us", `\dropcap{G}{od}\mediant{}save us`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial, firstWordRest, tail := splitDropCap(tc.text)
			if initial != tc.initial || firstWordRest != tc.firstWordRest || tail != tc.tail {
				t.Fatalf("splitDropCap(%q) = (%q, %q, %q), want (%q, %q, %q)", tc.text, initial, firstWordRest, tail, tc.initial, tc.firstWordRest, tc.tail)
			}
			if got := texDropCap(tc.text); got != tc.tex {
				t.Fatalf("texDropCap(%q) = %q, want %q", tc.text, got, tc.tex)
			}
		})
	}

	invalid := string([]byte{0xff, 'x'})
	initial, rest := splitInitial(invalid)
	if initial != string(utf8.RuneError) || rest != "x" {
		t.Fatalf("splitInitial malformed UTF-8 = (%q, %q), want RuneError then x", initial, rest)
	}
}

func TestTeXOpeningDropCapsAreLimitedToTheFirstHymnStanza(t *testing.T) {
	hymn := formatHymnTeX("Latin title\n\nFirst hymn stanza.\n\nSecond hymn stanza.", "", "", false)
	if strings.Count(hymn, `\dropcap{`) != 1 || !strings.Contains(hymn, `\dropcap{F}{irst} hymn stanza.`) {
		t.Fatalf("first hymn stanza must be the only drop-cap stanza:\n%s", hymn)
	}
	if !strings.Contains(hymn, `\noindent Second hymn stanza.`) || strings.Contains(hymn, `\dropcap{S}{econd}`) {
		t.Fatalf("later hymn stanza must remain ordinary text:\n%s", hymn)
	}
	if strings.Count(hymn, `\noindent `) != 1 || strings.Contains(hymn, `\noindent \dropcap{F}{irst}`) {
		t.Fatalf("only later hymn stanzas should be marked no-indent:\n%s", hymn)
	}
}

func TestFormatMultilineAntiphonTeXAnthemLineBoundaries(t *testing.T) {
	if got := formatMultilineAntiphonTeX(models.OfficeElement{}); got != "\n" {
		t.Fatalf("empty multiline antiphon = %q, want newline", got)
	}
	if got := formatMultilineAntiphonTeX(models.OfficeElement{Text: "Hail"}); got != "{\\itshape \\dropcap{H}{ail}}\\par\n\n" {
		t.Fatalf("one-line multiline antiphon = %q", got)
	}
}

func TestTeXMarianAndShortResponsoryOpeningBranches(t *testing.T) {
	marian := formatMultilineAntiphonTeX(models.OfficeElement{Type: models.Antiphon, Text: "Hail, holy Queen.\nOur life and sweetness.\n\nV. Pray for us.\nR. That we may be worthy."})
	if strings.Count(marian, `\dropcap{`) != 1 || !strings.Contains(marian, `\dropcap{H}{ail,} holy Queen. Our life and sweetness.`) {
		t.Fatalf("only the Marian anthem opening should be dropped:\n%s", marian)
	}
	if !strings.Contains(marian, `\Vbar{}Pray for us.`) || !strings.Contains(marian, `\Rbar{}That we may be worthy.`) {
		t.Fatalf("Marian antiphon's remaining liturgical block must be retained:\n%s", marian)
	}

	short := formatShortResponsoryTeX("Opening prose.\nR. Alpha * beta.\nV. A versicle.\nR. A later response.")
	if !strings.Contains(short, `\noindent Opening prose.\par`) || !strings.Contains(short, `\shortresponse{A}{lpha}\mediant{}beta.\par`) {
		t.Fatalf("first response must receive the only dropped opening:\n%s", short)
	}
	if strings.Count(short, `\shortresponse{`) != 1 || !strings.Contains(short, `\Rbar{}A later response.`) {
		t.Fatalf("later short-responsory response must retain its sigil:\n%s", short)
	}
	noResponse := formatShortResponsoryTeX("Opening prose.\nV. A versicle.")
	if strings.Contains(noResponse, `\shortresponse{`) || !strings.Contains(noResponse, `\Vbar{}A versicle.`) {
		t.Fatalf("a block without a response must not invent a dropped initial:\n%s", noResponse)
	}
}

func TestFormatLiturgicalBlockTeXFlowsProse(t *testing.T) {
	text := `Visit, we beseech thee, O Lord, this habitation, and drive far from it all snares of the enemy:
let thy holy Angels dwell herein to preserve us in peace, and let thy blessing be ever upon us.

Through Jesus Christ thy son, our LORD.`

	got := formatLiturgicalBlockTeX(text)

	if strings.Contains(got, `\\`) {
		t.Errorf("hard-wrapped prose should flow, not force line breaks:\n%s", got)
	}
	if !strings.Contains(got, "enemy: let thy holy Angels") {
		t.Errorf("consecutive prose lines should be joined with a space:\n%s", got)
	}
	if strings.Count(got, `\noindent`) != 2 {
		t.Errorf("blank line should start a new paragraph:\n%s", got)
	}
}

func TestFormatMultilineAntiphonTeX(t *testing.T) {
	elem := models.OfficeElement{
		Type:  models.Antiphon,
		Label: "Salve Regina",
		Text: `Hail, holy Queen, Mother of mercy,
our life, our sweetness, and our hope.

V. Pray for us, O holy Mother of God.
R. That we may be made worthy of the promises of Christ.

Let us pray.

Almighty, everlasting God, grant unto us thy servants health of mind and body.`,
	}

	got := formatMultilineAntiphonTeX(elem)

	if !strings.Contains(got, `\small\itshape Salve Regina`) {
		t.Error("expected label heading")
	}
	if !strings.Contains(got, `{\itshape \dropcap{H}{ail,} holy Queen, Mother of mercy, our life, our sweetness, and our hope.}`) {
		t.Errorf("anthem paragraph should be italic and flowed:\n%s", got)
	}
	if !strings.Contains(got, `\Vbar{}`) || !strings.Contains(got, `\Rbar{}`) {
		t.Error("versicle/response after the anthem should use liturgical block markup")
	}
	if !strings.Contains(got, "Almighty, everlasting God") {
		t.Error("collect after the anthem should be rendered")
	}
}

func TestFormatMultilineAntiphonTeXAnthemOnly(t *testing.T) {
	elem := models.OfficeElement{
		Type: models.Antiphon,
		Text: "Line one of the anthem,\nline two of the anthem.",
	}

	got := formatMultilineAntiphonTeX(elem)

	if !strings.Contains(got, `{\itshape \dropcap{L}{ine} one of the anthem, line two of the anthem.}`) {
		t.Errorf("anthem-only antiphon should be one italic paragraph:\n%s", got)
	}
}

func TestFormatLiturgicalBlockTeXBracketTitle(t *testing.T) {
	text := `[Ave Regina Caelorum]
Hail, O Queen of Heaven enthroned.`

	got := formatLiturgicalBlockTeX(text)

	if strings.Contains(got, "Ave Regina") {
		t.Error("bracket title should be silently skipped")
	}
	if !strings.Contains(got, "Hail") {
		t.Error("regular text should be present")
	}
}

func TestFormatLiturgicalBlockTeXScriptureRef(t *testing.T) {
	text := `!Rom. 13:12-13
The night is far spent, the day is at hand.
R. Thanks be to God.`

	got := formatLiturgicalBlockTeX(text)

	if !strings.Contains(got, `\scriptureref{`) {
		t.Error("scripture ref should be rendered in small rubric-red roman")
	}
	if !strings.Contains(got, "Rom. 13:12{-}13") || !strings.Contains(got, "Rom.") {
		// The colon in scripture ref is not a LaTeX special, just check presence
		if !strings.Contains(got, "Rom.") {
			t.Error("scripture ref content should be present")
		}
	}
	if !strings.Contains(got, `\Rbar{}`) {
		t.Error("expected \\Rbar{} in response")
	}
}

func TestFormatGloriaPatriTeX(t *testing.T) {
	text := "Glory be to the Father, and to the Son, and to the Holy Ghost;\nas it was in the beginning, is now, and ever shall be, world without end. Amen."

	got := formatGloriaPatriTeX(text)

	if !strings.Contains(got, `\gloriapatri{`) {
		t.Error("expected gloriapatri command")
	}
	// The line break is inside the \gloriapatri LaTeX macro definition, not the call site.
	// Just verify both lines are present as arguments.
	if !strings.Contains(got, "Glory be") {
		t.Error("expected first Gloria line")
	}
	if !strings.Contains(got, "as it was") {
		t.Error("expected second Gloria line")
	}
}

func TestFormatHymnTeX(t *testing.T) {
	// A composed hymn arrives with its Latin incipit already in the label and
	// its body starting at the first stanza (the engine peels it). The opening
	// stanza must survive: peeling a second time used to drop it.
	body := `To thee, before the close of day,
Creator of the world, we pray
That with thy wonted favour, thou
Wouldst be our guard and keeper now.

From all ill dreams defend our eyes,
From nightly fears and fantasies. Amen.`

	got := formatHymnTeX(body, "", "Te Lucis Ante Terminum", false)

	if !strings.Contains(got, `\dropcap{T}{o} thee, before the close of day,`) {
		t.Errorf("opening stanza was dropped: %s", got)
	}
	if !strings.Contains(got, "From all ill dreams defend our eyes") {
		t.Error("expected the remaining stanza")
	}
	if strings.Count(got, `\par\smallskip`) != 2 {
		t.Errorf("expected one paragraph per stanza: %s", got)
	}
	if !strings.Contains(got, `\\`) {
		t.Error("expected line breaks within stanza")
	}
	if !strings.Contains(got, `\dropcap{T}{o} thee, before the close of day,`) {
		t.Errorf("first hymn stanza needs a drop cap: %s", got)
	}
}

func TestFormatHymnTeXRendersTitleStillInTheBody(t *testing.T) {
	// Text that has not been through the engine keeps its incipit; render it
	// rather than silently dropping it, as the HTML renderer does.
	text := `Aeterne rerum conditor

O Framer of the earth and sky,
Ruler of all things high and low.`

	got := formatHymnTeX(text, "", "", false)

	if !strings.Contains(got, "Aeterne rerum conditor") {
		t.Errorf("Latin incipit should be rendered: %s", got)
	}
	if !strings.Contains(got, `\dropcap{O}{} Framer of the earth`) {
		t.Error("expected stanza content")
	}
}

// makeChantDir creates a temp dataDir containing a .gabc file at the
// expected path for the given category and slug, returning the dataDir.
func makeChantDir(t *testing.T, category, slug string) string {
	t.Helper()
	dir := t.TempDir()
	gabcDir := filepath.Join(dir, "texts", "chant", category)
	if err := os.MkdirAll(gabcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gabcDir, slug+".gabc"), []byte("name: test;\n%%\n(c4)(g)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFormatPsalmTeXWithChant(t *testing.T) {
	const psalmText = "Psalm 67\n\nGOD be merciful * and bless us:\n2. That thy way * may be known.\nGlory be to the Father;\nas it was in the beginning.\n"

	t.Run("chant=true with gabc file emits gregorioscore", func(t *testing.T) {
		dataDir := makeChantDir(t, "psalms", "067")
		got := formatPsalmTeX(psalmText, dataDir, "Psalm 67", models.Psalm, true)
		if !strings.Contains(got, `\gregorioscore{`) {
			t.Error("expected \\gregorioscore when chant=true and .gabc exists")
		}
		if strings.Contains(got, `\psalmverse`) {
			t.Error("should not emit text verses when using score")
		}
		if strings.Contains(got, ".gabc") {
			t.Error("\\gregorioscore path must not include .gabc extension")
		}
	})

	t.Run("chant=false with gabc file falls through to text", func(t *testing.T) {
		dataDir := makeChantDir(t, "psalms", "067")
		got := formatPsalmTeX(psalmText, dataDir, "Psalm 67", models.Psalm, false)
		if strings.Contains(got, `\gregorioscore`) {
			t.Error("should not emit \\gregorioscore when chant=false")
		}
		if !strings.Contains(got, `\psalmverse`) {
			t.Error("expected text verses when chant=false")
		}
	})

	t.Run("chant=true without gabc file falls through to text", func(t *testing.T) {
		got := formatPsalmTeX(psalmText, t.TempDir(), "Psalm 67", models.Psalm, true)
		if strings.Contains(got, `\gregorioscore`) {
			t.Error("should not emit \\gregorioscore when no .gabc file exists")
		}
		if !strings.Contains(got, `\psalmverse`) {
			t.Error("expected text verses when no .gabc file exists")
		}
	})
}

func TestFormatHymnTeXWithChant(t *testing.T) {
	const hymnText = "Aeterne rerum conditor\n\nO Framer of the earth and sky,\nRuler of all things high and low.\n"

	t.Run("chant=true with gabc file emits gregorioscore", func(t *testing.T) {
		dataDir := makeChantDir(t, "hymns", "aeterne-rerum-conditor")
		got := formatHymnTeX(hymnText, dataDir, "Aeterne Rerum Conditor", true)
		if !strings.Contains(got, `\gregorioscore{`) {
			t.Error("expected \\gregorioscore when chant=true and .gabc exists")
		}
		if strings.Contains(got, ".gabc") {
			t.Error("\\gregorioscore path must not include .gabc extension")
		}
	})

	t.Run("chant=false with gabc file falls through to text", func(t *testing.T) {
		dataDir := makeChantDir(t, "hymns", "aeterne-rerum-conditor")
		got := formatHymnTeX(hymnText, dataDir, "Aeterne Rerum Conditor", false)
		if strings.Contains(got, `\gregorioscore`) {
			t.Error("should not emit \\gregorioscore when chant=false")
		}
		if !strings.Contains(got, `\dropcap{O}{} Framer`) {
			t.Error("expected hymn text when chant=false")
		}
	})
}

func TestFormatOfficeHourTeXSmoke(t *testing.T) {
	hour := &models.OfficeHour{
		Hour:   "Lauds",
		Title:  "Lauds",
		Season: models.Season("Lent"),
		Color:  models.Color("violet"),
		Sections: []models.OfficeSection{
			{
				Label: "Psalmody",
				Elements: []models.OfficeElement{
					{
						Type:  models.Psalm,
						Label: "Psalm 67",
						Text:  "Psalm 67\n\nGOD be merciful * and bless us:\n2. That thy way * may be known.\nGlory be to the Father, and to the Son, and to the Holy Ghost;\nas it was in the beginning, is now, and ever shall be, world without end. Amen.\n",
					},
				},
			},
		},
	}

	got := FormatOfficeHourTeX(hour, "", false)

	if !strings.HasPrefix(got, "% Auto-generated") {
		t.Error("expected auto-generated comment at start")
	}
	if !strings.Contains(got, `\documentclass`) {
		t.Error("expected documentclass")
	}
	if !strings.Contains(got, `\begin{document}`) {
		t.Error("expected begin document")
	}
	if !strings.Contains(got, `\end{document}`) {
		t.Error("expected end document")
	}
	if !strings.Contains(got, "Lauds") {
		t.Error("expected hour name in title")
	}
	if !strings.Contains(got, `\psalmverse`) {
		t.Error("expected psalm verses")
	}
}

func TestTexAnnouncedAntiphonUsesDisplayText(t *testing.T) {
	got := texElement(models.OfficeElement{
		Type:     models.Antiphon,
		Text:     "Do away, O Lord, * mine offenses.",
		Announce: true,
	}, "", false)
	if !strings.Contains(got, `Do away, O Lord.`) {
		t.Fatalf("expected announced form in TeX:\n%s", got)
	}
	if strings.Contains(got, "mine offenses") {
		t.Fatalf("announced TeX antiphon must not include the rest:\n%s", got)
	}
}

func TestTexEmptyAntiphonEmitsNothing(t *testing.T) {
	got := texElement(models.OfficeElement{Type: models.Antiphon, Text: ""}, "", false)
	if got != "" {
		t.Fatalf("empty antiphon should emit no TeX, got %q", got)
	}
}
