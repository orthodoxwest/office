// Package texts provides loaders for the liturgical text corpus.
package texts

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TextCorpus holds all loaded liturgical texts, keyed by reference path.
type TextCorpus struct {
	texts   map[string]string
	aliases map[string]string

	// collectConclusions maps a collect's corpus key to the name of the
	// conclusion form it takes. It is loaded from data/collect-conclusions.txt
	// rather than from data/texts/, because it records a property of a text
	// rather than being one — keeping it out of texts also keeps it out of the
	// rendered-entry and zero-occurrence sweeps.
	collectConclusions map[string]string

	// incipits maps a psalm or canticle corpus key to its Latin incipit, the
	// subtitle a printed diurnal sets beside the psalm number. Loaded from
	// data/latin-incipits.txt for the same reason as collectConclusions: it
	// describes a text without being one, so it stays out of the corpus
	// sweeps — and out of the Latin lint, which would otherwise flag every
	// entry.
	incipits map[string]string
}

// LoadTexts loads all text files from the data/texts/ directory tree.
//
// Two formats are supported:
//   - INI-style files: sections like [ref-name] contain text, loaded as "dir/subdir/ref-name"
//   - Plain text files: loaded as "dir/subdir/filename" (without .txt extension)
//
// INI-style files are identified by having at least one [section] header.
// Plain text files are everything else.
func LoadTexts(dataDir string) (*TextCorpus, error) {
	textsDir := filepath.Join(dataDir, "texts")
	corpus := &TextCorpus{
		texts:              make(map[string]string),
		aliases:            make(map[string]string),
		collectConclusions: make(map[string]string),
		incipits:           make(map[string]string),
	}

	err := filepath.Walk(textsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".txt") {
			return nil
		}

		relPath, err := filepath.Rel(textsDir, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		text := string(content)

		// Check if this is an INI-style file (has [section] headers)
		if hasINISections(text) {
			return corpus.loadINIFile(relPath, text)
		}

		// Plain text file: key is path without .txt extension.
		// Comment-only files (e.g. proper scaffolds awaiting fill-in) strip to
		// empty and are not corpus entries — nothing would look them up, and
		// they would only pollute zero-occurrence / placeholder sweeps.
		key := strings.TrimSuffix(relPath, ".txt")
		key = filepath.ToSlash(key)
		body := strings.TrimSpace(stripCommentLines(text))
		if body == "" {
			return nil
		}
		corpus.texts[key] = body
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("loading texts: %w", err)
	}
	if err := corpus.extractAndValidateAliases(); err != nil {
		return nil, err
	}
	if err := corpus.loadCollectConclusions(dataDir); err != nil {
		return nil, err
	}
	if err := corpus.loadIncipits(dataDir); err != nil {
		return nil, err
	}

	return corpus, nil
}

// collectConclusionPrefix is the corpus-key prefix under which the conclusion
// formulas live. Kept here so the loader can validate forms as it reads them.
const collectConclusionPrefix = "shared/formulas/collect-conclusion-"

// loadCollectConclusions reads data/collect-conclusions.txt, whose lines pair a
// collect's corpus key with the name of its conclusion form. A missing file is
// not an error: every collect then takes the default form. A malformed line, an
// unknown key, an unknown form, or a duplicate key is an error — each would
// otherwise silently produce a collect with the wrong ending or none at all.
func (c *TextCorpus) loadCollectConclusions(dataDir string) error {
	path := filepath.Join(dataDir, "collect-conclusions.txt")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	for i, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) != 2 {
			return fmt.Errorf("%s:%d: expected \"<corpus-key> <form>\", got %q", path, i+1, trimmed)
		}
		key, form := fields[0], fields[1]
		// Fail loudly rather than let a typo silently drop the conclusion: an
		// unresolvable form would leave the collect ending mid-prayer, which is
		// exactly the defect this data exists to fix.
		if c.Get(collectConclusionPrefix+form) == "" {
			return fmt.Errorf("%s:%d: %s names conclusion form %q, but %s%s is not in the corpus",
				path, i+1, key, form, collectConclusionPrefix, form)
		}
		if c.Get(key) == "" {
			return fmt.Errorf("%s:%d: %q is not in the corpus", path, i+1, key)
		}
		if prior, dup := c.collectConclusions[key]; dup {
			return fmt.Errorf("%s:%d: %q listed twice (%q then %q)", path, i+1, key, prior, form)
		}
		c.collectConclusions[key] = form
	}
	return nil
}

// IncipitPrefixes are the corpus-key prefixes whose entries carry a Latin
// incipit. Everything under them must appear in data/latin-incipits.txt.
var IncipitPrefixes = []string{"psalms/", "canticles/"}

// loadIncipits reads data/latin-incipits.txt, whose lines pair a psalm or
// canticle corpus key with the Latin incipit printed beside its label. A
// missing file is not an error: no psalm then carries a subtitle. A malformed
// line, an unknown key, a duplicate key, or a key outside IncipitPrefixes is
// an error — an incipit attached to the wrong psalm is exactly the defect the
// Hebrew/Vulgate numbering split invites, so it must not pass silently.
func (c *TextCorpus) loadIncipits(dataDir string) error {
	path := filepath.Join(dataDir, "latin-incipits.txt")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	for i, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		rawKey, rawIncipit, found := strings.Cut(trimmed, "=")
		if !found {
			return fmt.Errorf("%s:%d: expected \"<corpus-key> = <incipit>\", got %q", path, i+1, trimmed)
		}
		key, incipit := strings.TrimSpace(rawKey), strings.TrimSpace(rawIncipit)
		if incipit == "" {
			return fmt.Errorf("%s:%d: %q has an empty incipit", path, i+1, key)
		}
		if !hasAnyPrefix(key, IncipitPrefixes) {
			return fmt.Errorf("%s:%d: %q is not a psalm or canticle key", path, i+1, key)
		}
		if c.Get(key) == "" {
			return fmt.Errorf("%s:%d: %q is not in the corpus", path, i+1, key)
		}
		if prior, dup := c.incipits[key]; dup {
			return fmt.Errorf("%s:%d: %q listed twice (%q then %q)", path, i+1, key, prior, incipit)
		}
		c.incipits[key] = incipit
	}
	return nil
}

// hasAnyPrefix reports whether key starts with one of the given prefixes.
func hasAnyPrefix(key string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// Incipit returns the Latin incipit recorded for a psalm or canticle corpus
// key, resolving aliases, or the empty string when none is recorded.
func (c *TextCorpus) Incipit(ref string) string {
	if incipit, ok := c.incipits[ref]; ok {
		return incipit
	}
	return c.incipits[c.CanonicalRef(ref)]
}

// SetIncipit records a Latin incipit, for use in tests. Both constructors
// initialize the map, so there is no nil case to guard.
func (c *TextCorpus) SetIncipit(key, incipit string) {
	c.incipits[key] = incipit
}

// MissingIncipits returns the psalm and canticle corpus keys that have no
// recorded incipit, sorted alphabetically.
func (c *TextCorpus) MissingIncipits() []string {
	var keys []string
	for key := range c.texts {
		if hasAnyPrefix(key, IncipitPrefixes) && c.incipits[key] == "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// CollectConclusionForm returns the conclusion form recorded for a collect's
// corpus key, and whether one was recorded. Collects with no entry take the
// default form.
func (c *TextCorpus) CollectConclusionForm(key string) (string, bool) {
	form, ok := c.collectConclusions[key]
	return form, ok
}

// SetCollectConclusionForm records a conclusion form, for use in tests. Both
// constructors initialize the map, so there is no nil case to guard.
func (c *TextCorpus) SetCollectConclusionForm(key, form string) {
	c.collectConclusions[key] = form
}

// stripCommentLines removes corpus annotations from plain-text files using
// the same whole-line rule as loadINIFile. Blank lines and inline hash
// characters remain liturgical content.
func stripCommentLines(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// NewTestCorpus creates a TextCorpus from a map, for use in tests. Conclusion
// forms start empty, so every collect takes the default; use
// SetCollectConclusionForm to record one.
func NewTestCorpus(texts map[string]string) *TextCorpus {
	return &TextCorpus{
		texts:              texts,
		aliases:            make(map[string]string),
		collectConclusions: make(map[string]string),
		incipits:           make(map[string]string),
	}
}

// Get returns the text for the given reference path, resolving @use aliases, or
// empty string if the reference does not exist.
func (c *TextCorpus) Get(ref string) string {
	canonical := c.CanonicalRef(ref)
	return c.texts[canonical]
}

// Has returns true if the reference exists in the corpus.
func (c *TextCorpus) Has(ref string) bool {
	return c.CanonicalRef(ref) != ""
}

// CanonicalRef returns the concrete corpus key behind ref. Aliases are
// resolved transitively. An unknown reference returns the empty string.
func (c *TextCorpus) CanonicalRef(ref string) string {
	seen := make(map[string]bool)
	for {
		if seen[ref] {
			return "" // LoadTexts rejects cycles; retain a safe guard for callers.
		}
		seen[ref] = true
		if target, ok := c.aliases[ref]; ok {
			ref = target
			continue
		}
		if _, ok := c.texts[ref]; ok {
			return ref
		}
		return ""
	}
}

// HasKeySuffix returns true if any corpus key ends with "/"+suffix.
// Used by the validator to check for feast-specific or seasonal refs.
func (c *TextCorpus) HasKeySuffix(suffix string) bool {
	target := "/" + suffix
	for k := range c.texts {
		if strings.HasSuffix(k, target) {
			return true
		}
	}
	for k := range c.aliases {
		if strings.HasSuffix(k, target) {
			return true
		}
	}
	return false
}

// Entries returns a copy of the concrete renderable-text entries keyed by
// reference path. Aliases and structural declarations in the psalmody/
// namespace are intentionally omitted so provenance and review queues count
// each shared liturgical text only once.
func (c *TextCorpus) Entries() map[string]string {
	out := make(map[string]string, len(c.texts))
	for k, v := range c.texts {
		if strings.HasPrefix(k, "psalmody/") {
			continue
		}
		out[k] = v
	}
	return out
}

// References returns every resolvable corpus key, including aliases, sorted
// alphabetically. It is intended for validators that must inspect declarations
// stored behind @use as well as concrete text entries.
func (c *TextCorpus) References() []string {
	refs := make([]string, 0, len(c.texts)+len(c.aliases))
	for key := range c.texts {
		refs = append(refs, key)
	}
	for key := range c.aliases {
		refs = append(refs, key)
	}
	sort.Strings(refs)
	return refs
}

// extractAndValidateAliases moves exact @use directives out of the concrete
// text map, then verifies that every alias terminates at a real corpus entry.
func (c *TextCorpus) extractAndValidateAliases() error {
	for key, body := range c.texts {
		trimmed := strings.TrimSpace(body)
		if !strings.HasPrefix(trimmed, "@use") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) != 2 || fields[0] != "@use" {
			return fmt.Errorf("invalid corpus alias %q: expected @use <corpus-key>", key)
		}
		c.aliases[key] = fields[1]
		delete(c.texts, key)
	}

	for alias := range c.aliases {
		if canonical := c.CanonicalRef(alias); canonical == "" {
			return fmt.Errorf("corpus alias %q does not resolve (target %q)", alias, c.aliases[alias])
		}
	}
	return nil
}

// FindPlaceholders returns all corpus keys whose text begins with "placeholder"
// (case-insensitive), sorted alphabetically. These are entries declared but not yet filled in.
func (c *TextCorpus) FindPlaceholders() []string {
	var keys []string
	for k, v := range c.texts {
		if strings.HasPrefix(strings.ToLower(v), "placeholder") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// hasINISections checks if the text contains any [section] headers.
// A valid section header is a line like [word-chars] — no spaces or colons inside.
func hasINISections(text string) bool {
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inner := line[1 : len(line)-1]
			// Valid section names contain only alphanumeric chars and hyphens
			if len(inner) > 0 && !strings.ContainsAny(inner, " :\t") {
				return true
			}
		}
	}
	return false
}

// loadINIFile parses an INI-style text file into corpus entries.
// Each [section] becomes a separate entry, keyed as "dir/stem/section-name"
// where stem is the filename without extension.
func (c *TextCorpus) loadINIFile(relPath, text string) error {
	dir := filepath.Dir(relPath)
	dir = filepath.ToSlash(dir)
	stem := strings.TrimSuffix(filepath.Base(relPath), ".txt")

	scanner := bufio.NewScanner(strings.NewReader(text))
	var currentKey string
	var lines []string
	sections := make(map[string]int)

	flush := func() {
		if currentKey != "" {
			c.texts[currentKey] = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Comment lines are dropped everywhere, including inside sections,
		// so data files can carry per-section annotations (e.g. source
		// markers) without them leaking into rendered text.
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		if trimmed == "" {
			if currentKey != "" {
				lines = append(lines, line)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inner := trimmed[1 : len(trimmed)-1]
			if len(inner) > 0 && !strings.ContainsAny(inner, " :\t") {
				if firstLine, duplicate := sections[inner]; duplicate {
					return fmt.Errorf("%s:%d: duplicate INI section [%s] (first declared at line %d)", relPath, lineNumber, inner, firstLine)
				}
				sections[inner] = lineNumber
				flush()
				if dir == "." {
					currentKey = stem + "/" + inner
				} else {
					currentKey = dir + "/" + stem + "/" + inner
				}
				lines = nil
				continue
			}
		}

		if currentKey != "" {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading %s: %w", relPath, err)
	}
	flush()
	return nil
}
