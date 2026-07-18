package review

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const zeroOccurrenceFile = "zero-occurrences.csv"

var zeroOccurrenceHeader = []string{"key", "content_hash", "disposition", "reason", "issue"}

// ZeroDisposition records why an atomic corpus entry is intentionally absent
// from a composition sweep. Unclassified is explicit: uncertainty should stay
// visible rather than being converted into a guess.
type ZeroDisposition string

const (
	ZeroShadowedFallback ZeroDisposition = "shadowed-fallback"
	ZeroDisplaced        ZeroDisposition = "displaced"
	ZeroDormantPolicy    ZeroDisposition = "dormant-policy"
	ZeroSuppressed       ZeroDisposition = "suppressed"
	ZeroDead             ZeroDisposition = "dead"
	ZeroDefect           ZeroDisposition = "defect"
	ZeroUnclassified     ZeroDisposition = "unclassified"
)

// ZeroClassification is one durable judgment bound to the corpus content on
// which it was made. A stale judgment remains reportable but is not treated as
// a current classification.
type ZeroClassification struct {
	Key         string
	ContentHash string
	Disposition ZeroDisposition
	Reason      string
	Issue       string
	Stale       bool
}

// Current reports whether the row still applies to the entry's current text.
func (c ZeroClassification) Current() bool { return !c.Stale }

// Classified reports whether the current row has a substantive disposition.
func (c ZeroClassification) Classified() bool {
	return c.Current() && c.Disposition != ZeroUnclassified
}

// LoadZeroClassifications reads and validates the zero-occurrence ledger. A
// missing file is an empty ledger. Unknown keys are errors; content edits mark
// rows stale so the old judgment remains visible without silently applying.
func LoadZeroClassifications(dataDir string, inventory *ProvenanceInventory) (map[string]ZeroClassification, error) {
	path := filepath.Join(dataDir, "review", zeroOccurrenceFile)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[string]ZeroClassification{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	byCorpusKey := inventory.ByKey()
	out := make(map[string]ZeroClassification)
	seen := map[string]int{}
	for i, row := range rows {
		if i == 0 {
			if strings.Join(row, "\x1f") != strings.Join(zeroOccurrenceHeader, "\x1f") {
				return nil, fmt.Errorf("%s: unexpected header", path)
			}
			continue
		}
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		if len(row) != len(zeroOccurrenceHeader) {
			return nil, fmt.Errorf("%s row %d: want %d columns, got %d", path, i+1, len(zeroOccurrenceHeader), len(row))
		}
		classification := ZeroClassification{
			Key: row[0], ContentHash: row[1], Disposition: ZeroDisposition(row[2]),
			Reason: row[3], Issue: row[4],
		}
		if err := validateZeroClassification(classification); err != nil {
			return nil, fmt.Errorf("%s row %d: %w", path, i+1, err)
		}
		if previous, duplicate := seen[classification.Key]; duplicate {
			return nil, fmt.Errorf("%s row %d: duplicate classification for %q (also row %d)", path, i+1, classification.Key, previous)
		}
		seen[classification.Key] = i + 1
		entry, ok := byCorpusKey[classification.Key]
		if !ok {
			return nil, fmt.Errorf("zero-occurrence classification references unknown corpus key %q", classification.Key)
		}
		classification.Stale = classification.ContentHash != entry.ContentHash
		out[classification.Key] = classification
	}
	return out, nil
}

func validateZeroClassification(c ZeroClassification) error {
	if c.Key == "" || c.ContentHash == "" || c.Reason == "" {
		return fmt.Errorf("classification for %q needs key, content_hash, and reason", c.Key)
	}
	switch c.Disposition {
	case ZeroShadowedFallback, ZeroDisplaced, ZeroDormantPolicy, ZeroSuppressed, ZeroDead, ZeroDefect, ZeroUnclassified:
	default:
		return fmt.Errorf("classification for %q has invalid disposition %q", c.Key, c.Disposition)
	}
	if c.Disposition == ZeroDefect && strings.TrimSpace(c.Issue) == "" {
		return fmt.Errorf("defect classification for %q needs an issue reference", c.Key)
	}
	for name, value := range map[string]string{"key": c.Key, "reason": c.Reason, "issue": c.Issue} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s of classification for %q may not contain a newline", name, c.Key)
		}
	}
	return nil
}

// ZeroHeuristic is a mechanical shape category used to organize manual
// classification. It deliberately describes the key, not the final cause.
type ZeroHeuristic string

const (
	ZeroGenericPsalmAntiphon ZeroHeuristic = "generic-psalm-antiphon"
	ZeroIndexedPsalmAntiphon ZeroHeuristic = "indexed-psalm-antiphon"
	ZeroCommemorationSlot    ZeroHeuristic = "commemoration-slot"
	ZeroWeekdayVariant       ZeroHeuristic = "weekday-variant"
	ZeroFirstVespersVariant  ZeroHeuristic = "first-vespers-variant"
	ZeroOther                ZeroHeuristic = "other"
)

// DetectZeroHeuristic assigns only mechanically evident key-shape categories.
// The durable disposition remains a separate, manually reviewed judgment.
func DetectZeroHeuristic(key string) ZeroHeuristic {
	section := key
	if slash := strings.LastIndexByte(key, '/'); slash >= 0 {
		section = key[slash+1:]
	}
	switch {
	case section == "psalm-antiphon":
		return ZeroGenericPsalmAntiphon
	case indexedPsalmAntiphon(section):
		return ZeroIndexedPsalmAntiphon
	case strings.HasPrefix(section, "commemoration-"):
		return ZeroCommemorationSlot
	case hasWeekdaySuffix(section):
		return ZeroWeekdayVariant
	case strings.Contains(section, "first-vespers") || strings.HasSuffix(section, "-first"):
		return ZeroFirstVespersVariant
	default:
		return ZeroOther
	}
}

func indexedPsalmAntiphon(section string) bool {
	const prefix = "psalm-antiphon-"
	if !strings.HasPrefix(section, prefix) || len(section) == len(prefix) {
		return false
	}
	for _, r := range section[len(prefix):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func hasWeekdaySuffix(section string) bool {
	for _, weekday := range []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday"} {
		if strings.HasSuffix(section, "-"+weekday) {
			return true
		}
	}
	return false
}

// ZeroOccurrenceEntry is one unverified corpus entry not selected anywhere
// in a requested sweep.
type ZeroOccurrenceEntry struct {
	QueueEntry ProvenanceQueueEntry
	Heuristic  ZeroHeuristic
}

// ZeroOccurrenceReport is the deterministic classification worklist for a
// composition sweep.
type ZeroOccurrenceReport struct {
	StartYear int
	Years     int
	Entries   []ZeroOccurrenceEntry
}

// BuildZeroOccurrenceReport reuses the provenance queue sweep so every tool
// has exactly the same definition of a zero occurrence.
func BuildZeroOccurrenceReport(dataDir string, startYear, years int) (*ZeroOccurrenceReport, error) {
	queue, err := BuildProvenanceQueue(dataDir, startYear, years, false)
	if err != nil {
		return nil, err
	}
	return zeroOccurrenceReportFromQueue(queue), nil
}

func zeroOccurrenceReportFromQueue(queue *ProvenanceQueue) *ZeroOccurrenceReport {
	report := &ZeroOccurrenceReport{StartYear: queue.StartYear, Years: queue.Years}
	for _, entry := range queue.Entries {
		if entry.Occurrences != 0 {
			continue
		}
		report.Entries = append(report.Entries, ZeroOccurrenceEntry{
			QueueEntry: entry,
			Heuristic:  DetectZeroHeuristic(entry.Key),
		})
	}
	sort.Slice(report.Entries, func(i, j int) bool {
		a, b := report.Entries[i], report.Entries[j]
		if zeroHeuristicRank(a.Heuristic) != zeroHeuristicRank(b.Heuristic) {
			return zeroHeuristicRank(a.Heuristic) < zeroHeuristicRank(b.Heuristic)
		}
		return a.QueueEntry.Key < b.QueueEntry.Key
	})
	return report
}

func zeroHeuristicRank(category ZeroHeuristic) int {
	switch category {
	case ZeroGenericPsalmAntiphon:
		return 0
	case ZeroIndexedPsalmAntiphon:
		return 1
	case ZeroCommemorationSlot:
		return 2
	case ZeroWeekdayVariant:
		return 3
	case ZeroFirstVespersVariant:
		return 4
	default:
		return 5
	}
}

// WriteZeroOccurrenceCSV writes the mechanical worklist. Content hashes are
// included here (unlike reviewer-facing queues) because they are the binding
// values needed to maintain the classification ledger.
func WriteZeroOccurrenceCSV(report *ZeroOccurrenceReport, w io.Writer) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"key", "content_hash", "status", "heuristic", "classification_state", "disposition", "issue", "reason"})
	for _, zero := range report.Entries {
		entry := zero.QueueEntry
		state, disposition, issue, reason := "missing", "", "", ""
		if entry.ZeroClassification != nil {
			classification := entry.ZeroClassification
			state = "current"
			if classification.Stale {
				state = "stale"
			}
			disposition = string(classification.Disposition)
			issue = classification.Issue
			reason = classification.Reason
		}
		_ = cw.Write([]string{
			entry.Key, entry.ContentHash, string(entry.Status), string(zero.Heuristic),
			state, disposition, issue, reason,
		})
	}
	cw.Flush()
	return cw.Error()
}

// PrintZeroOccurrenceSummary writes category and ledger coverage counts.
func PrintZeroOccurrenceSummary(report *ZeroOccurrenceReport, w io.Writer) {
	heuristics := map[ZeroHeuristic]int{}
	dispositions := map[ZeroDisposition]int{}
	classified, unclassified, stale := 0, 0, 0
	for _, zero := range report.Entries {
		heuristics[zero.Heuristic]++
		classification := zero.QueueEntry.ZeroClassification
		switch {
		case classification == nil:
			unclassified++
		case classification.Stale:
			stale++
			unclassified++
		case classification.Disposition == ZeroUnclassified:
			unclassified++
			dispositions[classification.Disposition]++
		default:
			classified++
			dispositions[classification.Disposition]++
		}
	}
	fmt.Fprintf(w, "=== Zero-occurrence corpus entries: %d-%d ===\n", report.StartYear, report.StartYear+report.Years-1)
	fmt.Fprintf(w, "  zero entries:         %d\n", len(report.Entries))
	fmt.Fprintf(w, "  classified:           %d\n", classified)
	fmt.Fprintf(w, "  need classification:  %d\n", unclassified)
	fmt.Fprintf(w, "  stale classifications:%5d\n", stale)
	for _, category := range []ZeroHeuristic{ZeroGenericPsalmAntiphon, ZeroIndexedPsalmAntiphon, ZeroCommemorationSlot, ZeroWeekdayVariant, ZeroFirstVespersVariant, ZeroOther} {
		fmt.Fprintf(w, "  heuristic %-25s %d\n", string(category)+":", heuristics[category])
	}
	for _, disposition := range []ZeroDisposition{ZeroShadowedFallback, ZeroDisplaced, ZeroDormantPolicy, ZeroSuppressed, ZeroDead, ZeroDefect, ZeroUnclassified} {
		fmt.Fprintf(w, "  disposition %-23s %d\n", string(disposition)+":", dispositions[disposition])
	}
}
