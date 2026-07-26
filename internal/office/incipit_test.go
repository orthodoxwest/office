package office

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// incipitCorpus is a psalm, a canticle and a non-psalter text, each with an
// incipit recorded — including one against a text that must never show it.
func incipitCorpus() *texts.TextCorpus {
	corpus := texts.NewTestCorpus(map[string]string{
		"psalms/067":              "Psalm 67\n\nGOD be merciful unto us * and bless us.",
		"psalms/100":              "Psalm 100\n\nO BE joyful in the Lord * all ye lands.",
		"canticles/magnificat":    "!Luke 1:46-55\n\nMy soul doth magnify the Lord * and my spirit.",
		"ordinary/lauds/versicle": "V. O God, make speed to save me.",
	})
	corpus.SetIncipit("psalms/067", "Deus misereatur nostri")
	corpus.SetIncipit("canticles/magnificat", "Magnificat anima mea Dominum")
	corpus.SetIncipit("ordinary/lauds/versicle", "Deus in adjutorium")
	return corpus
}

func TestResolveElementAttachesIncipit(t *testing.T) {
	tests := []struct {
		name        string
		elem        HourElement
		wantIncipit string
	}{
		{
			name:        "psalm carries its incipit",
			elem:        HourElement{Type: "psalm", Ref: "psalms/067"},
			wantIncipit: "Deus misereatur nostri",
		},
		{
			name:        "canticle carries its incipit",
			elem:        HourElement{Type: "canticle", Ref: "canticles/magnificat"},
			wantIncipit: "Magnificat anima mea Dominum",
		},
		{
			name:        "psalm without a recorded incipit gets none",
			elem:        HourElement{Type: "psalm", Ref: "psalms/100"},
			wantIncipit: "",
		},
		{
			// Only psalms and canticles take the diurnal's Latin subtitle;
			// a stray entry for anything else must not surface on the page.
			name:        "other element types never take an incipit",
			elem:        HourElement{Type: "versicle", Ref: "ordinary/lauds/versicle"},
			wantIncipit: "",
		},
	}
	corpus := incipitCorpus()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveElement(tt.elem, corpus)
			if got.Incipit != tt.wantIncipit {
				t.Errorf("Incipit = %q, want %q", got.Incipit, tt.wantIncipit)
			}
		})
	}
}

// The incipit is a subtitle beside the label, never a substitute for it, and
// it must not leak into the prayed text.
func TestResolveElementIncipitLeavesLabelAndTextAlone(t *testing.T) {
	got := resolveElement(HourElement{Type: "psalm", Ref: "psalms/067"}, incipitCorpus())
	if got.Type != models.Psalm {
		t.Errorf("Type = %q, want %q", got.Type, models.Psalm)
	}
	if got.Label != "Psalm 67" {
		t.Errorf("Label = %q, want %q", got.Label, "Psalm 67")
	}
	if want := "Psalm 67\n\nGOD be merciful unto us * and bless us."; got.Text != want {
		t.Errorf("Text = %q, want %q", got.Text, want)
	}
}
