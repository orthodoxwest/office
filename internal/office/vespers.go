package office

import (
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// VespersComposer composes the hour of Vespers.
type VespersComposer struct{}

type vespersPsalmodyProfile string

const (
	vespersPsalmodyStandard          vespersPsalmodyProfile = "standard"
	vespersPsalmodyApostlesII        vespersPsalmodyProfile = "apostles-ii"
	vespersPsalmodyMartyrsII         vespersPsalmodyProfile = "martyrs-ii"
	vespersPsalmodyConfessorBishopII vespersPsalmodyProfile = "confessor-bishop-ii"
	vespersPsalmodyBVM               vespersPsalmodyProfile = "bvm"
	vespersPsalmodyDedication        vespersPsalmodyProfile = "dedication"
	vespersPsalmodyNativityII        vespersPsalmodyProfile = "nativity-ii"
	vespersPsalmodyStephen           vespersPsalmodyProfile = "st-stephen"
	vespersPsalmodyJohn              vespersPsalmodyProfile = "st-john"
	vespersPsalmodyCorpusChristi     vespersPsalmodyProfile = "corpus-christi"
	vespersPsalmodyArchangel         vespersPsalmodyProfile = "archangel"
)

func (p vespersPsalmodyProfile) valid() bool {
	switch p {
	case vespersPsalmodyStandard,
		vespersPsalmodyApostlesII,
		vespersPsalmodyMartyrsII,
		vespersPsalmodyConfessorBishopII,
		vespersPsalmodyBVM,
		vespersPsalmodyDedication,
		vespersPsalmodyNativityII,
		vespersPsalmodyStephen,
		vespersPsalmodyJohn,
		vespersPsalmodyCorpusChristi,
		vespersPsalmodyArchangel:
		return true
	default:
		return false
	}
}

type vespersPsalmodyPair struct {
	first  vespersPsalmodyProfile
	second vespersPsalmodyProfile
}

// properFestalVespersPsalmody records the major-feast offices printed in the
// parish's Vespers book. An empty side is intentional: the book only attests
// the other Vespers, so the unprinted side remains on the psalter.
var properFestalVespersPsalmody = map[string]vespersPsalmodyPair{
	"christmas":             {first: vespersPsalmodyStandard, second: vespersPsalmodyNativityII},
	"st-stephen":            {first: vespersPsalmodyStephen, second: vespersPsalmodyStephen},
	"st-john-evangelist":    {first: vespersPsalmodyJohn, second: vespersPsalmodyJohn},
	"holy-innocents":        {first: vespersPsalmodyStephen, second: vespersPsalmodyStephen},
	"circumcision":          {first: vespersPsalmodyBVM, second: vespersPsalmodyBVM},
	"holy-name-jesus":       {first: vespersPsalmodyStandard, second: vespersPsalmodyStandard},
	"vigil-epiphany":        {first: vespersPsalmodyBVM, second: vespersPsalmodyBVM},
	"epiphany":              {first: vespersPsalmodyStandard, second: vespersPsalmodyStandard},
	"ascension":             {first: vespersPsalmodyStandard, second: vespersPsalmodyStandard},
	"pentecost":             {first: vespersPsalmodyStandard},
	"trinity-sunday":        {first: vespersPsalmodyStandard},
	"corpus-christi":        {first: vespersPsalmodyCorpusChristi, second: vespersPsalmodyCorpusChristi},
	"chair-peter-antioch":   {first: vespersPsalmodyStandard, second: vespersPsalmodyConfessorBishopII},
	"st-gregory-great":      {first: vespersPsalmodyStandard, second: vespersPsalmodyConfessorBishopII},
	"finding-holy-cross":    {first: vespersPsalmodyStandard, second: vespersPsalmodyStandard},
	"nativity-john-baptist": {first: vespersPsalmodyStandard, second: vespersPsalmodyStandard},
	"transfiguration":       {first: vespersPsalmodyStandard, second: vespersPsalmodyStandard},
	"exaltation-holy-cross": {first: vespersPsalmodyStandard},
	"dedication-st-michael": {first: vespersPsalmodyArchangel, second: vespersPsalmodyArchangel},
	"christ-the-king":       {first: vespersPsalmodyStandard},
	"all-saints":            {first: vespersPsalmodyStandard, second: vespersPsalmodyMartyrsII},
}

// commonFestalVespersPsalmody mirrors the offices printed in the Commons book.
// It deliberately omits categories for which that book supplies no common.
var commonFestalVespersPsalmody = map[models.FeastCategory]vespersPsalmodyPair{
	models.CategoryApostle:         {first: vespersPsalmodyStandard, second: vespersPsalmodyApostlesII},
	models.CategoryEvangelist:      {first: vespersPsalmodyStandard, second: vespersPsalmodyApostlesII},
	models.CategoryMartyr:          {first: vespersPsalmodyStandard, second: vespersPsalmodyMartyrsII},
	models.CategoryMartyrs:         {first: vespersPsalmodyStandard, second: vespersPsalmodyMartyrsII},
	models.CategoryBishopMartyr:    {first: vespersPsalmodyStandard, second: vespersPsalmodyMartyrsII},
	models.CategoryVirginMartyr:    {first: vespersPsalmodyStandard, second: vespersPsalmodyMartyrsII},
	models.CategoryConfessorBishop: {first: vespersPsalmodyStandard, second: vespersPsalmodyConfessorBishopII},
	models.CategoryConfessor:       {first: vespersPsalmodyStandard, second: vespersPsalmodyStandard},
	models.CategoryVirgin:          {first: vespersPsalmodyBVM, second: vespersPsalmodyBVM},
	models.CategoryHolyWoman:       {first: vespersPsalmodyBVM, second: vespersPsalmodyBVM},
	models.CategoryBlessedVirgin:   {first: vespersPsalmodyBVM, second: vespersPsalmodyBVM},
	models.CategoryDedication:      {first: vespersPsalmodyDedication, second: vespersPsalmodyDedication},
}

// festalVespersPsalmody returns the book-attested psalmody profile for this
// office. Proper major feasts take precedence over their broad common.
func festalVespersPsalmody(day *models.CalendarDay) vespersPsalmodyProfile {
	if day == nil || day.Celebration == nil {
		return ""
	}

	for _, id := range feastProperIDs(day.Celebration) {
		if pair, ok := properFestalVespersPsalmody[id]; ok {
			return pair.forOffice(day.FirstVespers)
		}
	}

	return commonFestalVespersPsalmody[day.Celebration.Category].forOffice(day.FirstVespers)
}

func usesFestalVespersPsalmody(day *models.CalendarDay) bool {
	return festalVespersPsalmody(day) != ""
}

func (p vespersPsalmodyPair) forOffice(first bool) vespersPsalmodyProfile {
	if first {
		return p.first
	}
	return p.second
}

// Compose builds a complete Vespers hour for the given day.
func (v *VespersComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) (*models.OfficeHour, error) {
	return composeMajorHour(day, sections, corpus, moveable, majorHourOptions{
		hourName:  "vespers",
		title:     "Vespers",
		officeDay: vespersOfficeDay,
	})
}

func vespersOfficeDay(day *models.CalendarDay) *models.CalendarDay {
	if day == nil {
		return day
	}

	officeDay := *day
	if day.Vespers.Owner == models.VespersNotApplicable || day.Vespers.Feast == nil {
		// No adjacent celebration owns Vespers. The office remains today's,
		// but its occurrence commemorations belong to tomorrow rather than
		// carrying today's Lauds commemorations one evening late (XIV.9).
		officeDay.Commemorations = day.Vespers.Commemorations
		return &officeDay
	}

	officeDay.Celebration = day.Vespers.Feast
	officeDay.Color = day.Vespers.Color
	if day.Vespers.Season != "" {
		officeDay.Season = day.Vespers.Season
	}

	switch day.Vespers.Owner {
	case models.VespersIOfFollowing:
		// Vespers belongs liturgically to tomorrow's feast; only the outgoing
		// office (today's celebration, if any) is commemorated (XIII.2-17).
		officeDay.Date = day.Date.Add(24 * time.Hour)
		officeDay.Commemorations = day.Vespers.Commemorations
		officeDay.Tempora = ""
		officeDay.WithinOctaveOf = ""
		officeDay.FirstVespers = true
	case models.VespersIIOfPreceding:
		// Calendar resolution has already combined and filtered today's
		// occurrence commemorations with the incoming concurrence boundary.
		officeDay.Commemorations = day.Vespers.Commemorations
	}

	return &officeDay
}
