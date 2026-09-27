package dump

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"runtime"
	"slices"
	"strconv"
	"sync"

	"github.com/orthodoxwest/office/internal/models"
)

// ParityFormat names the snapshot schema written by WriteParitySnapshot.
const ParityFormat = "office-parity/2"

// ParitySnapshot fingerprints a dump. Every digest is a SHA-256 over canonical
// lines, so any implementation that produces the same dump reproduces it. The
// digests stay separate so a PR diff shows which aspect changed; `office dump
// diff` then says exactly what.
type ParitySnapshot struct {
	StartYear int // zero when the dump selected individual dates
	YearCount int
	DateHours int // hour records in the private form: one per date and hour
	// Corpus digests the corpus_entry and appointment_scope records, which
	// depend on data/ alone; empty when the dump selected no corpus group.
	Corpus string
	Years  []YearDigest
	// CommemorationMerges lists every fuzzy-name commemoration suppression.
	// They deserve human review, so they are spelled out rather than hidden
	// inside a digest.
	CommemorationMerges []CommemorationMerge
}

// YearDigest covers one civil year. Calendar digests the calendar_year and
// calendar_day lines; Office digests the office_day lines.
type YearDigest struct {
	Year     int
	Calendar string
	Office   string
	Hours    []HourDigest
}

// HourDigest covers one hour and prayer form through a year, split by the
// field groups in hourFields and friends.
type HourDigest struct {
	Hour         string
	Form         string
	Content      string
	Presentation string
	Sources      string
	Decisions    string
}

// CommemorationMerge is one fuzzy-name commemoration suppression.
type CommemorationMerge struct {
	Date    string
	Surface string // "occurrence" or "vespers"
	Winner  string
	Rule    string
	Detail  string
}

// fieldGroup assigns each hour-record field to a digest. groupKey fields
// select the digest; groupNested fields hold records whose own fields are
// assigned in the next table down.
type fieldGroup int

const (
	groupKey fieldGroup = iota
	groupNested
	groupContent
	groupPresentation
	groupSources
	groupDecisions
)

var digestGroups = []fieldGroup{groupContent, groupPresentation, groupSources, groupDecisions}

// Content matches what review hashes cover (the words and their structure);
// presentation is how renderers show them; sources is provenance; decisions
// is the composer's rule trace. A new field must be added to one of these
// tables or digesting fails.
var (
	hourFields = map[string]fieldGroup{
		"kind": groupKey, "date": groupKey, "hour": groupKey, "form": groupKey,
		"hour_label": groupContent, "title": groupContent, "season": groupContent,
		"feast": groupContent, "color": groupContent,
		"sections":  groupNested,
		"decisions": groupDecisions,
	}
	sectionFields = map[string]fieldGroup{
		"label": groupContent, "collapsible": groupContent,
		"elements": groupNested,
	}
	elementFields = map[string]fieldGroup{
		"type": groupContent, "label": groupContent, "incipit": groupContent,
		"rubric": groupContent, "text": groupContent,
		"display_text": groupPresentation, "announce": groupPresentation,
		"leader_slot": groupPresentation, "rubric_spans": groupPresentation,
		"slot_ref": groupSources, "source_ref": groupSources, "source_refs": groupSources,
		"commemoration_owner_id": groupSources, "is_commemoration": groupSources,
		"voice": groupNested,
	}
	voiceFields = map[string]fieldGroup{
		"text": groupContent, "spoken": groupContent,
		"role": groupPresentation,
	}
)

// Digester accumulates a ParitySnapshot from records in dump order.
type Digester struct {
	startYear, yearCount int
	corpus               hash.Hash
	corpusLines          int
	years                map[int]*yearDigester
}

type yearDigester struct {
	calendar, office hash.Hash
	calendarLines    int
	officeLines      int
	hours            map[[2]string]*[4]hash.Hash
	merges           []CommemorationMerge
	dateHours        int
}

// NewDigester returns an empty Digester.
func NewDigester() *Digester {
	return &Digester{corpus: sha256.New(), years: map[int]*yearDigester{}}
}

func (d *Digester) year(y int) *yearDigester {
	yd, ok := d.years[y]
	if !ok {
		yd = &yearDigester{calendar: sha256.New(), office: sha256.New(), hours: map[[2]string]*[4]hash.Hash{}}
		d.years[y] = yd
	}
	return yd
}

// Add folds one record into the snapshot.
func (d *Digester) Add(r Record) error {
	kind, _ := r["kind"].(string)
	switch kind {
	case KindMeta:
		if d.startYear == 0 && d.yearCount == 0 {
			if sel, ok := r["selection"].(map[string]any); ok {
				d.startYear, _ = asInt(sel["start_year"])
				d.yearCount, _ = asInt(sel["years"])
			}
		}
		return nil
	case KindCorpusEntry, KindAppointmentScope:
		d.corpusLines++
		return writeLine(d.corpus, r)
	case KindCalendarYear:
		y, ok := asInt(r["year"])
		if !ok {
			return fmt.Errorf("calendar_year record without a year")
		}
		yd := d.year(y)
		yd.calendarLines++
		return writeLine(yd.calendar, r)
	case KindCalendarDay, KindOfficeDay, KindHour:
	default:
		return fmt.Errorf("unknown record kind %q", kind)
	}

	date, _ := r["date"].(string)
	y, err := yearOf(date)
	if err != nil {
		return fmt.Errorf("%s record: %w", kind, err)
	}
	yd := d.year(y)
	switch kind {
	case KindCalendarDay:
		yd.calendarLines++
		yd.merges = appendMerges(yd.merges, date, "occurrence", r["celebration"], r["occurrence_decisions"])
		return writeLine(yd.calendar, r)
	case KindOfficeDay:
		yd.officeLines++
		if v, ok := r["vespers"].(map[string]any); ok {
			yd.merges = appendMerges(yd.merges, date, "vespers", v["feast"], v["decisions"])
		}
		return writeLine(yd.office, r)
	default:
		return yd.addHour(date, r)
	}
}

func (yd *yearDigester) addHour(date string, r Record) error {
	hourName, _ := r["hour"].(string)
	form, _ := r["form"].(string)
	if !slices.Contains(HourNames, hourName) || !slices.Contains(models.PrayerForms, models.PrayerForm(form)) {
		return fmt.Errorf("%s: hour record for unknown hour %q or form %q", date, hourName, form)
	}
	key := [2]string{hourName, form}
	hashes, ok := yd.hours[key]
	if !ok {
		hashes = &[4]hash.Hash{sha256.New(), sha256.New(), sha256.New(), sha256.New()}
		yd.hours[key] = hashes
	}
	for i, group := range digestGroups {
		projected, err := projectHour(r, group)
		if err != nil {
			return fmt.Errorf("%s %s %s: %w", date, hourName, form, err)
		}
		if err := writeLine(hashes[i], projected); err != nil {
			return fmt.Errorf("%s %s %s: %w", date, hourName, form, err)
		}
	}
	if form == string(models.PrayerPrivate) {
		yd.dateHours++
	}
	return nil
}

// projectHour keeps the date and the fields assigned to group, preserving the
// section/element/voice nesting so positions stay part of the digest.
func projectHour(r Record, group fieldGroup) (Record, error) {
	out, sections, err := pick(r, hourFields, group, "sections")
	if err != nil {
		return nil, err
	}
	out["date"] = r["date"]
	if group == groupDecisions {
		return out, nil
	}
	outSections := make([]any, 0, len(sections))
	for _, s := range sections {
		section, ok := s.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("section is not an object")
		}
		ps, elements, err := pick(section, sectionFields, group, "elements")
		if err != nil {
			return nil, err
		}
		outElements := make([]any, 0, len(elements))
		for _, e := range elements {
			element, ok := e.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("element is not an object")
			}
			pe, voice, err := pick(element, elementFields, group, "voice")
			if err != nil {
				return nil, err
			}
			if group == groupContent || group == groupPresentation {
				outVoice := make([]any, 0, len(voice))
				for _, v := range voice {
					span, ok := v.(map[string]any)
					if !ok {
						return nil, fmt.Errorf("voice span is not an object")
					}
					pv, _, err := pick(span, voiceFields, group, "")
					if err != nil {
						return nil, err
					}
					outVoice = append(outVoice, pv)
				}
				pe["voice"] = outVoice
			}
			outElements = append(outElements, pe)
		}
		ps["elements"] = outElements
		outSections = append(outSections, ps)
	}
	out["sections"] = outSections
	return out, nil
}

// pick copies the fields of rec assigned to group and returns the nested list
// stored under nestedKey. Any field missing from the table is an error.
func pick(rec map[string]any, table map[string]fieldGroup, group fieldGroup, nestedKey string) (Record, []any, error) {
	out := Record{}
	var nested []any
	for k, v := range rec {
		g, ok := table[k]
		if !ok {
			return nil, nil, fmt.Errorf("field %q has no parity digest assignment", k)
		}
		switch {
		case g == groupNested && k == nestedKey:
			list, ok := v.([]any)
			if !ok {
				return nil, nil, fmt.Errorf("field %q is not a list", k)
			}
			nested = list
		case g == group:
			out[k] = v
		}
	}
	return out, nested, nil
}

func appendMerges(merges []CommemorationMerge, date, surface string, winner, decisionList any) []CommemorationMerge {
	winnerID := ""
	if w, ok := winner.(map[string]any); ok {
		winnerID, _ = w["id"].(string)
	}
	list, _ := decisionList.([]any)
	for _, item := range list {
		d, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rule, _ := d["rule"].(string)
		if rule != "commemoration:matches-winner" && rule != "commemoration:duplicate-name" {
			continue
		}
		detail, _ := d["detail"].(string)
		merges = append(merges, CommemorationMerge{Date: date, Surface: surface, Winner: winnerID, Rule: rule, Detail: detail})
	}
	return merges
}

func writeLine(h hash.Hash, r Record) error {
	line, err := Marshal(r)
	if err != nil {
		return err
	}
	h.Write(line)
	h.Write([]byte{'\n'})
	return nil
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

func yearOf(date string) (int, error) {
	if len(date) != len(dateLayout) {
		return 0, fmt.Errorf("invalid date %q", date)
	}
	return strconv.Atoi(date[:4])
}

// corpusDigest is the hex corpus digest, or "" when no corpus lines were seen.
func (d *Digester) corpusDigest() string {
	if d.corpusLines == 0 {
		return ""
	}
	return hex.EncodeToString(d.corpus.Sum(nil))
}

// absorb moves other's years into d. The years must not overlap. Every part
// of a parallel digest sees the same corpus records, so their corpus
// digests must agree; d adopts it.
func (d *Digester) absorb(other *Digester) error {
	if theirs := other.corpusDigest(); theirs != "" {
		if ours := d.corpusDigest(); ours != "" && ours != theirs {
			return fmt.Errorf("corpus digests differ between parts")
		}
		if d.corpusLines == 0 {
			d.corpus, d.corpusLines = other.corpus, other.corpusLines
		}
	}
	for y, yd := range other.years {
		if _, ok := d.years[y]; ok {
			return fmt.Errorf("year %d digested twice", y)
		}
		d.years[y] = yd
	}
	return nil
}

// Snapshot finishes the digests. Years ascend; hours follow HourNames and
// forms follow models.PrayerForms.
func (d *Digester) Snapshot() *ParitySnapshot {
	s := &ParitySnapshot{StartYear: d.startYear, YearCount: d.yearCount, Corpus: d.corpusDigest()}
	years := make([]int, 0, len(d.years))
	for y := range d.years {
		years = append(years, y)
	}
	slices.Sort(years)
	for _, y := range years {
		yd := d.years[y]
		out := YearDigest{Year: y}
		if yd.calendarLines > 0 {
			out.Calendar = hex.EncodeToString(yd.calendar.Sum(nil))
		}
		if yd.officeLines > 0 {
			out.Office = hex.EncodeToString(yd.office.Sum(nil))
		}
		for _, hourName := range HourNames {
			for _, form := range models.PrayerForms {
				hashes, ok := yd.hours[[2]string{hourName, string(form)}]
				if !ok {
					continue
				}
				out.Hours = append(out.Hours, HourDigest{
					Hour: hourName, Form: string(form),
					Content:      hex.EncodeToString(hashes[0].Sum(nil)),
					Presentation: hex.EncodeToString(hashes[1].Sum(nil)),
					Sources:      hex.EncodeToString(hashes[2].Sum(nil)),
					Decisions:    hex.EncodeToString(hashes[3].Sum(nil)),
				})
			}
		}
		s.Years = append(s.Years, out)
		s.DateHours += yd.dateHours
		s.CommemorationMerges = append(s.CommemorationMerges, yd.merges...)
	}
	return s
}

// BuildParitySnapshot generates the full dump for the year window in memory
// and digests it, one worker per civil year.
func BuildParitySnapshot(dataDir string, startYear, years int) (*ParitySnapshot, error) {
	if years < 1 {
		return nil, fmt.Errorf("years must be at least 1")
	}
	gen, err := NewGenerator(dataDir)
	if err != nil {
		return nil, err
	}
	return digestYears(gen, startYear, years, Selection{})
}

// digestYears digests each civil year of the window in parallel, narrowed by
// base's Hours, Forms, and Groups. The result equals digesting one sequential
// dump of the whole window.
func digestYears(gen *Generator, startYear, years int, base Selection) (*ParitySnapshot, error) {
	parts := make([]*Digester, years)
	errs := make([]error, years)
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i := range years {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			sel := base
			sel.StartYear, sel.Years = startYear+i, 1
			parts[i] = NewDigester()
			errs[i] = gen.Generate(sel, parts[i].Add)
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	combined := NewDigester()
	combined.startYear, combined.yearCount = startYear, years
	for _, p := range parts {
		if err := combined.absorb(p); err != nil {
			return nil, err
		}
	}
	return combined.Snapshot(), nil
}

// WriteParitySnapshot writes the snapshot in canonical indented form.
func WriteParitySnapshot(s *ParitySnapshot, w io.Writer) error {
	b, err := MarshalPretty(s.record())
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func (s *ParitySnapshot) record() Record {
	years := make([]any, 0, len(s.Years))
	for _, y := range s.Years {
		hours := make([]any, 0, len(y.Hours))
		for _, h := range y.Hours {
			hours = append(hours, Record{
				"hour": h.Hour, "form": h.Form,
				"content": h.Content, "presentation": h.Presentation,
				"sources": h.Sources, "decisions": h.Decisions,
			})
		}
		years = append(years, Record{"year": y.Year, "calendar": str(y.Calendar), "office": str(y.Office), "hours": hours})
	}
	merges := make([]any, 0, len(s.CommemorationMerges))
	for _, m := range s.CommemorationMerges {
		merges = append(merges, Record{
			"date": m.Date, "surface": m.Surface, "winner": str(m.Winner),
			"rule": m.Rule, "detail": str(m.Detail),
		})
	}
	return Record{
		"format":               ParityFormat,
		"dump_format":          Format,
		"start_year":           optionalInt(s.StartYear),
		"year_count":           optionalInt(s.YearCount),
		"date_hours":           s.DateHours,
		"corpus":               str(s.Corpus),
		"years":                years,
		"commemoration_merges": merges,
	}
}
