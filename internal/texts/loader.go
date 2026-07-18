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
		texts:   make(map[string]string),
		aliases: make(map[string]string),
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

		// Plain text file: key is path without .txt extension
		key := strings.TrimSuffix(relPath, ".txt")
		key = filepath.ToSlash(key)
		corpus.texts[key] = strings.TrimSpace(text)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("loading texts: %w", err)
	}
	if err := corpus.extractAndValidateAliases(); err != nil {
		return nil, err
	}

	return corpus, nil
}

// NewTestCorpus creates a TextCorpus from a map, for use in tests.
func NewTestCorpus(texts map[string]string) *TextCorpus {
	return &TextCorpus{texts: texts, aliases: make(map[string]string)}
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

	flush := func() {
		if currentKey != "" {
			c.texts[currentKey] = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	for scanner.Scan() {
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

	flush()
	return nil
}
