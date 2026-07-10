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

var provenanceHeader = []string{"key", "content_hash", "source", "locator", "page", "status", "reviewer", "reviewed_on", "notes"}

// ProvenanceStatus describes the strongest available evidence for a corpus
// entry. Only an explicit structured attestation may produce "verified".
type ProvenanceStatus string

const (
	ProvenanceSourceUnknown ProvenanceStatus = "source-unknown"
	ProvenanceNeedsReview   ProvenanceStatus = "needs-review"
	ProvenanceVerified      ProvenanceStatus = "verified"
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
	Stale       bool             `json:"stale,omitempty"`
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
			Status:      ProvenanceSourceUnknown,
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
		case len(e.Sources) > 0 || len(e.TODOs) > 0:
			e.Status = ProvenanceNeedsReview
		default:
			e.Status = ProvenanceSourceUnknown
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
			e.Stale = true
			e.Notes = joinNonempty(e.Notes, a.Notes, "stale attestation for an earlier text version")
		} else if a.Status != "" {
			e.Status = normalizedProvenanceStatus(a.Status)
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

// normalizedProvenanceStatus maps the retired documented/undocumented states
// if an older ledger is encountered. Source citations now remain needs-review
// until an explicit verified attestation is recorded.
func normalizedProvenanceStatus(status string) ProvenanceStatus {
	switch ProvenanceStatus(status) {
	case ProvenanceVerified:
		return ProvenanceVerified
	case ProvenanceNeedsReview, "documented":
		return ProvenanceNeedsReview
	default:
		return ProvenanceSourceUnknown
	}
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
		if i == 0 {
			if strings.Join(row, "\x1f") != strings.Join(provenanceHeader, "\x1f") {
				return nil, fmt.Errorf("%s: unexpected header", path)
			}
			continue
		}
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
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

// AttestOptions describes one word-for-word source verification. The current
// content hash is resolved from Key and recorded as an implementation detail.
type AttestOptions struct {
	Key        string
	Reviewer   string
	Source     string
	Locator    string
	Page       string
	ReviewedOn string
	Notes      string
	Replace    bool
}

// RecordAttestation validates and atomically records a verified provenance
// attestation. It never stores source-book contents.
func RecordAttestation(dataDir string, opts AttestOptions) (*EntryProvenance, error) {
	trimAttestOptions(&opts)
	if opts.Key == "" || opts.Reviewer == "" || opts.Source == "" {
		return nil, fmt.Errorf("key, reviewer, and source are required")
	}
	if opts.Page == "" && opts.Locator == "" {
		return nil, fmt.Errorf("page or locator is required")
	}
	if opts.ReviewedOn == "" {
		opts.ReviewedOn = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", opts.ReviewedOn); err != nil {
		return nil, fmt.Errorf("invalid review date %q", opts.ReviewedOn)
	}
	for name, value := range map[string]string{"reviewer": opts.Reviewer, "source": opts.Source, "locator": opts.Locator, "page": opts.Page} {
		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("%s may not contain a newline", name)
		}
	}

	inv, err := ScanProvenance(dataDir)
	if err != nil {
		return nil, err
	}
	entry, ok := inv.ByKey()[opts.Key]
	if !ok {
		return nil, fmt.Errorf("unknown corpus key %q", opts.Key)
	}
	existing, err := loadAttestations(dataDir)
	if err != nil {
		return nil, err
	}
	var kept []attestation
	found := false
	for _, a := range existing {
		if a.Key == opts.Key {
			found = true
			continue
		}
		kept = append(kept, a)
	}
	if found && !opts.Replace {
		return nil, fmt.Errorf("entry %q already has an attestation; use --replace to replace it", opts.Key)
	}
	kept = append(kept, attestation{
		Key: opts.Key, ContentHash: entry.ContentHash, Source: opts.Source, Locator: opts.Locator,
		Page: opts.Page, Status: string(ProvenanceVerified), Reviewer: opts.Reviewer,
		ReviewedOn: opts.ReviewedOn, Notes: opts.Notes,
	})
	sort.Slice(kept, func(i, j int) bool { return kept[i].Key < kept[j].Key })
	if err := writeAttestations(dataDir, kept); err != nil {
		return nil, err
	}
	result := entry
	result.Status = ProvenanceVerified
	result.Reviewer, result.ReviewedOn, result.Notes = opts.Reviewer, opts.ReviewedOn, opts.Notes
	result.Sources = append(result.Sources, SourceCitation{Kind: "attestation", Source: opts.Source, Locator: opts.Locator, Page: opts.Page, Note: opts.Notes})
	return &result, nil
}

func trimAttestOptions(opts *AttestOptions) {
	opts.Key = strings.TrimSpace(opts.Key)
	opts.Reviewer = strings.TrimSpace(opts.Reviewer)
	opts.Source = strings.TrimSpace(opts.Source)
	opts.Locator = strings.TrimSpace(opts.Locator)
	opts.Page = strings.TrimSpace(opts.Page)
	opts.ReviewedOn = strings.TrimSpace(opts.ReviewedOn)
	opts.Notes = strings.TrimSpace(opts.Notes)
}

func writeAttestations(dataDir string, attestations []attestation) error {
	dir := filepath.Join(dataDir, "review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".provenance-*.csv")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	cw := csv.NewWriter(tmp)
	if err := cw.Write(provenanceHeader); err != nil {
		return err
	}
	for _, a := range attestations {
		if err := cw.Write([]string{a.Key, a.ContentHash, a.Source, a.Locator, a.Page, a.Status, a.Reviewer, a.ReviewedOn, a.Notes}); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(dir, provenanceFile)); err != nil {
		return err
	}
	ok = true
	return nil
}

func validateAttestation(a attestation) error {
	switch ProvenanceStatus(a.Status) {
	case ProvenanceNeedsReview, "documented", ProvenanceSourceUnknown, "undocumented":
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
	stale := 0
	for _, e := range p.Entries {
		counts[e.Status]++
		if e.Stale {
			stale++
		}
		for _, s := range e.Sources {
			if s.Page != "" {
				withPage++
				break
			}
		}
	}
	fmt.Fprintf(w, "=== Corpus provenance: %d entries ===\n", len(p.Entries))
	for _, status := range []ProvenanceStatus{ProvenanceVerified, ProvenanceNeedsReview, ProvenanceSourceUnknown} {
		fmt.Fprintf(w, "  %-14s %5d\n", status, counts[status])
	}
	fmt.Fprintf(w, "  %-14s %5d\n", "page-located", withPage)
	fmt.Fprintf(w, "  %-14s %5d\n", "stale", stale)
}

// WriteProvenanceCSV writes one row per entry/source claim. Entries with no
// source still receive a row so missing provenance remains machine-visible.
func WriteProvenanceCSV(p *ProvenanceInventory, w io.Writer) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"key", "file", "section", "line", "status", "source_kind", "source", "locator", "page", "source_line", "reviewer", "reviewed_on", "notes"})
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
			_ = cw.Write([]string{e.Key, e.File, e.Section, fmt.Sprint(e.Line), string(e.Status), s.Kind, s.Source, s.Locator, s.Page, sourceLine, e.Reviewer, e.ReviewedOn, notes})
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
