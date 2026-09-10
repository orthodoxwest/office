package office

import (
	"fmt"
	"github.com/orthodoxwest/office/internal/models"
	"regexp"
)

// applyLeader resolves explicitly marked ordinary slots after calendar
// composition. It never searches or replaces words in unrelated prayers.
func (e *Engine) applyLeader(hour *models.OfficeHour, leader models.PrayerForm) error {
	hour.Form = leader
	recorded := map[string]bool{}
	for si := range hour.Sections {
		var resolved []models.OfficeElement
		for _, elem := range hour.Sections[si].Elements {
			if elem.LeaderSlot != "" && !recorded[elem.LeaderSlot] {
				outcome := "private"
				if leader != models.PrayerPrivate {
					outcome = "choir"
				}
				hour.Decisions = append(hour.Decisions, models.CompositionDecision{Rule: "prayer-form:" + elem.LeaderSlot, Outcome: outcome})
				recorded[elem.LeaderSlot] = true
			}
			var replacement []models.OfficeElement
			var err error
			switch elem.LeaderSlot {
			case "greeting":
				key := "greeting-clergy"
				if leader == models.PrayerPrivate {
					key = "greeting-lay"
				}
				var greeting models.OfficeElement
				greeting, err = e.leaderElement(models.Versicle, key)
				replacement = []models.OfficeElement{greeting}
			case "opening":
				if leader != models.PrayerPrivate {
					var opening models.OfficeElement
					opening, err = e.leaderElement(models.Versicle, "compline-opening-choir")
					replacement = []models.OfficeElement{opening}
				}
			case "confession":
				if leader != models.PrayerPrivate {
					replacement, err = e.choirConfession()
				}
			}
			if err != nil {
				return err
			}
			if replacement == nil {
				replacement = []models.OfficeElement{elem}
			}
			for i := range replacement {
				replacement[i].LeaderSlot = elem.LeaderSlot
			}
			resolved = append(resolved, replacement...)
		}
		hour.Sections[si].Elements = resolved
	}
	hour.Decisions = append(hour.Decisions, models.CompositionDecision{Rule: "context:prayer-form", Outcome: string(leader)})
	return nil
}

func (e *Engine) leaderElement(kind models.ElementType, slot string) (models.OfficeElement, error) {
	key := "shared/leader/" + slot
	text := e.corpus.Get(key)
	if text == "" {
		return models.OfficeElement{}, fmt.Errorf("missing leader formula %s", key)
	}
	return models.OfficeElement{Type: kind, Text: text, SourceRef: key, SourceRefs: []string{key}}, nil
}

var confiteorBrethren = regexp.MustCompile(`\byou,\s+brethren\b`)

// The printed choir rubric (Diurnal pp. 7–8, repeated at 147–148) directs
// repetition of the confession with "thee, father" for "you, brethren".
// Keep that derivation and both source dependencies explicit. For the app's
// present scope, private is available to everyone; ordained choices mean choir.
func (e *Engine) choirConfession() ([]models.OfficeElement, error) {
	var elements []models.OfficeElement
	for _, part := range []struct {
		kind models.ElementType
		slot string
	}{
		{models.Rubric, "confiteor-officiant-rubric"},
		{models.Prayer, "confiteor-officiant"},
		{models.Prayer, "confiteor-choir-response"},
		{models.Rubric, "confiteor-choir-rubric"},
		{models.Prayer, "confiteor-officiant"},
		{models.Rubric, "confiteor-response-rubric"},
		{models.Prayer, "confiteor-officiant-response"},
		{models.Prayer, "confiteor-absolution"},
	} {
		elem, err := e.leaderElement(part.kind, part.slot)
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
	}
	choir := &elements[4]
	if len(confiteorBrethren.FindAllStringIndex(choir.Text, -1)) != 2 {
		return nil, fmt.Errorf("choir confession requires the two printed brethren addresses")
	}
	choir.Text = confiteorBrethren.ReplaceAllString(choir.Text, "thee, father")
	choir.SourceRefs = append(choir.SourceRefs, "shared/leader/confiteor-choir-rubric")
	elements[2].Label = "Choir"
	return elements, nil
}
