package review

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/office"
)

// StructuralFeatureSchema is the tier-A decision/resolution feature universe
// version. Bump when new structural decision or resolution feature IDs are
// introduced so legacy sign-offs do not auto-credit branches that did not
// exist (or were not plan features) when the page was reviewed.
//
// Schema history:
//
//	0 — pre-schema sign-offs (no structural credit for residual planning)
//	1 — tier-A residual plan with preces/suffrage reasons, Marian selection
//	2 — hour-scoped dispositions, Marian boundary feature, credit gating
const StructuralFeatureSchema = 2

// Signoff records that a human reviewed one unit against the source books.
// File format (data/review/signoffs.txt), whitespace-separated:
//
//	hash hour unit-key reviewer date [schema=N] [note...]
//
// The hash binds the sign-off to the exact composition the reviewer saw; the
// hour and unit-key let a later sweep recognise the sign-off as stale (rather
// than orphaned) after the underlying texts change. schema=N gates structural
// feature credit in the residual review plan (see StructuralFeatureSchema).
type Signoff struct {
	Hash     string
	Hour     string
	UnitKey  string
	Reviewer string
	Date     string // YYYY-MM-DD
	Schema   int    // 0 = absent / legacy; see StructuralFeatureSchema
	Note     string
}

// SignoffPath returns the location of the sign-off file under dataDir.
func SignoffPath(dataDir string) string {
	return filepath.Join(dataDir, "review", "signoffs.txt")
}

// LoadSignoffs reads data/review/signoffs.txt. A missing file is not an error.
func LoadSignoffs(dataDir string) ([]Signoff, error) {
	f, err := os.Open(SignoffPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseSignoffs(f)
}

// ParseSignoffs parses sign-off records from r.
func ParseSignoffs(r io.Reader) ([]Signoff, error) {
	var signoffs []Signoff
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return nil, fmt.Errorf("signoffs line %d: want at least 5 fields (hash hour unit-key reviewer date), got %d", lineNum, len(fields))
		}
		s := Signoff{
			Hash:     fields[0],
			Hour:     fields[1],
			UnitKey:  fields[2],
			Reviewer: fields[3],
			Date:     fields[4],
		}
		rest := fields[5:]
		if len(rest) > 0 {
			if n, ok := parseSchemaToken(rest[0]); ok {
				s.Schema = n
				rest = rest[1:]
			}
		}
		if len(rest) > 0 {
			s.Note = strings.Join(rest, " ")
		}
		signoffs = append(signoffs, s)
	}
	return signoffs, scanner.Err()
}

func parseSchemaToken(tok string) (int, bool) {
	const prefix = "schema="
	if !strings.HasPrefix(tok, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(tok[len(prefix):])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// CreditsStructuralFeatures reports whether this sign-off may credit tier-A
// structural features in the residual plan. Legacy rows (schema 0) and rows
// recorded under an older feature universe do not credit, so observability
// expansions cannot silently cover unreviewed branches.
func (s Signoff) CreditsStructuralFeatures() bool {
	return s.Schema == StructuralFeatureSchema
}

// AppendSignoff appends one record to data/review/signoffs.txt, creating the
// file (and directory) if needed. New sign-offs always record the current
// StructuralFeatureSchema so residual planning can credit their features.
func AppendSignoff(dataDir string, s Signoff) error {
	path := SignoffPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if s.Schema == 0 {
		s.Schema = StructuralFeatureSchema
	}
	line := fmt.Sprintf("%s %s %s %s %s schema=%d", s.Hash, s.Hour, s.UnitKey, s.Reviewer, s.Date, s.Schema)
	if s.Note != "" {
		line += " " + s.Note
	}
	_, err = fmt.Fprintln(f, line)
	return err
}

// SignoffForPage resolves a reviewer-facing hour and date to the exact
// composition identity stored in the sign-off ledger. Reviewers never need to
// find or supply that internal identity themselves.
func SignoffForPage(dataDir, hourName string, date time.Time, reviewer, note string) (*Signoff, *Unit, error) {
	days, err := calendar.BuildCalendar(date.Year(), dataDir)
	if err != nil {
		return nil, nil, fmt.Errorf("building calendar: %w", err)
	}
	idx := date.YearDay() - 1
	if idx < 0 || idx >= len(days) {
		return nil, nil, fmt.Errorf("date out of range: %s", date.Format("2006-01-02"))
	}
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, nil, fmt.Errorf("creating office engine: %w", err)
	}
	day := &days[idx]
	hour, err := eng.ComposeHour(hourName, day, calendar.ComputeMoveableDates(date.Year()))
	if err != nil {
		return nil, nil, fmt.Errorf("composing %s: %w", hourName, err)
	}
	unit := &Unit{
		Hash: HashHour(hour), Hour: hourName, UnitKey: unitKey(day, hourName),
		Name: celebrationName(day), Rank: celebrationRank(day, hourName), Season: day.Season,
		Date: date, Occurrences: 1, Context: contextNote(day, hourName),
	}
	signoff := &Signoff{
		Hash: unit.Hash, Hour: hourName, UnitKey: unit.UnitKey,
		Reviewer: reviewer, Date: time.Now().Format("2006-01-02"),
		Schema: StructuralFeatureSchema, Note: note,
	}
	return signoff, unit, nil
}

// ReviewState classifies one unit against the recorded sign-offs.
type ReviewState int

const (
	Unreviewed ReviewState = iota // no sign-off touches this unit
	Stale                         // signed off, but the composition has since changed
	Current                       // a sign-off matches this unit's exact hash
)

func (s ReviewState) String() string {
	switch s {
	case Current:
		return "current"
	case Stale:
		return "stale"
	default:
		return "unreviewed"
	}
}

// UnitStatus pairs a unit with its review state and the most recent sign-off
// that applies to it (exact for Current, same hour+unit-key for Stale).
type UnitStatus struct {
	Unit    *Unit
	State   ReviewState
	Signoff *Signoff
}

// Classify matches sign-offs against manifest units. A unit is Current when a
// sign-off carries its exact hash. It is Stale when an *orphaned* sign-off —
// one whose hash no longer occurs anywhere in the manifest, meaning the
// underlying texts changed — exists for the same hour and unit-key. A unit
// whose sibling variant (same hour and unit-key, different commemoration or
// weekday context) was reviewed is still Unreviewed: that sign-off's hash is
// live elsewhere in the manifest and certifies only that variant.
func Classify(m *Manifest, signoffs []Signoff) []UnitStatus {
	liveHashes := make(map[string]bool, len(m.Units))
	for i := range m.Units {
		liveHashes[m.Units[i].Hash] = true
	}

	byHash := make(map[string]*Signoff)
	byKey := make(map[string]*Signoff) // hour\x1funit-key -> latest orphaned sign-off
	for i := range signoffs {
		s := &signoffs[i]
		if prev, ok := byHash[s.Hash]; !ok || s.Date > prev.Date {
			byHash[s.Hash] = s
		}
		if liveHashes[s.Hash] {
			continue
		}
		k := s.Hour + "\x1f" + s.UnitKey
		if prev, ok := byKey[k]; !ok || s.Date > prev.Date {
			byKey[k] = s
		}
	}

	statuses := make([]UnitStatus, 0, len(m.Units))
	for i := range m.Units {
		u := &m.Units[i]
		st := UnitStatus{Unit: u, State: Unreviewed}
		if s, ok := byHash[u.Hash]; ok {
			st.State = Current
			st.Signoff = s
		} else if s, ok := byKey[u.Hour+"\x1f"+u.UnitKey]; ok {
			st.State = Stale
			st.Signoff = s
		}
		statuses = append(statuses, st)
	}
	return statuses
}

// PrintStatus writes a human-readable coverage report to w.
func PrintStatus(statuses []UnitStatus, w io.Writer) {
	type bucket struct{ current, stale, unreviewed int }
	buckets := map[string]*bucket{"A": {}, "B": {}, "C": {}}
	total := bucket{}
	var stale, unreviewedA []UnitStatus

	for _, st := range statuses {
		b := buckets[st.Unit.Priority()]
		switch st.State {
		case Current:
			b.current++
			total.current++
		case Stale:
			b.stale++
			total.stale++
			stale = append(stale, st)
		default:
			b.unreviewed++
			total.unreviewed++
			if st.Unit.Priority() == "A" {
				unreviewedA = append(unreviewedA, st)
			}
		}
	}

	fmt.Fprintf(w, "=== Review coverage: %d unit(s) ===\n", len(statuses))
	fmt.Fprintf(w, "  %-10s %9s %7s %12s\n", "priority", "current", "stale", "unreviewed")
	for _, p := range []string{"A", "B", "C"} {
		b := buckets[p]
		fmt.Fprintf(w, "  %-10s %9d %7d %12d\n", p, b.current, b.stale, b.unreviewed)
	}
	fmt.Fprintf(w, "  %-10s %9d %7d %12d\n", "total", total.current, total.stale, total.unreviewed)
	fmt.Fprintln(w)

	if len(stale) > 0 {
		sortStatuses(stale)
		fmt.Fprintf(w, "=== Stale: %d unit(s) — texts changed since sign-off, need re-review ===\n", len(stale))
		for _, st := range stale {
			fmt.Fprintf(w, "  [%s] %s %s (%s)\n    %s  signed off %s by %s\n",
				st.Unit.Priority(), st.Unit.Hour, st.Unit.Name, st.Unit.UnitKey,
				st.Unit.URL(), st.Signoff.Date, st.Signoff.Reviewer)
		}
		fmt.Fprintln(w)
	}

	if len(unreviewedA) > 0 {
		sortStatuses(unreviewedA)
		fmt.Fprintf(w, "=== Unreviewed priority-A: %d unit(s) ===\n", len(unreviewedA))
		for _, st := range unreviewedA {
			fmt.Fprintf(w, "  %s %s (%s)  %s\n",
				st.Unit.Hour, st.Unit.Name, st.Unit.UnitKey, st.Unit.URL())
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Full checklist: ./office review manifest > manifest.csv")
	fmt.Fprintln(w, "Record a sign-off: ./office review sign HOUR YYYY-MM-DD REVIEWER [note...]")
}

func sortStatuses(sts []UnitStatus) {
	sort.Slice(sts, func(i, j int) bool {
		a, b := sts[i].Unit, sts[j].Unit
		if a.UnitKey != b.UnitKey {
			return a.UnitKey < b.UnitKey
		}
		return hourOrder[a.Hour] < hourOrder[b.Hour]
	})
}
