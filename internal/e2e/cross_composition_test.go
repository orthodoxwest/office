package e2e

import (
	"slices"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Diurnal pp. 43,146-147: the Cross has distinct Lauds/Vespers antiphons
// and a shared invitation, collect, and Through-the-same conclusion.
// Eligibility corrections remain deferred pending the ordo conflict noted
// in composition-audit.md; these cases check the agreed text appointments.
func TestPaschalCrossCommemorationAppointments(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	for _, tc := range []struct {
		date, hour string
		want       bool
	}{
		{"2026-04-19", "lauds", false}, // Low Sunday
		{"2026-04-20", "lauds", false}, // St Alphege, Double
		{"2026-05-16", "lauds", true},  // Saturday BVM
		{"2026-05-16", "vespers", true},
		{"2026-05-17", "lauds", true},
		{"2026-05-21", "lauds", false}, // Ascension
	} {
		for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
			v, err := engine.ComposeHourWithOptions(tc.hour, dayFor(t, yd, tc.date), yd.moveable, office.ComposeOptions{Form: form})
			if err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, sec := range v.Sections {
				if sec.Label != "Commemoration of the Cross" {
					continue
				}
				found++
				wantAnt := "He that was crucified * is risen from the dead; and hath redeemed us, alleluia, alleluia."
				if tc.hour == "vespers" {
					wantAnt = "He the holy Cross endured, * Burst the gates of hell in twain: Begirt with might and majesty, On Easter morn he rose again, alleluia."
				}
				texts := map[string]string{
					"ordinary/shared/cross-antiphon-" + tc.hour:       wantAnt,
					"shared/leader/let-us-pray":                       "Let us pray.",
					"shared/formulas/collect-conclusion-through-same": "Through the same thy Son Jesus Christ our Lord, who with thee in the unity of\nthe Holy Spirit, liveth and reigneth God, world without end.\nR. Amen.",
					"ordinary/shared/cross-versicle":                  "V. Tell it out among the nations, alleluia.\nR. That the Lord reigneth from the Tree, alleluia.",
				}
				var refs []string
				for _, e := range sec.Elements {
					refs = append(refs, e.SourceRef)
					if want, ok := texts[e.SourceRef]; ok {
						if e.Text != want {
							t.Errorf("%s %s %s: %s=%q, want %q", tc.date, tc.hour, form, e.SourceRef, e.Text, want)
						}
						delete(texts, e.SourceRef)
					}
				}
				if len(texts) != 0 {
					t.Errorf("%s %s %s: missing Cross texts %v", tc.date, tc.hour, form, texts)
				}
				wantRefs := []string{"ordinary/shared/cross-antiphon-" + tc.hour, "ordinary/shared/cross-versicle", "shared/leader/let-us-pray", "ordinary/shared/cross-collect", "shared/formulas/collect-conclusion-through-same"}
				if !slices.Equal(refs, wantRefs) {
					t.Errorf("%s %s %s: Cross sequence %v, want %v", tc.date, tc.hour, form, refs, wantRefs)
				}

			}
			wantCount := 0
			if tc.want {
				wantCount = 1
			}
			if found != wantCount {
				t.Errorf("%s %s %s: %d Cross sections, want %d", tc.date, tc.hour, form, found, wantCount)
			}
		}
	}
}
