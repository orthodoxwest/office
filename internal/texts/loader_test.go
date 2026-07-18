package texts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTexts(t *testing.T) {
	dir := t.TempDir()
	textsDir := filepath.Join(dir, "texts")

	// Create a plain text file
	psalmDir := filepath.Join(textsDir, "psalms")
	os.MkdirAll(psalmDir, 0755)
	os.WriteFile(filepath.Join(psalmDir, "004.txt"), []byte("Hear me when I call, O God.\n"), 0644)

	// Create an INI-style file at ordinary/compline.txt
	// Sections become keyed as "ordinary/compline/section-name"
	ordinaryDir := filepath.Join(textsDir, "ordinary")
	os.MkdirAll(ordinaryDir, 0755)
	os.WriteFile(filepath.Join(ordinaryDir, "compline.txt"), []byte(`[opening-versicle]
V. O God, make speed to save us.
R. O Lord, make haste to help us.

[psalm-antiphon]
Have mercy upon me, O Lord.
`), 0644)

	// Create a .gitkeep that should be ignored
	os.WriteFile(filepath.Join(textsDir, ".gitkeep"), []byte(""), 0644)

	corpus, err := LoadTexts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Plain text file
	if got := corpus.Get("psalms/004"); got != "Hear me when I call, O God." {
		t.Errorf("psalms/004 = %q", got)
	}

	// INI sections: keyed as dir/stem/section
	if got := corpus.Get("ordinary/compline/opening-versicle"); got != "V. O God, make speed to save us.\nR. O Lord, make haste to help us." {
		t.Errorf("ordinary/compline/opening-versicle = %q", got)
	}
	if got := corpus.Get("ordinary/compline/psalm-antiphon"); got != "Have mercy upon me, O Lord." {
		t.Errorf("ordinary/compline/psalm-antiphon = %q", got)
	}

	// Has
	if !corpus.Has("psalms/004") {
		t.Error("expected Has(psalms/004) = true")
	}
	if corpus.Has("nonexistent") {
		t.Error("expected Has(nonexistent) = false")
	}

	// Get missing
	if got := corpus.Get("nonexistent"); got != "" {
		t.Errorf("Get(nonexistent) = %q, want empty", got)
	}
}

func TestLoadTextsStripsComments(t *testing.T) {
	dir := t.TempDir()
	ordinaryDir := filepath.Join(dir, "texts", "ordinary")
	psalmDir := filepath.Join(dir, "texts", "psalms")
	os.MkdirAll(ordinaryDir, 0755)
	os.MkdirAll(psalmDir, 0755)
	os.WriteFile(filepath.Join(ordinaryDir, "lauds.txt"), []byte(`# File-level comment before any section.

[collect]
# SOURCE: divinum-officium Sancti/10-DU — check against diurnal
O God, who hast prepared for them that love thee.

[hymn]
# annotation between stanzas is dropped, blank lines kept
First stanza line one.

Second stanza line one.
`), 0644)
	os.WriteFile(filepath.Join(psalmDir, "116b.txt"), []byte(`Psalm 116:10-16

# SOURCE: printed psalter
I believed, and therefore will I speak * but I was sore troubled.
The # character in a liturgical line remains.
`), 0644)

	corpus, err := LoadTexts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := corpus.Get("ordinary/lauds/collect"); got != "O God, who hast prepared for them that love thee." {
		t.Errorf("collect = %q, comment not stripped", got)
	}
	if got := corpus.Get("ordinary/lauds/hymn"); got != "First stanza line one.\n\nSecond stanza line one." {
		t.Errorf("hymn = %q, want stanza break preserved", got)
	}
	if got := corpus.Get("psalms/116b"); got != "Psalm 116:10-16\n\nI believed, and therefore will I speak * but I was sore troubled.\nThe # character in a liturgical line remains." {
		t.Errorf("psalms/116b = %q, plain-text comment not stripped", got)
	}
}

func TestLoadTextsResolvesAliasesAndOmitsThemFromEntries(t *testing.T) {
	dir := t.TempDir()
	properDir := filepath.Join(dir, "texts", "proper")
	psalmodyDir := filepath.Join(dir, "texts", "psalmody")
	os.MkdirAll(properDir, 0755)
	os.MkdirAll(psalmodyDir, 0755)
	os.WriteFile(filepath.Join(properDir, "shared.txt"), []byte(`[responsory]
R. Shared text.

[alias]
@use proper/shared/responsory

[alias-chain]
@use proper/shared/alias
`), 0644)
	os.WriteFile(filepath.Join(psalmodyDir, "vespers.txt"), []byte(`[festal]
psalm-antiphon-1 = psalms/110
`), 0644)

	corpus, err := LoadTexts(dir)
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}
	if got := corpus.Get("proper/shared/alias-chain"); got != "R. Shared text." {
		t.Fatalf("Get(alias-chain) = %q", got)
	}
	if got := corpus.CanonicalRef("proper/shared/alias-chain"); got != "proper/shared/responsory" {
		t.Fatalf("CanonicalRef(alias-chain) = %q", got)
	}
	if !corpus.Has("proper/shared/alias") {
		t.Fatal("Has(alias) = false")
	}
	if _, ok := corpus.Entries()["proper/shared/alias"]; ok {
		t.Fatal("Entries includes alias")
	}
	if _, ok := corpus.Entries()["psalmody/vespers/festal"]; ok {
		t.Fatal("Entries includes structural psalmody declaration")
	}
	if !corpus.Has("psalmody/vespers/festal") {
		t.Fatal("Has(psalmody declaration) = false")
	}
}

func TestLoadTextsRejectsBrokenAliases(t *testing.T) {
	tests := map[string]string{
		"missing target": `[alias]
@use proper/missing/text
`,
		"cycle": `[one]
@use proper/shared/two

[two]
@use proper/shared/one
`,
		"malformed": `[alias]
@use proper/shared/text extra
`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			properDir := filepath.Join(dir, "texts", "proper")
			os.MkdirAll(properDir, 0755)
			os.WriteFile(filepath.Join(properDir, "shared.txt"), []byte(body), 0644)
			if _, err := LoadTexts(dir); err == nil {
				t.Fatal("LoadTexts accepted broken alias")
			}
		})
	}
}

func TestFindPlaceholders(t *testing.T) {
	corpus := NewTestCorpus(map[string]string{
		// Plain text placeholder (e.g. canticles/benedicite)
		"canticles/benedicite": "Placeholder",
		// INI-section placeholder (e.g. ordinary/lauds/hymn)
		"ordinary/lauds/hymn": "Placeholder",
		// Case variation — should still be detected
		"canticles/magnificat": "placeholder",
		// Real content — must not be flagged
		"canticles/benedictus":            "Blessed be the Lord God of Israel.",
		"ordinary/lauds/opening-versicle": "V. O God, make speed to save us.",
	})

	got := corpus.FindPlaceholders()

	want := []string{
		"canticles/benedicite",
		"canticles/magnificat",
		"ordinary/lauds/hymn",
	}
	if len(got) != len(want) {
		t.Fatalf("FindPlaceholders() = %v, want %v", got, want)
	}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("FindPlaceholders()[%d] = %q, want %q", i, got[i], k)
		}
	}
}
