package texts

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ValidateAll scans text files for leftover import directives that should not
// appear in the normalized corpus.
func ValidateAll(dataDir string) []string {
	textsDir := filepath.Join(dataDir, "texts")
	var errs []string

	err := filepath.Walk(textsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".txt") {
			return nil
		}

		relPath, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		errs = append(errs, validateFile(filepath.ToSlash(relPath), string(content))...)
		return nil
	})
	if err != nil {
		return []string{fmt.Sprintf("validating texts: %v", err)}
	}

	corpus, err := LoadTexts(dataDir)
	if err != nil {
		return []string{fmt.Sprintf("loading text corpus: %v", err)}
	}
	for _, key := range corpus.FindPlaceholders() {
		errs = append(errs, fmt.Sprintf("placeholder text remains in corpus: %s", key))
	}

	sort.Strings(errs)
	return errs
}

func validateFile(relPath, text string) []string {
	var errs []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	currentSection := ""
	hasSections := hasINISections(text)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if hasSections && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inner := trimmed[1 : len(trimmed)-1]
			if len(inner) > 0 && !strings.ContainsAny(inner, " :\t") {
				if suggestion := nearPsalmodySectionName(relPath, inner); suggestion != "" {
					errs = append(errs, fmt.Sprintf(
						"%s:%d: unrecognized section name %q (did you mean %q?)",
						relPath, lineNo, inner, suggestion,
					))
				}
				currentSection = inner
				continue
			}
		}

		if reason := unexpectedDirectiveReason(trimmed); reason != "" {
			location := fmt.Sprintf("%s:%d", relPath, lineNo)
			if currentSection != "" {
				location += fmt.Sprintf(" [%s]", currentSection)
			}
			errs = append(errs, fmt.Sprintf("%s: %s: %q", location, reason, trimmed))
		}
	}

	errs = append(errs, validateControlCharacters(relPath, []byte(text))...)
	return errs
}

func nearPsalmodySectionName(relPath, section string) string {
	if !strings.HasPrefix(relPath, "texts/proper/") && !strings.HasPrefix(relPath, "texts/commons/") {
		return ""
	}
	reserved := []string{"vespers-psalmody", "vespers-psalmody-first"}
	for _, name := range reserved {
		if section == name {
			return ""
		}
	}
	best := ""
	bestDistance := 3
	for _, name := range reserved {
		if distance := editDistance(section, name); distance < bestDistance {
			best = name
			bestDistance = distance
		}
	}
	if bestDistance <= 2 {
		return best
	}
	return ""
}

func editDistance(a, b string) int {
	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(current[j-1]+1, previous[j]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(b)]
}

func unexpectedDirectiveReason(trimmed string) string {
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 2 && fields[0] == "@use" {
		return ""
	}
	switch {
	case strings.HasPrefix(trimmed, "@"):
		return "unexpected Divinum Officium directive"
	case strings.HasPrefix(trimmed, "ex "):
		return "unexpected Divinum Officium extract directive"
	default:
		return ""
	}
}

func validateControlCharacters(relPath string, content []byte) []string {
	var errs []string
	lineNo := 1
	columnNo := 1

	for _, b := range content {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			errs = append(errs, fmt.Sprintf(
				"%s:%d:%d: unexpected control character U+%04X",
				relPath, lineNo, columnNo, b,
			))
		}

		switch b {
		case '\n':
			lineNo++
			columnNo = 1
		case '\r':
			columnNo = 1
		default:
			columnNo++
		}
	}

	if bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
		errs = append(errs, fmt.Sprintf("%s:1:1: unexpected UTF-8 BOM", relPath))
	}

	return errs
}
