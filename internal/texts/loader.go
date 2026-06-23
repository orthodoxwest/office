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
	texts map[string]string
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
	corpus := &TextCorpus{texts: make(map[string]string)}

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

	return corpus, nil
}

// NewTestCorpus creates a TextCorpus from a map, for use in tests.
func NewTestCorpus(texts map[string]string) *TextCorpus {
	return &TextCorpus{texts: texts}
}

// Get returns the text for the given reference path, or empty string if not found.
func (c *TextCorpus) Get(ref string) string {
	return c.texts[ref]
}

// Has returns true if the reference exists in the corpus.
func (c *TextCorpus) Has(ref string) bool {
	_, ok := c.texts[ref]
	return ok
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
	return false
}

// Entries returns a copy of the loaded corpus entries keyed by reference path.
func (c *TextCorpus) Entries() map[string]string {
	out := make(map[string]string, len(c.texts))
	for k, v := range c.texts {
		out[k] = v
	}
	return out
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
