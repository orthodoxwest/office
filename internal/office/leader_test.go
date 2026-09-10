package office_test

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/output"
)

func TestPrayerFormsAcrossHoursAndExceptionalOffices(t *testing.T) {
	engine, err := office.NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	moveable := calendar.ComputeMoveableDates(2026)
	for _, date := range []string{"2026-03-11", "2026-04-09", "2026-11-01", "2026-11-02"} {
		d, _ := time.Parse(time.DateOnly, date)
		day := &days[d.YearDay()-1]
		for _, name := range []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"} {
			t.Run(name+"/"+date, func(t *testing.T) {
				var texts []string
				for _, form := range models.PrayerForms {
					hour, err := engine.ComposeHourWithOptions(name, day, moveable, office.ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					if hour.Form != form {
						t.Fatalf("form = %s", hour.Form)
					}
					greetings, confessions := 0, 0
					for _, section := range hour.Sections {
						for _, elem := range section.Elements {
							if strings.Contains(elem.Text, "The Lord be with you") && elem.LeaderSlot != "greeting" {
								t.Errorf("unmarked greeting in %s", elem.SourceRef)
							}
							if elem.LeaderSlot == "greeting" {
								greetings++
								expected := "shared/leader/greeting-clergy"
								if form == models.PrayerPrivate {
									expected = "shared/leader/greeting-lay"
								}
								if elem.SourceRef != expected || len(elem.SourceRefs) != 1 || elem.SourceRefs[0] != expected {
									t.Errorf("wrong greeting provenance: %+v", elem)
								}
							}
							if elem.SourceRef == "shared/leader/confiteor-officiant" {
								confessions++
								if !strings.Contains(elem.Text, "by my own fault") {
									t.Error("Confiteor lost repeated phrase")
								}
								if strings.Contains(elem.Text, "thee, father") && len(elem.SourceRefs) != 2 {
									t.Error("choir adaptation lost rubric provenance")
								}
							}
						}
					}
					if date == "2026-04-09" && (name == "prime" || name == "terce" || name == "sext" || name == "none") && greetings != 0 {
						t.Error("greeting introduced into Triduum")
					}
					if date == "2026-03-11" && greetings == 0 {
						t.Error("ordinary hour has no greeting")
					}
					if (name == "prime" || name == "compline") && date == "2026-03-11" && form != models.PrayerPrivate && confessions != 2 {
						t.Errorf("choir confessions=%d", confessions)
					}
					texts = append(texts, output.FormatOfficeHour(hour))
				}
				if texts[1] != texts[2] {
					t.Error("deacon and priest diverged without an appointed distinction")
				}
				again, err := engine.ComposeHour(name, day, moveable)
				if err != nil {
					t.Fatal(err)
				}
				if output.FormatOfficeHour(again) != texts[0] {
					t.Error("prayer context leaked into shared engine")
				}
			})
		}
	}
	if _, err := engine.ComposeHourWithOptions("lauds", &days[0], moveable, office.ComposeOptions{Form: "invalid"}); err == nil {
		t.Error("invalid form accepted")
	}
}
