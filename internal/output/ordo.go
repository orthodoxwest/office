// Package output provides formatters for calendar output.
package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

var dayAbbrevs = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

func dayAbbrev(wd time.Weekday) string {
	// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6
	// We want: Monday=0, ..., Sunday=6
	idx := (int(wd) + 6) % 7
	return dayAbbrevs[idx]
}

func formatName(name string, rank models.Rank) string {
	if rank == models.Double1stClass {
		return strings.ToUpper(name)
	}
	return name
}

// commSummary is one commemoration extracted from a composed hour: its name and
// the incipit of its gospel-canticle antiphon, mirroring the ordo's
// `Comm. Name ("incipit")` form.
type commSummary struct {
	name    string
	incipit string
}

// hourSummary is the ordo-relevant digest of a single composed hour.
type hourSummary struct {
	color     models.Color
	gospelAnt string
	preces    bool
	suffrage  bool
	comms     []commSummary
}

// summarizeHour walks a composed hour and pulls out the fields the ordo prints:
// the Benedictus/Magnificat antiphon incipit, whether preces and the Suffrage
// are said, and each commemoration with its antiphon incipit.
func summarizeHour(hour *models.OfficeHour) hourSummary {
	s := hourSummary{color: hour.Color}
	for _, sec := range hour.Sections {
		if strings.Contains(sec.Label, "Suffrage") {
			s.suffrage = true
		}
		for i, el := range sec.Elements {
			switch {
			case el.Type == models.Preces:
				s.preces = true
			case el.Type == models.Heading && strings.HasPrefix(el.Text, "Commemoration of "):
				c := commSummary{name: strings.TrimPrefix(el.Text, "Commemoration of ")}
				// The commemoration antiphon is the next Antiphon element.
				for _, next := range sec.Elements[i+1:] {
					if next.Type == models.Antiphon {
						c.incipit = incipit(next.Text)
						break
					}
					if next.Type == models.Heading {
						break
					}
				}
				s.comms = append(s.comms, c)
			case s.gospelAnt == "" && (el.SlotRef == "benedictus-antiphon" || el.SlotRef == "magnificat-antiphon"):
				s.gospelAnt = incipit(el.Text)
			}
		}
	}
	return s
}

// incipit reduces an antiphon text to a short opening phrase for cross-checking
// against the ordo: it takes the text up to the mediant asterisk (or the first
// nine words) and strips trailing punctuation.
func incipit(text string) string {
	s := strings.ReplaceAll(text, "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "*"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	words := strings.Fields(s)
	truncated := false
	if len(words) > 9 {
		words = words[:9]
		truncated = true
	}
	s = strings.TrimRight(strings.Join(words, " "), " ,;:.")
	if truncated {
		s += "…"
	}
	return s
}

func precesLabel(b bool) string {
	if b {
		return "Preces"
	}
	return "No Preces"
}

func suffrageLabel(b bool) string {
	if b {
		return "Suff."
	}
	return "No Suff."
}

func joinBar(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// FormatDay formats a single CalendarDay as text line(s), enriched with the
// composed Lauds/Hours/Vespers digests so the block reads alongside the printed
// ordo. When the office engine is nil (or composition fails) it falls back to
// the calendar-only header line.
func FormatDay(day *models.CalendarDay, eng *office.Engine, moveable *calendar.MoveableDates) string {
	dow := dayAbbrev(day.Date.Weekday())
	dayNum := fmt.Sprintf("%3d", day.Date.Day())
	marker := fmt.Sprintf("%-2s", day.Penitential.Marker())
	color := day.Color.Abbrev()

	var line string
	if day.Celebration == nil {
		name := "Feria"
		if day.Tempora != "" {
			name = day.Tempora
		}
		line = fmt.Sprintf("%s  %s  %s %-42s       %s", dayNum, dow, marker, name, color)
	} else {
		feast := day.Celebration
		name := formatName(feast.Name, feast.Rank)
		rankStr := fmt.Sprintf("[%s]", feast.Rank.Abbrev())
		line = fmt.Sprintf("%s  %s  %s %-42s %s %s", dayNum, dow, marker, name, fmt.Sprintf("%-5s", rankStr), color)
	}

	lines := []string{line}
	// Commemorated concurrent/incoming feasts keep their existing summary lines.
	for _, comm := range day.Commemorations {
		lines = append(lines, fmt.Sprintf("           %s (%s)", comm.CommemorationName(), comm.Rank.Abbrev()))
	}

	const indent = "           "

	compose := func(hourName string) *hourSummary {
		if eng == nil {
			return nil
		}
		hour, err := eng.ComposeHour(hourName, day, moveable)
		if err != nil {
			return nil
		}
		s := summarizeHour(hour)
		return &s
	}

	commLines := func(comms []commSummary) []string {
		var out []string
		for _, c := range comms {
			if c.incipit != "" {
				out = append(out, fmt.Sprintf("%s    Com. %s (%q)", indent, c.name, c.incipit))
			} else {
				out = append(out, fmt.Sprintf("%s    Com. %s", indent, c.name))
			}
		}
		return out
	}

	if lauds := compose("lauds"); lauds != nil {
		ben := ""
		if lauds.gospelAnt != "" {
			ben = "Ben. " + lauds.gospelAnt
		}
		lines = append(lines, fmt.Sprintf("%sLauds   %s", indent,
			joinBar(lauds.color.Abbrev(), ben, precesLabel(lauds.preces), suffrageLabel(lauds.suffrage))))
		lines = append(lines, commLines(lauds.comms)...)
	}

	// The minor hours share one preces disposition; Prime is representative.
	if hours := compose("prime"); hours != nil {
		lines = append(lines, fmt.Sprintf("%sHours   %s", indent, precesLabel(hours.preces)))
	}

	if vespers := compose("vespers"); vespers != nil {
		owner := ""
		switch day.Vespers.Owner {
		case models.VespersIIOfPreceding:
			owner = "II prec."
		case models.VespersIOfFollowing:
			owner = fmt.Sprintf("I fol. (%s)", day.Vespers.Feast.Name)
		}
		mag := ""
		if vespers.gospelAnt != "" {
			mag = "Mag. " + vespers.gospelAnt
		}
		lines = append(lines, fmt.Sprintf("%sVespers %s", indent,
			joinBar(vespers.color.Abbrev(), owner, mag, suffrageLabel(vespers.suffrage))))
		lines = append(lines, commLines(vespers.comms)...)
	} else if day.Vespers.Owner != models.VespersNotApplicable {
		// Fallback (no engine): keep the terse owner line.
		switch day.Vespers.Owner {
		case models.VespersIIOfPreceding:
			lines = append(lines, fmt.Sprintf("%sVespers: II prec. %s", indent, day.Vespers.Color.Abbrev()))
		case models.VespersIOfFollowing:
			lines = append(lines, fmt.Sprintf("%sVespers: I fol. %s (%s)", indent, day.Vespers.Color.Abbrev(), day.Vespers.Feast.Name))
		}
	}

	return strings.Join(lines, "\n")
}

// FormatCalendar formats a full year's calendar as a text ordo, opening with the
// Tabula Temporaria and rendering each day with its composed office digests.
// eng may be nil to emit the calendar-only skeleton.
func FormatCalendar(days []models.CalendarDay, eng *office.Engine, moveable *calendar.MoveableDates) string {
	var lines []string

	if len(days) > 0 {
		lines = append(lines, FormatTabula(days[0].Date.Year(), moveable))
	}

	currentMonth := 0
	separator := strings.Repeat("-", 60)

	for i := range days {
		day := &days[i]
		month := int(day.Date.Month())
		if month != currentMonth {
			currentMonth = month
			monthName := strings.ToUpper(day.Date.Month().String())
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, monthName, separator)
		}
		lines = append(lines, FormatDay(day, eng, moveable))
	}

	return strings.Join(lines, "\n") + "\n"
}

// FormatTabula renders the Tabula Temporaria block: computus figures, the
// principal moveable feasts, and the four Ember sets.
func FormatTabula(year int, moveable *calendar.MoveableDates) string {
	t := calendar.ComputeTabula(year)
	if moveable == nil {
		moveable = calendar.ComputeMoveableDates(year)
	}

	md := func(d time.Time) string { return d.Format("January 2") }
	ember := func(e calendar.EmberSet) string {
		return fmt.Sprintf("%s, %d, %d", e.Wed.Format("January 2"), e.Fri.Day(), e.Sat.Day())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "TABULA TEMPORARIA — ANNO DOMINI %d\n", year)
	fmt.Fprintln(&b, strings.Repeat("=", 60))
	fmt.Fprintf(&b, "  %-28s %s\n", "Golden Number", calendar.Roman(t.GoldenNumber))
	fmt.Fprintf(&b, "  %-28s %s\n", "Dominical Letter", t.DominicalLetter)
	fmt.Fprintf(&b, "  %-28s %d\n", "Sundays after Epiphany", t.SundaysAfterEpiphany)
	fmt.Fprintf(&b, "  %-28s %d\n", "Sundays after Pentecost", t.SundaysAfterPentecost)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "  MOVEABLE FEASTS")
	fmt.Fprintf(&b, "    %-24s %s\n", "Septuagesima Sunday", md(moveable.Septuagesima))
	fmt.Fprintf(&b, "    %-24s %s\n", "Ash Wednesday", md(moveable.AshWednesday))
	fmt.Fprintf(&b, "    %-24s %s\n", "Easter (Pascha) Day", md(moveable.Easter))
	fmt.Fprintf(&b, "    %-24s %s\n", "Ascension Day", md(moveable.Ascension))
	fmt.Fprintf(&b, "    %-24s %s\n", "Pentecost", md(moveable.Pentecost))
	fmt.Fprintf(&b, "    %-24s %s\n", "Corpus Christi", md(moveable.CorpusChristi))
	fmt.Fprintf(&b, "    %-24s %s\n", "Advent Sunday", md(moveable.Advent1))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "  EMBER DAYS")
	fmt.Fprintf(&b, "    %-24s %s\n", "Spring (Lent)", ember(t.Spring))
	fmt.Fprintf(&b, "    %-24s %s\n", "Summer (Whitsun)", ember(t.Summer))
	fmt.Fprintf(&b, "    %-24s %s\n", "Autumn (Holy Cross)", ember(t.Autumn))
	fmt.Fprintf(&b, "    %-24s %s\n", "Winter (Advent)", ember(t.Winter))

	return b.String()
}
