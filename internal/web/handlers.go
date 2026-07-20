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

type homeData struct {
	DateStr        string
	DateSlug       string
	PrevDate       string
	NextDate       string
	PrevLink       string
	NextLink       string
	TodayLink      string
	ShowToday      bool
	FeastName      string
	FeastRank      string
	Commemorations []string
	Season         string
	Color          string
	OctaveNote     string
	Penitential    []string
	CalendarLink   string
	PrayNowLabel   string
	PrayNowLink    string
	Hours          []homeHourLink
	NavDate        string
	Theme          string
	Page           string
	ShowBanner     bool
}

type homeHourLink struct {
	Name      string
	Slug      string
	URL       string
	IsCurrent bool
}

type hourData struct {
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
	Assurance        hourAssuranceData
}

type hourAssuranceData struct {
	Verified      int
	NeedsReview   int
	SourceUnknown int
	Flagged       int
	Dependencies  []assuranceDependency
	Resolutions   []assuranceResolution
	Decisions     []models.CompositionDecision
}

type assuranceDependency struct {
	Key       string
	Status    review.ProvenanceStatus
	Flags     []review.Suspicion
	ReportURL string
}

type assuranceResolution struct {
	Slot   string
	Tier   string
	Source string
}

// repoIssuesURL is the GitHub new-issue endpoint used by the per-hour
// "Report a problem" link.
const repoIssuesURL = "https://github.com/orthodoxwest/office/issues/new"

// reportURL builds a prefilled GitHub issue link identifying the exact page
// the reviewer was looking at, with the three review categories as checkboxes.
func reportURL(hour *models.OfficeHour, hourName, dateSlug string) string {
	celebration := hour.Feast
	if celebration == "" {
		celebration = titleCase(string(hour.Season)) + " feria"
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

`, hourName, dateSlug, celebration, titleCase(string(hour.Season)))

	q := url.Values{}
	q.Set("title", title)
	q.Set("body", body)
	q.Set("labels", "review")
	return repoIssuesURL + "?" + q.Encode()
}

func dependencyReportURL(hour *models.OfficeHour, hourName, dateSlug, key string, status review.ProvenanceStatus) string {
	celebration := hour.Feast
	if celebration == "" {
		celebration = titleCase(string(hour.Season)) + " feria"
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

func (s *Server) hourAssurance(hour *models.OfficeHour, hourName, dateSlug string) hourAssuranceData {
	data := hourAssuranceData{Decisions: review.UniqueCompositionDecisions(hour.Decisions)}
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
		data.Dependencies = append(data.Dependencies, assuranceDependency{
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
			data.Resolutions = append(data.Resolutions, assuranceResolution{Slot: element.SlotRef, Tier: tier, Source: element.SourceRef})
		}
	}
	return data
}

type calendarData struct {
	Year       int
	PrevYear   int
	NextYear   int
	Months     []monthData
	NavDate    string
	Theme      string
	Page       string
	ShowBanner bool
}

type monthData struct {
	Name string
	Slug string
	Days []dayRow
}

type dayRow struct {
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

// themeParam returns "dark" or "light" if the ?theme= param is one of those,
// or "" otherwise. Used to force a color scheme for testing without JS.
func themeParam(r *http.Request) string {
	t := r.URL.Query().Get("theme")
	if t == "dark" || t == "light" {
		return t
	}
	return ""
}

func homeLink(date, theme string) string {
	href := "/"
	if date != "" {
		href = "/?date=" + date
	}
	return appendTheme(href, theme)
}

func hourLink(hour, date, theme string) string {
	href := "/" + hour
	if date != "" {
		href += "/" + date
	}
	return appendTheme(href, theme)
}

func calendarLink(date, theme string) string {
	if date != "" {
		if parsed, err := time.Parse("2006-01-02", date); err == nil {
			href := fmt.Sprintf("/calendar/%d#d-%s", parsed.Year(), date)
			return appendTheme(href, theme)
		}
	}
	return appendTheme("/calendar", theme)
}

func calendarYearLink(year int, theme string) string {
	return appendTheme(fmt.Sprintf("/calendar/%d", year), theme)
}

func appendTheme(href, theme string) string {
	if theme == "" {
		return href
	}
	fragment := ""
	if idx := strings.Index(href, "#"); idx >= 0 {
		fragment = href[idx:]
		href = href[:idx]
	}
	sep := "?"
	if strings.Contains(href, "?") {
		sep = "&"
	}
	return href + sep + "theme=" + theme + fragment
}

func currentHourEntry(now time.Time) (string, string) {
	h := now.Hour()
	switch {
	case h >= 5 && h < 7:
		return "lauds", "Lauds"
	case h >= 7 && h < 9:
		return "prime", "Prime"
	case h >= 9 && h < 11:
		return "terce", "Terce"
	case h >= 11 && h < 13:
		return "sext", "Sext"
	case h >= 13 && h < 17:
		return "none", "None"
	case h >= 17 && h < 20:
		return "vespers", "Vespers"
	default:
		return "compline", "Compline"
	}
}

func buildHomeHours(dateSlug, theme, current string) []homeHourLink {
	links := make([]homeHourLink, 0, len(orderedHours))
	for _, hour := range orderedHours {
		links = append(links, homeHourLink{
			Name:      hour.Name,
			Slug:      hour.Slug,
			URL:       hourLink(hour.Slug, dateSlug, theme),
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
			previousLink = hourLink(orderedHours[i-1].Slug, date, theme)
		}
		if i+1 < len(orderedHours) {
			nextName = orderedHours[i+1].Name
			nextLink = hourLink(orderedHours[i+1].Slug, date, theme)
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

// handleError renders the styled error page for 4xx/5xx conditions where the
// response has not yet been started (i.e. not template-execution failures).
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	title := http.StatusText(status)
	w.WriteHeader(status)
	data := struct {
		Title      string
		Message    string
		NavDate    string
		Theme      string
		Page       string
		ShowBanner bool
	}{
		Title:      title,
		Message:    msg,
		NavDate:    "",
		Theme:      themeParam(r),
		Page:       "",
		ShowBanner: false,
	}
	if err := s.tmplError.ExecuteTemplate(w, "layout", data); err != nil {
		// Template itself failed — fall back to plain text (header already sent).
		log.Printf("error rendering error template: %v", err)
	}
}

// handle404 renders the styled 404 page.
func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	data := struct {
		NavDate    string
		Theme      string
		Page       string
		ShowBanner bool
	}{
		NavDate:    "",
		Theme:      themeParam(r),
		Page:       "",
		ShowBanner: false,
	}
	if err := s.tmpl404.ExecuteTemplate(w, "layout", data); err != nil {
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
	var date time.Time
	navDate := "" // non-empty only when the user has chosen a specific date
	if ds := r.URL.Query().Get("date"); ds != "" {
		var err error
		date, err = time.Parse("2006-01-02", ds)
		if err != nil {
			s.handleError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid date %q — please use YYYY-MM-DD format.", ds))
			return
		}
		navDate = ds
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
	feastRank := ""
	var commemorations []string
	if day.Celebration != nil {
		feastName = day.Celebration.Name
		feastRank = day.Celebration.Rank.DisplayName()
	} else if day.Tempora != "" {
		feastName = day.Tempora
	}
	for _, c := range day.Commemorations {
		commemorations = append(commemorations, c.Name)
	}

	octaveNote := ""
	if day.WithinOctaveOf != "" && !strings.Contains(strings.ToLower(feastName), "octave") {
		octaveNote = "Within the Octave of " + calendar.OctaveDisplayName(day.WithinOctaveOf)
	}

	theme := themeParam(r)
	nowSlug := time.Now().In(loc).Format("2006-01-02")
	currentHourSlug := ""
	currentHourLabel := "Open Lauds"
	if dateSlug == nowSlug {
		currentHourSlug, currentHourLabel = currentHourEntry(time.Now().In(loc))
		currentHourLabel = "Pray " + currentHourLabel
	}

	data := homeData{
		DateStr:        date.Format("Monday, January 2, 2006"),
		DateSlug:       dateSlug,
		PrevDate:       date.AddDate(0, 0, -1).Format("2006-01-02"),
		NextDate:       date.AddDate(0, 0, 1).Format("2006-01-02"),
		PrevLink:       homeLink(date.AddDate(0, 0, -1).Format("2006-01-02"), theme),
		NextLink:       homeLink(date.AddDate(0, 0, 1).Format("2006-01-02"), theme),
		TodayLink:      homeLink("", theme),
		ShowToday:      dateSlug != nowSlug,
		FeastName:      feastName,
		FeastRank:      feastRank,
		Commemorations: commemorations,
		Season:         titleCase(string(day.Season)),
		Color:          string(day.Color),
		OctaveNote:     octaveNote,
		Penitential:    day.Penitential.Labels(),
		CalendarLink:   calendarLink(dateSlug, theme),
		PrayNowLabel:   currentHourLabel,
		PrayNowLink:    hourLink(defaultHourSlug(currentHourSlug), dateSlug, theme),
		Hours:          buildHomeHours(dateSlug, theme, currentHourSlug),
		NavDate:        navDate,
		Theme:          theme,
		Page:           "home",
		ShowBanner:     false,
	}
	if err := s.tmplHome.ExecuteTemplate(w, "layout", data); err != nil {
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
	navDate := "" // propagate through nav only when date was explicit in the URL
	if dateStr == "" {
		if ds := r.URL.Query().Get("date"); ds != "" {
			date, err = time.Parse("2006-01-02", ds)
			if err != nil {
				s.handleError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid date %q — please use YYYY-MM-DD format.", ds))
				return
			}
			dateStr = ds
			navDate = ds
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
		navDate = dateStr
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
	data := hourData{
		HourName:         hourName,
		DateStr:          date.Format("Monday, January 2, 2006"),
		DateSlug:         dateStr,
		PrevDate:         date.AddDate(0, 0, -1).Format("2006-01-02"),
		NextDate:         date.AddDate(0, 0, 1).Format("2006-01-02"),
		PrevLink:         hourLink(hourName, date.AddDate(0, 0, -1).Format("2006-01-02"), theme),
		NextLink:         hourLink(hourName, date.AddDate(0, 0, 1).Format("2006-01-02"), theme),
		TodayLink:        hourLink(hourName, "", theme),
		ShowToday:        dateStr != todaySlug,
		DayLink:          homeLink(dateStr, theme),
		PreviousHourName: previousHourName,
		PreviousHourLink: previousHourLink,
		NextHourName:     nextHourName,
		NextHourLink:     nextHourLink,
		NavDate:          navDate,
		Hour:             hour,
		ReportURL:        reportURL(hour, hourName, dateStr),
		Theme:            theme,
		Page:             hourName,
		ShowBanner:       s.showVettingBanner(hour),
		Assurance:        s.hourAssurance(hour, hourName, dateStr),
	}
	if err := s.tmplHour.ExecuteTemplate(w, "layout", data); err != nil {
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
		if t := themeParam(r); t != "" {
			target = fmt.Sprintf("/calendar/%d?theme=%s#%s", year, t, slug)
		}
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

	days, _, err := s.cache.get(year)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, fmt.Sprintf("error building calendar: %v", err))
		return
	}

	data := calendarData{
		Year:       year,
		PrevYear:   year - 1,
		NextYear:   year + 1,
		Months:     buildMonthData(days),
		Theme:      themeParam(r),
		Page:       "calendar",
		ShowBanner: false,
	}
	if err := s.tmplCalendar.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func buildMonthData(days []models.CalendarDay) []monthData {
	var months []monthData
	currentMonthName := ""
	currentIdx := -1

	for i := range days {
		d := &days[i]
		mName := d.Date.Month().String()
		if mName != currentMonthName {
			months = append(months, monthData{Name: mName, Slug: strings.ToLower(mName)})
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
			feastName = titleCase(string(d.Season)) + " feria"
		}

		var commemorations []string
		for _, c := range d.Commemorations {
			commemorations = append(commemorations, c.Name)
		}

		months[currentIdx].Days = append(months[currentIdx].Days, dayRow{
			DayNum:         d.Date.Day(),
			Weekday:        d.Date.Weekday().String()[:3],
			DateSlug:       d.Date.Format("2006-01-02"),
			Rank:           rank,
			RankFull:       rankFull,
			Color:          string(d.Color),
			ColorClass:     "color-" + string(d.Color),
			FeastName:      feastName,
			Fast:           d.Penitential.Fast,
			Abstinence:     d.Penitential.Abstinence,
			Commemorations: commemorations,
		})
	}

	return months
}
