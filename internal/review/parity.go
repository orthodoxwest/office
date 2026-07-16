package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// ParitySnapshot is a compact, date-sensitive contract for the calendar and
// every composed hour in a sweep. It deliberately keeps content, selected
// corpus sources, and decision traces in separate digests so a refactor cannot
// hide a provenance or observability change behind identical rendered text.
type ParitySnapshot struct {
	StartYear           int                    `json:"start_year"`
	Years               int                    `json:"years"`
	CandidateDateHours  int                    `json:"candidate_date_hours"`
	Calendars           []CalendarParityDigest `json:"calendars"`
	Hours               []HourParityDigest     `json:"hours"`
	CommemorationMerges []CommemorationMerge   `json:"commemoration_merges"`
}

// CommemorationMerge inventories every fuzzy-name suppression in the sweep.
// It is intentionally readable: these decisions deserve human review rather
// than being hidden only inside a whole-calendar digest.
type CommemorationMerge struct {
	Date    string `json:"date"`
	Surface string `json:"surface"`
	Winner  string `json:"winner,omitempty"`
	Rule    string `json:"rule"`
	Detail  string `json:"detail"`
}

// CalendarParityDigest covers the complete ordered CalendarDay model for one
// civil year, including occurrence and concurrence decisions.
type CalendarParityDigest struct {
	Year   int    `json:"year"`
	Digest string `json:"digest"`
}

// HourParityDigest covers one hour throughout one civil year. Content uses
// HashHour; Sources covers the ordered SlotRef/SourceRef/SourceRefs selection;
// Decisions covers the complete ordered decision records, including detail.
type HourParityDigest struct {
	Year      int    `json:"year"`
	Hour      string `json:"hour"`
	Content   string `json:"content"`
	Sources   string `json:"sources"`
	Decisions string `json:"decisions"`
}

type paritySource struct {
	Section    int      `json:"section"`
	Element    int      `json:"element"`
	SlotRef    string   `json:"slot_ref,omitempty"`
	SourceRef  string   `json:"source_ref,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type paritySourcesForDate struct {
	Date    string         `json:"date"`
	Sources []paritySource `json:"sources"`
}

type parityDecisionsForDate struct {
	Date      string                       `json:"date"`
	Decisions []models.CompositionDecision `json:"decisions"`
}

// BuildParitySnapshot composes every hour for every date in the requested
// window and returns compact SHA-256 digests of the ordered observable state.
func BuildParitySnapshot(dataDir string, startYear, years int) (*ParitySnapshot, error) {
	if years < 1 {
		return nil, fmt.Errorf("years must be at least 1")
	}
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}

	snapshot := &ParitySnapshot{StartYear: startYear, Years: years}
	for year := startYear; year < startYear+years; year++ {
		days, err := calendar.BuildCalendar(year, dataDir)
		if err != nil {
			return nil, fmt.Errorf("building calendar for %d: %w", year, err)
		}
		moveable := calendar.ComputeMoveableDates(year)

		calendarHash := sha256.New()
		for i := range days {
			if err := writeJSONLine(calendarHash, &days[i]); err != nil {
				return nil, fmt.Errorf("serializing calendar for %s: %w", days[i].Date.Format("2006-01-02"), err)
			}
			snapshot.CommemorationMerges = appendCommemorationMerges(snapshot.CommemorationMerges, &days[i])
		}
		snapshot.Calendars = append(snapshot.Calendars, CalendarParityDigest{
			Year: year, Digest: finishDigest(calendarHash),
		})

		for _, hourName := range HourNames {
			contentHash := sha256.New()
			sourceHash := sha256.New()
			decisionHash := sha256.New()
			for i := range days {
				day := &days[i]
				hour, err := eng.ComposeHour(hourName, day, moveable)
				if err != nil {
					return nil, fmt.Errorf("composing %s for %s: %w", hourName, day.Date.Format("2006-01-02"), err)
				}
				date := day.Date.Format("2006-01-02")
				fmt.Fprintf(contentHash, "%s\x1f%s\n", date, HashHour(hour))
				if err := writeJSONLine(sourceHash, paritySources(date, hour)); err != nil {
					return nil, fmt.Errorf("serializing sources for %s %s: %w", hourName, date, err)
				}
				if err := writeJSONLine(decisionHash, parityDecisionsForDate{Date: date, Decisions: hour.Decisions}); err != nil {
					return nil, fmt.Errorf("serializing decisions for %s %s: %w", hourName, date, err)
				}
				snapshot.CandidateDateHours++
			}
			snapshot.Hours = append(snapshot.Hours, HourParityDigest{
				Year: year, Hour: hourName,
				Content: finishDigest(contentHash), Sources: finishDigest(sourceHash), Decisions: finishDigest(decisionHash),
			})
		}
	}
	return snapshot, nil
}

func appendCommemorationMerges(merges []CommemorationMerge, day *models.CalendarDay) []CommemorationMerge {
	date := day.Date.Format("2006-01-02")
	winner := ""
	if day.Celebration != nil {
		winner = day.Celebration.ID
	}
	for _, decision := range day.OccurrenceDecisions {
		if isFuzzyCommemorationDecision(decision) {
			merges = append(merges, CommemorationMerge{
				Date: date, Surface: "occurrence", Winner: winner,
				Rule: decision.Rule, Detail: decision.Detail,
			})
		}
	}
	vespersWinner := ""
	if day.Vespers.Feast != nil {
		vespersWinner = day.Vespers.Feast.ID
	}
	for _, decision := range day.Vespers.Decisions {
		if isFuzzyCommemorationDecision(decision) {
			merges = append(merges, CommemorationMerge{
				Date: date, Surface: "vespers", Winner: vespersWinner,
				Rule: decision.Rule, Detail: decision.Detail,
			})
		}
	}
	return merges
}

func isFuzzyCommemorationDecision(decision models.CompositionDecision) bool {
	return decision.Rule == "commemoration:matches-winner" ||
		decision.Rule == "commemoration:duplicate-name"
}

func paritySources(date string, hour *models.OfficeHour) paritySourcesForDate {
	record := paritySourcesForDate{Date: date}
	for sectionIndex, section := range hour.Sections {
		for elementIndex, element := range section.Elements {
			if element.SlotRef == "" && element.SourceRef == "" && len(element.SourceRefs) == 0 {
				continue
			}
			record.Sources = append(record.Sources, paritySource{
				Section: sectionIndex, Element: elementIndex,
				SlotRef: element.SlotRef, SourceRef: element.SourceRef,
				SourceRefs: element.SourceRefs,
			})
		}
	}
	return record
}

func writeJSONLine(w io.Writer, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}

func finishDigest(h hash.Hash) string {
	return hex.EncodeToString(h.Sum(nil))
}

// WriteParitySnapshot writes the stable, human-diffable JSON representation.
func WriteParitySnapshot(snapshot *ParitySnapshot, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(snapshot)
}
