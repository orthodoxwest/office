package review

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/texts"
)

const provenanceFile = "provenance.csv"

// ProvenanceStatus describes the strongest available evidence for a corpus
// entry. Only an explicit structured attestation may produce "verified".
type ProvenanceStatus string

const (
	ProvenanceUndocumented ProvenanceStatus = "undocumented"
	ProvenanceNeedsReview  ProvenanceStatus = "needs-review"
	ProvenanceDocumented   ProvenanceStatus = "documented"
	ProvenanceVerified     ProvenanceStatus = "verified"
)

// SourceCitation is one source claim attached to a corpus entry.
type SourceCitation struct {
	Kind    string `json:"kind"`
	Source  string `json:"source"`
	Locator string `json:"locator,omitempty"`
	Page    string `json:"page,omitempty"`
	Note    string `json:"note,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// EntryProvenance is the generated assurance record for one corpus key.
type EntryProvenance struct {
	Key         string           `json:"key"`
	File        string           `json:"file"`
	Section     string           `json:"section,omitempty"`
	Line        int              `json:"line"`
	ContentHash string           `json:"content_hash"`
	Status      ProvenanceStatus `json:"status"`
	Reviewer    string           `json:"reviewer,omitempty"`
	ReviewedOn  string           `json:"reviewed_on,omitempty"`
	Notes       string           `json:"notes,omitempty"`
	Sources     []SourceCitation `json:"sources,omitempty"`
	TODOs       []string         `json:"todos,omitempty"`
}

// ProvenanceInventory is a generated, sorted snapshot of corpus provenance.
type ProvenanceInventory struct {
	Entries []EntryProvenance `json:"entries"`
}

type attestation struct {
	Key, ContentHash, Source, Locator, Page, Status, Reviewer, ReviewedOn, Notes string
}

var pageRE = regexp.MustCompile(`(?i)\bp(?:age)?\.?\s*([0-9]+(?:[-–][0-9]+)?)`)
var pdfRE = regexp.MustCompile(`^([^\s]+\.pdf)\s*(.*)$`)

// ScanProvenance combines source comments already living beside corpus
// sections with explicit attestations from data/review/provenance.csv.
func ScanProvenance(dataDir string) (*ProvenanceInventory, error) {
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		return nil, err
	}

	entries := make(map[string]*EntryProvenance, len(corpus.Entries()))
	for key, body := range corpus.Entries() {
		entries[key] = &EntryProvenance{
			Key:         key,
			ContentHash: contentHash(body),
			Status:      ProvenanceUndocumented,
		}
	}

	textsDir := filepath.Join(dataDir, "texts")
	err = filepath.Walk(textsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".txt") {
			return nil
		}
		return scanProvenanceFile(textsDir, path, entries)
	})
	if err != nil {
		return nil, fmt.Errorf("scanning text provenance: %w", err)
	}

	for _, e := range entries {
		switch {
		case len(e.TODOs) > 0 || hasReviewPendingSource(e.Sources):
			e.Status = ProvenanceNeedsReview
		case len(e.Sources) > 0:
			e.Status = ProvenanceDocumented
		default:
			e.Status = ProvenanceUndocumented
		}
	}

	atts, err := loadAttestations(dataDir)
	if err != nil {
		return nil, err
	}
	for _, a := range atts {
		e, ok := entries[a.Key]
		if !ok {
			return nil, fmt.Errorf("provenance attestation references unknown corpus key %q", a.Key)
		}
		if a.Source != "" {
			e.Sources = append(e.Sources, SourceCitation{
				Kind: "attestation", Source: a.Source, Locator: a.Locator,
				Page: a.Page, Note: a.Notes,
			})
		}
		if a.ContentHash != "" && a.ContentHash != e.ContentHash {
			e.Status = ProvenanceNeedsReview
			e.Notes = joinNonempty(e.Notes, a.Notes, "stale attestation for content hash "+a.ContentHash)
		} else if a.Status != "" {
			e.Status = ProvenanceStatus(a.Status)
			e.Notes = a.Notes
		}
		e.Reviewer, e.ReviewedOn = a.Reviewer, a.ReviewedOn
	}

	inv := &ProvenanceInventory{Entries: make([]EntryProvenance, 0, len(entries))}
	for _, e := range entries {
		inv.Entries = append(inv.Entries, *e)
	}
	sort.Slice(inv.Entries, func(i, j int) bool { return inv.Entries[i].Key < inv.Entries[j].Key })
	return inv, nil
}

func contentHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:12]
}

func scanProvenanceFile(textsDir, path string, entries map[string]*EntryProvenance) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	rel, err := filepath.Rel(textsDir, path)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	dir := filepath.ToSlash(filepath.Dir(rel))
	stem := strings.TrimSuffix(filepath.Base(rel), ".txt")
	plainKey := strings.TrimSuffix(rel, ".txt")
	currentKey := plainKey
	hasSections := false

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		trimmed := strings.TrimSpace(scanner.Text())
		if section, ok := provenanceSection(trimmed); ok {
			hasSections = true
			if dir == "." {
				currentKey = stem + "/" + section
			} else {
				currentKey = dir + "/" + stem + "/" + section
			}
			if e := entries[currentKey]; e != nil {
				e.File, e.Section, e.Line = rel, section, lineNum
			}
			continue
		}
		if strings.HasPrefix(trimmed, "# SOURCE:") {
			if e := entries[currentKey]; e != nil {
				e.Sources = append(e.Sources, parseCitation(strings.TrimSpace(strings.TrimPrefix(trimmed, "# SOURCE:")), lineNum))
			}
		}
		if strings.Contains(trimmed, "TODO(diurnal)") {
			if e := entries[currentKey]; e != nil {
				e.TODOs = append(e.TODOs, strings.TrimSpace(strings.TrimPrefix(trimmed, "#")))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !hasSections {
		if e := entries[plainKey]; e != nil {
			e.File, e.Line = rel, 1
		}
	}
	return nil
}

func provenanceSection(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	inner := line[1 : len(line)-1]
	if inner == "" || strings.ContainsAny(inner, " :\t") {
		return "", false
	}
	return inner, true
}

func parseCitation(raw string, line int) SourceCitation {
	cite, note, _ := strings.Cut(raw, " — ")
	c := SourceCitation{Source: strings.TrimSpace(cite), Note: strings.TrimSpace(note), Line: line}
	if strings.HasPrefix(cite, "divinum-officium") {
		c.Kind = "divinum-officium"
		c.Source = "divinum-officium"
		c.Locator = strings.TrimSpace(strings.TrimPrefix(cite, "divinum-officium"))
	} else if m := pdfRE.FindStringSubmatch(cite); m != nil {
		c.Kind, c.Source, c.Locator = "local-pdf", m[1], strings.TrimSpace(m[2])
	} else {
		c.Kind = "other"
	}
	if m := pageRE.FindStringSubmatch(raw); m != nil {
		c.Page = m[1]
	}
	return c
}

func hasReviewPendingSource(sources []SourceCitation) bool {
	for _, s := range sources {
		if s.Kind == "divinum-officium" || strings.Contains(strings.ToLower(s.Note), "check against") {
			return true
		}
	}
	return false
}

func loadAttestations(dataDir string) ([]attestation, error) {
	path := filepath.Join(dataDir, "review", provenanceFile)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var out []attestation
	for i, row := range rows {
		if i == 0 || len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		if len(row) != 9 {
			return nil, fmt.Errorf("%s row %d: want 9 columns, got %d", path, i+1, len(row))
		}
		a := attestation{row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7], row[8]}
		if err := validateAttestation(a); err != nil {
			return nil, fmt.Errorf("%s row %d: %w", path, i+1, err)
		}
		out = append(out, a)
	}
	return out, nil
}

func validateAttestation(a attestation) error {
	switch ProvenanceStatus(a.Status) {
	case ProvenanceNeedsReview, ProvenanceDocumented:
		return nil
	case ProvenanceVerified:
		if a.ContentHash == "" {
			return fmt.Errorf("verified entry %q needs content_hash", a.Key)
		}
		if a.Source == "" || (a.Locator == "" && a.Page == "") {
			return fmt.Errorf("verified entry %q needs a source and locator or page", a.Key)
		}
		if a.Reviewer == "" || a.ReviewedOn == "" {
			return fmt.Errorf("verified entry %q needs reviewer and reviewed_on", a.Key)
		}
		if _, err := time.Parse("2006-01-02", a.ReviewedOn); err != nil {
			return fmt.Errorf("verified entry %q has invalid reviewed_on %q", a.Key, a.ReviewedOn)
		}
		return nil
	default:
		return fmt.Errorf("entry %q has invalid status %q", a.Key, a.Status)
	}
}

// ByKey returns a lookup copy keyed by corpus reference.
func (p *ProvenanceInventory) ByKey() map[string]EntryProvenance {
	out := make(map[string]EntryProvenance, len(p.Entries))
	for _, e := range p.Entries {
		out[e.Key] = e
	}
	return out
}

// PrintProvenanceSummary writes generated, non-stale corpus assurance counts.
func PrintProvenanceSummary(p *ProvenanceInventory, w io.Writer) {
	counts := map[ProvenanceStatus]int{}
	withPage := 0
	for _, e := range p.Entries {
		counts[e.Status]++
		for _, s := range e.Sources {
			if s.Page != "" {
				withPage++
				break
			}
		}
	}
	fmt.Fprintf(w, "=== Corpus provenance: %d entries ===\n", len(p.Entries))
	for _, status := range []ProvenanceStatus{ProvenanceVerified, ProvenanceDocumented, ProvenanceNeedsReview, ProvenanceUndocumented} {
		fmt.Fprintf(w, "  %-14s %5d\n", status, counts[status])
	}
	fmt.Fprintf(w, "  %-14s %5d\n", "page-located", withPage)
}

// WriteProvenanceCSV writes one row per entry/source claim. Entries with no
// source still receive a row so missing provenance remains machine-visible.
func WriteProvenanceCSV(p *ProvenanceInventory, w io.Writer) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"key", "file", "section", "line", "content_hash", "status", "source_kind", "source", "locator", "page", "source_line", "reviewer", "reviewed_on", "notes"})
	for _, e := range p.Entries {
		sources := e.Sources
		if len(sources) == 0 {
			sources = []SourceCitation{{}}
		}
		for _, s := range sources {
			notes := joinNonempty(append([]string{s.Note, e.Notes}, e.TODOs...)...)
			sourceLine := ""
			if s.Line > 0 {
				sourceLine = fmt.Sprint(s.Line)
			}
			_ = cw.Write([]string{e.Key, e.File, e.Section, fmt.Sprint(e.Line), e.ContentHash, string(e.Status), s.Kind, s.Source, s.Locator, s.Page, sourceLine, e.Reviewer, e.ReviewedOn, notes})
		}
	}
	cw.Flush()
	return cw.Error()
}

func joinNonempty(values ...string) string {
	var out []string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, " | ")
}
