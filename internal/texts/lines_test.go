package texts

import "testing"

func TestSplitLeadingVerseNumber(t *testing.T) {
	tests := []struct {
		line     string
		wantNum  string
		wantRest string
		wantOK   bool
	}{
		{"2. That thy way may be known", "2", "That thy way may be known", true},
		{"10. Make me a clean heart", "10", "Make me a clean heart", true},
		{"2 O ye Angels of the Lord", "2", "O ye Angels of the Lord", true},
		{"20 Blessed art thou, O Lord", "20", "Blessed art thou, O Lord", true},
		{"O ALL ye Works of the Lord", "", "O ALL ye Works of the Lord", false},
		{"2.No space after period", "", "2.No space after period", false},
	}
	for _, tt := range tests {
		num, rest, ok := splitLeadingVerseNumber(tt.line)
		if ok != tt.wantOK || num != tt.wantNum || rest != tt.wantRest {
			t.Errorf("splitLeadingVerseNumber(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.line, num, rest, ok, tt.wantNum, tt.wantRest, tt.wantOK)
		}
	}
}

func TestParsePsalmTitleBlockAndVerses(t *testing.T) {
	psalm := ParsePsalm("Psalm 67\n!Deus misereatur\n\n" +
		"1. God be merciful unto us * and bless us.\n" +
		"2 O ye Angels of the Lord, bless ye the Lord.\n" +
		"An unnumbered line * with a mediant.\n" +
		"Glory be to the Father, and to the Son,\n" +
		"as it was in the beginning, is now, and ever shall be.\n")

	if psalm.ScriptureRef != "Deus misereatur" {
		t.Errorf("ScriptureRef = %q", psalm.ScriptureRef)
	}
	want := []PsalmItem{
		{Kind: PsalmVerse, Number: "1", First: "God be merciful unto us", Second: "and bless us."},
		{Kind: PsalmVerse, Number: "2", First: "O ye Angels of the Lord, bless ye the Lord."},
		{Kind: PsalmVerse, First: "An unnumbered line", Second: "with a mediant."},
		{Kind: PsalmGloria,
			First:  "Glory be to the Father, and to the Son,",
			Second: "as it was in the beginning, is now, and ever shall be."},
	}
	if len(psalm.Items) != len(want) {
		t.Fatalf("Items = %#v, want %d", psalm.Items, len(want))
	}
	for i, item := range psalm.Items {
		if item != want[i] {
			t.Errorf("item %d = %#v, want %#v", i, item, want[i])
		}
	}
}

func TestParsePsalmCanticleSections(t *testing.T) {
	psalm := ParsePsalm("Song of the Three Children\n\n" +
		"1. O all ye Works of the Lord.\n" +
		"[section: The Second Part]\n" +
		"2. O ye Angels of the Lord.\n")

	if psalm.ScriptureRef != "" {
		t.Errorf("ScriptureRef = %q, want empty", psalm.ScriptureRef)
	}
	if len(psalm.Items) != 3 {
		t.Fatalf("Items = %#v, want 3", psalm.Items)
	}
	if got := psalm.Items[1]; got.Kind != PsalmSection || got.Heading != "The Second Part" {
		t.Errorf("section item = %#v", got)
	}
}

func TestParsePsalmDanglingGloriaStaysInPlace(t *testing.T) {
	// A "Glory be" with no closing line must not swallow what follows.
	psalm := ParsePsalm("Psalm 1\n\nGlory be to the Father,\n1. Blessed is the man.\n")

	if len(psalm.Items) != 2 {
		t.Fatalf("Items = %#v, want 2", psalm.Items)
	}
	if psalm.Items[0].Kind != PsalmGloria || psalm.Items[0].Second != "" {
		t.Errorf("gloria = %#v, want an unclosed gloria", psalm.Items[0])
	}
	if psalm.Items[1].Kind != PsalmVerse || psalm.Items[1].Number != "1" {
		t.Errorf("verse after gloria = %#v", psalm.Items[1])
	}
}

func TestParseBlockClassifiesLines(t *testing.T) {
	block := ParseBlock("[Ad Laudes]\n!Romans 13\n" +
		"Prose one,\nprose two.\n\n" +
		"V. O Lord, hear my prayer.\n" +
		"R. And let my cry come unto thee.\n" +
		"Blessing. May the Lord bless us.\n")

	var got []BlockKind
	for _, line := range block {
		got = append(got, line.Kind)
	}
	want := []BlockKind{
		BlockScriptureRef, BlockProse, BlockProse, BlockGap,
		BlockVersicle, BlockResponse, BlockBlessing, BlockGap,
	}
	if len(got) != len(want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", got, want)
		}
	}

	if block[0].Text != "Romans 13" {
		t.Errorf("scripture ref text = %q", block[0].Text)
	}
	if block[4].Text != "O Lord, hear my prayer." {
		t.Errorf("versicle text = %q (sigil should be stripped)", block[4].Text)
	}
	if block[6].Text != "May the Lord bless us." {
		t.Errorf("blessing text = %q", block[6].Text)
	}
}

func TestParseBlockOffsetsPointAtTheirText(t *testing.T) {
	// Voiced prayers map spoken/silent spans onto the source by byte offset,
	// so every parsed line must be locatable in the original string.
	source := "  Let us pray.\n\n\tV. O Lord, hear my prayer.\n!Psalm 102\n"
	for _, line := range ParseBlock(source) {
		if line.Kind == BlockGap {
			continue
		}
		if got := source[line.Offset : line.Offset+len(line.Text)]; got != line.Text {
			t.Errorf("offset %d yields %q, want %q", line.Offset, got, line.Text)
		}
	}
}

func TestParseHymnTitleOnlyWhenItStandsAlone(t *testing.T) {
	titled := ParseHymn("Te lucis ante terminum\n\nTo thee, before the close of day,\nCreator of the world, we pray.\n\nFrom all ill dreams defend our eyes.\n")
	if titled.Title != "Te lucis ante terminum" {
		t.Errorf("Title = %q", titled.Title)
	}
	if len(titled.Stanzas) != 2 {
		t.Fatalf("Stanzas = %#v, want 2", titled.Stanzas)
	}
	if len(titled.Stanzas[0]) != 2 || titled.Stanzas[0][0] != "To thee, before the close of day," {
		t.Errorf("first stanza = %#v", titled.Stanzas[0])
	}

	// A multi-line first block is a stanza, not a title. The engine peels the
	// incipit before rendering, so this is what a renderer normally sees —
	// treating it as a title would drop the hymn's opening stanza.
	peeled := ParseHymn("To thee, before the close of day,\nCreator of the world, we pray.\n\nFrom all ill dreams defend our eyes.\n")
	if peeled.Title != "" {
		t.Errorf("Title = %q, want empty", peeled.Title)
	}
	if len(peeled.Stanzas) != 2 || peeled.Stanzas[0][0] != "To thee, before the close of day," {
		t.Fatalf("Stanzas = %#v, want the opening stanza kept", peeled.Stanzas)
	}
}

func TestParseHymnSingleStanza(t *testing.T) {
	hymn := ParseHymn("O Trinity of blessed light.\n")
	if hymn.Title != "" || len(hymn.Stanzas) != 1 || len(hymn.Stanzas[0]) != 1 {
		t.Errorf("hymn = %#v", hymn)
	}
}
