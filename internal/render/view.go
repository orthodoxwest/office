package render

import (
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/review"
)

// The types below are the view models the templates read. They are plain data:
// callers resolve the day, compose the hour, and fill these in; nothing here
// consults the calendar or office engines.

// HomeData is the day-landing page.
type HomeData struct {
	DateStr        string
	DateSlug       string
	PrevDate       string
	NextDate       string
	PrevLink       string
	NextLink       string
	TodayLink      string
	ShowToday      bool
	FeastName      string
	Commemorations []string
	Color          string
	Season         string
	OctaveNote     string
	Penitential    []string
	CalendarLink   string
	PrayNowLabel   string
	PrayNowLink    string
	Hours          []HomeHourLink
	NavDate        string
	Theme          string
	Page           string
	ShowBanner     bool
}

// HomeHourLink is one entry in the home page's grid of hours.
type HomeHourLink struct {
	Name      string
	Slug      string
	URL       string
	IsCurrent bool
}

// HourData is a composed office hour and its surrounding chrome.
type HourData struct {
	HourName         string
	DateStr          string
	DateSlug         string
	PrevDate         string
	NextDate         string
	PrevLink         string
	NextLink         string
	TodayLink        string
	ShowToday        bool
	DayLink          string
	PreviousHourName string
	PreviousHourLink string
	NextHourName     string
	NextHourLink     string
	NavDate          string
	Hour             *models.OfficeHour
	ReportURL        string
	Theme            string
	Page             string
	ShowBanner       bool
	Assurance        HourAssurance
}

// HourAssurance is the hour page's review-metadata disclosure. It carries
// corpus keys, provenance states, and rule decisions only — never the contents
// of a source, and never a local path.
type HourAssurance struct {
	Verified      int
	NeedsReview   int
	SourceUnknown int
	Flagged       int
	Dependencies  []AssuranceDependency
	Resolutions   []AssuranceResolution
	Decisions     []models.CompositionDecision
}

// AssuranceDependency is one text-corpus entry the hour draws on.
type AssuranceDependency struct {
	Key       string
	Status    review.ProvenanceStatus
	Flags     []review.Suspicion
	ReportURL string
}

// AssuranceResolution records which tier and source filled a composition slot.
type AssuranceResolution struct {
	Slot   string
	Tier   string
	Source string
}

// CalendarData is the year calendar.
type CalendarData struct {
	Year       int
	PrevYear   int
	NextYear   int
	Months     []MonthData
	NavDate    string
	Theme      string
	Page       string
	ShowBanner bool
}

// MonthData groups a month's day rows.
type MonthData struct {
	Name string
	Slug string
	Days []DayRow
}

// DayRow is one day of the calendar.
type DayRow struct {
	DayNum         int
	Weekday        string
	DateSlug       string
	Rank           string
	RankFull       string
	Color          string
	ColorClass     string
	FeastName      string
	Fast           bool
	Abstinence     bool
	Commemorations []string

	// Desktop-only office digest (hidden on narrow viewports; see .day-office-digest
	// in style.css). Sourced from the same composed-hour summaries as `office rubrics`
	// and FormatDay, so per-hour commemorations (with antiphon incipits) are kept
	// separate from the calendar-level Commemorations list above, since a
	// commemoration can appear at one hour and not the other.
	BenedictusAntiphon string
	MagnificatAntiphon string
	LaudsPreces        bool
	LaudsSuffrage      bool
	LaudsComms         []CommemorationRow
	HoursPreces        bool
	VespersPreces      bool
	VespersSuffrage    bool
	VespersComms       []CommemorationRow
	VespersNote        string // "II Vespers of preceding" / "I Vespers of <feast>", when applicable
}

// CommemorationRow pairs a commemoration's name with its antiphon incipit
// (when the office engine is available), mirroring the ordo's
// `Comm. Name ("incipit")` form.
type CommemorationRow struct {
	Name    string
	Incipit string
}

// RemindersData is the reminder-subscription settings page.
type RemindersData struct {
	Hours                []ReminderHour
	Days                 []ReminderDay
	NavDate, Theme, Page string
	ShowBanner           bool
}

// ReminderHour is one hour's row in the reminder schedule form.
type ReminderHour struct {
	Name, Slug, Default string
	Checked             bool
}

// ReminderDay is one weekday toggle in the reminder schedule form.
type ReminderDay struct {
	Name, Slug string
}

// NotFoundData is the styled 404 page.
type NotFoundData struct {
	NavDate    string
	Theme      string
	Page       string
	ShowBanner bool
}

// ErrorData is the styled error page for other 4xx/5xx conditions.
type ErrorData struct {
	Title      string
	Message    string
	NavDate    string
	Theme      string
	Page       string
	ShowBanner bool
}
