package review

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orthodoxwest/office/internal/audit"
)

const prescreenFile = "prescreen.csv"

var prescreenHeader = []string{"key", "content_hash", "severity", "reason", "flagged", "issue"}

// PrescreenSeverity ranks how likely a pre-flagged text is to be wrong.
type PrescreenSeverity string

const (
	PrescreenHigh   PrescreenSeverity = "high"
	PrescreenMedium PrescreenSeverity = "medium"
)

// PrescreenFlag is one durable read-through finding: a corpus entry believed
// wrong before any book check, bound to the text version it was flagged
// against. The ledger records suspicions only, never source-book contents.
type PrescreenFlag struct {
	Key         string
	ContentHash string
	Severity    PrescreenSeverity
	Reason      string
	Flagged     string // batch identifier, e.g. "2026-07"
	Issue       string // optional GitHub issue number
}

// SuspicionState says whether a flag still applies as written.
type SuspicionState string

const (
	// SuspicionOpen: the flagged text is unchanged and unverified.
	SuspicionOpen SuspicionState = "open"
	// SuspicionAddressed: the text changed since it was flagged; the fix
	// still needs confirming against the book.
	SuspicionAddressed SuspicionState = "addressed"
)

// Suspicion is one reason to prioritize a corpus entry for book time,
// from either the prescreen ledger or an advisory corpus lint.
type Suspicion struct {
	Label  string // "prescreen:high", "prescreen:medium", "lint:<class>"
	State  SuspicionState
	Reason string
}

// LoadPrescreen reads and validates the durable prescreen ledger.
// A missing file is an empty ledger.
func LoadPrescreen(dataDir string) ([]PrescreenFlag, error) {
	path := filepath.Join(dataDir, "review", prescreenFile)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var out []PrescreenFlag
	seen := map[string]int{}
	for i, row := range rows {
		if i == 0 {
			if strings.Join(row, "\x1f") != strings.Join(prescreenHeader, "\x1f") {
				return nil, fmt.Errorf("%s: unexpected header", path)
			}
			continue
		}
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		if len(row) != len(prescreenHeader) {
			return nil, fmt.Errorf("%s row %d: want %d columns, got %d", path, i+1, len(prescreenHeader), len(row))
		}
		flag := PrescreenFlag{
			Key: row[0], ContentHash: row[1], Severity: PrescreenSeverity(row[2]),
			Reason: row[3], Flagged: row[4], Issue: row[5],
		}
		if err := validatePrescreenFlag(flag); err != nil {
			return nil, fmt.Errorf("%s row %d: %w", path, i+1, err)
		}
		if prev, dup := seen[flag.Key]; dup {
			return nil, fmt.Errorf("%s row %d: duplicate flag for %q (also row %d)", path, i+1, flag.Key, prev)
		}
		seen[flag.Key] = i + 1
		out = append(out, flag)
	}
	return out, nil
}

func validatePrescreenFlag(f PrescreenFlag) error {
	if f.Key == "" || f.ContentHash == "" || f.Reason == "" || f.Flagged == "" {
		return fmt.Errorf("flag for %q needs key, content_hash, reason, and flagged", f.Key)
	}
	if f.Severity != PrescreenHigh && f.Severity != PrescreenMedium {
		return fmt.Errorf("flag for %q has invalid severity %q", f.Key, f.Severity)
	}
	for name, value := range map[string]string{"key": f.Key, "reason": f.Reason, "flagged": f.Flagged, "issue": f.Issue} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s of flag for %q may not contain a newline", name, f.Key)
		}
	}
	return nil
}

// suspectLintClasses are the advisory lint classes that suggest the text
// itself is wrong and worth book time. duplicate-candidate is excluded: an
// identical pair is a deduplication task, not a transcription suspicion.
var suspectLintClasses = map[string]bool{
	"truncated":          true,
	"unpointed-antiphon": true,
	"near-duplicate":     true,
	"latin":              true,
}

// SuspicionByKey combines the prescreen ledger with advisory corpus lints
// into per-key reasons to prioritize review. Verification resolves a
// prescreen flag; an edit since flagging demotes it to "addressed" so the
// fix still gets a book check. Advisory lints are recomputed from the
// corpus, so they clear themselves when the text is fixed — but like
// prescreen flags they are silenced by an up-to-date verified attestation.
func SuspicionByKey(dataDir string, inv *ProvenanceInventory) (map[string][]Suspicion, error) {
	flags, err := LoadPrescreen(dataDir)
	if err != nil {
		return nil, err
	}
	byKey := inv.ByKey()
	out := make(map[string][]Suspicion)
	for _, f := range flags {
		entry, ok := byKey[f.Key]
		if !ok {
			return nil, fmt.Errorf("prescreen flag references unknown corpus key %q", f.Key)
		}
		if entry.Status == ProvenanceVerified {
			continue // attested word-for-word; the suspicion is resolved
		}
		state := SuspicionOpen
		if f.ContentHash != entry.ContentHash {
			state = SuspicionAddressed
		}
		out[f.Key] = append(out[f.Key], Suspicion{
			Label: "prescreen:" + string(f.Severity), State: state, Reason: f.Reason,
		})
	}

	lints, err := audit.Lint(dataDir)
	if err != nil {
		return nil, err
	}
	for _, finding := range lints.Advisory {
		if !suspectLintClasses[finding.Class] {
			continue
		}
		entry, ok := byKey[finding.Key]
		if !ok || entry.Status == ProvenanceVerified {
			continue
		}
		out[finding.Key] = append(out[finding.Key], Suspicion{
			Label: "lint:" + finding.Class, State: SuspicionOpen, Reason: finding.Detail,
		})
	}

	for key := range out {
		sort.SliceStable(out[key], func(i, j int) bool {
			return suspicionRank(out[key][i]) < suspicionRank(out[key][j])
		})
	}
	return out, nil
}

func suspicionRank(s Suspicion) int {
	switch s.Label {
	case "prescreen:" + string(PrescreenHigh):
		return 0
	case "prescreen:" + string(PrescreenMedium):
		return 1
	default:
		return 2
	}
}

// String renders one suspicion for CSV cells and reports.
func (s Suspicion) String() string {
	label := s.Label
	if s.State == SuspicionAddressed {
		label += " (addressed)"
	}
	if s.Reason == "" {
		return label
	}
	return label + " — " + s.Reason
}

// RecordPrescreenFlag validates and atomically appends one flag to the
// ledger, binding it to the entry's current content version.
func RecordPrescreenFlag(dataDir string, flag PrescreenFlag, replace bool) (*PrescreenFlag, error) {
	flag.Key = strings.TrimSpace(flag.Key)
	flag.Reason = strings.TrimSpace(flag.Reason)
	flag.Flagged = strings.TrimSpace(flag.Flagged)
	flag.Issue = strings.TrimSpace(flag.Issue)

	inv, err := ScanProvenance(dataDir)
	if err != nil {
		return nil, err
	}
	entry, ok := inv.ByKey()[flag.Key]
	if !ok {
		return nil, fmt.Errorf("unknown corpus key %q", flag.Key)
	}
	flag.ContentHash = entry.ContentHash
	if err := validatePrescreenFlag(flag); err != nil {
		return nil, err
	}

	existing, err := LoadPrescreen(dataDir)
	if err != nil {
		return nil, err
	}
	var kept []PrescreenFlag
	found := false
	for _, f := range existing {
		if f.Key == flag.Key {
			found = true
			continue
		}
		kept = append(kept, f)
	}
	if found && !replace {
		return nil, fmt.Errorf("entry %q already has a prescreen flag; use --replace to replace it", flag.Key)
	}
	kept = append(kept, flag)
	sort.Slice(kept, func(i, j int) bool { return kept[i].Key < kept[j].Key })
	if err := writePrescreen(dataDir, kept); err != nil {
		return nil, err
	}
	return &flag, nil
}

// prunePrescreenFlag removes any ledger row for key, reporting whether one
// was present. Called when an attestation lands: the verified wording
// supersedes the suspicion, and the flag's history stays in git.
func prunePrescreenFlag(dataDir, key string) (bool, error) {
	flags, err := LoadPrescreen(dataDir)
	if err != nil {
		return false, err
	}
	kept := flags[:0]
	for _, f := range flags {
		if f.Key != key {
			kept = append(kept, f)
		}
	}
	if len(kept) == len(flags) {
		return false, nil
	}
	return true, writePrescreen(dataDir, kept)
}

func writePrescreen(dataDir string, flags []PrescreenFlag) error {
	dir := filepath.Join(dataDir, "review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".prescreen-*.csv")
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
	if err := cw.Write(prescreenHeader); err != nil {
		return err
	}
	for _, f := range flags {
		if err := cw.Write([]string{f.Key, f.ContentHash, string(f.Severity), f.Reason, f.Flagged, f.Issue}); err != nil {
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
	if err := os.Rename(tmpName, filepath.Join(dir, prescreenFile)); err != nil {
		return err
	}
	ok = true
	return nil
}
