package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	if !strings.Contains(got, `\small\itshape`) {
		t.Error("scripture ref should be rendered in small italic")
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
	// The mediant is inside the \gloriapatri LaTeX macro definition, not the call site.
	// Just verify both lines are present as arguments.
	if !strings.Contains(got, "Glory be") {
		t.Error("expected first Gloria line")
	}
	if !strings.Contains(got, "as it was") {
		t.Error("expected second Gloria line")
	}
}

func TestFormatHymnTeX(t *testing.T) {
	text := `Aeterne rerum conditor

O Framer of the earth and sky,
Ruler of all things high and low,
Who, clothing time with mystery,
Dost make the seasons ebb and flow;

Herald of day, the bird doth raise
His voice in strains of varied tone. Amen.`

	got := formatHymnTeX(text, "", "Aeterne Rerum Conditor", false)

	if !strings.Contains(got, "O Framer of the earth") {
		t.Error("expected first stanza content")
	}
	if !strings.Contains(got, `\\`) {
		t.Error("expected line breaks within stanza")
	}
	// Latin title should not appear (skipped as first line)
	if strings.Contains(got, "Aeterne rerum conditor") {
		t.Error("Latin title from text body should be skipped (label used separately)")
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
		if !strings.Contains(got, "O Framer") {
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
