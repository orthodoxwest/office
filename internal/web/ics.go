package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

// hourOrder is the canonical liturgical order of the hours, used so events
// for the same day appear in sequence regardless of query-param order.
var hourOrder = []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}

// eventDuration is the block of time each office event occupies. Reminder
// timing comes from the VALARM, so this only affects calendar display.
const eventDuration = 15 * time.Minute

const (
	defaultHorizonDays = 60
	maxHorizonDays     = 366
)

// icsHour is one configured hour: its name and wall-clock time.
type icsHour struct {
	name   string
	hh, mm int
}

// icsConfig is the reminder schedule encoded in the subscription URL.
type icsConfig struct {
	hours   []icsHour
	days    [7]bool // indexed by time.Weekday
	alarm   int     // minutes before start; -1 disables the VALARM
	loc     *time.Location
	horizon int // days ahead to generate
}

var dayNames = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// parseDays parses the days parameter: comma-separated three-letter day
// names or ranges ("mon-fri,sun"). Ranges may wrap the week end ("sat-sun").
func parseDays(spec string) ([7]bool, error) {
	var days [7]bool
	if spec == "" {
		for i := range days {
			days[i] = true
		}
		return days, nil
	}
	for _, token := range strings.Split(spec, ",") {
		token = strings.ToLower(strings.TrimSpace(token))
		if from, to, isRange := strings.Cut(token, "-"); isRange {
			start, ok1 := dayNames[from]
			end, ok2 := dayNames[to]
			if !ok1 || !ok2 {
				return days, fmt.Errorf("invalid day range %q", token)
			}
			for d := start; ; d = (d + 1) % 7 {
				days[d] = true
				if d == end {
					break
				}
			}
			continue
		}
		d, ok := dayNames[token]
		if !ok {
			return days, fmt.Errorf("invalid day %q", token)
		}
		days[d] = true
	}
	return days, nil
}

// parseICSConfig validates the subscription query parameters.
func parseICSConfig(q url.Values) (*icsConfig, error) {
	cfg := &icsConfig{alarm: 10, loc: time.UTC, horizon: defaultHorizonDays}

	for _, name := range hourOrder {
		v := q.Get(name)
		if v == "" {
			continue
		}
		t, err := time.Parse("15:04", v)
		if err != nil {
			return nil, fmt.Errorf("invalid time %q for %s — use HH:MM (24-hour)", v, name)
		}
		cfg.hours = append(cfg.hours, icsHour{name: name, hh: t.Hour(), mm: t.Minute()})
	}
	if len(cfg.hours) == 0 {
		return nil, fmt.Errorf("no hours configured — set at least one, e.g. ?lauds=06:45")
	}

	days, err := parseDays(q.Get("days"))
	if err != nil {
		return nil, err
	}
	cfg.days = days

	if v := q.Get("alarm"); v != "" {
		if v == "none" {
			cfg.alarm = -1
		} else {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 24*60 {
				return nil, fmt.Errorf("invalid alarm %q — minutes before the hour, or \"none\"", v)
			}
			cfg.alarm = n
		}
	}

	if v := q.Get("tz"); v != "" {
		loc, err := time.LoadLocation(v)
		if err != nil {
			return nil, fmt.Errorf("unknown timezone %q — use an IANA name like America/New_York", v)
		}
		cfg.loc = loc
	}

	if v := q.Get("horizon"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxHorizonDays {
			return nil, fmt.Errorf("invalid horizon %q — days from 1 to %d", v, maxHorizonDays)
		}
		cfg.horizon = n
	}

	return cfg, nil
}

// escapeICS escapes a text value per RFC 5545 §3.3.11.
func escapeICS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// foldLine folds a content line to lines of at most 75 octets, continued
// with CRLF + space, breaking only at rune boundaries (RFC 5545 §3.1).
func foldLine(sb *strings.Builder, line string) {
	const limit = 75
	for len(line) > limit {
		cut := limit
		for cut > 0 && !isRuneStart(line[cut]) {
			cut--
		}
		sb.WriteString(line[:cut])
		sb.WriteString("\r\n ")
		line = line[cut:]
	}
	sb.WriteString(line)
	sb.WriteString("\r\n")
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// celebrationName returns the display name for a day, mirroring the ordo:
// the winning feast, else the tempora, else "<Season> feria".
func celebrationName(day *models.CalendarDay) string {
	if day.Celebration != nil {
		return day.Celebration.Name
	}
	if day.Tempora != "" {
		return day.Tempora
	}
	return titleCase(string(day.Season)) + " feria"
}

// buildICS renders the iCalendar document for the configured schedule,
// starting from now and extending cfg.horizon days. baseURL ("https://host")
// is used for per-event links back to the hour pages.
func (s *Server) buildICS(cfg *icsConfig, baseURL string, now time.Time) (string, error) {
	var sb strings.Builder
	write := func(line string) { foldLine(&sb, line) }

	write("BEGIN:VCALENDAR")
	write("VERSION:2.0")
	write("PRODID:-//AWRV Divine Office//office//EN")
	write("CALSCALE:GREGORIAN")
	write("METHOD:PUBLISH")
	write("X-WR-CALNAME:Divine Office")
	write("X-WR-CALDESC:Hours of the Benedictine Office")
	write("REFRESH-INTERVAL;VALUE=DURATION:P1D")
	write("X-PUBLISHED-TTL:P1D")

	dtstamp := now.UTC().Format("20060102T150405Z")
	start := now.In(cfg.loc)

	for i := 0; i < cfg.horizon; i++ {
		date := start.AddDate(0, 0, i)
		if !cfg.days[date.Weekday()] {
			continue
		}

		days, _, err := s.cache.get(date.Year())
		if err != nil {
			return "", err
		}
		dayIndex := date.YearDay() - 1
		if dayIndex < 0 || dayIndex >= len(days) {
			continue
		}
		day := &days[dayIndex]
		slug := date.Format("2006-01-02")
		feast := celebrationName(day)

		var descParts []string
		if day.Celebration != nil {
			descParts = append(descParts, day.Celebration.Rank.DisplayName())
		}
		descParts = append(descParts, titleCase(string(day.Season)), string(day.Color))
		for _, c := range day.Commemorations {
			descParts = append(descParts, "Comm. "+c.Name)
		}
		desc := strings.Join(descParts, " · ")

		for _, h := range cfg.hours {
			begin := time.Date(date.Year(), date.Month(), date.Day(), h.hh, h.mm, 0, 0, cfg.loc)
			summary := titleCase(h.name) + " — " + feast

			write("BEGIN:VEVENT")
			write("UID:" + h.name + "-" + slug + "@awrv-office")
			write("DTSTAMP:" + dtstamp)
			write("DTSTART:" + begin.UTC().Format("20060102T150405Z"))
			write("DTEND:" + begin.Add(eventDuration).UTC().Format("20060102T150405Z"))
			write("SUMMARY:" + escapeICS(summary))
			write("DESCRIPTION:" + escapeICS(desc))
			write("URL:" + baseURL + "/" + h.name + "/" + slug)
			if cfg.alarm >= 0 {
				write("BEGIN:VALARM")
				write("ACTION:DISPLAY")
				write("DESCRIPTION:" + escapeICS(summary))
				if cfg.alarm == 0 {
					write("TRIGGER:PT0M")
				} else {
					write("TRIGGER:-PT" + strconv.Itoa(cfg.alarm) + "M")
				}
				write("END:VALARM")
			}
			write("END:VEVENT")
		}
	}

	write("END:VCALENDAR")
	return sb.String(), nil
}

// requestBaseURL reconstructs the external base URL for event links,
// honouring the proxy's X-Forwarded-Proto (set by Fly).
func requestBaseURL(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host
}

type remindersData struct {
	Hours                []reminderHour
	Days                 []reminderDay
	NavDate, Theme, Page string
	ShowBanner           bool
}

type reminderHour struct {
	Name, Slug, Default string
	Checked             bool
}

type reminderDay struct {
	Name, Slug string
}

// handleReminders renders the reminder-subscription settings page.
func (s *Server) handleReminders(w http.ResponseWriter, r *http.Request) {
	// Dated nav so chrome links match SW precache keys even from this page.
	navDate := time.Now().In(userLocation(r)).Format("2006-01-02")
	data := remindersData{
		Hours: []reminderHour{
			{Name: "Lauds", Slug: "lauds", Default: "06:45", Checked: true},
			{Name: "Prime", Slug: "prime", Default: "07:30"},
			{Name: "Terce", Slug: "terce", Default: "09:00"},
			{Name: "Sext", Slug: "sext", Default: "12:00"},
			{Name: "None", Slug: "none", Default: "15:00"},
			{Name: "Vespers", Slug: "vespers", Default: "18:00", Checked: true},
			{Name: "Compline", Slug: "compline", Default: "21:00", Checked: true},
		},
		Days: []reminderDay{
			{Name: "Mon", Slug: "mon"}, {Name: "Tue", Slug: "tue"}, {Name: "Wed", Slug: "wed"},
			{Name: "Thu", Slug: "thu"}, {Name: "Fri", Slug: "fri"}, {Name: "Sat", Slug: "sat"},
			{Name: "Sun", Slug: "sun"},
		},
		NavDate:    navDate,
		Theme:      themeParam(r),
		Page:       "reminders",
		ShowBanner: false,
	}
	setHTMLCacheHeaders(w)
	if err := s.tmplReminders.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleICS serves the office.ics subscription feed.
func (s *Server) handleICS(w http.ResponseWriter, r *http.Request) {
	cfg, err := parseICSConfig(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body, err := s.buildICS(cfg, requestBaseURL(r), time.Now())
	if err != nil {
		http.Error(w, fmt.Sprintf("error building calendar: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(body))
}
