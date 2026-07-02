package audit

import "testing"

func TestLintMechanical(t *testing.T) {
	cases := []struct {
		name, text string
		classes    []string
	}{
		{"clean", "Blessed is the man * that walketh.", nil},
		{"double space", "O God, make  speed to save me.", []string{"double-space"}},
		{"trailing space", "O God, make speed. \nSecond line.", []string{"trailing-space"}},
		{"nbsp", "O God.", []string{"control-char"}},
		{"doubled asterisk", "text ** more", []string{"asterisk"}},
	}
	for _, c := range cases {
		r := &LintReport{}
		lintMechanical(r, "test/key", c.text)
		if len(r.Mechanical) != len(c.classes) {
			t.Errorf("%s: got %d findings %v, want %d", c.name, len(r.Mechanical), r.Mechanical, len(c.classes))
			continue
		}
		for i, f := range r.Mechanical {
			if f.Class != c.classes[i] {
				t.Errorf("%s: finding %d class = %s, want %s", c.name, i, f.Class, c.classes[i])
			}
		}
	}
}

func TestLintLatin(t *testing.T) {
	r := &LintReport{}
	lintLatin(r, "proper/x/collect", "Deus, qui nobis sub sacramento...")
	if len(r.Advisory) != 1 {
		t.Fatalf("two Latin words should flag, got %v", r.Advisory)
	}

	r = &LintReport{}
	lintLatin(r, "proper/x/collect", "O Lord, hear my prayer through Christ our Lord.")
	if len(r.Advisory) != 0 {
		t.Fatalf("English text should not flag, got %v", r.Advisory)
	}

	// Hymn title line (Latin by convention) is skipped.
	r = &LintReport{}
	lintLatin(r, "proper/x/hymn-lauds", "Lucis creator Domine optime\n\nO blest Creator of the light.")
	if len(r.Advisory) != 0 {
		t.Fatalf("hymn title should be skipped, got %v", r.Advisory)
	}
}

func TestLintTruncation(t *testing.T) {
	r := &LintReport{}
	lintTruncation(r, "a", "This text was cut of")
	lintTruncation(r, "b", "This text is complete.")
	lintTruncation(r, "c", "Amen.")
	if len(r.Advisory) != 1 || r.Advisory[0].Key != "a" {
		t.Fatalf("want single truncation finding for key a, got %v", r.Advisory)
	}
}
