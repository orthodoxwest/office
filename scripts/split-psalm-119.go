//go:build ignore

// Script to split Psalm 119 into 22 sections of 8 verses each.
// Each section file gets the Gloria Patri appended.
//
// Usage: go run scripts/split-psalm-119.go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var sectionNames = []string{
	"i", "ii", "iii", "iv", "v", "vi", "vii", "viii",
	"ix", "x", "xi", "xii", "xiii", "xiv", "xv", "xvi",
	"xvii", "xviii", "xix", "xx", "xxi", "xxii",
}

const gloriaPetri = `
Glory be to the Father, and to the Son, and to the Holy Ghost;
as it was in the beginning, is now, and ever shall be, world without end. Amen.
`

func main() {
	psalmDir := filepath.Join("data", "texts", "psalms")
	srcPath := filepath.Join(psalmDir, "119.txt")

	f, err := os.Open(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", srcPath, err)
		os.Exit(1)
	}
	defer f.Close()

	// Parse into sections. Each section's first verse has no number prefix
	// (it starts with ALL-CAPS acrostic text). Subsequent verses start with
	// "N." where N is the verse number. A new section starts at each
	// unnumbered, non-empty line after the title.
	var sections [][]string
	var current []string

	scanner := bufio.NewScanner(f)
	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()

		// Skip the title line "Psalm 119"
		if firstLine {
			firstLine = false
			continue
		}

		// Skip empty lines between title and first verse
		if len(sections) == 0 && len(current) == 0 && strings.TrimSpace(line) == "" {
			continue
		}

		// Skip Gloria Patri at end of full psalm
		if strings.HasPrefix(line, "Glory be to the Father") {
			break
		}
		if strings.HasPrefix(line, "as it was in the beginning") {
			break
		}

		// Detect section boundary: line is non-empty and doesn't start with a digit
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && (trimmed[0] < '0' || trimmed[0] > '9') {
			if len(current) > 0 {
				sections = append(sections, current)
				current = nil
			}
		}

		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, current)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading: %v\n", err)
		os.Exit(1)
	}

	if len(sections) != 22 {
		fmt.Fprintf(os.Stderr, "Expected 22 sections, got %d\n", len(sections))
		os.Exit(1)
	}

	// Write each section file
	for i, sec := range sections {
		name := sectionNames[i]
		outPath := filepath.Join(psalmDir, fmt.Sprintf("119-%s.txt", name))

		// Section header showing which verses
		startVerse := i*8 + 1
		endVerse := startVerse + 7
		header := fmt.Sprintf("Psalm 119:%d-%d", startVerse, endVerse)

		var buf strings.Builder
		buf.WriteString(header)
		buf.WriteString("\n\n")
		for _, line := range sec {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
		buf.WriteString(strings.TrimLeft(gloriaPetri, "\n"))

		if err := os.WriteFile(outPath, []byte(buf.String()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outPath, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", outPath)
	}
}
