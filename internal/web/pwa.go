package web

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
)

func init() {
	// Not in the standard mime table on all systems; without it the embedded
	// file server would send text/plain and browsers would reject the manifest.
	if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
		log.Printf("warn: registering .webmanifest mime type: %v", err)
	}
}

// computeVersion derives a deterministic build identifier from the server
// binary and the data directory. It changes exactly when a deploy changes
// anything that can affect rendered pages, which makes it suitable for
// service-worker cache busting; it is stable across restarts of the same
// build. The Docker build has no git available, so a VCS hash is not an
// option here.
func computeVersion(dataDir string) string {
	h := sha256.New()

	if exe, err := os.Executable(); err == nil {
		if f, err := os.Open(exe); err == nil {
			_, _ = io.Copy(h, f)
			f.Close()
		}
	}

	dataFS := os.DirFS(dataDir)
	// WalkDir visits in lexical order, so the hash is deterministic.
	_ = fs.WalkDir(dataFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		io.WriteString(h, path)
		if f, err := dataFS.Open(path); err == nil {
			_, _ = io.Copy(h, f)
			f.Close()
		}
		return nil
	})

	return hex.EncodeToString(h.Sum(nil))[:12]
}

// staticFileServer serves embedded static assets with Cache-Control:
// no-cache so browsers and the service worker revalidate on every network
// fetch. The service worker still caches copies for offline use; this only
// prevents the browser HTTP cache from replaying a pre-deploy response under
// the same /static/ URL when the SW installs or does SWR.
func staticFileServer(fsys http.FileSystem) http.Handler {
	files := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}

// handleServiceWorker serves the embedded service worker at the site root
// (required for a "/" scope) with the build version stamped in, so any
// deploy produces a byte-different worker and triggers a cache refresh.
func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	src, err := files.ReadFile("static/sw.js")
	if err != nil {
		http.Error(w, "service worker unavailable", http.StatusInternalServerError)
		return
	}
	body := strings.ReplaceAll(string(src), "__VERSION__", s.version)
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(body))
}
