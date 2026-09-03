package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const corpusUsage = `Usage: office corpus <subcommand> [args]

Subcommands:
  show KEY                                  Print a corpus section body
  put KEY --file BODY.txt --source SOURCE   Replace or activate a corpus section`

var (
	corpusSegmentRE         = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	corpusLiveSectionRE     = regexp.MustCompile(`^\[([a-z0-9-]+)\]\s*$`)
	corpusCommentSectionRE  = regexp.MustCompile(`^#\s*\[([a-z0-9-]+)\]\s*$`)
	corpusAllowedNamespaces = map[string]bool{
		"proper": true, "commons": true, "seasonal": true, "ordinary": true,
		"shared": true, "psalms": true, "canticles": true,
	}
)

type corpusLocation struct {
	path    string
	section string
	plain   bool
}

type corpusLine struct {
	start int
	end   int
	text  string
	eol   string
}

func cmdCorpus(e env, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", corpusUsage)
	}
	switch args[0] {
	case "show":
		if len(args) != 2 {
			return fmt.Errorf("usage: office corpus show KEY")
		}
		body, err := readCorpusBody(e.dataDir, args[1])
		if err != nil {
			return err
		}
		fmt.Fprintln(e.out, body)
		return nil
	case "put":
		key := args[1]
		fs := e.newFlagSet("corpus put")
		bodyFile := fs.String("file", "", "file containing the replacement body")
		source := fs.String("source", "", "new source citation, without the SOURCE prefix")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *bodyFile == "" || *source == "" {
			return fmt.Errorf("usage: office corpus put KEY --file BODY.txt --source SOURCE")
		}
		body, err := os.ReadFile(*bodyFile)
		if err != nil {
			return fmt.Errorf("reading body file: %w", err)
		}
		if err := putCorpusBody(e.dataDir, key, string(body), *source); err != nil {
			return err
		}
		fmt.Fprintf(e.out, "Updated %s\n", key)
		return nil
	default:
		return fmt.Errorf("%s", corpusUsage)
	}
}

func resolveCorpusLocation(dataDir, key string) (corpusLocation, error) {
	if strings.Contains(key, "\\") || strings.HasPrefix(key, "/") {
		return corpusLocation{}, fmt.Errorf("invalid corpus key %q", key)
	}
	parts := strings.Split(key, "/")
	if len(parts) < 2 || !corpusAllowedNamespaces[parts[0]] {
		return corpusLocation{}, fmt.Errorf("unsupported corpus key %q", key)
	}
	for _, part := range parts {
		if !corpusSegmentRE.MatchString(part) {
			return corpusLocation{}, fmt.Errorf("invalid corpus key %q", key)
		}
	}

	textsDir := filepath.Join(dataDir, "texts")
	plainPath := filepath.Join(append([]string{textsDir}, parts...)...) + ".txt"
	if info, err := os.Stat(plainPath); err == nil && !info.IsDir() {
		content, readErr := os.ReadFile(plainPath)
		if readErr != nil {
			return corpusLocation{}, fmt.Errorf("reading corpus file: %w", readErr)
		}
		if !containsLiveCorpusSection(string(content)) {
			return corpusLocation{path: plainPath, plain: true}, nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		return corpusLocation{}, fmt.Errorf("stat corpus file: %w", err)
	}

	section := parts[len(parts)-1]
	sectionPath := filepath.Join(append([]string{textsDir}, parts[:len(parts)-1]...)...) + ".txt"
	if info, err := os.Stat(sectionPath); err != nil {
		if os.IsNotExist(err) {
			return corpusLocation{}, fmt.Errorf("corpus file for %q does not exist", key)
		}
		return corpusLocation{}, fmt.Errorf("stat corpus file: %w", err)
	} else if info.IsDir() {
		return corpusLocation{}, fmt.Errorf("corpus file for %q is a directory", key)
	}
	return corpusLocation{path: sectionPath, section: section}, nil
}

func containsLiveCorpusSection(content string) bool {
	for _, line := range splitCorpusLines(content) {
		if corpusLiveSectionRE.MatchString(strings.TrimSpace(line.text)) {
			return true
		}
	}
	return false
}

func splitCorpusLines(content string) []corpusLine {
	if content == "" {
		return nil
	}
	var lines []corpusLine
	for start := 0; start < len(content); {
		relative := strings.IndexByte(content[start:], '\n')
		end := len(content)
		if relative >= 0 {
			end = start + relative + 1
		}
		raw := content[start:end]
		eol := ""
		text := raw
		if strings.HasSuffix(text, "\n") {
			eol = "\n"
			text = strings.TrimSuffix(text, "\n")
			if strings.HasSuffix(text, "\r") {
				eol = "\r\n"
				text = strings.TrimSuffix(text, "\r")
			}
		}
		lines = append(lines, corpusLine{start: start, end: end, text: text, eol: eol})
		start = end
	}
	return lines
}

func stripCorpusComments(body string) string {
	var kept []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func corpusSectionBounds(content, section string) (header corpusLine, bodyEnd int, commented bool, err error) {
	lines := splitCorpusLines(content)
	matchIndex := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line.text)
		match := corpusLiveSectionRE.FindStringSubmatch(trimmed)
		isCommented := false
		if match == nil {
			match = corpusCommentSectionRE.FindStringSubmatch(trimmed)
			isCommented = match != nil
		}
		if match == nil || match[1] != section {
			continue
		}
		if matchIndex >= 0 {
			return corpusLine{}, 0, false, fmt.Errorf("duplicate corpus section [%s]", section)
		}
		matchIndex, header, commented = index, line, isCommented
	}
	if matchIndex < 0 {
		return corpusLine{}, 0, false, fmt.Errorf("corpus section [%s] does not exist", section)
	}
	bodyEnd = len(content)
	for _, line := range lines[matchIndex+1:] {
		trimmed := strings.TrimSpace(line.text)
		if corpusLiveSectionRE.MatchString(trimmed) || corpusCommentSectionRE.MatchString(trimmed) {
			bodyEnd = line.start
			break
		}
	}
	return header, bodyEnd, commented, nil
}

func readCorpusBody(dataDir, key string) (string, error) {
	location, err := resolveCorpusLocation(dataDir, key)
	if err != nil {
		return "", err
	}
	contentBytes, err := os.ReadFile(location.path)
	if err != nil {
		return "", fmt.Errorf("reading corpus file: %w", err)
	}
	content := string(contentBytes)
	if location.plain {
		body := stripCorpusComments(content)
		if body == "" {
			return "", fmt.Errorf("corpus key %q has no live body", key)
		}
		return body, nil
	}
	header, bodyEnd, commented, err := corpusSectionBounds(content, location.section)
	if err != nil {
		return "", err
	}
	if commented {
		return "", fmt.Errorf("corpus key %q is only a commented scaffold", key)
	}
	body := stripCorpusComments(content[header.end:bodyEnd])
	if body == "" {
		return "", fmt.Errorf("corpus key %q has an empty body", key)
	}
	return body, nil
}

func normalizeSourceCitation(source string) (string, error) {
	source = strings.TrimSpace(source)
	source = strings.TrimSpace(strings.TrimPrefix(source, "#"))
	if strings.HasPrefix(strings.ToUpper(source), "SOURCE:") {
		source = strings.TrimSpace(source[len("SOURCE:"):])
	}
	if source == "" || strings.ContainsAny(source, "\r\n") {
		return "", fmt.Errorf("source citation must be one non-empty line")
	}
	return source, nil
}

func replacementBody(body, source, eol string) (string, error) {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	if body == "" {
		return "", fmt.Errorf("replacement body must not be empty")
	}
	var bodyLines []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "# SOURCE:") {
			continue
		}
		if corpusLiveSectionRE.MatchString(trimmed) {
			return "", fmt.Errorf("replacement body must not contain a corpus section header")
		}
		bodyLines = append(bodyLines, line)
	}
	body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if body == "" {
		return "", fmt.Errorf("replacement body must contain liturgical text")
	}
	source, err := normalizeSourceCitation(source)
	if err != nil {
		return "", err
	}
	if eol == "" {
		eol = "\n"
	}
	body = strings.ReplaceAll(body, "\n", eol)
	return "# SOURCE: " + source + eol + body + eol + eol, nil
}

func putCorpusBody(dataDir, key, body, source string) error {
	location, err := resolveCorpusLocation(dataDir, key)
	if err != nil {
		return err
	}
	contentBytes, err := os.ReadFile(location.path)
	if err != nil {
		return fmt.Errorf("reading corpus file: %w", err)
	}
	content := string(contentBytes)
	updated := ""
	if location.plain {
		replacement, replaceErr := replacementBody(body, source, "\n")
		if replaceErr != nil {
			return replaceErr
		}
		updated = replacement
	} else {
		header, bodyEnd, commented, boundsErr := corpusSectionBounds(content, location.section)
		if boundsErr != nil {
			return boundsErr
		}
		eol := header.eol
		if eol == "" {
			eol = "\n"
		}
		replacement, replaceErr := replacementBody(body, source, eol)
		if replaceErr != nil {
			return replaceErr
		}
		headerText := content[header.start:header.end]
		if commented {
			headerText = "[" + location.section + "]" + eol
		} else if header.eol == "" {
			headerText += eol
		}
		updated = content[:header.start] + headerText + replacement + content[bodyEnd:]
	}
	info, err := os.Stat(location.path)
	if err != nil {
		return fmt.Errorf("stat corpus file: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(location.path), ".corpus-put-*")
	if err != nil {
		return fmt.Errorf("creating temporary corpus file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		temporary.Close()
		return fmt.Errorf("setting temporary corpus permissions: %w", err)
	}
	if _, err := temporary.WriteString(updated); err != nil {
		temporary.Close()
		return fmt.Errorf("writing temporary corpus file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing temporary corpus file: %w", err)
	}
	if err := os.Rename(temporaryPath, location.path); err != nil {
		return fmt.Errorf("writing corpus file: %w", err)
	}
	return nil
}
