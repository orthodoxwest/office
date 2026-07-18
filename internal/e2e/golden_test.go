// Package e2e contains end-to-end golden-file tests for the office engine.
//
// Golden files live in testdata/golden/ and are committed to the repository.
// To regenerate after intentional changes to data or logic:
//
//	go test ./internal/e2e/ -update
//
// or via the Makefile:
//
//	make golden
package e2e

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/audit"
	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/output"
	"github.com/orthodoxwest/office/internal/review"
)

var update = flag.Bool("update", false, "update golden files instead of comparing")

const (
	dataDir   = "../../data"
	goldenDir = "testdata/golden"
)

// hourCase is a single golden-file test case: one hour on one date.
type hourCase struct {
	hour string
	date string
	// note describes which conditional branch this case exercises.
	note string
}

var hourCases = []hourCase{
	// Lauds — covers the main branches in the largest hour
	{"lauds", "2026-01-06", "Epiphany — proper psalmody and short responsory"},
	{"lauds", "2026-01-11", "Sunday within the Epiphany octave — complete proper Lauds office"},
	{"lauds", "2026-01-18", "Epiphany Sunday — simplified-double commemoration suppresses suffrage"},
	{"lauds", "2026-02-08", "Septuagesima Sunday — suffrage retained"},
	{"lauds", "2026-03-01", "Lent Sunday — suffrage retained after ordinary commemoration"},
	{"lauds", "2026-03-11", "Lent feria (Wed) — weekday psalmody, preces on"},
	{"lauds", "2026-03-15", "Lent Sunday — Sunday psalmody, no preces"},
	{"lauds", "2026-03-19", "St. Joseph (double) — feast, no preces, proper antiphon"},
	{"lauds", "2026-04-12", "Easter Sunday — 1st class feast, Easter season"},
	{"lauds", "2026-04-13", "Easter Monday — within octave, no preces"},
	{"lauds", "2026-04-23", "St. George — Common of One Martyr in Paschaltide"},
	{"lauds", "2026-05-21", "Ascension Day — Ascensiontide hymn doxology"},
	{"lauds", "2026-06-07", "Trinity Sunday — 1st class feast on a Sunday, Festal psalmody only (not also Sunday psalmody)"},
	{"lauds", "2026-06-28", "Green Sunday in summer hymn window — Sunday Lauds summer hymn (Ecce iam noctis)"},

	// Vespers
	{"vespers", "2026-01-06", "Epiphany — proper short responsory"},
	{"vespers", "2026-01-10", "Saturday within the Epiphany octave — Sunday proper with Saturday psalms"},
	{"vespers", "2026-01-31", "Saturday before a per-annum Sunday — Saturday psalter and ordinary with Sunday collect"},
	{"vespers", "2026-03-07", "Saturday in Lent — Saturday psalter with Lenten chapter, responsory, hymn, and versicle"},
	{"vespers", "2026-03-15", "Lent Sunday — Sunday psalmody, Magnificat"},
	{"vespers", "2026-03-18", "I Vespers of following St. Joseph — concurrence owner"},
	{"vespers", "2026-03-19", "St. Joseph — double feast, proper antiphon"},
	{"vespers", "2027-04-17", "Saturday in Passiontide — Saturday psalter with Passiontide responsory, hymn, and versicle"},
	{"vespers", "2026-04-18", "Saturday before Low Sunday — Lord-category Sunday still uses the Saturday Paschaltide office"},
	{"vespers", "2026-04-22", "I Vespers of St. George — Common of One Martyr in Paschaltide"},
	{"vespers", "2026-04-23", "II Vespers of St. George — Common of One Martyr in Paschaltide"},
	{"vespers", "2026-06-10", "Corpus Christi I Vespers — Psalm 128 fourth"},
	{"vespers", "2026-06-11", "Corpus Christi II Vespers — Psalm 147b fourth"},
	{"vespers", "2026-06-12", "Corpus Christi octave Friday — Psalm 128 fourth"},
	{"vespers", "2026-06-15", "Corpus Christi octave Monday — Psalm 147b fourth"},
	{"vespers", "2026-06-16", "Corpus Christi octave Tuesday — Psalm 128 fourth"},
	{"vespers", "2026-06-17", "Corpus Christi octave Wednesday — Psalm 147b fourth"},
	{"vespers", "2026-06-18", "Corpus Christi octave day — Psalm 128 fourth"},
	{"vespers", "2026-09-28", "Dedication of St Michael I Vespers — Psalm 113 fourth"},
	{"vespers", "2026-09-29", "Dedication of St Michael II Vespers — Psalm 138 fourth"},
	{"vespers", "2026-09-30", "St Jerome — non-bishop Doctor common with standard festal psalms"},
	{"vespers", "2026-10-02", "Guardian Angels — generic Angel psalmody with Psalm 138 fourth"},
	{"vespers", "2026-12-02", "St Peter Chrysologus — bishop-Doctor II Vespers psalmody"},
	{"vespers", "2026-12-25", "Nativity II Vespers — Psalm 130 fourth"},

	// Vespers — Easter exercises hour-qualified hymn (vespers vs lauds)
	{"vespers", "2026-04-12", "Easter Sunday — proper vespers hymn, hour-qualified"},

	// Compline — one date per Marian antiphon + alleluia seasonal variation
	{"compline", "2026-04-11", "Easter Eve (Holy Saturday) — no hymn/chapter, Nunc Dimittis"},
	{"compline", "2026-02-15", "Septuagesima (Sexagesima Sun) — Ave Regina, Praise be"},
	{"compline", "2026-03-11", "Lent feria — Ave Regina Caelorum, Praise be"},
	{"compline", "2026-04-05", "Passiontide (Palm Sunday) — Ave Regina, Praise be"},
	{"compline", "2026-04-12", "Easter Sunday — Regina Caeli, Alleluia"},
	{"compline", "2026-07-15", "Pentecost season (Wed) — Salve Regina, Alleluia"},
	{"compline", "2026-12-10", "mid-Advent — Alma Redemptoris (Advent tone), Alleluia"},
	{"compline", "2026-12-25", "Christmas — Alma Redemptoris (Christmas tone), Alleluia"},

	// Prime
	{"prime", "2026-03-15", "Sunday — Ps 119 sections i–iii"},
	{"prime", "2026-03-16", "Monday — weekday psalms"},
	{"prime", "2026-03-17", "Tuesday — Psalm 9 ends at verse 18"},
	{"prime", "2026-03-18", "Wednesday — Psalm 9:19–20 continues directly into Psalm 10"},

	// Terce — minor hour Ps 119 sections on Sun/Mon, ordinary on Tue–Sat
	{"terce", "2026-03-15", "Sunday — Ps 119 sections iv–vi"},
	{"terce", "2026-03-17", "Tuesday — ordinary psalms 120–122"},

	// Branch coverage from review-plan (comprehensive engine branch exercises)
	{"vespers", "2026-06-14", "Branch coverage from review-plan"},
	{"lauds", "2026-04-09", "Branch coverage from review-plan"},
	{"vespers", "2026-01-19", "Branch coverage from review-plan"},
	{"prime", "2026-03-03", "Branch coverage from review-plan"},
	{"lauds", "2026-09-05", "Branch coverage from review-plan"},
	{"vespers", "2026-02-23", "Branch coverage from review-plan"},
	{"vespers", "2026-05-13", "Branch coverage from review-plan"},
	{"prime", "2026-05-08", "Branch coverage from review-plan"},
	{"vespers", "2026-04-10", "Branch coverage from review-plan"},
	{"vespers", "2026-01-14", "Branch coverage from review-plan"},
	{"vespers", "2026-12-26", "Branch coverage from review-plan"},
	{"vespers", "2026-12-18", "Branch coverage from review-plan"},
	{"vespers", "2026-06-28", "Branch coverage from review-plan"},
	{"vespers", "2026-01-17", "Branch coverage from review-plan"},
	{"vespers", "2026-04-16", "Branch coverage from review-plan"},
	{"vespers", "2026-03-11", "Branch coverage from review-plan"},
	{"lauds", "2026-03-21", "Branch coverage from review-plan"},
	{"vespers", "2026-04-19", "Branch coverage from review-plan"},
	{"compline", "2026-02-22", "Branch coverage from review-plan"},
	{"vespers", "2026-08-22", "Branch coverage from review-plan"},
	{"vespers", "2026-09-04", "Branch coverage from review-plan"},
	{"lauds", "2026-01-16", "Branch coverage from review-plan"},
	{"lauds", "2026-05-18", "Branch coverage from review-plan"},
	{"vespers", "2026-04-25", "Branch coverage from review-plan"},
	{"vespers", "2026-07-26", "Branch coverage from review-plan"},
	{"prime", "2026-04-20", "Branch coverage from review-plan"},
	{"prime", "2026-05-30", "Branch coverage from review-plan"},
	{"prime", "2026-06-03", "Branch coverage from review-plan"},
	{"terce", "2026-04-26", "Branch coverage from review-plan"},
	{"vespers", "2026-01-20", "Branch coverage from review-plan"},
	{"vespers", "2026-02-10", "Branch coverage from review-plan"},
	{"vespers", "2026-06-30", "Branch coverage from review-plan"},
	{"vespers", "2026-11-09", "Branch coverage from review-plan"},
	{"prime", "2026-02-05", "Branch coverage from review-plan"},
	{"vespers", "2026-01-03", "Branch coverage from review-plan"},
	{"vespers", "2026-02-22", "Branch coverage from review-plan"},
	{"lauds", "2026-02-25", "Branch coverage from review-plan"},
	{"vespers", "2026-02-28", "Branch coverage from review-plan"},
	{"vespers", "2026-04-06", "Branch coverage from review-plan"},
	{"lauds", "2026-04-07", "Branch coverage from review-plan"},
	{"vespers", "2026-04-07", "Branch coverage from review-plan"},
	{"vespers", "2026-04-11", "Branch coverage from review-plan"},
	{"vespers", "2026-04-29", "Branch coverage from review-plan"},
	{"vespers", "2026-05-02", "Branch coverage from review-plan"},
	{"lauds", "2026-05-30", "Branch coverage from review-plan"},
	{"lauds", "2026-12-24", "Branch coverage from review-plan"},
	{"vespers", "2026-05-25", "Branch coverage from review-plan"},
	{"lauds", "2026-06-18", "Branch coverage from review-plan"},
	{"vespers", "2026-01-02", "Branch coverage from review-plan"},
	{"lauds", "2026-05-16", "Branch coverage from review-plan"},
}

// yearData caches a built calendar and moveable dates for one civil year.
type yearData struct {
	days     []models.CalendarDay
	moveable *calendar.MoveableDates
}

func buildYear(t *testing.T, year int) *yearData {
	t.Helper()
	days, err := calendar.BuildCalendar(year, dataDir)
	if err != nil {
		t.Fatalf("BuildCalendar(%d): %v", year, err)
	}
	return &yearData{
		days:     days,
		moveable: calendar.ComputeMoveableDates(year),
	}
}

func dayFor(t *testing.T, yd *yearData, dateStr string) *models.CalendarDay {
	t.Helper()
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		t.Fatalf("parse date %q: %v", dateStr, err)
	}
	idx := d.YearDay() - 1
	if idx < 0 || idx >= len(yd.days) {
		t.Fatalf("date %s out of calendar range", dateStr)
	}
	return &yd.days[idx]
}

func TestHourGolden(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Build each required year once.
	cache := map[int]*yearData{}
	getYear := func(year int) *yearData {
		if yd, ok := cache[year]; ok {
			return yd
		}
		yd := buildYear(t, year)
		cache[year] = yd
		return yd
	}

	for _, tc := range hourCases {
		t.Run(tc.hour+"/"+tc.date, func(t *testing.T) {
			date, _ := time.Parse("2006-01-02", tc.date)
			yd := getYear(date.Year())
			day := dayFor(t, yd, tc.date)

			hour, err := eng.ComposeHour(tc.hour, day, yd.moveable)
			if err != nil {
				t.Fatalf("ComposeHour(%s, %s): %v", tc.hour, tc.date, err)
			}

			checkGolden(t, tc.hour+"-"+tc.date+".txt", output.FormatOfficeHour(hour))
		})
	}
}

func TestAuditGolden(t *testing.T) {
	r, err := audit.Run(dataDir)
	if err != nil {
		t.Fatalf("audit.Run: %v", err)
	}
	var buf bytes.Buffer
	audit.Print(r, &buf)
	checkGolden(t, "audit-report.txt", buf.String())
}

func TestOrdoGolden(t *testing.T) {
	days, err := calendar.BuildCalendar(2026, dataDir)
	if err != nil {
		t.Fatalf("BuildCalendar: %v", err)
	}
	moveable := calendar.ComputeMoveableDates(2026)
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	checkGolden(t, "ordo-2026.txt", output.FormatCalendar(days, engine, moveable))
}

func TestAssuranceGolden(t *testing.T) {
	baseline, err := review.LoadAssuranceBaseline(dataDir)
	if err != nil {
		t.Fatalf("LoadAssuranceBaseline: %v", err)
	}
	report, err := review.BuildAssuranceReport(dataDir, baseline.StartYear, baseline.Years)
	if err != nil {
		t.Fatalf("BuildAssuranceReport: %v", err)
	}
	var buf bytes.Buffer
	review.WriteAssuranceSnapshot(report, &buf)
	checkGolden(t, "assurance-report.md", buf.String())
}

func TestParityGolden(t *testing.T) {
	baseline, err := review.LoadAssuranceBaseline(dataDir)
	if err != nil {
		t.Fatalf("LoadAssuranceBaseline: %v", err)
	}
	snapshot, err := review.BuildParitySnapshot(dataDir, baseline.StartYear, baseline.Years)
	if err != nil {
		t.Fatalf("BuildParitySnapshot: %v", err)
	}
	var buf bytes.Buffer
	if err := review.WriteParitySnapshot(snapshot, &buf); err != nil {
		t.Fatalf("WriteParitySnapshot: %v", err)
	}
	checkGolden(t, "parity-snapshot.json", buf.String())
}

// checkGolden compares got against the named golden file, or writes it when -update is set.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldenDir, name)

	if *update {
		if err := os.MkdirAll(goldenDir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", goldenDir, err)
		}
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file missing: %s\n\trun: go test ./internal/e2e/ -update", path)
	}
	if got != string(want) {
		t.Errorf("golden mismatch: %s\n%s\n\trun: go test ./internal/e2e/ -update to regenerate",
			name, firstDiff(string(want), got))
	}
}

// firstDiff returns a concise description of the first line that differs.
func firstDiff(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if wl[i] != gl[i] {
			return fmt.Sprintf("first diff at line %d:\n\twant: %q\n\tgot:  %q", i+1, wl[i], gl[i])
		}
	}
	return fmt.Sprintf("line count differs: want %d lines, got %d lines", len(wl), len(gl))
}
