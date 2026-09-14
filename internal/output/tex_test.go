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

	if !strings.Contains(got, `\Vsig{}`) {
		t.Error("expected red \\Vsig{} versicle sigil")
	}
	if !strings.Contains(got, `\Rsig{}`) {
		t.Error("expected red \\Rsig{} response sigil")
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
	if !strings.Contains(short, `\shortresponse{T}{he} Lord`) || strings.Count(short, `\Rsig{}`) != 1 {
		t.Fatalf("short responsory must drop only its first response sigil:\n%s", short)
	}
	dialogue := formatLiturgicalBlockTeX("V. Kyrie, eleison.\nR. Christe, eleison.\nAll: Kyrie, eleison.")
	if !strings.Contains(dialogue, `\allsig{}`) {
		t.Fatalf("expected All speaker mark:\n%s", dialogue)
	}
	prayer := texElement(models.OfficeElement{Type: models.CorporateLordPrayer, Voice: []models.VoiceSpan{
		{Text: "Our Father.\nAnd lead us not into temptation,\n", Spoken: true, Role: models.VoiceOfficiant},
		{Text: "But deliver us from evil. Amen.", Spoken: true, Role: models.VoiceResponse},
	}}, "", false)
	if !strings.Contains(prayer, `\dropcap{O}{ur} Father.`) || !strings.Contains(prayer, `\Rsig{}But deliver us from evil. Amen.`) {
		t.Fatalf("expected corporate response with Amen:\n%s", prayer)
	}
}

func TestTeXTypographyParity(t *testing.T) {
	preamble := texPreamble(&models.OfficeHour{})
	for _, want := range []string{
		`\definecolor{ornamentgold}`,            // gilded drop caps
		`\definecolor{mutedgray}`,               // verse numbers, labels, mediants
		`\definecolor{goldline}`,                // title hairlines
		`\newcommand{\crux}{{\color{rubricred}`, // rubric-red crosses
		`\newcommand{\rubric}[1]{{\color{rubricred}\small\normalfont #1}}`,
		`\newcommand{\rubricprayed}[1]{{\color{black}\normalfont #1}}`,
		`\newcommand{\scriptureref}[1]{{\color{rubricred}\small\normalfont #1}}`,
		`\renewcommand{\LettrineTextFont}{\normalfont}`, // gilt initial only, rest prose
		`\newcommand{\dropcap}[2]{\lettrine`,
		`\newcommand{\shortresponse}[2]{\dropcap{#1}{#2}}`,
		`\newcommand{\Vsig}{{\color{rubricred}\Vbar{}}}`,
		`\newcommand{\Rsig}{{\color{rubricred}\Rbar{}}}`,
		`\newcommand{\allsig}{{\color{rubricred}\scshape All:}`,
		`\newcommand{\blessingsig}{{\color{rubricred}Blessing.}`,
		`\newcommand{\canticlesection}[1]{`,
		`\setstretch{1.35}`, // verse leading deeper than prose
		`\newcommand{\gloriapatri}[2]{\medskip\noindent`,
		`\newcommand{\ant}[1]{\noindent\hangindent=1.35em\hangafter=1{\color{rubricred}\scshape Ant.}`,
	} {
		if !strings.Contains(preamble, want) {
			t.Errorf("preamble is missing %q", want)
		}
	}
	if strings.Contains(preamble, `\newcommand{\rubric}[1]{{\color{rubricred}\small\itshape`) {
		t.Error("rubrics must be roman, not italic")
	}
	sectionHeading := preamble[strings.Index(preamble, `\newcommand{\sectionheading}`):strings.Index(preamble, `\newcommand{\canticlesection}`)]
	if !strings.Contains(sectionHeading, `\begin{center}{\scshape #1}\end{center}`) {
		t.Error("section headings must be centered small caps")
	}
	if strings.Contains(sectionHeading, `\small\scshape`) {
		t.Error("section headings must be body-sized rather than small")
	}
	psalmLabel := preamble[strings.Index(preamble, `\newcommand{\psalmlabel}`):strings.Index(preamble, `% Pointing mediant`)]
	if !strings.Contains(psalmLabel, `\begin{center}`) || !strings.Contains(psalmLabel, `\color{mutedgray}`) {
		t.Error("psalm/canticle labels must be centered and muted")
	}
	if strings.Contains(psalmLabel, `{\small\scshape #1}`) || strings.Contains(psalmLabel, `{\small\itshape\enspace`) {
		t.Error("psalm/canticle labels must be body-sized rather than small")
	}
	mediant := preamble[strings.Index(preamble, `\newcommand{\mediant}`):strings.Index(preamble, `% Psalm verses environment`)]
	if !strings.Contains(mediant, `\raisebox{0.25em}`) || !strings.Contains(mediant, `\color{mutedgray}`) {
		t.Error("mediant must be muted and raised")
	}
	verse := preamble[strings.Index(preamble, `\newcommand{\psalmverse}`):strings.Index(preamble, `% Gloria Patri`)]
	if !strings.Contains(verse, `\makebox[1.4em][r]`) || !strings.Contains(verse, `\color{mutedgray}`) {
		t.Error("numbered verses must hang from a muted right-aligned gutter")
	}
	if strings.Contains(verse, `\textbf`) {
		t.Error("verse numbers must not be bold")
	}
	title := texTitleBlock(&models.OfficeHour{})
	for _, want := range []string{
		`{\color{goldline}\rule{\linewidth}{0.6pt}}`,
		`{\color{ornamentgold}\small$\diamond$}`,
		`{\small\color{mutedgray}Season:`,
	} {
		if !strings.Contains(title, want) {
			t.Errorf("title block is missing %q", want)
		}
	}
	if strings.Contains(title, `\hrule`) {
		t.Error("title rules must be gold, not a black \\hrule")
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

func TestTeXHymnRubricIsNotADropCapTitle(t *testing.T) {
	hymn := formatHymnTeX("/:The first stanza of the following hymn is said kneeling.:/\n\nStar of ocean fairest,\nMother, God who barest.", "", "", false)
	if !strings.Contains(hymn, `\rubric{The first stanza of the following hymn is said kneeling.}`) {
		t.Fatalf("expected hymn rubric macro:\n%s", hymn)
	}
	if strings.Contains(hymn, `/:`) || strings.Contains(hymn, `:/`) {
		t.Fatalf("rubric delimiters must not survive into TeX:\n%s", hymn)
	}
	if strings.Contains(hymn, `\dropcap{/}`) || strings.Contains(hymn, `\small\itshape /:`) {
		t.Fatalf("kneeling rubric must not be a Latin title or take the drop cap:\n%s", hymn)
	}
	if !strings.Contains(hymn, `\dropcap{S}{tar} of ocean fairest,`) {
		t.Fatalf("drop cap belongs on the first English stanza:\n%s", hymn)
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
	if !strings.Contains(marian, `\Vsig{}Pray for us.`) || !strings.Contains(marian, `\Rsig{}That we may be worthy.`) {
		t.Fatalf("Marian antiphon's remaining liturgical block must be retained:\n%s", marian)
	}

	short := formatShortResponsoryTeX("Opening prose.\nR. Alpha * beta.\nV. A versicle.\nR. A later response.")
	// Expansion must keep the rest of the response in the same paragraph as
	// the lettrine: \shortresponse has no internal \par, then tail, then one \par.
	if !strings.Contains(short, `\noindent Opening prose.\par`) || !strings.Contains(short, `\shortresponse{A}{lpha}\mediant{}beta.\par`) {
		t.Fatalf("first response must receive the only dropped opening:\n%s", short)
	}
	if strings.Contains(short, `\shortresponse{A}{lpha}\par`) {
		t.Fatalf("shortresponse must not end the paragraph after the first word:\n%s", short)
	}
	if strings.Count(short, `\shortresponse{`) != 1 || !strings.Contains(short, `\Rsig{}A later response.`) {
		t.Fatalf("later short-responsory response must retain its sigil:\n%s", short)
	}
	noResponse := formatShortResponsoryTeX("Opening prose.\nV. A versicle.")
	if strings.Contains(noResponse, `\shortresponse{`) || !strings.Contains(noResponse, `\Vsig{}A versicle.`) {
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
	if !strings.Contains(got, `\Vsig{}`) || !strings.Contains(got, `\Rsig{}`) {
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
	if !strings.Contains(got, `\Rsig{}`) {
		t.Error("expected \\Rsig{} in response")
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

func TestSoftenTeXOpening(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"GOD be merciful unto us,", "God be merciful unto us,"},
		{"HAVE mercy upon me,", "Have mercy upon me,"},
		{"MY SOUL cleaveth to the dust,", "My Soul cleaveth to the dust,"},
		{"O GIVE thanks unto the Lord,", "O Give thanks unto the Lord,"},
		{"O God, thou art my God,", "O God, thou art my God,"},
		{"Almighty God, who hast", "Almighty God, who hast"},
		{"I said, I will take heed", "I said, I will take heed"},
	}
	for _, tt := range tests {
		if got := softenTeXOpening(tt.in); got != tt.want {
			t.Errorf("softenTeXOpening(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatPsalmTeXOpeningDropCap(t *testing.T) {
	psalmText := `Psalm 67

GOD be merciful unto us, and bless us * and shew us the light of his countenance:
2. That thy way may be known upon earth * thy saving health among all nations.
Glory be to the Father, and to the Son, and to the Holy Ghost;
as it was in the beginning, is now, and ever shall be, world without end. Amen.
`
	got := formatPsalmTeX(psalmText, "", "Psalm 67", models.Psalm, false)
	if !strings.Contains(got, `\psalmverse{}{\dropcap{G}{od} be merciful unto us, and bless us\mediant{}and shew us`) {
		t.Errorf("opening verse needs a softened gilt initial:\n%s", got)
	}
	if strings.Count(got, `\dropcap{`) != 1 {
		t.Errorf("only the opening verse takes the initial:\n%s", got)
	}
	if strings.Contains(got, "GOD be merciful") {
		t.Error("ALL-CAPS opening must be softened beside the initial")
	}

	// A psalm whose first verse is numbered has no unnumbered paragraph for
	// the initial, so it stays plain.
	numbered := formatPsalmTeX("2. That thy way may be known * among all nations.\n", "", "Psalm 67", models.Psalm, false)
	if strings.Contains(numbered, `\dropcap{`) {
		t.Errorf("numbered opening must not take the initial:\n%s", numbered)
	}

	// A canticle section break re-arms the opening for the second block.
	sectioned := formatPsalmTeX("Canticle\n\nFirst opening * alpha.\n\n[section: Part II]\n\nSECOND opening * beta.\n", "", "Canticle", models.Canticle, false)
	if strings.Count(sectioned, `\dropcap{`) != 2 {
		t.Errorf("each canticle block needs its own opening initial:\n%s", sectioned)
	}
	if !strings.Contains(sectioned, `\canticlesection{Part II}`) {
		t.Errorf("section break must use the centered muted macro:\n%s", sectioned)
	}
	if !strings.Contains(sectioned, `\dropcap{S}{econd} opening`) {
		t.Errorf("second block opening must be softened:\n%s", sectioned)
	}
}

func TestTeXAntiphonRendersPointedMediant(t *testing.T) {
	got := texElement(models.OfficeElement{
		Type: models.Antiphon,
		Text: "Wash me throughly, * O Lord, from my wickedness.",
	}, "", false)
	if !strings.Contains(got, `\mediant{}`) {
		t.Fatalf("antiphon pointing must use the mediant mark, not a literal asterisk:\n%s", got)
	}
	if strings.Contains(got, " * ") {
		t.Fatalf("literal asterisk must not survive in the antiphon:\n%s", got)
	}
	if !strings.HasPrefix(strings.TrimSpace(got), `\ant{`) {
		t.Fatalf("antiphon must use the hanging red-sigil macro:\n%s", got)
	}
	// A trailing mediant (first half of a pair continued on the next source
	// line, as in the Ave Regina opening) keeps the mark with no second half.
	if got := texMediantLine("Queen of the heavens, we hail thee *"); got != `Queen of the heavens, we hail thee\mediant{}` {
		t.Fatalf("trailing mediant = %q", got)
	}
}

func TestTeXChapterTakesCollectInitial(t *testing.T) {
	chapter := texElement(models.OfficeElement{Type: models.Chapter, Text: "Brethren, the night is far spent."}, "", false)
	if !strings.Contains(chapter, `\dropcap{B}{rethren,} the night is far spent.`) {
		t.Fatalf("chapter opening needs the spoken initial:\n%s", chapter)
	}
	collect := texElement(models.OfficeElement{Type: models.Collect, Text: "Brethren, the night is far spent."}, "", false)
	if collect != chapter {
		t.Fatalf("chapter and collect openings must match:\n%s\n%s", chapter, collect)
	}
}

func TestTeXFlowingProseRendersMediant(t *testing.T) {
	got := formatLiturgicalBlockTeX("Glory be to the Father, and to the Son, * and to the Holy Ghost;\nas it was in the beginning, is now, and ever shall be, * world without end. Amen.")
	if !strings.Contains(got, `\mediant{}`) {
		t.Fatalf("pointed prose must use the mediant mark:\n%s", got)
	}
	if strings.Contains(got, " * ") {
		t.Fatalf("literal asterisk must not survive in flowing prose:\n%s", got)
	}
}

func TestTeXHymnTitlesAreCentered(t *testing.T) {
	got := formatHymnTeX("Aeterne rerum conditor\n\nO Framer of the earth and sky.", "", "", false)
	if !strings.Contains(got, `\begin{center}{\small\itshape Aeterne rerum conditor}\end{center}`) {
		t.Fatalf("hymn Latin title must be centered:\n%s", got)
	}
	if !strings.Contains(got, `\begin{hymnverses}`) || !strings.Contains(got, `\end{hymnverses}`) {
		t.Fatalf("hymn stanzas must sit in the verse-leading environment:\n%s", got)
	}
}

func TestTeXPrayerSpeakerLabelsAndAmen(t *testing.T) {
	elem := models.OfficeElement{Type: models.Prayer, Text: "Have mercy upon thee.\nR. Amen.", Voice: []models.VoiceSpan{
		{Text: "Have mercy upon thee.\n", Spoken: true, Role: models.VoiceResponse},
		{Text: "R. Amen.", Spoken: true, Role: models.VoicePriest},
	}}
	got := texElement(elem, "", false)
	for _, want := range []string{`\rubric{\scshape People}\par\nopagebreak`, `\rubric{\scshape Priest}\par\nopagebreak`, `\Rsig{}Amen.`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
	if strings.Index(got, "Priest}") > strings.Index(got, "Amen.") {
		t.Errorf("Amen assigned before speaker: %s", got)
	}
}
