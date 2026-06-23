// Package web provides the HTTP server for the Divine Office application.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/review"
)

//go:embed templates static
var files embed.FS

// escCross replaces the ✠ cross character with a styled HTML span.
func escCross(s string) string {
	return strings.ReplaceAll(template.HTMLEscapeString(s), "✠", `<span class="cross">✠</span>`)
}

var templateFuncs = template.FuncMap{
	// navLink builds a navigation href preserving the active date and theme.
	// For the home page ("/") the date is a query param (?date=…).
	// For hour pages ("/lauds" etc.) the date is a path segment (/lauds/DATE).
	"navLink": func(base, theme, date string) string {
		switch base {
		case "/":
			return homeLink(date, theme)
		case "/calendar":
			return calendarLink(date, theme)
		case "/reminders":
			return appendTheme("/reminders", theme)
		default:
			return hourLink(strings.TrimPrefix(base, "/"), date, theme)
		}
	},
	"homeLink":              func(date, theme string) string { return homeLink(date, theme) },
	"hourLink":              func(hour, date, theme string) string { return hourLink(hour, date, theme) },
	"calendarYearLink":      func(year int, theme string) string { return calendarYearLink(year, theme) },
	"renderSectionElements": func(elems []models.OfficeElement) template.HTML { return renderSectionElements(elems) },
	"typeEq": func(t models.ElementType, s string) bool {
		return string(t) == s
	},
	"titleCase":       func(v interface{}) string { return titleCase(fmt.Sprint(v)) },
	"psalmVerses":     renderPsalmVerses,
	"liturgicalBlock": renderLiturgicalBlock,
	"hymnStanzas":     renderHymnStanzas,
	"gloriaPatri":     renderGloriaPatri,
}

func renderSectionElements(elems []models.OfficeElement) template.HTML {
	var sb strings.Builder

	for i := 0; i < len(elems); i++ {
		elem := elems[i]
		var doxologyText string
		if i+1 < len(elems) && (elem.Type == models.Psalm || elem.Type == models.Canticle) && elems[i+1].Type == models.PsalmDoxology {
			doxologyText = elems[i+1].Text
			i++
		}
		sb.WriteString(renderOfficeElement(elem, doxologyText))
	}

	return template.HTML(sb.String())
}

func renderOfficeElement(elem models.OfficeElement, doxologyText string) string {
	var sb strings.Builder

	switch elem.Type {
	case models.Heading:
		sb.WriteString(`<h2 class="section-heading">`)
		sb.WriteString(template.HTMLEscapeString(elem.Text))
		sb.WriteString(`</h2>`)
	case models.Rubric:
		sb.WriteString(`<p class="rubric">`)
		sb.WriteString(template.HTMLEscapeString(elem.Text))
		sb.WriteString(`</p>`)
	case models.Antiphon:
		if elem.Label != "" {
			sb.WriteString(`<div class="marian-antiphon"><h3 class="item-label">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			sb.WriteString(`</h3>`)
			sb.WriteString(string(renderLiturgicalBlock(elem.Text)))
			sb.WriteString(`</div>`)
		} else {
			sb.WriteString(`<p class="antiphon"><em>Ant.</em> `)
			sb.WriteString(escCross(elem.Text))
			sb.WriteString(`</p>`)
		}
	case models.Psalm, models.Canticle:
		className := string(elem.Type)
		sb.WriteString(`<div class="`)
		sb.WriteString(className)
		sb.WriteString(`">`)
		if elem.Label != "" {
			sb.WriteString(`<h3 class="item-label">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			sb.WriteString(`</h3>`)
		}
		sb.WriteString(string(renderPsalmVerses(elem.Text)))
		if doxologyText != "" {
			sb.WriteString(string(renderGloriaPatri(doxologyText)))
		}
		sb.WriteString(`</div>`)
	case models.Hymn:
		sb.WriteString(`<div class="hymn"><h2 class="section-heading">Hymn</h2>`)
		if elem.Label != "" {
			sb.WriteString(`<p class="hymn-title">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			sb.WriteString(`</p>`)
		}
		sb.WriteString(string(renderHymnStanzas(elem.Text)))
		sb.WriteString(`</div>`)
	case models.Versicle, models.Response, models.Collect, models.Prayer, models.Blessing, models.Doxology:
		sb.WriteString(string(renderLiturgicalBlock(elem.Text)))
	case models.Chapter:
		sb.WriteString(`<div class="chapter"><h2 class="section-heading">Chapter</h2>`)
		if elem.Label != "" {
			sb.WriteString(`<p class="chapter-ref">`)
			sb.WriteString(template.HTMLEscapeString(elem.Label))
			sb.WriteString(`</p>`)
		}
		sb.WriteString(string(renderLiturgicalBlock(elem.Text)))
		sb.WriteString(`</div>`)
	case models.Preces:
		sb.WriteString(`<div class="preces">`)
		sb.WriteString(string(renderLiturgicalBlock(elem.Text)))
		sb.WriteString(`</div>`)
	case models.PsalmDoxology:
		sb.WriteString(string(renderGloriaPatri(elem.Text)))
	default:
		sb.WriteString(`<p class="element">`)
		sb.WriteString(template.HTMLEscapeString(elem.Text))
		sb.WriteString(`</p>`)
	}

	return sb.String()
}

// renderPsalmVerses parses a raw psalm or canticle text (title/scripture-ref line,
// numbered verses with " * " mediants, Gloria Patri) into structured HTML.
//
// Scripture references (lines beginning with "!") are emitted as a
// <p class="scripture-ref"> BEFORE the psalm-verses div, so that the first
// verse is always the first child of .psalm-verses and receives the drop cap.
func renderPsalmVerses(text string) template.HTML {
	lines := strings.Split(text, "\n")
	var sb strings.Builder

	scriptureRef := ""
	contentStart := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			contentStart = i + 1
			break
		}
		if strings.HasPrefix(trimmed, "!") {
			scriptureRef = trimmed[1:]
		}
	}

	if scriptureRef != "" {
		sb.WriteString(`<p class="scripture-ref">`)
		sb.WriteString(template.HTMLEscapeString(scriptureRef))
		sb.WriteString(`</p>`)
	}

	sb.WriteString(`<div class="psalm-verses">`)

	gloriaOpen := false
	for _, line := range lines[contentStart:] {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[section:") && strings.HasSuffix(line, "]") {
			heading := strings.TrimSpace(line[9 : len(line)-1])
			sb.WriteString(`</div>`)
			sb.WriteString(`<p class="canticle-section">`)
			sb.WriteString(template.HTMLEscapeString(heading))
			sb.WriteString(`</p>`)
			sb.WriteString(`<div class="psalm-verses">`)
			continue
		}

		if strings.HasPrefix(line, "Glory be") {
			sb.WriteString(`<p class="verse">`)
			sb.WriteString(template.HTMLEscapeString(line))
			gloriaOpen = true
			continue
		}
		if (strings.HasPrefix(line, "as it was") || strings.HasPrefix(line, "As it was")) && gloriaOpen {
			sb.WriteString(` <span class="mediant">*</span> `)
			sb.WriteString(template.HTMLEscapeString(line))
			sb.WriteString(`</p>`)
			gloriaOpen = false
			continue
		}

		verseNum := ""
		verseText := line
		if dotIdx := strings.Index(line, ". "); dotIdx > 0 && dotIdx < 4 {
			numStr := line[:dotIdx]
			allDigits := true
			for _, c := range numStr {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				verseNum = numStr
				verseText = strings.TrimSpace(line[dotIdx+2:])
			}
		}

		parts := strings.SplitN(verseText, " * ", 2)

		if verseNum != "" {
			sb.WriteString(`<p class="verse numbered"><span class="verse-num">`)
			sb.WriteString(template.HTMLEscapeString(verseNum))
			sb.WriteString(`</span><span class="verse-body">`)
			sb.WriteString(escCross(parts[0]))
			if len(parts) == 2 {
				sb.WriteString(` <span class="mediant">*</span> `)
				sb.WriteString(escCross(parts[1]))
			}
			sb.WriteString(`</span></p>`)
		} else {
			sb.WriteString(`<p class="verse">`)
			sb.WriteString(escCross(parts[0]))
			if len(parts) == 2 {
				sb.WriteString(` <span class="mediant">*</span> `)
				sb.WriteString(escCross(parts[1]))
			}
			sb.WriteString(`</p>`)
		}
	}

	if gloriaOpen {
		sb.WriteString(`</p>`)
	}
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

// renderLiturgicalBlock renders a multi-line liturgical text (versicles, responses,
// prayers, chapters, Marian antiphons) with proper markup for each line type.
func renderLiturgicalBlock(text string) template.HTML {
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	sb.WriteString(`<div class="liturgical-block">`)

	var proseLines []string
	pendingGap := false

	emitGap := func() {
		if pendingGap {
			sb.WriteString(`<div class="liturgical-gap"></div>`)
			pendingGap = false
		}
	}

	flushProse := func() {
		if len(proseLines) == 0 {
			return
		}
		emitGap()
		sb.WriteString(`<p class="plain-line">`)
		for i, l := range proseLines {
			if i > 0 {
				sb.WriteString(`<br>`)
			}
			sb.WriteString(escCross(l))
		}
		sb.WriteString(`</p>`)
		proseLines = nil
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			flushProse()
			pendingGap = true
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") &&
			strings.Contains(line[1:len(line)-1], " ") {
			continue
		}

		if strings.HasPrefix(line, "!") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="scripture-ref">`)
			sb.WriteString(template.HTMLEscapeString(line[1:]))
			sb.WriteString(`</p>`)
			continue
		}

		if strings.HasPrefix(line, "V. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="versicle-line"><span class="sigil">℣.</span><span class="sigil-text">`)
			sb.WriteString(escCross(line[3:]))
			sb.WriteString(`</span></p>`)
			continue
		}

		if strings.HasPrefix(line, "R. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="response-line"><span class="sigil">℟.</span><span class="sigil-text">`)
			sb.WriteString(escCross(line[3:]))
			sb.WriteString(`</span></p>`)
			continue
		}

		if strings.HasPrefix(line, "Blessing. ") {
			flushProse()
			emitGap()
			sb.WriteString(`<p class="versicle-line"><span class="sigil">Blessing.</span><span class="sigil-text">`)
			sb.WriteString(escCross(line[10:]))
			sb.WriteString(`</span></p>`)
			continue
		}

		proseLines = append(proseLines, line)
	}

	flushProse()
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

// renderHymnStanzas parses a hymn text into structured HTML with per-stanza <p> elements.
func renderHymnStanzas(text string) template.HTML {
	var sb strings.Builder

	title := ""
	body := strings.TrimSpace(text)
	if firstBlock, rest, found := strings.Cut(body, "\n\n"); found {
		if !strings.ContainsRune(strings.TrimSpace(firstBlock), '\n') {
			title = strings.TrimSpace(firstBlock)
			body = strings.TrimSpace(rest)
		}
	}

	sb.WriteString(`<div class="hymn-verses">`)
	if title != "" {
		sb.WriteString(`<p class="hymn-latin">`)
		sb.WriteString(template.HTMLEscapeString(title))
		sb.WriteString(`</p>`)
	}

	var stanza []string
	emitStanza := func() {
		if len(stanza) == 0 {
			return
		}
		sb.WriteString(`<p class="hymn-stanza">`)
		for i, l := range stanza {
			if i > 0 {
				sb.WriteString(`<br>`)
			}
			sb.WriteString(escCross(l))
		}
		sb.WriteString(`</p>`)
		stanza = nil
	}
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "" {
			emitStanza()
		} else {
			stanza = append(stanza, trimmed)
		}
	}
	emitStanza()

	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

// renderGloriaPatri renders the two-line Gloria Patri text with a * mediant break.
func renderGloriaPatri(text string) template.HTML {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var line1, line2 string
	if len(lines) >= 2 {
		line1 = strings.TrimSpace(lines[0])
		line2 = strings.TrimSpace(lines[1])
	} else {
		line1 = strings.TrimSpace(text)
	}
	var sb strings.Builder
	sb.WriteString(`<p class="gloria-patri">`)
	sb.WriteString(template.HTMLEscapeString(line1))
	if line2 != "" {
		sb.WriteString(` <span class="mediant">*</span> `)
		sb.WriteString(template.HTMLEscapeString(line2))
	}
	sb.WriteString(`</p>`)
	return template.HTML(sb.String())
}

// Server handles HTTP requests for the Divine Office web interface.
type Server struct {
	engine        *office.Engine
	cache         *yearCache
	tmplHome      *template.Template
	tmplHour      *template.Template
	tmplCalendar  *template.Template
	tmpl404       *template.Template
	tmplError     *template.Template
	tmplReminders *template.Template
	addr          string
	version       string
	reviewed      map[string]bool
}

// New creates a new Server, loading the office engine and parsing templates.
func New(dataDir, addr string) (*Server, error) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}

	tmplHome, err := template.New("").Funcs(templateFuncs).ParseFS(files,
		"templates/layout.html", "templates/home.html")
	if err != nil {
		return nil, fmt.Errorf("parsing home template: %w", err)
	}

	tmplHour, err := template.New("").Funcs(templateFuncs).ParseFS(files,
		"templates/layout.html", "templates/hour.html")
	if err != nil {
		return nil, fmt.Errorf("parsing hour template: %w", err)
	}

	tmplCalendar, err := template.New("").Funcs(templateFuncs).ParseFS(files,
		"templates/layout.html", "templates/calendar.html")
	if err != nil {
		return nil, fmt.Errorf("parsing calendar template: %w", err)
	}

	tmpl404, err := template.New("").Funcs(templateFuncs).ParseFS(files,
		"templates/layout.html", "templates/404.html")
	if err != nil {
		return nil, fmt.Errorf("parsing 404 template: %w", err)
	}

	tmplError, err := template.New("").Funcs(templateFuncs).ParseFS(files,
		"templates/layout.html", "templates/error.html")
	if err != nil {
		return nil, fmt.Errorf("parsing error template: %w", err)
	}

	tmplReminders, err := template.New("").Funcs(templateFuncs).ParseFS(files,
		"templates/layout.html", "templates/reminders.html")
	if err != nil {
		return nil, fmt.Errorf("parsing reminders template: %w", err)
	}

	reviewed, err := loadReviewedHashes(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading review signoffs: %w", err)
	}

	return &Server{
		engine:        eng,
		cache:         newYearCache(dataDir),
		tmplHome:      tmplHome,
		tmplHour:      tmplHour,
		tmplCalendar:  tmplCalendar,
		tmpl404:       tmpl404,
		tmplError:     tmplError,
		tmplReminders: tmplReminders,
		addr:          addr,
		version:       computeVersion(dataDir),
		reviewed:      reviewed,
	}, nil
}

func loadReviewedHashes(dataDir string) (map[string]bool, error) {
	signoffs, err := review.LoadSignoffs(dataDir)
	if err != nil {
		return nil, err
	}
	reviewed := make(map[string]bool, len(signoffs))
	for _, s := range signoffs {
		reviewed[s.Hash] = true
	}
	return reviewed, nil
}

func (s *Server) showVettingBanner(hour *models.OfficeHour) bool {
	if hour == nil {
		return true
	}
	return !s.reviewed[review.HashHour(hour)]
}

// ListenAndServe registers routes and starts the HTTP server.
func (s *Server) ListenAndServe() error {
	go func() {
		year := time.Now().Year()
		if _, _, err := s.cache.get(year); err != nil {
			log.Printf("warn: pre-warming cache for %d: %v", year, err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(files)))
	mux.HandleFunc("/sw.js", s.handleServiceWorker)
	mux.HandleFunc("/office.ics", s.handleICS)
	mux.HandleFunc("/reminders", s.handleReminders)
	mux.HandleFunc("/calendar/", s.handleCalendar)
	mux.HandleFunc("/calendar", s.handleCalendar)
	mux.HandleFunc("/", s.handleRoot)
	return http.ListenAndServe(s.addr, mux)
}
