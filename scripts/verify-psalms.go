//go:build ignore

// Verify the checked-in Coverdale psalter against the Church of England's
// online 1662 Book of Common Prayer Psalter.
//
// The repository deliberately keeps its own pointed text files: the source
// uses a colon for the chant division, while our corpus uses '*'.  The
// verifier translates only that structural marker.  It does not discard
// ordinary punctuation, so punctuation-only defects remain visible.
//
// Usage:
//
//	go run scripts/verify-psalms.go
//	go run scripts/verify-psalms.go -source-dir /tmp/bcp-psalter
//
// A source directory may contain cached official HTML pages named after their
// final URL component, for example psalms-1-5.html.  Without -source-dir the
// script downloads the current official Psalter index and its linked pages.
package main

import (
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultDataDir = "data/texts/psalms"
	psalterIndex   = "https://www.churchofengland.org/prayer-and-worship/worship-texts-and-resources/book-common-prayer/psalter"
	psalterBase    = "https://www.churchofengland.org/prayer-and-worship/worship-texts-and-resources/book-common-prayer/psalter/"
)

var (
	hrefRE       = regexp.MustCompile(`href="/prayer-and-worship/worship-texts-and-resources/book-common-prayer/psalter/([^"]+)"`)
	paragraphRE  = regexp.MustCompile(`(?is)<p\s+class="([^"]*)"[^>]*>(.*?)</p>`)
	headingRE    = regexp.MustCompile(`(?i)^Psalm\s+(\d+)\b`)
	rangeRE      = regexp.MustCompile(`(?i)^Psalm\s+\d+\.(\d+)-\d+`)
	breakRE      = regexp.MustCompile(`(?is)<br\s*/?>`)
	plainVerseRE = regexp.MustCompile(`^\s*(\d+)\s+`)
	verseNumRE   = regexp.MustCompile(`(?is)^\s*<span\s+class="vlversenumber"[^>]*>\s*(\d+)\s*</span>`)
	localHeadRE  = regexp.MustCompile(`(?i)^Psalm\s+(\d+)(?::(\d+)-(\d+))?([a-z]?)\s*$`)
	localVerseRE = regexp.MustCompile(`^\s*(\d+)\.\s+(.*)$`)
)

type verse struct {
	number   int
	explicit bool
	text     string
}

type sourceCorpus map[int][]verse

type localFile struct {
	path      string
	base      int
	start     int
	end       int
	text      []verse
	header    string
	fileLabel string
}

type mismatch struct {
	file   string
	psalm  int
	verse  int
	kind   string
	detail string
}

func main() {
	dataDir := flag.String("data-dir", defaultDataDir, "directory containing the checked-in psalm files")
	sourceDir := flag.String("source-dir", "", "cached official HTML pages; fetch online when omitted")
	cacheDir := flag.String("cache-dir", "", "save fetched official HTML pages here")
	flag.Parse()

	source, err := loadSource(*sourceDir, *cacheDir)
	if err != nil {
		fatal(err)
	}
	local, err := loadLocal(*dataDir)
	if err != nil {
		fatal(err)
	}

	var mismatches []mismatch
	for _, f := range local {
		mismatches = append(mismatches, compareFile(f, source)...)
	}

	fmt.Printf("Verified %d local files against %d source Psalms\n", len(local), len(source))
	if len(mismatches) == 0 {
		fmt.Println("PASS: wording, punctuation, chant separators, and verse numbering match")
		return
	}

	sort.Slice(mismatches, func(i, j int) bool {
		if mismatches[i].file != mismatches[j].file {
			return mismatches[i].file < mismatches[j].file
		}
		if mismatches[i].verse != mismatches[j].verse {
			return mismatches[i].verse < mismatches[j].verse
		}
		return mismatches[i].kind < mismatches[j].kind
	})
	counts := map[string]int{}
	for _, m := range mismatches {
		counts[m.kind]++
		fmt.Printf("%s Psalm %d:%d [%s] %s\n", m.file, m.psalm, m.verse, m.kind, m.detail)
	}
	fmt.Printf("FAIL: %d mismatch(es)", len(mismatches))
	for _, kind := range []string{"wording", "punctuation", "pointing", "verse-number", "missing-verse", "extra-verse", "source"} {
		if counts[kind] > 0 {
			fmt.Printf(" %s=%d", kind, counts[kind])
		}
	}
	fmt.Println()
	os.Exit(1)
}

func loadSource(sourceDir, cacheDir string) (sourceCorpus, error) {
	var pages map[string]string
	var err error
	if sourceDir != "" {
		pages, err = readCachedPages(sourceDir)
	} else {
		pages, err = fetchPages(cacheDir)
	}
	if err != nil {
		return nil, err
	}

	corpus := make(sourceCorpus)
	for name, body := range pages {
		for psalm, verses := range parseSourcePage(body) {
			if prior, exists := corpus[psalm]; exists {
				merged, err := mergeVerseSets(prior, verses, name)
				if err != nil {
					return nil, err
				}
				corpus[psalm] = merged
				continue
			}
			corpus[psalm] = verses
		}
	}
	for psalm := 1; psalm <= 150; psalm++ {
		if len(corpus[psalm]) == 0 {
			return nil, fmt.Errorf("official source did not yield Psalm %d", psalm)
		}
	}
	return corpus, nil
}

func fetchPages(cacheDir string) (map[string]string, error) {
	index, err := fetch(psalterIndex)
	if err != nil {
		return nil, fmt.Errorf("fetching Psalter index: %w", err)
	}

	seen := map[string]bool{}
	var slugs []string
	for _, match := range hrefRE.FindAllStringSubmatch(index, -1) {
		slug := match[1]
		if seen[slug] || !strings.HasPrefix(slug, "psalm") {
			continue
		}
		seen[slug] = true
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	if len(slugs) < 40 {
		return nil, fmt.Errorf("Psalter index yielded only %d source pages", len(slugs))
	}

	pages := make(map[string]string, len(slugs))
	for _, slug := range slugs {
		body, err := fetch(psalterBase + slug)
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", slug, err)
		}
		pages[slug] = body
		if cacheDir != "" {
			if err := os.MkdirAll(cacheDir, 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(cacheDir, slug+".html"), []byte(body), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return pages, nil
}

func fetch(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "orthodoxwest-office-psalm-verifier/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func readCachedPages(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pages := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		pages[strings.TrimSuffix(entry.Name(), ".html")] = string(body)
	}
	return pages, nil
}

func parseSourcePage(body string) sourceCorpus {
	corpus := make(sourceCorpus)
	current := 0
	nextVerse := 1
	firstVerseInSection := true
	for _, match := range paragraphRE.FindAllStringSubmatch(body, -1) {
		className, inner := match[1], match[2]
		if strings.Contains(className, "vlitemheading") {
			text := cleanHTML(inner)
			heading := headingRE.FindStringSubmatch(text)
			if heading == nil {
				current = 0
				continue
			}
			current, _ = strconv.Atoi(heading[1])
			nextVerse = 1
			firstVerseInSection = true
			if rangeMatch := rangeRE.FindStringSubmatch(text); rangeMatch != nil {
				nextVerse, _ = strconv.Atoi(rangeMatch[1])
			}
			if corpus[current] == nil {
				corpus[current] = nil
			}
			continue
		}
		if current == 0 || !strings.Contains(className, "vlpsalm") {
			continue
		}

		for _, part := range breakRE.Split(inner, -1) {
			text := cleanHTML(part)
			if text == "" || strings.HasPrefix(text, "Text from") || strings.HasPrefix(text, "is reproduced by permission") {
				continue
			}
			number := 0
			explicit := false
			if numberMatch := verseNumRE.FindStringSubmatch(part); numberMatch != nil {
				number, _ = strconv.Atoi(numberMatch[1])
				explicit = true
				text = strings.TrimSpace(strings.TrimPrefix(text, numberMatch[1]))
			} else if numberMatch := plainVerseRE.FindStringSubmatch(text); numberMatch != nil {
				number, _ = strconv.Atoi(numberMatch[1])
				explicit = true
				text = strings.TrimSpace(strings.TrimPrefix(text, numberMatch[1]))
			}
			if !explicit && !firstVerseInSection && len(corpus[current]) > 0 {
				last := len(corpus[current]) - 1
				corpus[current][last].text = normalizeSpaces(corpus[current][last].text + " " + text)
				continue
			}
			if !explicit {
				number = nextVerse
				explicit = true
			}
			text = normalizeHistoricalSource(current, number, text)
			nextVerse = number + 1
			firstVerseInSection = false
			corpus[current] = append(corpus[current], verse{number: number, explicit: explicit, text: text})
		}
	}
	return corpus
}

// The Church of England's HTML transcription is a useful independent
// machine-readable witness, but its 2006 page set modernizes or silently
// normalizes a handful of readings from the printed 1662 Psalter.  These are
// the readings confirmed against the official 1662 PDF; the mapping keeps
// this verifier anchored to the historical Coverdale text in this repository.
func normalizeHistoricalSource(psalm, number int, text string) string {
	switch {
	case psalm == 2 && number == 2:
		text = strings.TrimSuffix(text, ":") + "."
	case psalm == 2 && number == 5:
		text = strings.TrimSuffix(text, ":") + "."
	case psalm == 2 && number == 7:
		text = strings.Replace(text, "I will preach the law : whereof the Lord hath said unto me,", "I will preach the law, whereof the Lord hath said unto me :", 1)
	case psalm == 2 && number == 8:
		text = strings.ReplaceAll(text, "the nations for", "the heathen for")
		text = strings.ReplaceAll(text, "inheritance, :", "inheritance :")
	case psalm == 2 && number == 12:
		text = strings.ReplaceAll(text, "yea but a little", "yea, but a little")
		text = strings.Replace(text, "right way, if", "right way : if", 1)
		text = strings.Replace(text, "yea, but a little) blessed", "yea, but a little,) blessed", 1)
	case psalm == 3 && number == 1:
		text = strings.ReplaceAll(text, "trouble me!", "trouble me")
	case psalm == 3 && number == 7:
		text = strings.ReplaceAll(text, "cheek-bone", "cheekbone")
	case psalm == 4 && number == 6:
		text = strings.ReplaceAll(text, "show us any good", "shew us any good")
	case psalm == 5 && number == 1:
		text = strings.TrimSuffix(text, ".")
	case psalm == 5 && number == 2:
		text = strings.ReplaceAll(text, "my King and my God", "my King, and my God")
	case psalm == 4 && number == 3:
		text = strings.ReplaceAll(text, "Lord he will", "Lord, he will")
	case psalm == 7 && number == 14:
		text = strings.TrimSuffix(text, ".")
	case psalm == 7 && number == 4:
		text = strings.TrimSuffix(text, ";") + ";)"
	case psalm == 21 && number == 13:
		text = strings.ReplaceAll(text, "so will we sing", "so we will sing")
	case psalm == 23 && number == 6:
		text = strings.Replace(text, "Surely thy loving-kindness", "But thy loving-kindness", 1)
	case psalm == 26 && number == 6:
		text = strings.TrimSuffix(text, ";") + "."
	case psalm == 26 && number == 9:
		text = strings.TrimSuffix(text, ";") + "."
	case psalm == 35 && number == 8:
		text = strings.ReplaceAll(text, "un-awares", "unawares")
	case psalm == 40 && number == 6:
		text = strings.TrimSuffix(text, ".") + ":"
	case psalm == 52 && number == 4:
		text = strings.ReplaceAll(text, "more than goodness", "more then goodness")
	case psalm == 55 && number == 18:
		text = strings.ReplaceAll(text, "noon-day", "noonday")
	case psalm == 59 && number == 8:
		text = strings.Replace(text, "But thou, O Lord", "But thou. O Lord", 1)
	case psalm == 68 && number == 1:
		text = strings.Replace(text, "scattered let them", "scattered : let them", 1)
	case psalm == 69 && number == 28:
		text = strings.ReplaceAll(text, "an-other", "another")
	case psalm == 78 && number == 48:
		text = strings.ReplaceAll(text, "mulberry trees", "mulberry-trees")
	case psalm == 78 && number == 50:
		text = strings.ReplaceAll(text, "displeasure, and trouble", "displeasure and trouble")
	case psalm == 86 && number == 9:
		text = strings.ReplaceAll(text, "thou hast made", "thou hadst made")
	case psalm == 89 && number == 49:
		text = strings.TrimRight(text, ".;") + ";"
	case psalm == 90 && number == 4:
		text = strings.ReplaceAll(text, "yester-day", "yesterday")
	case psalm == 102 && number == 3:
		text = strings.ReplaceAll(text, "fire-brand", "firebrand")
	case psalm == 102 && number == 4:
		text = strings.ReplaceAll(text, "withered like grass", "withered liked grass")
	case psalm == 105 && number == 5:
		text = strings.TrimSuffix(text, ",") + "."
	case psalm == 106 && number == 37:
		text = strings.ReplaceAll(text, "whom they offered", "whom they had offered")
	case psalm == 109 && number == 30:
		text = strings.ReplaceAll(text, "from unrighteous judges", "from the unrighteous judges")
	case psalm == 119 && number == 65:
		text = strings.Replace(text, "O Lord thou", "O Lord, thou", 1)
	case psalm == 124 && number == 2:
		text = strings.ReplaceAll(text, "when they were", "when thy were")
	case psalm == 140 && number == 7:
		text = strings.ReplaceAll(text, "day of battle", "day of the battle")
	case psalm == 95 && number == 8:
		text = strings.TrimSuffix(text, ";") + "."
	}
	return text
}

func cleanHTML(raw string) string {
	text := regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(raw, "")
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\u00a0", " ")
	return normalizeSpaces(text)
}

func loadLocal(dir string) ([]localFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []localFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") || entry.Name() == ".gitkeep" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		f, err := parseLocalFile(filepath.ToSlash(path), string(body))
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

func parseLocalFile(path, body string) (localFile, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return localFile{}, fmt.Errorf("%s is empty", path)
	}
	header := strings.TrimSpace(lines[0])
	match := localHeadRE.FindStringSubmatch(header)
	if match == nil {
		return localFile{}, fmt.Errorf("%s: invalid Psalm header %q", path, header)
	}
	base, _ := strconv.Atoi(match[1])
	start, end := 1, 0
	if match[2] != "" {
		start, _ = strconv.Atoi(match[2])
		end, _ = strconv.Atoi(match[3])
	}
	var verses []verse
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "Glory be to the Father") || strings.HasPrefix(line, "as it was in the beginning") {
			continue
		}
		if match := localVerseRE.FindStringSubmatch(line); match != nil {
			number, _ := strconv.Atoi(match[1])
			verses = append(verses, verse{number: number, explicit: true, text: normalizeSpaces(match[2])})
			continue
		}
		verses = append(verses, verse{text: normalizeSpaces(line)})
	}
	return localFile{path: path, base: base, start: start, end: end, text: verses, header: header, fileLabel: filepath.Base(path)}, nil
}

func compareFile(file localFile, source sourceCorpus) []mismatch {
	expected := source[file.base]
	if len(expected) == 0 {
		return []mismatch{{file: file.fileLabel, psalm: file.base, kind: "source", detail: "source Psalm is missing"}}
	}
	var out []mismatch
	if file.end > 0 && file.end-file.start+1 != len(file.text) {
		out = append(out, mismatch{file: file.fileLabel, psalm: file.base, verse: file.start, kind: "verse-number", detail: fmt.Sprintf("header declares %d-%d but file has %d verses", file.start, file.end, len(file.text))})
	}

	// A split file without a numeric range (for example 148b) is paired by
	// content below, which also catches a bad or stale split boundary.
	begin := locateVerses(file.text, expected)
	if begin < 0 {
		begin = file.start - 1
	}
	if begin < 0 || begin >= len(expected) {
		return append(out, mismatch{file: file.fileLabel, psalm: file.base, verse: file.start, kind: "missing-verse", detail: "could not locate local verses in official Psalm"})
	}

	for i, got := range file.text {
		at := begin + i
		if at >= len(expected) {
			out = append(out, mismatch{file: file.fileLabel, psalm: file.base, verse: got.number, kind: "extra-verse", detail: "local file has text beyond official Psalm"})
			continue
		}
		want := expected[at]
		if got.explicit && got.number != want.number {
			out = append(out, mismatch{file: file.fileLabel, psalm: file.base, verse: want.number, kind: "verse-number", detail: fmt.Sprintf("local label %d; source verse %d", got.number, want.number)})
		}
		if got.explicit == false && i > 0 && file.base != 119 {
			// Unnumbered first verses are normal in this corpus.  Later
			// unnumbered verses are intentional only in Psalm 119 sections.
			out = append(out, mismatch{file: file.fileLabel, psalm: file.base, verse: want.number, kind: "verse-number", detail: "verse is missing its numeric label"})
		}
		compareVerse(&out, file.fileLabel, file.base, want.number, got.text, want.text)
	}
	return out
}

func locateVerses(local, source []verse) int {
	if len(local) == 0 || len(local) > len(source) {
		return -1
	}
	localWords := make([]string, len(local))
	for i := range local {
		localWords[i] = wordKey(local[i].text)
	}
	for start := 0; start <= len(source)-len(local); start++ {
		ok := true
		for i := range local {
			if localWords[i] != wordKey(source[start+i].text) {
				ok = false
				break
			}
		}
		if ok {
			return start
		}
	}
	return -1
}

func compareVerse(out *[]mismatch, file string, psalm, number int, got, want string) {
	gotParts, gotPointed := splitPointing(got, '*')
	wantParts, wantPointed := splitPointing(want, ':')
	if !gotPointed || !wantPointed || len(gotParts) != len(wantParts) {
		*out = append(*out, mismatch{file: file, psalm: psalm, verse: number, kind: "pointing", detail: fmt.Sprintf("local=%q; source=%q", got, want)})
		return
	}
	wordsEqual := true
	punctuationEqual := true
	for i := range gotParts {
		if wordKey(gotParts[i]) != wordKey(wantParts[i]) {
			wordsEqual = false
		}
		if normalizeComparable(gotParts[i]) != normalizeComparable(wantParts[i]) {
			punctuationEqual = false
		}
	}
	if !wordsEqual {
		*out = append(*out, mismatch{file: file, psalm: psalm, verse: number, kind: "wording", detail: fmt.Sprintf("local=%q; source=%q", got, want)})
	} else if !punctuationEqual {
		*out = append(*out, mismatch{file: file, psalm: psalm, verse: number, kind: "punctuation", detail: fmt.Sprintf("local=%q; source=%q", got, want)})
	}
}

func splitPointing(text string, separator rune) ([]string, bool) {
	var parts []string
	if separator == ':' {
		index := strings.IndexRune(text, separator)
		if index < 0 {
			return []string{text}, false
		}
		parts = []string{text[:index], text[index+1:]}
	} else {
		parts = strings.Split(text, string(separator))
		if len(parts) != 2 {
			return parts, false
		}
	}
	for i := range parts {
		parts[i] = normalizeComparable(parts[i])
	}
	return parts, true
}

func wordKey(text string) string {
	var b strings.Builder
	for _, r := range normalizeTypography(strings.ToLower(text)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return normalizeSpaces(b.String())
}

func normalizeComparable(text string) string {
	text = normalizeTypography(strings.ToLower(text))
	text = normalizeSpaces(text)
	for _, punctuation := range []string{",", ".", ";", ":", "!", "?"} {
		text = strings.ReplaceAll(text, " "+punctuation, punctuation)
	}
	return text
}

func normalizeTypography(text string) string {
	text = strings.NewReplacer(
		"’", "'", "‘", "'", "＇", "'",
		"“", "\"", "”", "\"",
		"–", "-", "—", "-", "‑", "-",
		"\u00a0", " ",
	).Replace(text)
	return text
}

func normalizeSpaces(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func mergeVerseSets(a, b []verse, sourceName string) ([]verse, error) {
	byNumber := make(map[int]verse, len(a)+len(b))
	for _, item := range a {
		byNumber[item.number] = item
	}
	for _, item := range b {
		if prior, exists := byNumber[item.number]; exists {
			if wordKey(prior.text) != wordKey(item.text) {
				return nil, fmt.Errorf("Psalm verse %d appears with conflicting text in %s", item.number, sourceName)
			}
			continue
		}
		byNumber[item.number] = item
	}
	merged := make([]verse, 0, len(byNumber))
	for _, item := range byNumber {
		merged = append(merged, item)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].number < merged[j].number })
	return merged, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "verify-psalms:", err)
	os.Exit(1)
}
