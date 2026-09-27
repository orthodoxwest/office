package dump

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

const dataDir = "../../data"

var (
	genOnce   sync.Once
	sharedGen *Generator
	genErr    error
)

func generator(t *testing.T) *Generator {
	t.Helper()
	genOnce.Do(func() { sharedGen, genErr = NewGenerator(dataDir) })
	if genErr != nil {
		t.Fatalf("NewGenerator: %v", genErr)
	}
	return sharedGen
}

func collect(t *testing.T, sel Selection) []Record {
	t.Helper()
	var out []Record
	if err := generator(t).Generate(sel, func(r Record) error { out = append(out, r); return nil }); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGenerateOrdersRecordsCanonically(t *testing.T) {
	// Lists are given out of order and with a duplicate; the stream must not
	// depend on how the selection was spelled.
	records := collect(t, Selection{
		Dates:  []time.Time{date(t, "2027-01-01"), date(t, "2026-12-25"), date(t, "2026-12-25")},
		Hours:  []string{"vespers", "lauds"},
		Forms:  []models.PrayerForm{models.PrayerPriest, models.PrayerPrivate},
		Groups: []string{GroupHours, GroupCalendar, GroupOffice},
	})
	var keys []string
	for _, r := range records {
		keys = append(keys, recordKey(r))
	}
	want := []string{
		"meta",
		"calendar_year 2026",
		"calendar_day 2026-12-25", "office_day 2026-12-25",
		"hour 2026-12-25 lauds private", "hour 2026-12-25 lauds priest",
		"hour 2026-12-25 vespers private", "hour 2026-12-25 vespers priest",
		"calendar_year 2027",
		"calendar_day 2027-01-01", "office_day 2027-01-01",
		"hour 2027-01-01 lauds private", "hour 2027-01-01 lauds priest",
		"hour 2027-01-01 vespers private", "hour 2027-01-01 vespers priest",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("record order:\n got %q\nwant %q", keys, want)
	}
	sel := records[0]["selection"].(map[string]any)
	if !reflect.DeepEqual(sel["hours"], []any{"lauds", "vespers"}) || sel["start_year"] != nil {
		t.Fatalf("meta selection = %v", sel)
	}
}

func TestGenerateGroupsNarrowTheStream(t *testing.T) {
	records := collect(t, Selection{StartYear: 2026, Years: 1, Groups: []string{GroupOffice}})
	if len(records) != 1+365 {
		t.Fatalf("got %d records, want meta + 365 office days", len(records))
	}
	for _, r := range records[1:] {
		if r["kind"] != KindOfficeDay {
			t.Fatalf("unexpected %s record", r["kind"])
		}
	}
}

func TestSelectionValidation(t *testing.T) {
	d := []time.Time{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	for name, sel := range map[string]Selection{
		"nothing selected":  {},
		"dates and window":  {Dates: d, StartYear: 2026, Years: 1},
		"zero years":        {StartYear: 2026},
		"unknown hour":      {Dates: d, Hours: []string{"matins"}},
		"unknown form":      {Dates: d, Forms: []models.PrayerForm{"choir"}},
		"unknown group":     {Dates: d, Groups: []string{"mass"}},
		"hour name casing":  {Dates: d, Hours: []string{"Lauds"}},
		"form name casing":  {Dates: d, Forms: []models.PrayerForm{"Private"}},
		"group name casing": {Dates: d, Groups: []string{"Hours"}},
	} {
		if err := generator(t).Generate(sel, func(Record) error { return nil }); err == nil {
			t.Errorf("%s: Generate succeeded, want an error", name)
		}
	}
}

func TestHourRecordFieldsAreAllAssignedToADigest(t *testing.T) {
	records := collect(t, Selection{Dates: []time.Time{date(t, "2026-11-02")}, Groups: []string{GroupHours}})
	for _, r := range records[1:] {
		for _, g := range digestGroups {
			if _, err := projectHour(r, g); err != nil {
				t.Fatalf("%s: %v", recordKey(r), err)
			}
		}
	}

	r := records[1]
	r["new_field"] = "x"
	if _, err := projectHour(r, groupContent); err == nil || !strings.Contains(err.Error(), "new_field") {
		t.Fatalf("unassigned field accepted: %v", err)
	}
}

func TestProjectionsPartitionTheRecord(t *testing.T) {
	r := Record{
		"kind": KindHour, "date": "2026-01-01", "hour": "lauds", "form": "private",
		"hour_label": "Lauds", "title": "T", "season": nil, "feast": nil, "color": "white",
		"decisions": []any{Record{"rule": "r", "outcome": "o", "detail": nil}},
		"sections": []any{Record{"label": "L", "collapsible": false, "elements": []any{Record{
			"type": "antiphon", "label": nil, "incipit": nil, "rubric": nil, "text": "Words * more",
			"display_text": "Words.", "announce": true, "leader_slot": nil, "rubric_spans": []any{},
			"slot_ref": "s", "source_ref": "p/s", "source_refs": []any{}, "commemoration_owner_id": nil,
			"is_commemoration": false,
			"voice":            []any{Record{"text": "Words * more", "spoken": true, "role": "all"}},
		}}}},
	}
	want := map[fieldGroup]string{
		groupContent: `{"color":"white","date":"2026-01-01","feast":null,"hour_label":"Lauds","season":null,` +
			`"sections":[{"collapsible":false,"elements":[{"incipit":null,"label":null,"rubric":null,` +
			`"text":"Words * more","type":"antiphon","voice":[{"spoken":true,"text":"Words * more"}]}],"label":"L"}],"title":"T"}`,
		groupPresentation: `{"date":"2026-01-01","sections":[{"elements":[{"announce":true,"display_text":"Words.",` +
			`"leader_slot":null,"rubric_spans":[],"voice":[{"role":"all"}]}]}]}`,
		groupSources: `{"date":"2026-01-01","sections":[{"elements":[{"commemoration_owner_id":null,` +
			`"is_commemoration":false,"slot_ref":"s","source_ref":"p/s","source_refs":[]}]}]}`,
		groupDecisions: `{"date":"2026-01-01","decisions":[{"detail":null,"outcome":"o","rule":"r"}]}`,
	}
	for group, w := range want {
		p, err := projectHour(r, group)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != w {
			t.Errorf("group %d:\n got %s\nwant %s", group, got, w)
		}
	}
}

func TestCalendarDayRejectsSyntheticFirstVespersState(t *testing.T) {
	d := &models.CalendarDay{Date: date(t, "2026-01-01"), FirstVespers: true}
	if _, err := calendarDayRecord(d); err == nil {
		t.Fatal("synthetic I Vespers day accepted")
	}
	d = &models.CalendarDay{Date: date(t, "2026-01-01"), Vespers: models.VespersDesignation{Owner: 9}}
	if _, err := officeDayRecord(d); err == nil {
		t.Fatal("unknown Vespers owner accepted")
	}
}

func TestDigestOfWrittenDumpMatchesInProcessDigest(t *testing.T) {
	sel := Selection{
		Dates: []time.Time{date(t, "2026-04-12"), date(t, "2026-11-01"), date(t, "2027-01-06")},
		Hours: []string{"lauds", "vespers", "compline"},
	}
	var buf bytes.Buffer
	direct := NewDigester()
	err := generator(t).Generate(sel, func(r Record) error {
		line, err := Marshal(r)
		if err != nil {
			return err
		}
		buf.Write(line)
		buf.WriteByte('\n')
		return direct.Add(r)
	})
	if err != nil {
		t.Fatal(err)
	}
	fromFile, err := Digest(&buf)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !reflect.DeepEqual(fromFile, direct.Snapshot()) {
		t.Fatal("digest of the written dump differs from the in-process digest")
	}
	if fromFile.DateHours != 9 || len(fromFile.Years) != 2 || fromFile.StartYear != 0 {
		t.Fatalf("snapshot = %+v", fromFile)
	}
}

func TestParallelYearDigestsMatchOneSequentialDump(t *testing.T) {
	base := Selection{Hours: []string{"compline"}, Forms: []models.PrayerForm{models.PrayerPrivate}}
	parallel, err := digestYears(generator(t), 2026, 3, base)
	if err != nil {
		t.Fatal(err)
	}
	sequential := NewDigester()
	sel := base
	sel.StartYear, sel.Years = 2026, 3
	if err := generator(t).Generate(sel, sequential.Add); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parallel, sequential.Snapshot()) {
		t.Fatal("parallel per-year digests differ from the sequential digest")
	}
	var out bytes.Buffer
	if err := WriteParitySnapshot(parallel, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"format": "office-parity/2"`, `"corpus": "`, `"year_count": 3`, `"hour": "compline"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("snapshot lacks %s", want)
		}
	}
}

func TestDigestRejectsMalformedStreams(t *testing.T) {
	for name, input := range map[string]string{
		"unknown kind":       `{"kind":"mass"}` + "\n",
		"non-canonical":      `{"kind": "meta"}` + "\n",
		"hour without date":  `{"form":"private","hour":"lauds","kind":"hour"}` + "\n",
		"unknown hour":       `{"date":"2026-01-01","form":"private","hour":"matins","kind":"hour"}` + "\n",
		"year without value": `{"kind":"calendar_year"}` + "\n",
	} {
		if _, err := Digest(strings.NewReader(input)); err == nil {
			t.Errorf("%s: Digest succeeded, want an error", name)
		}
	}
}

func TestCorpusRecordsPrecedeTheYearsInKeyOrder(t *testing.T) {
	records := collect(t, Selection{Dates: []time.Time{date(t, "2026-04-12")}, Groups: []string{GroupCalendar, GroupCorpus}})
	if records[0]["kind"] != KindMeta || records[len(records)-2]["kind"] != KindCalendarYear {
		t.Fatalf("corpus records must follow meta and precede the years: %v ... %v", records[0]["kind"], records[len(records)-2]["kind"])
	}
	var keys []string
	directives := map[any]int{}
	scopes := 0
	for _, r := range records[1 : len(records)-2] {
		switch r["kind"] {
		case KindCorpusEntry:
			if scopes > 0 {
				t.Fatal("corpus entries must precede the appointment scopes")
			}
			keys = append(keys, r["key"].(string))
			directives[r["directive"]]++
			if r["canonical"] == nil || r["body"] == nil {
				t.Fatalf("%v: unresolved corpus entry", r["key"])
			}
			if (r["directive"] == "use") != (r["use_target"] != nil) {
				t.Fatalf("%v: use_target must accompany exactly the @use directive", r["key"])
			}
		case KindAppointmentScope:
			scopes++
		default:
			t.Fatalf("unexpected record %v", r["kind"])
		}
	}
	if !slices.IsSorted(keys) || len(keys) < 1000 {
		t.Fatalf("%d corpus keys, sorted=%v", len(keys), slices.IsSorted(keys))
	}
	if directives["use"] == 0 || directives["omit"] == 0 || directives[nil] == 0 || scopes == 0 {
		t.Fatalf("directives %v, scopes %d: the live corpus uses @use, @omit, plain bodies, and scopes", directives, scopes)
	}
}
