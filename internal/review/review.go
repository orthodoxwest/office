// Package review tracks human review coverage of composed office hours.
//
// The unit of review is not a calendar date but a distinct composition: the
// same celebration produces the same hour year after year, so reviewing
// "Trinity Sunday Lauds" once covers every year in which that composition
// recurs. BuildManifest sweeps several calendar years, composes every hour of
// every day, and dedupes by a content hash of the composed output (excluding
// the date). Sign-offs recorded in data/review/signoffs.txt reference that
// hash, so any later edit to the underlying texts automatically marks the
// affected units stale.
package review

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// HourNames lists the hours swept by the manifest, in liturgical order.
var HourNames = []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}

var hourOrder = map[string]int{
	"lauds": 0, "prime": 1, "terce": 2, "sext": 3, "none": 4, "vespers": 5, "compline": 6,
}

// hourTier groups hours for review ordering: 0 (Lauds and Vespers, the
// principal hours with the most proper material), 1 (Prime and Compline),
// 2 (the minor day hours, which vary least from the ordinary).
var hourTier = map[string]int{
	"lauds": 0, "vespers": 0,
	"prime": 1, "compline": 1,
	"terce": 2, "sext": 2, "none": 2,
}

// Unit is one distinct composition of one hour: the atom of human review.
type Unit struct {
	Hash        string // content hash of the composed hour (date excluded)
	Hour        string // lauds, prime, ...
	UnitKey     string // stable celebration key (feast ID, tempora slug, or feria-{season})
	Name        string // display name of the celebration
	Rank        models.Rank
	Season      models.Season
	Date        time.Time // earliest date this composition occurs in the sweep
	Occurrences int       // times this exact composition appears in the sweep
	Context     string    // octave / commemorations / I Vespers notes
}

// Priority buckets units for review ordering: A (Sundays and 1st/2nd class),
// B (greater double and double), C (everything else).
func (u *Unit) Priority() string {
	if u.Rank.Weight() >= models.Double2ndClass.Weight() || u.Date.Weekday() == time.Sunday {
		return "A"
	}
	if u.Rank.Weight() >= models.Double.Weight() {
		return "B"
	}
	return "C"
}

// URL returns the web path where this unit's representative composition renders.
func (u *Unit) URL() string {
	return "/" + u.Hour + "/" + u.Date.Format("2006-01-02")
}

// Manifest is the full set of review units for a sweep window.
type Manifest struct {
	StartYear int
	Years     int
	Units     []Unit
}

// HashHour returns a short content hash of a composed hour. The Date field is
// deliberately excluded so that identical compositions on different dates
// hash the same. Liturgical content is included; assurance metadata such as
// source keys and decision traces is excluded so observability changes do not
// invalidate a sign-off on an otherwise unchanged office.
func HashHour(h *models.OfficeHour) string {
	var b strings.Builder
	b.WriteString(h.Hour)
	b.WriteByte(0x1f)
	b.WriteString(h.Title)
	b.WriteByte(0x1f)
	b.WriteString(string(h.Season))
	b.WriteByte(0x1f)
	b.WriteString(h.Feast)
	b.WriteByte(0x1f)
	b.WriteString(string(h.Color))
	b.WriteByte(0x1e)
	for _, s := range h.Sections {
		b.WriteString(s.Label)
		b.WriteByte(0x1f)
		b.WriteString(strconv.FormatBool(s.Collapsible))
		b.WriteByte(0x1e)
		for _, e := range s.Elements {
			b.WriteString(string(e.Type))
			b.WriteByte(0x1f)
			b.WriteString(e.Label)
			b.WriteByte(0x1f)
			b.WriteString(e.Rubric)
			b.WriteByte(0x1f)
			b.WriteString(e.Text)
			b.WriteByte(0x1e)
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:12]
}

// BuildManifest sweeps years [startYear, startYear+years) and returns the
// deduplicated review units.
func BuildManifest(dataDir string, startYear, years int) (*Manifest, error) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}

	byHash := make(map[string]*Unit)
	var order []*Unit

	for y := startYear; y < startYear+years; y++ {
		moveable := calendar.ComputeMoveableDates(y)
		days, err := calendar.BuildCalendar(y, dataDir)
		if err != nil {
			return nil, fmt.Errorf("building calendar for %d: %w", y, err)
		}
		for i := range days {
			day := &days[i]
			for _, hourName := range HourNames {
				hour, err := engine.ComposeHour(hourName, day, moveable)
				if err != nil {
					return nil, fmt.Errorf("composing %s for %s: %w", hourName, day.Date.Format("2006-01-02"), err)
				}
				h := HashHour(hour)
				if u, ok := byHash[h]; ok {
					u.Occurrences++
					continue
				}
				u := &Unit{
					Hash:        h,
					Hour:        hourName,
					UnitKey:     unitKey(day, hourName),
					Name:        celebrationName(day),
					Rank:        celebrationRank(day, hourName),
					Season:      day.Season,
					Date:        day.Date,
					Occurrences: 1,
					Context:     contextNote(day, hourName),
				}
				byHash[h] = u
				order = append(order, u)
			}
		}
	}

	units := make([]Unit, 0, len(order))
	for _, u := range order {
		units = append(units, *u)
	}
	sort.Slice(units, func(i, j int) bool {
		a, b := &units[i], &units[j]
		if pa, pb := a.Priority(), b.Priority(); pa != pb {
			return pa < pb
		}
		if ta, tb := hourTier[a.Hour], hourTier[b.Hour]; ta != tb {
			return ta < tb
		}
		// Higher Occurrences means the composition recurs across many days
		// (ferial offices, commons shared by a feast category); these pay off
		// review effort faster than one-off edge cases like vigils.
		if a.Occurrences != b.Occurrences {
			return a.Occurrences > b.Occurrences
		}
		if wa, wb := a.Rank.Weight(), b.Rank.Weight(); wa != wb {
			return wa > wb
		}
		if a.UnitKey != b.UnitKey {
			return a.UnitKey < b.UnitKey
		}
		if hourOrder[a.Hour] != hourOrder[b.Hour] {
			return hourOrder[a.Hour] < hourOrder[b.Hour]
		}
		return a.Date.Before(b.Date)
	})

	return &Manifest{StartYear: startYear, Years: years, Units: units}, nil
}

// unitKey returns a stable identifier for the celebration that owns this
// hour, used to relate stale sign-offs to their current units. Vespers keys
// on the office that owns the evening, since I Vespers belongs liturgically
// to the following day's feast.
func unitKey(day *models.CalendarDay, hourName string) string {
	if hourName == "vespers" && day.Vespers.Feast != nil {
		if day.Vespers.Owner == models.VespersIOfFollowing {
			return day.Vespers.Feast.ID + "-1v"
		}
		return day.Vespers.Feast.ID
	}
	if day.Celebration != nil {
		return day.Celebration.ID
	}
	if day.Tempora != "" {
		return slugify(day.Tempora)
	}
	return "feria-" + string(day.Season)
}

func celebrationName(day *models.CalendarDay) string {
	if day.Celebration != nil {
		return day.Celebration.Name
	}
	if day.Tempora != "" {
		return day.Tempora
	}
	return titleCase(string(day.Season)) + " feria"
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func celebrationRank(day *models.CalendarDay, hourName string) models.Rank {
	if hourName == "vespers" && day.Vespers.Feast != nil {
		return day.Vespers.Feast.Rank
	}
	if day.Celebration != nil {
		return day.Celebration.Rank
	}
	return ""
}

func contextNote(day *models.CalendarDay, hourName string) string {
	var parts []string
	if hourName == "vespers" && day.Vespers.Owner == models.VespersIOfFollowing && day.Vespers.Feast != nil {
		parts = append(parts, "I Vespers of "+day.Vespers.Feast.Name)
	}
	if day.WithinOctaveOf != "" {
		parts = append(parts, "within octave of "+day.WithinOctaveOf)
	}
	if len(day.Commemorations) > 0 {
		names := make([]string, 0, len(day.Commemorations))
		for _, c := range day.Commemorations {
			names = append(names, c.Name)
		}
		parts = append(parts, "comm: "+strings.Join(names, "; "))
	}
	return strings.Join(parts, " | ")
}

func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// DefaultBaseURL is the deployed site, prefixed to unit paths in the CSV so
// checklist links are clickable when pasted into a spreadsheet.
const DefaultBaseURL = "https://office.fly.dev"

// WriteCSV writes the manifest as a reviewer-facing checklist, one row per
// unit. baseURL (no trailing slash) is prefixed to each unit's path; if
// empty, DefaultBaseURL is used.
func WriteCSV(m *Manifest, w io.Writer, baseURL string) error {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	cw := csv.NewWriter(w)
	header := []string{"priority", "hash", "hour", "date", "unit_key", "celebration", "rank", "season", "context", "occurrences", "url"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for i := range m.Units {
		u := &m.Units[i]
		rank := ""
		if u.Rank != "" {
			rank = u.Rank.Abbrev()
		}
		row := []string{
			u.Priority(),
			u.Hash,
			u.Hour,
			u.Date.Format("2006-01-02"),
			u.UnitKey,
			u.Name,
			rank,
			string(u.Season),
			u.Context,
			strconv.Itoa(u.Occurrences),
			baseURL + u.URL(),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
