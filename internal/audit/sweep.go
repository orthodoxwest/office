package audit

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// sweepHours lists every hour composed by the sweep (for not-found markers).
var sweepHours = []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}

// fallbackHours lists the hours where a Double-or-above celebration is
// expected to bring its own proper or common texts; Prime, Compline and the
// minor hours use the ordinary by design, so ordinary resolution there is
// not a finding.
var fallbackHours = map[string]bool{"lauds": true, "vespers": true}

// properizableSlots lists the slot ref bases (trailing -N stripped) that can
// carry a feast-specific text at Lauds or Vespers. Slots outside this set
// (e.g. "alleluia", "easter-antiphon-N") are fixed texts that legitimately
// resolve from the ordinary on every day.
var properizableSlots = map[string]bool{
	"psalm-antiphon":      true,
	"benedictus-antiphon": true,
	"magnificat-antiphon": true,
	"hymn":                true,
	"chapter":             true,
	"versicle":            true,
	"short-responsory":    true,
	"collect":             true,
}

// notFoundMarkerRE matches the bracketed markers the engine renders when no
// text resolves at all: "[Text not found: ref]", "[Proper text not found:
// ref]" and lookupCommemoration's "[commemoration-*: feast-id]". The
// commemoration alternative is kept narrow so "[section: …]" markup inside
// canticle texts does not match.
var notFoundMarkerRE = regexp.MustCompile(`\[(Text not found|Proper text not found|commemoration-[a-z-]+): [^\]]+\]`)

// OrdinaryFallback is one Lauds/Vespers slot on a Double-or-above day that
// rendered from the ordinary tier instead of a proper, common, or seasonal
// text.
type OrdinaryFallback struct {
	FeastID   string
	Name      string
	Rank      models.Rank
	Hour      string
	Slot      string // requested ref, e.g. "hymn", "psalm-antiphon-1"
	SourceRef string // ordinary corpus key it resolved from
	FirstDate time.Time
	Count     int // days in the sweep this fallback rendered
}

// NotFoundText is a rendered not-found marker: a ref that resolved to
// nothing anywhere in the corpus.
type NotFoundText struct {
	Hour      string
	Marker    string
	FirstDate time.Time
	Count     int
}

// SweepReport is the result of composing every hour of every day of a year
// and inspecting what actually rendered.
type SweepReport struct {
	Year              int
	NotFound          []NotFoundText
	OrdinaryFallbacks []OrdinaryFallback
}

// SweepYear composes all hours for every day of year and reports rendered
// not-found markers and ordinary-tier fallbacks on Double-or-above days.
// Findings are deduplicated across the year and suppressible per feast/slot
// via data/audit-ok.txt (slot names match hour-definition refs; a base name
// like "psalm-antiphon" suppresses all indexed variants).
func SweepYear(dataDir string, year int) (*SweepReport, error) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}
	days, err := calendar.BuildCalendar(year, dataDir)
	if err != nil {
		return nil, fmt.Errorf("building calendar for %d: %w", year, err)
	}
	suppress, err := loadSuppressFile(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading audit-ok.txt: %w", err)
	}
	moveable := calendar.ComputeMoveableDates(year)

	notFound := make(map[string]*NotFoundText)
	fallbacks := make(map[string]*OrdinaryFallback)

	for i := range days {
		day := &days[i]
		for _, hourName := range sweepHours {
			hour, err := engine.ComposeHour(hourName, day, moveable)
			if err != nil {
				return nil, fmt.Errorf("composing %s for %s: %w", hourName, day.Date.Format("2006-01-02"), err)
			}

			feast := sweepFeast(day, hourName)
			checkFallbacks := fallbackHours[hourName] && feast != nil &&
				feast.Rank.Weight() >= models.Double.Weight()

			for _, sec := range hour.Sections {
				for _, el := range sec.Elements {
					if m := notFoundMarkerRE.FindString(el.Text); m != "" {
						key := hourName + "\x1f" + m
						if f, ok := notFound[key]; ok {
							f.Count++
						} else {
							notFound[key] = &NotFoundText{Hour: hourName, Marker: m, FirstDate: day.Date, Count: 1}
						}
					}

					if !checkFallbacks || el.SlotRef == "" {
						continue
					}
					base := trimIndexSuffix(el.SlotRef)
					if !properizableSlots[base] {
						continue
					}
					if !strings.HasPrefix(el.SourceRef, "ordinary/") {
						continue
					}
					if s := suppress[feast.ID]; s["*"] || s[el.SlotRef] || s[base] {
						continue
					}
					// A line "* slot" in audit-ok.txt suppresses that slot for
					// every feast (e.g. a slot the rite always takes from the
					// ordinary at Lauds/Vespers).
					if g := suppress["*"]; g[el.SlotRef] || g[base] {
						continue
					}
					key := feast.ID + "\x1f" + hourName + "\x1f" + el.SlotRef + "\x1f" + el.SourceRef
					if f, ok := fallbacks[key]; ok {
						f.Count++
					} else {
						fallbacks[key] = &OrdinaryFallback{
							FeastID:   feast.ID,
							Name:      feast.Name,
							Rank:      feast.Rank,
							Hour:      hourName,
							Slot:      el.SlotRef,
							SourceRef: el.SourceRef,
							FirstDate: day.Date,
							Count:     1,
						}
					}
				}
			}
		}
	}

	r := &SweepReport{Year: year}
	for _, f := range notFound {
		r.NotFound = append(r.NotFound, *f)
	}
	sort.Slice(r.NotFound, func(i, j int) bool {
		if r.NotFound[i].Marker != r.NotFound[j].Marker {
			return r.NotFound[i].Marker < r.NotFound[j].Marker
		}
		return r.NotFound[i].Hour < r.NotFound[j].Hour
	})
	for _, f := range fallbacks {
		r.OrdinaryFallbacks = append(r.OrdinaryFallbacks, *f)
	}
	sort.Slice(r.OrdinaryFallbacks, func(i, j int) bool {
		a, b := &r.OrdinaryFallbacks[i], &r.OrdinaryFallbacks[j]
		if wa, wb := a.Rank.Weight(), b.Rank.Weight(); wa != wb {
			return wa > wb
		}
		if a.FeastID != b.FeastID {
			return a.FeastID < b.FeastID
		}
		if a.Hour != b.Hour {
			return a.Hour < b.Hour
		}
		return a.Slot < b.Slot
	})
	return r, nil
}

// sweepFeast returns the celebration that owns the given hour on this day:
// for Vespers the evening may belong to the following day's feast.
func sweepFeast(day *models.CalendarDay, hourName string) *models.Feast {
	if hourName == "vespers" && day.Vespers.Feast != nil {
		return day.Vespers.Feast
	}
	return day.Celebration
}

// trimIndexSuffix strips a trailing -N from an indexed slot ref
// ("psalm-antiphon-3" → "psalm-antiphon").
func trimIndexSuffix(ref string) string {
	i := len(ref) - 1
	for i >= 0 && ref[i] >= '0' && ref[i] <= '9' {
		i--
	}
	if i >= 0 && i < len(ref)-1 && ref[i] == '-' {
		return ref[:i]
	}
	return ref
}

// PrintSweep writes a human-readable sweep report to w.
func PrintSweep(r *SweepReport, w io.Writer) {
	fmt.Fprintf(w, "=== Sweep %d: unresolved texts: %d ===\n", r.Year, len(r.NotFound))
	if len(r.NotFound) > 0 {
		fmt.Fprintln(w, "These refs rendered a not-found marker on at least one day.")
		for _, f := range r.NotFound {
			fmt.Fprintf(w, "  %-8s %s (%d day(s), first %s)\n", f.Hour, f.Marker, f.Count, f.FirstDate.Format("2006-01-02"))
		}
	}
	fmt.Fprintln(w)

	// Group fallbacks by feast for readable output.
	fmt.Fprintf(w, "=== Sweep %d: ordinary fallbacks on Double+ days: %d slot(s) ===\n", r.Year, len(r.OrdinaryFallbacks))
	if len(r.OrdinaryFallbacks) > 0 {
		fmt.Fprintln(w, "Lauds/Vespers slots that rendered ordinary texts on a Double-or-above day —")
		fmt.Fprintln(w, "check the diurnal for a proper; add to data/audit-ok.txt if intentional.")
		lastFeast := ""
		for _, f := range r.OrdinaryFallbacks {
			if f.FeastID != lastFeast {
				fmt.Fprintf(w, "  [%s] %s (%s)\n", f.Rank.Abbrev(), f.Name, f.FeastID)
				lastFeast = f.FeastID
			}
			fmt.Fprintf(w, "    %-8s %-20s → %s (%d day(s), first %s)\n",
				f.Hour, f.Slot, f.SourceRef, f.Count, f.FirstDate.Format("2006-01-02"))
		}
	}
	fmt.Fprintln(w)
}
