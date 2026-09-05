// Package web provides the HTTP server for the Divine Office application.
//
// It resolves requests to a liturgical day, drives the office engine, and
// hands finished view models to internal/render for HTML. Page markup, the
// template FuncMap, and text-to-HTML formatting all live there, not here.
package web

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/render"
	"github.com/orthodoxwest/office/internal/review"
	"github.com/orthodoxwest/office/internal/usage"
)

//go:embed static
var files embed.FS

// Server handles HTTP requests for the Divine Office web interface.
type Server struct {
	usage      *usage.Store
	engine     *office.Engine
	cache      *yearCache
	pages      *render.Pages
	addr       string
	version    string
	reviewed   map[string]bool
	provenance map[string]review.EntryProvenance
	suspicions map[string][]review.Suspicion
}

// New creates a new Server, loading the office engine and parsing templates.
func New(dataDir, addr string) (*Server, error) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}

	version := computeVersion(dataDir)
	pages, err := render.New(version)
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	reviewed, err := loadReviewedHashes(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading review signoffs: %w", err)
	}
	provenanceInventory, err := review.ScanProvenance(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading provenance: %w", err)
	}
	suspicions, err := review.SuspicionByKey(dataDir, provenanceInventory)
	if err != nil {
		return nil, fmt.Errorf("loading review suspicions: %w", err)
	}

	return &Server{
		engine:     eng,
		cache:      newYearCache(dataDir),
		pages:      pages,
		addr:       addr,
		version:    version,
		reviewed:   reviewed,
		provenance: provenanceInventory.ByKey(),
		suspicions: suspicions,
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
	if path := os.Getenv("OFFICE_USAGE_DB"); path != "" {
		var err error
		s.usage, err = usage.Open(path)
		if err != nil {
			log.Printf("warn: usage metrics disabled: %v", err)
		} else {
			defer s.usage.Close()
		}
	}
	go func() {
		year := time.Now().Year()
		if _, _, err := s.cache.get(year); err != nil {
			log.Printf("warn: pre-warming cache for %d: %v", year, err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/usage", s.handleUsageEvent)
	mux.HandleFunc("/admin/usage", s.handleUsageDashboard)
	// Static assets are served with long-lived cache headers when requested
	// with ?v=… (see staticFileServer). HTML stamps that query via static().
	mux.Handle("/static/", staticFileServer(http.FS(files)))
	mux.HandleFunc("/sw.js", s.handleServiceWorker)
	mux.HandleFunc("/office.ics", s.handleICS)
	mux.HandleFunc("/reminders", s.handleReminders)
	mux.HandleFunc("/calendar/", s.handleCalendar)
	mux.HandleFunc("/calendar", s.handleCalendar)
	mux.HandleFunc("/", s.handleRoot)
	return http.ListenAndServe(s.addr, mux)
}
