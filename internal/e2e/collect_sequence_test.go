package e2e

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Diurnal General Rubrics XII (p. xxxi), ordinary pp. 42–43,144–147,
// and Breviary XXXIII.3,5 (p. 50): every collect has its invitation, but
// only the first and last have conclusions. These calendar fixtures exercise
// sequencing; they do not adjudicate which commemorations ought to occur.
func TestMajorHourCollectSequence(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	for _, tc := range []struct {
		date, hour string
		comms      int
		finalRef   string
	}{
		{"2026-01-01", "lauds", 0, ""},
		{"2026-01-01", "vespers", 0, ""},
		{"2026-01-03", "vespers", 2, ""},
		{"2026-01-04", "lauds", 2, ""},
		{"2026-01-05", "lauds", 1, ""},
		{"2026-01-17", "vespers", 4, ""},
		{"2026-01-19", "lauds", 2, "ordinary/shared/suffrage-collect"},
		{"2026-02-03", "vespers", 2, "ordinary/shared/suffrage-collect"},
		{"2026-01-30", "vespers", 0, "ordinary/shared/suffrage-collect-bvm"},
		{"2026-01-31", "lauds", 0, "ordinary/shared/suffrage-collect-bvm"},
		{"2026-06-20", "lauds", 1, "ordinary/shared/suffrage-collect-bvm"},
		{"2026-04-21", "vespers", 1, "ordinary/shared/cross-collect"},
		{"2026-04-22", "lauds", 1, "ordinary/shared/cross-collect"},
	} {
		for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
			t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
				hour, err := engine.ComposeHourWithOptions(tc.hour, dayFor(t, yd, tc.date), yd.moveable, office.ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var elems []models.OfficeElement
				for _, s := range hour.Sections {
					elems = append(elems, s.Elements...)
				}
				start, end := -1, -1
				for i, e := range elems {
					if start < 0 && e.Type == models.Collect {
						start = i - 1
					}
					if start >= 0 && e.LeaderSlot == "greeting" {
						end = i
						break
					}
				}
				if start < 0 || end < 0 {
					t.Fatal("missing collect run or closing greeting")
				}
				run := elems[start:end]
				var positions []int
				invitations, comms := 0, 0
				for i, e := range run {
					if e.SourceRef == "shared/leader/let-us-pray" {
						invitations++
					}
					if e.Type != models.Collect {
						continue
					}
					positions = append(positions, i)
					if i == 0 || run[i-1].Type != models.Prayer || run[i-1].SourceRef != "shared/leader/let-us-pray" || run[i-1].Text != "Let us pray." {
						t.Errorf("collect %s lacks its invitation", e.SourceRef)
					}
					if e.IsCommemoration {
						comms++
						if i < 3 || run[i-2].Type != models.Versicle || run[i-3].Type != models.Antiphon {
							t.Errorf("commemoration %s must have antiphon, versicle, invitation, collect", e.CommemorationOwnerID)
						}
						if i > 0 && (!run[i-1].IsCommemoration || run[i-1].CommemorationOwnerID != e.CommemorationOwnerID) {
							t.Errorf("invitation lost commemoration owner %s", e.CommemorationOwnerID)
						}
					}
				}
				wantCollects := 1 + tc.comms
				if tc.finalRef != "" {
					wantCollects++
				}
				if len(positions) != wantCollects || comms != tc.comms {
					t.Fatalf("fixture has %d collects/%d commemorations, want %d/%d", len(positions), comms, wantCollects, tc.comms)
				}
				if invitations != len(positions) {
					t.Errorf("%d invitations for %d collects", invitations, len(positions))
				}
				if tc.finalRef != "" && run[positions[len(positions)-1]].SourceRef != tc.finalRef {
					t.Errorf("wrong final collect, want %s", tc.finalRef)
				}
				for j, i := range positions {
					stop := len(run)
					if j+1 < len(positions) {
						stop = positions[j+1]
					}
					var text strings.Builder
					for _, e := range run[i:stop] {
						text.WriteString(e.Text)
						text.WriteByte(' ')
					}
					normalized := strings.Join(strings.Fields(text.String()), " ")
					want := 0
					if j == 0 || j == len(positions)-1 {
						want = 1
					}
					if got := strings.Count(normalized, "world without end."); got != want {
						t.Errorf("collect %d of %d has %d conclusions, want %d", j+1, len(positions), got, want)
					}
				}
			})
		}
	}
}
