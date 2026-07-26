package render

import "github.com/orthodoxwest/office/internal/models"

// Body classes that retune the page's gold ornaments to the season. They tint
// only the gilding — drop caps, the hairlines framing the hour title, the ✦
// diamonds, the ✠ cross, the scroll-progress hairline — via the --ornament
// tokens in style.css. The liturgical color band and every functional use of
// gold (focus rings, disclosure markers, the Pray-now button) are untouched.
const (
	// SeasonClassPassiontide veils the gold to a muted violet-gray, as the
	// church veils its images from Passion Sunday through Holy Saturday.
	SeasonClassPassiontide = "season-passiontide"

	// SeasonClassEastertide lets the gold run a shade warmer and brighter
	// through Paschaltide.
	SeasonClassEastertide = "season-eastertide"
)

// SeasonClass returns the ornament body class for a liturgical season, or ""
// for the seasons that keep the ordinary gold. Callers pass the season of the
// thing on the page: the day's season for the home page, and the composed
// hour's season for an hour page — the latter follows the Vespers designation,
// so I Vespers of Easter on Holy Saturday evening is already unveiled.
//
// Only date-bearing pages carry a class. The year calendar spans every season
// at once and stays neutral, as do the error and reminder pages.
func SeasonClass(season models.Season) string {
	switch season {
	case models.Passiontide:
		return SeasonClassPassiontide
	case models.Easter:
		return SeasonClassEastertide
	}
	return ""
}
