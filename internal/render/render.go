// Package render turns composed office data into HTML pages.
//
// It owns the embedded page templates, the template FuncMap, and every
// text-to-HTML conversion those templates depend on. Nothing in here touches
// the calendar or office engines: callers (internal/web) resolve the day,
// compose the hour, fill in a view model from this package, and hand it back
// to be rendered. Keeping presentation behind that boundary means the engine
// packages can be exercised — and mutated — without HTML formatting noise.
package render

import (
	"embed"
	"fmt"
	"html/template"
	"io"

	"github.com/orthodoxwest/office/internal/models"
)

//go:embed templates
var templates embed.FS

// Pages is the parsed set of page templates. Build one with New and reuse it
// for the process lifetime; the parsed templates are safe for concurrent use.
type Pages struct {
	home      *template.Template
	hour      *template.Template
	calendar  *template.Template
	notFound  *template.Template
	errorPage *template.Template
	reminders *template.Template
}

// New parses the embedded templates. version stamps static asset URLs (see
// StaticURL) so a deploy that changes CSS or JS produces new asset URLs and
// cached HTML cannot pair with a mismatched body.
func New(version string) (*Pages, error) {
	funcs := templateFuncs(version)
	p := &Pages{}
	for _, page := range []struct {
		file string
		dst  **template.Template
	}{
		{"home.html", &p.home},
		{"hour.html", &p.hour},
		{"calendar.html", &p.calendar},
		{"404.html", &p.notFound},
		{"error.html", &p.errorPage},
		{"reminders.html", &p.reminders},
	} {
		tmpl, err := template.New("").Funcs(funcs).ParseFS(templates,
			"templates/layout.html", "templates/"+page.file)
		if err != nil {
			return nil, fmt.Errorf("parsing %s template: %w", page.file, err)
		}
		*page.dst = tmpl
	}
	return p, nil
}

// Home renders the day's landing page.
func (p *Pages) Home(w io.Writer, data HomeData) error {
	return p.home.ExecuteTemplate(w, "layout", data)
}

// Hour renders a composed office hour.
func (p *Pages) Hour(w io.Writer, data HourData) error {
	return p.hour.ExecuteTemplate(w, "layout", data)
}

// Calendar renders the year calendar.
func (p *Pages) Calendar(w io.Writer, data CalendarData) error {
	return p.calendar.ExecuteTemplate(w, "layout", data)
}

// Reminders renders the reminder-subscription settings page.
func (p *Pages) Reminders(w io.Writer, data RemindersData) error {
	return p.reminders.ExecuteTemplate(w, "layout", data)
}

// NotFound renders the styled 404 page.
func (p *Pages) NotFound(w io.Writer, data NotFoundData) error {
	return p.notFound.ExecuteTemplate(w, "layout", data)
}

// ErrorPage renders the styled error page for other 4xx/5xx conditions.
func (p *Pages) ErrorPage(w io.Writer, data ErrorData) error {
	return p.errorPage.ExecuteTemplate(w, "layout", data)
}

// TemplateSource returns the raw source of an embedded template
// ("layout.html", "calendar.html", …). Tests assert on markup that no view
// model can reach, such as the SVG sprite definitions.
func TemplateSource(name string) ([]byte, error) {
	return templates.ReadFile("templates/" + name)
}

// templateFuncs builds the template FuncMap, including static() which stamps
// asset URLs with the build version so HTML and CSS/JS stay matched.
func templateFuncs(version string) template.FuncMap {
	return template.FuncMap{
		// Link helpers are documented in links.go. Templates should pass the
		// page's liturgical day (YYYY-MM-DD) so chrome hrefs hit the same URLs
		// the service worker precaches.
		"navLink":               NavLink,
		"homeLink":              HomeLink,
		"hourLink":              HourLink,
		"calendarYearLink":      CalendarYearLink,
		"static":                func(name string) string { return StaticURL(name, version) },
		"renderSectionElements": renderSectionElements,
		"typeEq": func(t models.ElementType, s string) bool {
			return string(t) == s
		},
		"titleCase":       func(v any) string { return TitleCase(fmt.Sprint(v)) },
		"psalmVerses":     renderPsalmVerses,
		"liturgicalBlock": renderLiturgicalBlock,
		"hymnStanzas":     renderHymnStanzas,
		"gloriaPatri":     renderGloriaPatri,
	}
}
