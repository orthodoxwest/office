package dump

import (
	"fmt"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

// Format names the record schema. Bump it for any change a consumer could
// observe: a field added, removed, renamed, or given a different meaning.
const Format = "office-dump/1"

// Record kinds, in the order they appear for a date (see Generator.Generate).
const (
	KindMeta         = "meta"
	KindCalendarYear = "calendar_year"
	KindCalendarDay  = "calendar_day"
	KindOfficeDay    = "office_day"
	KindHour         = "hour"
)

const dateLayout = "2006-01-02"

// str maps Go's empty-string zero value to null.
func str[S ~string](s S) any {
	if s == "" {
		return nil
	}
	return string(s)
}

// optionalInt maps a zero integer to null, for fields where 0 means "not set".
func optionalInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func day(t time.Time) string { return t.Format(dateLayout) }

// feast serializes a Feast. Notes is omitted: it is documentation in the data
// files and never reaches the calendar, composition, or any rendered page.
func feast(f *models.Feast) any {
	if f == nil {
		return nil
	}
	return Record{
		"id":                       str(f.ID),
		"name":                     str(f.Name),
		"rank":                     str(f.Rank),
		"color":                    str(f.Color),
		"category":                 str(f.Category),
		"proper_name":              str(f.ProperName),
		"proper_id":                str(f.ProperID),
		"date_rule":                str(f.DateRule),
		"month":                    optionalInt(f.Month),
		"day":                      optionalInt(f.Day),
		"has_octave":               f.HasOctave,
		"has_vigil":                f.HasVigil,
		"octave_class":             str(f.OctaveClass),
		"commemoration_class":      str(f.CommemorationClass),
		"is_privileged_octave_day": f.IsPrivilegedOctaveDay,
		"is_vigil":                 f.IsVigil,
		"vigil_of":                 str(f.VigilOf),
		"companion_of":             str(f.CompanionOf),
		"primary_of_our_lord":      f.PrimaryOfOurLord,
		"only_with":                str(f.OnlyWith),
		"skip_roman_leap_shift":    f.SkipRomanLeapShift,
		"source":                   str(f.Source),
	}
}

func feasts(list []*models.Feast) []any {
	out := make([]any, 0, len(list))
	for _, f := range list {
		out = append(out, feast(f))
	}
	return out
}

func decisions(list []models.CompositionDecision) []any {
	out := make([]any, 0, len(list))
	for _, d := range list {
		out = append(out, Record{"rule": str(d.Rule), "outcome": str(d.Outcome), "detail": str(d.Detail)})
	}
	return out
}

func calendarYearRecord(year int, t *calendar.Tabula, m *calendar.MoveableDates) Record {
	ember := func(set calendar.EmberSet) Record {
		return Record{"wednesday": day(set.Wed), "friday": day(set.Fri), "saturday": day(set.Sat)}
	}
	return Record{
		"kind": KindCalendarYear,
		"year": year,
		"tabula": Record{
			"golden_number":           t.GoldenNumber,
			"dominical_letter":        str(t.DominicalLetter),
			"sundays_after_epiphany":  t.SundaysAfterEpiphany,
			"sundays_after_pentecost": t.SundaysAfterPentecost,
			"ember_days": Record{
				"spring": ember(t.Spring), "summer": ember(t.Summer),
				"autumn": ember(t.Autumn), "winter": ember(t.Winter),
			},
		},
		"moveable_dates": Record{
			"septuagesima": day(m.Septuagesima), "sexagesima": day(m.Sexagesima), "quinquagesima": day(m.Quinquagesima),
			"ash_wednesday": day(m.AshWednesday),
			"lent_1":        day(m.Lent1), "lent_2": day(m.Lent2), "lent_3": day(m.Lent3), "lent_4": day(m.Lent4),
			"passion_sunday": day(m.PassionSunday), "palm_sunday": day(m.PalmSunday),
			"holy_monday": day(m.HolyMonday), "holy_tuesday": day(m.HolyTuesday), "holy_wednesday": day(m.HolyWednesday),
			"holy_thursday": day(m.HolyThursday), "good_friday": day(m.GoodFriday), "holy_saturday": day(m.HolySaturday),
			"easter": day(m.Easter), "easter_monday": day(m.EasterMonday), "easter_tuesday": day(m.EasterTuesday),
			"low_sunday": day(m.LowSunday), "ascension": day(m.Ascension), "pentecost": day(m.Pentecost),
			"trinity_sunday": day(m.TrinitySunday), "corpus_christi": day(m.CorpusChristi),
			"advent_1": day(m.Advent1), "advent_2": day(m.Advent2), "advent_3": day(m.Advent3), "advent_4": day(m.Advent4),
		},
	}
}

// calendarDayRecord holds the observance of the day that every product shares:
// the Office, the Missal, the vicariate calendar feed, and the ordo.
func calendarDayRecord(d *models.CalendarDay) (Record, error) {
	// These two fields exist only on the synthetic office-day the Vespers
	// composer builds; a calendar-built day carrying them would fall outside
	// both the calendar_day and office_day records.
	if d.FirstVespers || d.FollowingOfficeCommemorationID != "" {
		return nil, fmt.Errorf("%s: calendar day carries synthetic I Vespers state", day(d.Date))
	}
	return Record{
		"kind":                 KindCalendarDay,
		"date":                 day(d.Date),
		"season":               str(d.Season),
		"tempora":              str(d.Tempora),
		"celebration":          feast(d.Celebration),
		"commemorations":       feasts(d.Commemorations),
		"feria_commemoration":  feast(d.FeriaCommemoration),
		"color":                str(d.Color),
		"notes":                str(d.Notes),
		"resolution_rule":      str(d.ResolutionRule),
		"occurrence_decisions": decisions(d.OccurrenceDecisions),
		"temporal_week_id":     str(d.TemporalWeekID),
		"within_octave_of":     str(d.WithinOctaveOf),
		"penitential": Record{
			"fast":       d.Penitential.Fast,
			"abstinence": d.Penitential.Abstinence,
		},
	}, nil
}

func vespersOwner(o models.VespersOwner) (string, error) {
	switch o {
	case models.VespersNotApplicable:
		return "not-applicable", nil
	case models.VespersIIOfPreceding:
		return "ii-of-preceding", nil
	case models.VespersIOfFollowing:
		return "i-of-following", nil
	default:
		return "", fmt.Errorf("unknown Vespers owner %d", o)
	}
}

// officeDayRecord holds the Office-only resolution of the day: concurrence and
// the Compline Marian antiphon. Mass and the calendar feed never read it.
func officeDayRecord(d *models.CalendarDay) (Record, error) {
	v := d.Vespers
	owner, err := vespersOwner(v.Owner)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", day(d.Date), err)
	}
	return Record{
		"kind":            KindOfficeDay,
		"date":            day(d.Date),
		"marian_antiphon": str(d.MarianAntiphon),
		"vespers": Record{
			"owner":                             owner,
			"feast":                             feast(v.Feast),
			"color":                             str(v.Color),
			"season":                            str(v.Season),
			"within_octave_of":                  str(v.WithinOctaveOf),
			"rule":                              str(v.Rule),
			"decisions":                         decisions(v.Decisions),
			"commemorations":                    feasts(v.Commemorations),
			"following_office_commemoration_id": str(v.FollowingOfficeCommemorationID),
			"following_office_octave_of":        str(v.FollowingOfficeOctaveOf),
			"psalmody_from_preceding":           v.PsalmodyFromPreceding,
			"appended_office_of_the_dead":       v.AppendedOfficeOfTheDead,
			"appended_feast":                    feast(v.AppendedFeast),
		},
	}, nil
}

// hourRecord holds one composed hour. Every field lands in exactly one parity
// digest; see hourFieldGroups.
func hourRecord(date time.Time, hourName string, form models.PrayerForm, h *models.OfficeHour) (Record, error) {
	if h.Form != form || day(h.Date) != day(date) {
		return nil, fmt.Errorf("%s %s %s: composer returned %s %s", day(date), hourName, form, day(h.Date), h.Form)
	}
	sections := make([]any, 0, len(h.Sections))
	for _, s := range h.Sections {
		elements := make([]any, 0, len(s.Elements))
		for _, e := range s.Elements {
			elements = append(elements, element(e))
		}
		sections = append(sections, Record{
			"label":       str(s.Label),
			"collapsible": s.Collapsible,
			"elements":    elements,
		})
	}
	return Record{
		"kind":       KindHour,
		"date":       day(date),
		"hour":       hourName,
		"form":       string(form),
		"hour_label": str(h.Hour),
		"title":      str(h.Title),
		"season":     str(h.Season),
		"feast":      str(h.Feast),
		"color":      str(h.Color),
		"sections":   sections,
		"decisions":  decisions(h.Decisions),
	}, nil
}

func element(e models.OfficeElement) Record {
	voice := make([]any, 0, len(e.Voice))
	for _, span := range e.Voice {
		voice = append(voice, Record{"text": str(span.Text), "spoken": span.Spoken, "role": str(span.Role)})
	}
	spans := make([]any, 0, len(e.RubricSpans))
	for _, span := range e.RubricSpans {
		spans = append(spans, Record{"text": str(span.Text), "prayed": span.Prayed})
	}
	return Record{
		"type":                   str(e.Type),
		"text":                   str(e.Text),
		"display_text":           str(e.DisplayText()),
		"label":                  str(e.Label),
		"incipit":                str(e.Incipit),
		"rubric":                 str(e.Rubric),
		"voice":                  voice,
		"rubric_spans":           spans,
		"leader_slot":            str(e.LeaderSlot),
		"slot_ref":               str(e.SlotRef),
		"source_ref":             str(e.SourceRef),
		"source_refs":            anyList(e.SourceRefs),
		"commemoration_owner_id": str(e.CommemorationOwnerID),
		"is_commemoration":       e.IsCommemoration,
		"announce":               e.Announce,
	}
}
