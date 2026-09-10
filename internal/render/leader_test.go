package render

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestLeaderSectionsShareCommonTextAndExpandBoundedSlots(t *testing.T) {
	var forms []LeaderForm
	for _, form := range models.PrayerForms {
		elements := []models.OfficeElement{{Type: models.Prayer, Text: "Common before"}}
		if form == models.PrayerPrivate {
			elements = append(elements, models.OfficeElement{Type: models.Prayer, Text: "Private confession", LeaderSlot: "confession"})
		} else {
			elements = append(elements, models.OfficeElement{Type: models.Rubric, Text: "Choir rubric", LeaderSlot: "confession"}, models.OfficeElement{Type: models.Prayer, Text: "Choir confession", LeaderSlot: "confession"})
		}
		elements = append(elements, models.OfficeElement{Type: models.Prayer, Text: "Common after"})
		forms = append(forms, LeaderForm{Form: form, Hour: &models.OfficeHour{Sections: []models.OfficeSection{{Elements: elements}}}})
	}
	sections, err := leaderSections(forms)
	if err != nil {
		t.Fatal(err)
	}
	html := string(sections[0].HTML)
	for _, text := range []string{"Common before", "Common after", "Private confession", "Choir confession", "Choir rubric"} {
		if strings.Count(html, text) != 1 {
			t.Errorf("%q not emitted exactly once: %s", text, html)
		}
	}
	if !strings.Contains(html, `data-leaders="deacon priest"`) {
		t.Error("identical choir forms not shared")
	}
	forms[2].Hour.Sections[0].Elements[0].Text = "unexpected variant"
	if _, err := leaderSections(forms); err == nil {
		t.Error("unmarked variation accepted")
	}
}

func TestPrayerSpeakerLabelsKeepResponsesInOrder(t *testing.T) {
	elem := models.OfficeElement{Type: models.Prayer, Text: "Have mercy upon thee.\nR. Amen.", Voice: []models.VoiceSpan{
		{Text: "Have mercy upon thee.\n", Spoken: true, Role: models.VoiceResponse},
		{Text: "R. Amen.", Spoken: true, Role: models.VoicePriest},
	}}
	html := renderOfficeElement(elem, "")
	for _, want := range []string{`data-speaker="response"><p class="prayer-speaker">People</p>`, `data-speaker="priest"><p class="prayer-speaker">Priest</p>`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing speaker: %s", html)
		}
	}
	if strings.Index(html, "People</p>") > strings.Index(html, "Have mercy") || strings.Index(html, "Priest</p>") > strings.Index(html, "Amen.") {
		t.Errorf("speaker must precede their words: %s", html)
	}
	// Invalid metadata must not truncate the source prayer or invent labels.
	elem.Voice = elem.Voice[:1]
	html = renderOfficeElement(elem, "")
	if !strings.Contains(html, "Amen.") || strings.Contains(html, "prayer-speaker") {
		t.Errorf("invalid partition lost text: %s", html)
	}
}
