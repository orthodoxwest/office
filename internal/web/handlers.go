package web

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/render"
	"github.com/orthodoxwest/office/internal/review"
)

var validHours = map[string]bool{
	"lauds":    true,
	"prime":    true,
	"terce":    true,
	"sext":     true,
	"none":     true,
	"vespers":  true,
	"compline": true,
}

// repoIssuesURL is the GitHub new-issue endpoint used by the per-hour
// "Report a problem" link.
const repoIssuesURL = "https://github.com/orthodoxwest/office/issues/new"

// reportURL builds a prefilled GitHub issue link identifying the exact page
// the reviewer was looking at, with the three review categories as checkboxes.
func reportURL(hour *models.OfficeHour, hourName, dateSlug string) string {
	celebration := hour.Feast
	if celebration == "" {
		celebration = render.TitleCase(string(hour.Season)) + " feria"
	}
	title := fmt.Sprintf("[review] %s — %s (%s)", hour.Title, dateSlug, celebration)
	body := fmt.Sprintf(`**Page:** /%s/%s
**Celebration:** %s
**Season:** %s

**Category** (check all that apply):
- [ ] Missing proper — the app shows a generic/ordinary text where the diurnal or archdiocese supplement has a specific one
- [ ] Incorrect translation — wording differs from our diocesan books
- [ ] Logic or rubric error — wrong structure, missing or extra element, wrong psalms/antiphons for the day

**What the books say** (cite diurnal/supplement page if possible):


**What the app shows:**

`, hourName, dateSlug, celebration, render.TitleCase(string(hour.Season)))

	q := url.Values{}
	q.Set("title", title)
	q.Set("body", body)
	q.Set("labels", "review")
	return repoIssuesURL + "?" + q.Encode()
}

func dependencyReportURL(hour *models.OfficeHour, hourName, dateSlug, key string, status review.ProvenanceStatus) string {
	celebration := hour.Feast
	if celebration == "" {
		celebration = render.TitleCase(string(hour.Season)) + " feria"
	}
	title := fmt.Sprintf("[review] Source verification — %s", key)
	body := fmt.Sprintf(`**Page:** /%s/%s
**Celebration:** %s
**Corpus entry:** %s
**Current provenance status:** %s

**Source and page/section locator:**


**Finding:**

`, hourName, dateSlug, celebration, key, status)
	q := url.Values{}
	q.Set("title", title)
	q.Set("body", body)
	q.Set("labels", "review")
	return repoIssuesURL + "?" + q.Encode()
}

func (s *Server) hourAssurance(hour *models.OfficeHour, hourName, dateSlug string) render.HourAssurance {
	data := render.HourAssurance{Decisions: review.UniqueCompositionDecisions(hour.Decisions)}
	for _, key := range review.HourDependencies(hour) {
		status := review.ProvenanceSourceUnknown
		if entry, ok := s.provenance[key]; ok {
			status = entry.Status
		}
		switch status {
		case review.ProvenanceVerified:
			data.Verified++
		case review.ProvenanceNeedsReview:
			data.NeedsReview++
		default:
			data.SourceUnknown++
		}
		if len(s.suspicions[key]) > 0 {
			data.Flagged++
		}
		data.Dependencies = append(data.Dependencies, render.AssuranceDependency{
			Key: key, Status: status, Flags: s.suspicions[key],
			ReportURL: dependencyReportURL(hour, hourName, dateSlug, key, status),
		})
	}
	seenResolutions := map[string]bool{}
	for _, section := range hour.Sections {
		for _, element := range section.Elements {
			if element.SlotRef == "" || element.SourceRef == "" {
				continue
			}
			tier, _, _ := strings.Cut(element.SourceRef, "/")
			key := element.SlotRef + "\x1f" + tier + "\x1f" + element.SourceRef
			if seenResolutions[key] {
				continue
			}
			seenResolutions[key] = true
			data.Resolutions = append(data.Resolutions, render.AssuranceResolution{Slot: element.SlotRef, Tier: tier, Source: element.SourceRef})
		}
	}
	return data
}

// userLocation returns the *time.Location from the "tz" cookie (set by the
// browser via Intl.DateTimeFormat), falling back to time.Local if absent or
// unrecognised.
func userLocation(r *http.Request) *time.Location {
	c, err := r.Cookie("tz")
	if err != nil {
		return time.Local
	}
	loc, err := time.LoadLocation(c.Value)
	if err != nil {
		return time.Local
	}
	return loc
}

// themeParam is retained for call-site compatibility but always returns "".
// Appearance is client-only (localStorage + data-theme); ?theme= is ignored.
func themeParam(r *http.Request) string {
	_ = r
	return ""
}

type currentHourBoundary struct {
	Start  int
	Slug   string
	Label  string
	Offset int
}

// currentHourSchedule is mirrored in app.js so a cached home page can update
// itself from the device clock. schedule_contract_test.go keeps both copies
// byte-for-value aligned at every boundary.
var currentHourSchedule = []currentHourBoundary{
	{Start: 0, Slug: "compline", Label: "Compline", Offset: -1},
	{Start: 2, Slug: "lauds", Label: "Lauds", Offset: 0},
	{Start: 7, Slug: "prime", Label: "Prime", Offset: 0},
	{Start: 9, Slug: "terce", Label: "Terce", Offset: 0},
	{Start: 11, Slug: "sext", Label: "Sext", Offset: 0},
	{Start: 13, Slug: "none", Label: "None", Offset: 0},
	{Start: 17, Slug: "vespers", Label: "Vespers", Offset: 0},
	{Start: 20, Slug: "compline", Label: "Compline", Offset: 0},
}

// currentHourEntry returns the office most likely being prayed at now, its
// display name, and a day offset (0 or -1) locating which calendar day it
// belongs to. Midnight-2am belongs to the previous day's Compline, not the
// day that has just begun.
func currentHourEntry(now time.Time) (slug string, label string, dayOffset int) {
	for i := len(currentHourSchedule) - 1; i >= 0; i-- {
		boundary := currentHourSchedule[i]
		if now.Hour() >= boundary.Start {
			return boundary.Slug, boundary.Label, boundary.Offset
		}
	}
	return "compline", "Compline", -1
}

func buildHomeHours(dateSlug, theme, current string) []render.HomeHourLink {
	links := make([]render.HomeHourLink, 0, len(orderedHours))
	for _, hour := range orderedHours {
		links = append(links, render.HomeHourLink{
			Name:      hour.Name,
			Slug:      hour.Slug,
			URL:       render.HourLink(hour.Slug, dateSlug, theme),
			IsCurrent: hour.Slug == current,
		})
	}
	return links
}

var orderedHours = []struct {
	Name string
	Slug string
}{
	{Name: "Lauds", Slug: "lauds"},
	{Name: "Prime", Slug: "prime"},
	{Name: "Terce", Slug: "terce"},
	{Name: "Sext", Slug: "sext"},
	{Name: "None", Slug: "none"},
	{Name: "Vespers", Slug: "vespers"},
	{Name: "Compline", Slug: "compline"},
}

func adjacentHours(hour, date, theme string) (previousName, previousLink, nextName, nextLink string) {
	for i, candidate := range orderedHours {
		if candidate.Slug != hour {
			continue
		}
		if i > 0 {
			previousName = orderedHours[i-1].Name
			previousLink = render.HourLink(orderedHours[i-1].Slug, date, theme)
		}
		if i+1 < len(orderedHours) {
			nextName = orderedHours[i+1].Name
			nextLink = render.HourLink(orderedHours[i+1].Slug, date, theme)
		}
		break
	}
	return
}

func defaultHourSlug(hour string) string {
	if hour == "" {
		return "lauds"
	}
	return hour
}

// setHTMLCacheHeaders marks HTML as revalidate-always so SW SWR revalidation
// and non-SW browsers do not keep a pre-deploy body under a stable dated URL.
func setHTMLCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache")
}

// navDateNow is local today for chrome links on pages without a liturgical day.
func navDateNow(r *http.Request) string {
	return time.Now().In(userLocation(r)).Format("2006-01-02")
}

// handleError renders the styled error page for 4xx/5xx conditions where the
// response has not yet been started (i.e. not template-execution failures).
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	title := http.StatusText(status)
	setHTMLCacheHeaders(w)
	w.WriteHeader(status)
	data := render.ErrorData{
		Title:      title,
		Message:    msg,
		NavDate:    navDateNow(r),
		Theme:      themeParam(r),
		Page:       "",
		ShowBanner: false,
	}
	if err := s.pages.ErrorPage(w, data); err != nil {
		// Template itself failed — fall back to plain text (header already sent).
		log.Printf("error rendering error template: %v", err)
	}
}

// handle404 renders the styled 404 page.
func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
	setHTMLCacheHeaders(w)
	w.WriteHeader(http.StatusNotFound)
	data := render.NotFoundData{
		NavDate:    navDateNow(r),
		Theme:      themeParam(r),
		Page:       "",
		ShowBanner: false,
	}
	if err := s.pages.NotFound(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleRoot dispatches "/" to handleHome, "/{hour}" and "/{hour}/{date}"
// to handleHour, and returns 404 for anything else.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	if path == "" {
		s.handleHome(w, r)
		return
	}
	parts := strings.Split(path, "/")
	switch len(parts) {
	case 1:
		s.handleHour(w, r, parts[0], "")
	case 2:
		s.handleHour(w, r, parts[0], parts[1])
	default:
		s.handle404(w, r)
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	loc := userLocation(r)

	// Resolve the displayed date: ?date= override or today.
	// Nav always uses the resolved date so chrome links hit the SW precache
	// keys (/?date=…, /lauds/…) rather than undated URLs.
	var date time.Time
	if ds := r.URL.Query().Get("date"); ds != "" {
		var err error
		date, err = time.Parse("2006-01-02", ds)
		if err != nil {
			s.handleError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid date %q — please use YYYY-MM-DD format.", ds))
			return
		}
	} else {
		date = time.Now().In(loc)
	}

	dateSlug := date.Format("2006-01-02")
	year := date.Year()

	days, _, err := s.cache.get(year)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, fmt.Sprintf("error building calendar: %v", err))
		return
	}

	dayIndex := date.YearDay() - 1
	if dayIndex < 0 || dayIndex >= len(days) {
		s.handleError(w, r, http.StatusInternalServerError, "date out of range")
		return
	}
	day := &days[dayIndex]

	feastName := ""
	var commemorations []string
	if day.Celebration != nil {
		feastName = day.Celebration.Name
	} else if day.Tempora != "" {
		feastName = day.Tempora
	}
	for _, c := range day.Commemorations {
		commemorations = append(commemorations, c.Name)
	}

	// Short home-facing note; full wording lives in the ordo.
	octaveNote := ""
	if day.WithinOctaveOf != "" && !strings.Contains(strings.ToLower(feastName), "octave") {
		octaveNote = "Octave of " + calendar.OctaveDisplayName(day.WithinOctaveOf)
	}

	// Where in the year we are, for readers orienting themselves. Suppressed
	// when the celebration already names the season (Pentecost Sunday, the
	// Sundays in Lent) so home never prints the same word twice.
	season := render.TitleCase(string(day.Season))
	if season != "" && strings.Contains(strings.ToLower(feastName), strings.ToLower(season)) {
		season = ""
	}

	theme := themeParam(r)
	nowSlug := time.Now().In(loc).Format("2006-01-02")
	currentHourSlug := ""
	currentHourLabel := "Open Lauds"
	prayNowDateSlug := dateSlug
	gridCurrentSlug := ""
	if dateSlug == nowSlug {
		var offset int
		currentHourSlug, currentHourLabel, offset = currentHourEntry(time.Now().In(loc))
		currentHourLabel = "Pray " + currentHourLabel
		if offset != 0 {
			prayNowDateSlug = time.Now().In(loc).AddDate(0, 0, offset).Format("2006-01-02")
		} else {
			gridCurrentSlug = currentHourSlug
		}
	}

	data := render.HomeData{
		DateStr:        date.Format("Monday, January 2, 2006"),
		DateSlug:       dateSlug,
		PrevDate:       date.AddDate(0, 0, -1).Format("2006-01-02"),
		NextDate:       date.AddDate(0, 0, 1).Format("2006-01-02"),
		PrevLink:       render.HomeLink(date.AddDate(0, 0, -1).Format("2006-01-02"), theme),
		NextLink:       render.HomeLink(date.AddDate(0, 0, 1).Format("2006-01-02"), theme),
		TodayLink:      render.HomeLink(nowSlug, theme),
		ShowToday:      dateSlug != nowSlug,
		FeastName:      feastName,
		Commemorations: commemorations,
		Color:          string(day.Color),
		Season:         season,
		OctaveNote:     octaveNote,
		Penitential:    day.Penitential.Labels(),
		CalendarLink:   render.CalendarLink(dateSlug, theme),
		PrayNowLabel:   currentHourLabel,
		PrayNowLink:    render.HourLink(defaultHourSlug(currentHourSlug), prayNowDateSlug, theme),
		Hours:          buildHomeHours(dateSlug, theme, gridCurrentSlug),
		NavDate:        dateSlug,
		Theme:          theme,
		Page:           "home",
		// Ornament tint follows the day itself; `season` above is the
		// display string and is blanked when the feast already names it.
		SeasonClass: render.SeasonClass(day.Season),
		ShowBanner:  false,
	}
	setHTMLCacheHeaders(w)
	if err := s.pages.Home(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleHour(w http.ResponseWriter, r *http.Request, hourName, dateStr string) {
	if !validHours[hourName] {
		s.handle404(w, r)
		return
	}

	var date time.Time
	var err error
	// Nav always uses the resolved date so chrome links match SW precache keys.
	if dateStr == "" {
		if ds := r.URL.Query().Get("date"); ds != "" {
			date, err = time.Parse("2006-01-02", ds)
			if err != nil {
				s.handleError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid date %q — please use YYYY-MM-DD format.", ds))
				return
			}
			dateStr = ds
		} else {
			date = time.Now().In(userLocation(r))
			dateStr = date.Format("2006-01-02")
		}
	} else {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			s.handleError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid date %q — please use YYYY-MM-DD format.", dateStr))
			return
		}
	}

	year := date.Year()
	theme := themeParam(r)
	days, moveable, err := s.cache.get(year)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, fmt.Sprintf("error building calendar: %v", err))
		return
	}

	dayIndex := date.YearDay() - 1
	if dayIndex < 0 || dayIndex >= len(days) {
		s.handleError(w, r, http.StatusBadRequest, "That date is outside the supported range.")
		return
	}
	day := &days[dayIndex]

	hour, err := s.engine.ComposeHour(hourName, day, moveable)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, fmt.Sprintf("error composing hour: %v", err))
		return
	}

	previousHourName, previousHourLink, nextHourName, nextHourLink := adjacentHours(hourName, dateStr, theme)
	todaySlug := time.Now().In(userLocation(r)).Format("2006-01-02")
	data := render.HourData{
		HourName:         hourName,
		DateStr:          date.Format("Monday, January 2, 2006"),
		DateSlug:         dateStr,
		PrevDate:         date.AddDate(0, 0, -1).Format("2006-01-02"),
		NextDate:         date.AddDate(0, 0, 1).Format("2006-01-02"),
		PrevLink:         render.HourLink(hourName, date.AddDate(0, 0, -1).Format("2006-01-02"), theme),
		NextLink:         render.HourLink(hourName, date.AddDate(0, 0, 1).Format("2006-01-02"), theme),
		TodayLink:        render.HourLink(hourName, todaySlug, theme),
		ShowToday:        dateStr != todaySlug,
		DayLink:          render.HomeLink(dateStr, theme),
		PreviousHourName: previousHourName,
		PreviousHourLink: previousHourLink,
		NextHourName:     nextHourName,
		NextHourLink:     nextHourLink,
		NavDate:          dateStr,
		Hour:             hour,
		ReportURL:        reportURL(hour, hourName, dateStr),
		Theme:            theme,
		Page:             hourName,
		// The hour's own season, not the day's: Vespers takes the season of
		// the office that owns the evening, so I Vespers of Easter on Holy
		// Saturday is unveiled while that day's Lauds is still veiled.
		SeasonClass: render.SeasonClass(hour.Season),
		ShowBanner:  s.showVettingBanner(hour),
		Assurance:   s.hourAssurance(hour, hourName, dateStr),
	}
	setHTMLCacheHeaders(w)
	if err := s.pages.Hour(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	// parts[0] == "calendar"; parts[1] (optional) == year
	now := time.Now().In(userLocation(r))
	year := now.Year()
	if len(parts) == 1 || parts[1] == "" {
		// No year specified: redirect to current year anchored at today's row.
		slug := "d-" + now.Format("2006-01-02")
		target := fmt.Sprintf("/calendar/%d#%s", year, slug)
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	if len(parts) == 2 && parts[1] != "" {
		y, err := strconv.Atoi(parts[1])
		if err != nil || y < 1 || y > 9999 {
			s.handleError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid year %q.", parts[1]))
			return
		}
		year = y
	}

	days, moveable, err := s.cache.get(year)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, fmt.Sprintf("error building calendar: %v", err))
		return
	}

	// NavDate is local today so chrome hour links hit dated precache keys.
	navDate := now.Format("2006-01-02")
	data := render.CalendarData{
		Year:       year,
		PrevYear:   year - 1,
		NextYear:   year + 1,
		Months:     buildMonthData(days, s.engine, moveable),
		NavDate:    navDate,
		Theme:      themeParam(r),
		Page:       "calendar",
		ShowBanner: false,
	}
	setHTMLCacheHeaders(w)
	if err := s.pages.Calendar(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// buildMonthData assembles the calendar's per-day rows, enriched with the
// composed Lauds/Hours/Vespers digest (preces, suffrage, gospel-antiphon
// incipits, per-hour commemorations) that the desktop calendar view reveals.
// eng may be nil (falls back to the calendar-only fields, digest omitted).
func buildMonthData(days []models.CalendarDay, eng *office.Engine, moveable *calendar.MoveableDates) []render.MonthData {
	var months []render.MonthData
	currentMonthName := ""
	currentIdx := -1

	summarize := func(hourName string, day *models.CalendarDay) *office.HourSummary {
		if eng == nil {
			return nil
		}
		hour, err := eng.ComposeHour(hourName, day, moveable)
		if err != nil {
			return nil
		}
		s := office.SummarizeHour(hour)
		return &s
	}

	commRows := func(comms []office.CommSummary) []render.CommemorationRow {
		var out []render.CommemorationRow
		for _, c := range comms {
			out = append(out, render.CommemorationRow{Name: c.Name, Incipit: c.Incipit})
		}
		return out
	}

	for i := range days {
		d := &days[i]
		mName := d.Date.Month().String()
		if mName != currentMonthName {
			months = append(months, render.MonthData{Name: mName, Slug: strings.ToLower(mName)})
			currentIdx = len(months) - 1
			currentMonthName = mName
		}

		feastName := ""
		rank := ""
		rankFull := ""
		if d.Celebration != nil {
			feastName = d.Celebration.Name
			rank = string(d.Celebration.Rank.Abbrev())
			rankFull = d.Celebration.Rank.DisplayName()
		} else if d.Tempora != "" {
			feastName = d.Tempora
		} else {
			feastName = render.TitleCase(string(d.Season)) + " feria"
		}

		var commemorations []string
		for _, c := range d.Commemorations {
			commemorations = append(commemorations, c.Name)
		}

		row := render.DayRow{
			DayNum:         d.Date.Day(),
			Weekday:        d.Date.Weekday().String()[:3],
			DateSlug:       d.Date.Format("2006-01-02"),
			Rank:           rank,
			RankFull:       rankFull,
			Color:          string(d.Color),
			ColorClass:     "day-color-" + string(d.Color),
			FeastName:      feastName,
			Fast:           d.Penitential.Fast,
			Abstinence:     d.Penitential.Abstinence,
			Commemorations: commemorations,
		}

		if lauds := summarize("lauds", d); lauds != nil {
			row.BenedictusAntiphon = lauds.GospelAnt
			row.LaudsPreces = lauds.Preces
			row.LaudsSuffrage = lauds.Suffrage
			row.LaudsComms = commRows(lauds.Comms)
		}
		// The minor hours share one preces disposition; Prime is representative
		// (mirrors cmdRubrics and FormatDay).
		if hours := summarize("prime", d); hours != nil {
			row.HoursPreces = hours.Preces
		}
		if vespers := summarize("vespers", d); vespers != nil {
			row.MagnificatAntiphon = vespers.GospelAnt
			row.VespersPreces = vespers.Preces
			row.VespersSuffrage = vespers.Suffrage
			row.VespersComms = commRows(vespers.Comms)
		}
		switch d.Vespers.Owner {
		case models.VespersIIOfPreceding:
			row.VespersNote = "II Vespers of preceding"
		case models.VespersIOfFollowing:
			if d.Vespers.Feast != nil {
				row.VespersNote = "I Vespers of " + d.Vespers.Feast.Name
			}
		}

		months[currentIdx].Days = append(months[currentIdx].Days, row)
	}

	return months
}
