//go:build ignore

// Script to fetch the Coverdale Psalter from the 1662 BCP and split into individual files.
// Source: https://www.eskimo.com/~lhowell/bcp1662/psalter/
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// Actual HTML format: <strong><a name="N">Psalm N</a></strong>
	psalmHeadingRE = regexp.MustCompile(`<a name="(\d+)">Psalm \d+\s*</a>`)
	// HTML tags
	tagRE = regexp.MustCompile(`<[^>]+>`)
	// Multiple spaces
	multiSpaceRE = regexp.MustCompile(`[ \t]+`)
)

func main() {
	outDir := "data/texts/psalms"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	urls := []string{
		"https://www.eskimo.com/~lhowell/bcp1662/psalter/psalms_1.html",
		"https://www.eskimo.com/~lhowell/bcp1662/psalter/psalms_2.html",
		"https://www.eskimo.com/~lhowell/bcp1662/psalter/psalms_3.html",
		"https://www.eskimo.com/~lhowell/bcp1662/psalter/psalms_4.html",
		"https://www.eskimo.com/~lhowell/bcp1662/psalter/psalms_5.html",
	}

	var allHTML string
	for _, url := range urls {
		fmt.Fprintf(os.Stderr, "Fetching %s...\n", url)
		html, err := fetch(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching %s: %v\n", url, err)
			os.Exit(1)
		}
		allHTML += html + "\n"
	}

	psalms := splitPsalms(allHTML)
	fmt.Fprintf(os.Stderr, "Found %d psalms\n", len(psalms))

	if len(psalms) != 150 {
		fmt.Fprintf(os.Stderr, "WARNING: expected 150 psalms, got %d\n", len(psalms))
		// List missing
		for i := 1; i <= 150; i++ {
			if _, ok := psalms[i]; !ok {
				fmt.Fprintf(os.Stderr, "  Missing: Psalm %d\n", i)
			}
		}
	}

	for num, text := range psalms {
		filename := filepath.Join(outDir, fmt.Sprintf("%03d.txt", num))
		text = strings.TrimSpace(text) + "\n"
		if err := os.WriteFile(filename, []byte(text), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", filename, err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "Done. Wrote %d psalm files to %s\n", len(psalms), outDir)
}

func fetch(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func splitPsalms(html string) map[int]string {
	matches := psalmHeadingRE.FindAllStringSubmatchIndex(html, -1)
	if len(matches) == 0 {
		fmt.Fprintln(os.Stderr, "No psalm headings found!")
		os.Exit(1)
	}

	psalms := make(map[int]string)

	for i, match := range matches {
		numStr := html[match[2]:match[3]]
		num := 0
		fmt.Sscanf(numStr, "%d", &num)

		// Extract from end of the heading line to start of next psalm heading
		// Find the end of the current heading's </center> line
		headingEnd := match[1]

		var blockEnd int
		if i+1 < len(matches) {
			blockEnd = matches[i+1][0]
		} else {
			blockEnd = len(html)
		}

		raw := html[headingEnd:blockEnd]
		text := cleanPsalmText(raw, num)
		if text != "" {
			psalms[num] = text
		}
	}

	return psalms
}

func cleanPsalmText(raw string, psalmNum int) string {
	// Remove the Latin incipit (in <em> tags right after heading)
	// Two variants: inline ". <em>...</em></center>" and separate "<center><em>...</em></center>"
	emRE := regexp.MustCompile(`(?s)</strong>\.?\s*<em>[^<]*</em>\s*</center>`)
	raw = emRE.ReplaceAllString(raw, "")
	emRE2 := regexp.MustCompile(`(?i)<center>\s*<em>[^<]*</em>\s*</center>`)
	raw = emRE2.ReplaceAllString(raw, "")

	// Remove day/prayer headers
	dayRE := regexp.MustCompile(`(?i)<p>\s*<center>\s*<strong>Day\s+\d+.*?</strong>\s*</center>`)
	raw = dayRE.ReplaceAllString(raw, "")
	prayerRE := regexp.MustCompile(`(?i)<center>\s*<strong>\s*(Morning|Evening)\s+Prayer.*?</strong>\s*</center>`)
	raw = prayerRE.ReplaceAllString(raw, "")

	// Convert drop-cap images to their alt text letter
	// Pattern: <img src="..." alt="B">LESSED → BLESSED
	imgRE := regexp.MustCompile(`<img[^>]*alt="([A-Z])"[^>]*>`)
	raw = imgRE.ReplaceAllString(raw, "$1")

	// Replace <br> with newlines
	brRE := regexp.MustCompile(`(?i)<br\s*/?>`)
	raw = brRE.ReplaceAllString(raw, "\n")

	// Replace <p> tags with newlines
	pRE := regexp.MustCompile(`(?i)</?p[^>]*>`)
	raw = pRE.ReplaceAllString(raw, "\n")

	// Remove all remaining HTML tags (strong, em, a, center, etc.)
	raw = tagRE.ReplaceAllString(raw, "")

	// Decode HTML entities
	raw = strings.ReplaceAll(raw, "&amp;", "&")
	raw = strings.ReplaceAll(raw, "&lt;", "<")
	raw = strings.ReplaceAll(raw, "&gt;", ">")
	raw = strings.ReplaceAll(raw, "&nbsp;", " ")
	raw = strings.ReplaceAll(raw, "&mdash;", "—")
	raw = strings.ReplaceAll(raw, "&ndash;", "–")

	// Collapse multiple spaces (not newlines)
	raw = multiSpaceRE.ReplaceAllString(raw, " ")

	// Split into lines and clean
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip day/prayer header remnants
		if strings.HasPrefix(line, "Day ") && strings.Contains(line, "Prayer") {
			continue
		}
		if line == "Morning Prayer." || line == "Evening Prayer." {
			continue
		}
		// Skip stray period from headings like "Psalm 119."
		if line == "." {
			continue
		}
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return ""
	}

	header := fmt.Sprintf("Psalm %d", psalmNum)
	gloria := "Glory be to the Father, and to the Son, and to the Holy Ghost;\nas it was in the beginning, is now, and ever shall be, world without end. Amen."

	return header + "\n\n" + strings.Join(lines, "\n") + "\n\n" + gloria
}
