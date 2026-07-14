package audit

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/orthodoxwest/office/internal/texts"
)

// LintFinding is one lint hit on one corpus entry.
type LintFinding struct {
	Key    string // corpus key, or file path for non-corpus findings
	Class  string
	Detail string
}

// LintReport separates mechanical findings (deterministic formatting errors
// that fail `make check`) from advisory findings (heuristics that need a
// human eye and never fail the build).
type LintReport struct {
	Mechanical []LintFinding
	Advisory   []LintFinding
}

// latinWords are distinctly Latin tokens that should not appear in the
// English corpus. An entry is flagged only when two or more distinct words
// match, so isolated proper terms in rubrics do not trip it.
var latinWords = map[string]bool{
	"saecula": true, "saeculorum": true, "saeculi": true,
	"domine": true, "dominus": true, "dominum": true, "domini": true,
	"deus": true, "deum": true,
	"nobis": true, "omnia": true, "semper": true,
	"sancto": true, "sancti": true, "spiritui": true,
	"misericordia": true, "peccata": true, "caeli": true, "terrae": true,
	"eleison": true, "oremus": true,
}

var wordRE = regexp.MustCompile(`[A-Za-z]+`)

// Lint scans the text corpus (and chant files) for formatting and content
// anomalies.
func Lint(dataDir string) (*LintReport, error) {
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading texts: %w", err)
	}

	r := &LintReport{}
	entries := corpus.Entries()
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		text := entries[key]
		lintMechanical(r, key, text)
		lintLatin(r, key, text)
		lintTruncation(r, key, text)
		lintUnpointedAntiphon(r, key, text)
	}

	lintDuplicateCandidates(r, entries, keys)
	lintNearDuplicates(r, entries, keys)
	lintChantOrphans(r, dataDir)

	return r, nil
}

// lintDuplicateCandidates keeps identical concrete entries visible after the
// unambiguous cases have been replaced by corpus aliases. Remaining matches
// may be intentional independent witnesses, so they are advisory rather than
// mechanical failures.
func lintDuplicateCandidates(r *LintReport, entries map[string]string, keys []string) {
	byText := make(map[string][]string)
	for _, key := range keys {
		if len(entries[key]) < 20 {
			continue
		}
		byText[entries[key]] = append(byText[entries[key]], key)
	}
	for _, duplicates := range byText {
		if len(duplicates) < 2 {
			continue
		}
		sort.Strings(duplicates)
		r.Advisory = append(r.Advisory, LintFinding{
			Key: duplicates[0], Class: "duplicate-candidate",
			Detail: "identical concrete entries: " + strings.Join(duplicates[1:], ", "),
		})
	}
}

// lintMechanical flags deterministic formatting errors.
func lintMechanical(r *LintReport, key, text string) {
	add := func(class, detail string) {
		r.Mechanical = append(r.Mechanical, LintFinding{Key: key, Class: class, Detail: detail})
	}

	for _, ru := range text {
		switch {
		case ru == '�':
			add("control-char", "contains U+FFFD replacement character")
		case ru == ' ':
			add("control-char", "contains non-breaking space")
		case ru < 0x20 && ru != '\n':
			add("control-char", fmt.Sprintf("contains control character U+%04X", ru))
		default:
			continue
		}
		break // one finding per entry is enough
	}

	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "  ") {
			add("double-space", fmt.Sprintf("line %d has doubled spaces", i+1))
			break
		}
	}
	for i, line := range strings.Split(text, "\n") {
		if line != strings.TrimRight(line, " \t") {
			add("trailing-space", fmt.Sprintf("line %d has trailing whitespace", i+1))
			break
		}
	}
	if strings.Contains(text, "**") {
		add("asterisk", "contains doubled asterisk")
	}
	if strings.Contains(text, `\n`) {
		add("escape-sequence", `contains literal \n escape`)
	}
	if strings.Contains(text, "`") {
		add("backtick", "contains backtick (use apostrophe)")
	}
}

// indexedAntiphonRE matches indexed psalm-antiphon slot keys, whose texts are
// expected to carry a " * " pointing mediant.
var indexedAntiphonRE = regexp.MustCompile(`/psalm-antiphon(-\d+)?$`)

// lintUnpointedAntiphon flags indexed psalm antiphons with no pointing
// asterisk — usually a transcription slip, occasionally a legitimately short
// antiphon (hence advisory).
func lintUnpointedAntiphon(r *LintReport, key, text string) {
	if !indexedAntiphonRE.MatchString(key) {
		return
	}
	if !strings.Contains(text, "*") {
		r.Advisory = append(r.Advisory, LintFinding{
			Key: key, Class: "unpointed-antiphon",
			Detail: "no pointing asterisk",
		})
	}
}

// lintLatin flags entries containing two or more distinct Latin words —
// likely untranslated leftovers from Divinum Officium seeding. Hymn title
// lines (Latin by convention) and canticle "[section: …]" markup are
// skipped.
func lintLatin(r *LintReport, key, text string) {
	lines := strings.Split(text, "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[section:") {
			continue
		}
		// First line of a hymn entry is its Latin title.
		if i == 0 && strings.Contains(key, "hymn") {
			continue
		}
		for _, w := range wordRE.FindAllString(line, -1) {
			lw := strings.ToLower(w)
			if latinWords[lw] {
				seen[lw] = true
			}
		}
	}
	if len(seen) >= 2 {
		words := make([]string, 0, len(seen))
		for w := range seen {
			words = append(words, w)
		}
		sort.Strings(words)
		r.Advisory = append(r.Advisory, LintFinding{
			Key: key, Class: "latin",
			Detail: "Latin words: " + strings.Join(words, ", "),
		})
	}
}

// lintTruncation flags entries whose final character is a letter — text is
// expected to end with punctuation, so a bare letter suggests a cut-off.
func lintTruncation(r *LintReport, key, text string) {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return
	}
	if unicode.IsLetter(runes[len(runes)-1]) {
		tail := string(runes)
		if len(runes) > 40 {
			tail = "…" + string(runes[len(runes)-40:])
		}
		r.Advisory = append(r.Advisory, LintFinding{
			Key: key, Class: "truncated",
			Detail: fmt.Sprintf("no terminal punctuation: %q", tail),
		})
	}
}

// lintNearDuplicates flags pairs of entries sharing the same slot name whose
// word sets nearly coincide without being identical — usually the same text
// seeded twice with a transcription difference, one of which is wrong.
func lintNearDuplicates(r *LintReport, entries map[string]string, keys []string) {
	type doc struct {
		key   string
		words map[string]bool
	}
	bySlot := map[string][]doc{}
	for _, key := range keys {
		text := entries[key]
		if len(text) < 60 {
			continue
		}
		slot := trimIndexSuffix(key[strings.LastIndex(key, "/")+1:])
		ws := map[string]bool{}
		for _, w := range wordRE.FindAllString(strings.ToLower(text), -1) {
			ws[w] = true
		}
		bySlot[slot] = append(bySlot[slot], doc{key, ws})
	}

	jaccard := func(a, b map[string]bool) float64 {
		inter := 0
		for w := range a {
			if b[w] {
				inter++
			}
		}
		union := len(a) + len(b) - inter
		if union == 0 {
			return 0
		}
		return float64(inter) / float64(union)
	}

	for _, docs := range bySlot {
		for i := range docs {
			for j := i + 1; j < len(docs); j++ {
				if entries[docs[i].key] == entries[docs[j].key] {
					continue // identical is intentional reuse, not an error
				}
				if s := jaccard(docs[i].words, docs[j].words); s >= 0.85 {
					r.Advisory = append(r.Advisory, LintFinding{
						Key: docs[i].key, Class: "near-duplicate",
						Detail: fmt.Sprintf("differs slightly from %s (similarity %.2f)", docs[j].key, s),
					})
				}
			}
		}
	}
	sort.Slice(r.Advisory, func(i, j int) bool {
		if r.Advisory[i].Class != r.Advisory[j].Class {
			return r.Advisory[i].Class < r.Advisory[j].Class
		}
		return r.Advisory[i].Key < r.Advisory[j].Key
	})
}

// lintChantOrphans flags .gabc files under chant/psalms and chant/canticles
// with no matching text entry. Hymn scores are keyed by title slug rather
// than file name and are not checked.
func lintChantOrphans(r *LintReport, dataDir string) {
	for _, category := range []string{"psalms", "canticles"} {
		dir := filepath.Join(dataDir, "texts", "chant", category)
		matches, _ := filepath.Glob(filepath.Join(dir, "*.gabc"))
		for _, m := range matches {
			base := strings.TrimSuffix(filepath.Base(m), ".gabc")
			text := filepath.Join(dataDir, "texts", category, base+".txt")
			if _, err := os.Stat(text); err != nil {
				r.Mechanical = append(r.Mechanical, LintFinding{
					Key: filepath.ToSlash(filepath.Join("chant", category, base+".gabc")), Class: "gabc-orphan",
					Detail: "no matching " + category + "/" + base + ".txt",
				})
			}
		}
	}
}

// PrintLint writes the lint report to w and reports whether any mechanical
// finding was present (callers exit nonzero on true).
func PrintLint(r *LintReport, w io.Writer) bool {
	byClass := func(fs []LintFinding) map[string]int {
		m := map[string]int{}
		for _, f := range fs {
			m[f.Class]++
		}
		return m
	}

	fmt.Fprintf(w, "=== Mechanical lint: %d finding(s) ===\n", len(r.Mechanical))
	for _, f := range r.Mechanical {
		fmt.Fprintf(w, "  [%s] %s: %s\n", f.Class, f.Key, f.Detail)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "=== Advisory lint: %d finding(s) ===\n", len(r.Advisory))
	if len(r.Advisory) > 0 {
		classes := byClass(r.Advisory)
		names := make([]string, 0, len(classes))
		for c := range classes {
			names = append(names, c)
		}
		sort.Strings(names)
		for _, c := range names {
			fmt.Fprintf(w, "  %s: %d\n", c, classes[c])
		}
		fmt.Fprintln(w)
		for _, f := range r.Advisory {
			fmt.Fprintf(w, "  [%s] %s: %s\n", f.Class, f.Key, f.Detail)
		}
	}
	fmt.Fprintln(w)

	return len(r.Mechanical) > 0
}
