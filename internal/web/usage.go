package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/orthodoxwest/office/internal/render"
	"github.com/orthodoxwest/office/internal/usage"
)

func usageHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
}

func (s *Server) handleUsageEvent(w http.ResponseWriter, r *http.Request) {
	usageHeaders(w)
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// A custom header prevents cross-origin form posts and requires a CORS
	// preflight for scripts. We do not grant CORS access to this endpoint.
	if r.Header.Get("X-Office-Usage") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host || (u.Scheme != "https" && u.Scheme != "http") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32))
	if err != nil || !usage.ValidScope(string(body)) {
		http.Error(w, "Invalid office", http.StatusBadRequest)
		return
	}
	if s.usage == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	const cookieName = "office-usage"
	var id string
	if c, err := r.Cookie(cookieName); err == nil {
		if decoded, err := hex.DecodeString(c.Value); err == nil && len(decoded) == 16 {
			id = c.Value
		}
	}
	if id == "" {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		id = hex.EncodeToString(random[:])
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: id, Path: "/api/usage", MaxAge: 30 * 24 * 60 * 60,
			HttpOnly: true, SameSite: http.SameSiteStrictMode,
			Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"})
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.usage.Record(ctx, time.Now(), id, string(body)); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUsageDashboard(w http.ResponseWriter, r *http.Request) {
	usageHeaders(w)
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.usage == nil {
		http.NotFound(w, r)
		return
	}
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		var err error
		days, err = strconv.Atoi(raw)
		if err != nil || (days != 7 && days != 30 && days != 90 && days != 366) {
			http.Error(w, "Choose 7, 30, 90 or 366 days", http.StatusBadRequest)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	rows, err := s.usage.Daily(ctx, time.Now(), days)
	if err != nil {
		http.Error(w, "Usage temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	data := render.NewUsageData(rows, days)
	var buf bytes.Buffer
	if err := s.pages.Usage(&buf, data); err != nil {
		http.Error(w, "Unable to render usage", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodGet {
		_, _ = w.Write(buf.Bytes())
	}
}
