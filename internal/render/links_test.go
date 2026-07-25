package render

import "testing"

func TestNavLinkKeepsChromeDated(t *testing.T) {
	const date = "2026-06-07"
	tests := []struct {
		base, want string
	}{
		{"/", "/?date=" + date},
		{"/lauds", "/lauds/" + date},
		{"/calendar", "/calendar/2026#d-" + date},
		{"/reminders", "/reminders"},
	}
	for _, tt := range tests {
		// The theme argument is ignored: appearance is client-side only.
		if got := NavLink(tt.base, "dark", date); got != tt.want {
			t.Errorf("NavLink(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}

	if got := NavLink("/lauds", "", ""); got != "/lauds" {
		t.Errorf("undated NavLink = %q, want /lauds", got)
	}
}

func TestStaticURL(t *testing.T) {
	if got := StaticURL("style.css", "abc"); got != "/static/style.css?v=abc" {
		t.Errorf("StaticURL = %q", got)
	}
	if got := StaticURL("/fonts/x.woff2", "v1"); got != "/static/fonts/x.woff2?v=v1" {
		t.Errorf("StaticURL = %q", got)
	}
	if got := StaticURL("app.js", ""); got != "/static/app.js" {
		t.Errorf("empty version should omit query, got %q", got)
	}
}
