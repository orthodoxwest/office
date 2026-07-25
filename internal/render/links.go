package render

import (
	"fmt"
	"strings"
	"time"
)

// NavLink builds a chrome navigation href for base ("/", "/lauds",
// "/calendar", "/reminders"), carrying the page's liturgical day so links hit
// the same URLs the service worker precaches. theme is ignored; appearance is
// client-side only (localStorage + data-theme), so no ?theme= is ever emitted.
func NavLink(base, theme, date string) string {
	_ = theme
	switch base {
	case "/":
		return HomeLink(date, "")
	case "/calendar":
		return CalendarLink(date, "")
	case "/reminders":
		return "/reminders"
	default:
		return HourLink(strings.TrimPrefix(base, "/"), date, "")
	}
}

// HomeLink builds the day-landing href, dated via ?date= when date is set.
func HomeLink(date, theme string) string {
	_ = theme
	href := "/"
	if date != "" {
		href = "/?date=" + date
	}
	return href
}

// HourLink builds an hour-page href, dated via a path segment when date is set.
func HourLink(hour, date, theme string) string {
	_ = theme
	href := "/" + hour
	if date != "" {
		href += "/" + date
	}
	return href
}

// CalendarLink links into the year calendar, anchored at the day's row.
func CalendarLink(date, theme string) string {
	_ = theme
	if date != "" {
		if parsed, err := time.Parse("2006-01-02", date); err == nil {
			return fmt.Sprintf("/calendar/%d#d-%s", parsed.Year(), date)
		}
	}
	return "/calendar"
}

// CalendarYearLink links to a calendar year without a day anchor.
func CalendarYearLink(year int, theme string) string {
	_ = theme
	return fmt.Sprintf("/calendar/%d", year)
}

// StaticURL returns a build-stamped static asset path.
// HTML and the service worker always reference assets this way so a deploy
// that changes CSS/JS produces a new URL; PWA cache entries for the previous
// stamp stay unused instead of pairing with new HTML.
func StaticURL(name, version string) string {
	name = strings.TrimPrefix(name, "/")
	if version == "" {
		return "/static/" + name
	}
	return fmt.Sprintf("/static/%s?v=%s", name, version)
}

// TitleCase capitalises the first letter, for display of lowercase season and
// hour identifiers ("advent" → "Advent").
func TitleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
