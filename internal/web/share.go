package web

import (
	"net/http"
	"time"

	"github.com/orthodoxwest/office/internal/render"
)

// handleShare renders the QR-code page for passing the app along.
//
// The code encodes the undated homepage of whatever host served the request,
// so a parish running its own copy shares its own address, and a printed card
// keeps working: the homepage resolves to the reader's today, not to the day
// the card was printed.
func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	shareURL := requestBaseURL(r) + "/"
	svg, err := render.QRCodeSVG(shareURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Dated nav so chrome links match SW precache keys even from this page.
	navDate := time.Now().In(userLocation(r)).Format("2006-01-02")
	data := render.ShareData{
		ShareURL:   shareURL,
		QRCode:     svg,
		NavDate:    navDate,
		Theme:      themeParam(r),
		Page:       "share",
		ShowBanner: false,
	}
	setHTMLCacheHeaders(w)
	if err := s.pages.Share(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
