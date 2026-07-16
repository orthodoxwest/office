package calendar

import (
	"strconv"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

func buildCalendar2026(t *testing.T) []models.CalendarDay {
	t.Helper()
	dataDir := findDataDir(t)
	days, err := BuildCalendar(2026, dataDir)
	if err != nil {
		t.Fatalf("BuildCalendar error: %v", err)
	}
	return days
}

func TestBuildCalendar365Days(t *testing.T) {
	days := buildCalendar2026(t)
	if len(days) != 365 {
		t.Fatalf("expected 365 days, got %d", len(days))
	}
}

func TestBuildCalendarEasterDate(t *testing.T) {
	days := buildCalendar2026(t)
	// Easter 2026 = April 12
	easterDay := findDay(days, 2026, 4, 12)
	if easterDay == nil {
		t.Fatal("April 12 not found")
	}
	if easterDay.Celebration == nil || easterDay.Celebration.ID != "easter-sunday" {
		t.Error("expected easter-sunday on April 12")
	}
}

func TestBuildCalendarChristmas(t *testing.T) {
	days := buildCalendar2026(t)
	xmas := findDay(days, 2026, 12, 25)
	if xmas == nil {
		t.Fatal("Dec 25 not found")
	}
	if xmas.Celebration == nil || xmas.Celebration.ID != "christmas" {
		t.Error("expected christmas on Dec 25")
	}
	if xmas.Season != models.Christmas {
		t.Errorf("Dec 25 season = %v, want christmas", xmas.Season)
	}
}

func TestBuildCalendarEpiphanySundays(t *testing.T) {
	days := buildCalendar2026(t)
	// 2026: Epiphany Jan 6 (Tue), Septuagesima Feb 8 (Sun)
	// Sundays after Epiphany: Jan 11, 18, 25, Feb 1 (4 Sundays)
	jan11 := findDay(days, 2026, 1, 11)
	if jan11 == nil || jan11.Celebration == nil {
		t.Fatal("Jan 11 missing or no celebration")
	}
	if jan11.Celebration.ID != "epiphany-sunday-1" {
		t.Errorf("Jan 11 = %v, want epiphany-sunday-1", jan11.Celebration.ID)
	}
	if jan11.Celebration.ProperID != "epiphany-sunday-within-octave" {
		t.Errorf("Jan 11 ProperID = %q, want Sunday-within-octave proper", jan11.Celebration.ProperID)
	}

	feb1 := findDay(days, 2026, 2, 1)
	if feb1 == nil || feb1.Celebration == nil {
		t.Fatal("Feb 1 missing or no celebration")
	}
	if feb1.Celebration.ID != "epiphany-sunday-4" {
		t.Errorf("Feb 1 = %v, want epiphany-sunday-4", feb1.Celebration.ID)
	}
}

func TestBuildCalendarAdditionalSundaysAfterEpiphanyUsePentecostPropers(t *testing.T) {
	tests := []struct {
		year              int
		month             int
		day               int
		wantCelebrationID string
		wantProperID      string
		commMonth         int
		commDay           int
		commID            string
		commProperID      string
	}{
		{
			year:              2021,
			month:             2,
			day:               21,
			wantCelebrationID: "epiphany-sunday-7",
			wantProperID:      "pentecost-sunday-23",
		},
		{
			year:              2024,
			month:             2,
			day:               18,
			wantCelebrationID: "epiphany-sunday-7",
			wantProperID:      "pentecost-sunday-22",
			commMonth:         2,
			commDay:           25,
			commID:            "epiphany-sunday-8",
			commProperID:      "pentecost-sunday-23",
		},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.year), func(t *testing.T) {
			days, err := BuildCalendar(tt.year, findDataDir(t))
			if err != nil {
				t.Fatalf("BuildCalendar(%d) error: %v", tt.year, err)
			}

			day := findDay(days, tt.year, tt.month, tt.day)
			if day == nil || day.Celebration == nil {
				t.Fatalf("%04d-%02d-%02d missing celebration", tt.year, tt.month, tt.day)
			}
			if day.Celebration.ID != tt.wantCelebrationID {
				t.Fatalf("%04d-%02d-%02d celebration = %v, want %v", tt.year, tt.month, tt.day, day.Celebration.ID, tt.wantCelebrationID)
			}
			if day.Celebration.ProperID != tt.wantProperID {
				t.Fatalf("%04d-%02d-%02d ProperID = %q, want %q", tt.year, tt.month, tt.day, day.Celebration.ProperID, tt.wantProperID)
			}

			if tt.commID == "" {
				return
			}

			commDay := findDay(days, tt.year, tt.commMonth, tt.commDay)
			if commDay == nil {
				t.Fatalf("%04d-%02d-%02d missing day", tt.year, tt.commMonth, tt.commDay)
			}

			found := false
			for _, comm := range commDay.Commemorations {
				if comm.ID == tt.commID {
					found = true
					if comm.ProperID != tt.commProperID {
						t.Fatalf("%04d-%02d-%02d comm ProperID = %q, want %q", tt.year, tt.commMonth, tt.commDay, comm.ProperID, tt.commProperID)
					}
					break
				}
			}
			if !found {
				t.Fatalf("expected %s commemoration on %04d-%02d-%02d", tt.commID, tt.year, tt.commMonth, tt.commDay)
			}
		})
	}
}

func TestBuildCalendarAdventSundays(t *testing.T) {
	days := buildCalendar2026(t)
	nov29 := findDay(days, 2026, 11, 29)
	if nov29 == nil || nov29.Celebration == nil {
		t.Fatal("Nov 29 missing celebration")
	}
	if nov29.Celebration.ID != "advent-sunday-1" {
		t.Errorf("Nov 29 = %v, want advent-sunday-1", nov29.Celebration.ID)
	}

	dec13 := findDay(days, 2026, 12, 13)
	if dec13 == nil || dec13.Celebration == nil {
		t.Fatal("Dec 13 missing celebration")
	}
	if dec13.Celebration.ID != "advent-sunday-3" {
		t.Errorf("Dec 13 = %v, want advent-sunday-3", dec13.Celebration.ID)
	}
}

func TestBuildCalendarPentecostSundays(t *testing.T) {
	days := buildCalendar2026(t)
	// Pentecost = May 31, Trinity = June 7
	// 2nd Sunday after Pentecost = June 14
	jun14 := findDay(days, 2026, 6, 14)
	if jun14 == nil || jun14.Celebration == nil {
		t.Fatal("Jun 14 missing celebration")
	}
	if jun14.Celebration.ID != "pentecost-sunday-2" {
		t.Errorf("Jun 14 = %v, want pentecost-sunday-2", jun14.Celebration.ID)
	}
	if got := jun14.Celebration.Name; got != "Sunday within the Octave of Corpus Christi" {
		t.Errorf("Jun 14 name = %q, want Corpus Christi octave title first", got)
	}
}

func TestBuildCalendarLastSundayAfterPentecostUses24thPropers(t *testing.T) {
	days, err := BuildCalendar(2024, findDataDir(t))
	if err != nil {
		t.Fatalf("BuildCalendar(2024) error: %v", err)
	}

	nov24 := findDay(days, 2024, 11, 24)
	if nov24 == nil || nov24.Celebration == nil {
		t.Fatal("2024-11-24 missing celebration")
	}
	if nov24.Celebration.Name != "XXIV & Last Sunday after Pentecost" {
		t.Fatalf("2024-11-24 celebration = %q, want XXIV & Last Sunday after Pentecost", nov24.Celebration.Name)
	}
	if nov24.Celebration.ProperID != "pentecost-sunday-24" {
		t.Fatalf("2024-11-24 ProperID = %q, want pentecost-sunday-24", nov24.Celebration.ProperID)
	}
}

func TestNativityOctaveSundayFeastObservedDate(t *testing.T) {
	tests := []struct {
		year  int
		month int
		day   int
	}{
		{2017, 12, 31},
		{2018, 12, 30},
		{2019, 12, 29},
		{2021, 12, 29},
		{2022, 12, 29},
		{2024, 12, 29},
		{2025, 12, 29},
		{2026, 12, 29},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.year), func(t *testing.T) {
			feast := nativityOctaveSundayFeast(tt.year)
			if feast.Month != tt.month || feast.Day != tt.day {
				t.Fatalf("%d observed on %02d-%02d, want %02d-%02d", tt.year, feast.Month, feast.Day, tt.month, tt.day)
			}
		})
	}
}

func TestBuildCalendarNativityOctaveSunday(t *testing.T) {
	tests := []struct {
		year  int
		day   int
		comms []string
	}{
		{2017, 31, []string{"st-sylvester", "christmas-octave-day-7"}},
		{2018, 30, []string{"christmas-octave-day-6"}},
		{2019, 29, []string{"christmas-octave-day-5"}},
		{2021, 29, []string{"christmas-octave-day-5"}},
		{2023, 31, []string{"st-sylvester", "christmas-octave-day-7"}},
		{2026, 29, []string{"christmas-octave-day-5"}},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.year), func(t *testing.T) {
			days, err := BuildCalendar(tt.year, findDataDir(t))
			if err != nil {
				t.Fatalf("BuildCalendar(%d) error: %v", tt.year, err)
			}

			day := findDay(days, tt.year, 12, tt.day)
			if day == nil || day.Celebration == nil {
				t.Fatalf("%04d-12-%02d missing celebration", tt.year, tt.day)
			}
			if day.Celebration.ID != "nativity-sunday-within-octave" {
				t.Fatalf("%04d-12-%02d celebration = %v, want nativity-sunday-within-octave", tt.year, tt.day, day.Celebration.ID)
			}

			for _, want := range tt.comms {
				found := false
				for _, comm := range day.Commemorations {
					if comm.ID == want {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected %s commemoration on %04d-12-%02d", want, tt.year, tt.day)
				}
			}
		})
	}
}

func TestBuildCalendarAllSaintsOfAntiochOnSecondSundayAfterPentecost(t *testing.T) {
	tests := []struct {
		year  int
		month int
		day   int
	}{
		{2024, 7, 7},
		{2025, 6, 22},
		{2026, 6, 14},
	}

	for _, tt := range tests {
		t.Run(time.Date(tt.year, time.Month(tt.month), tt.day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), func(t *testing.T) {
			dataDir := findDataDir(t)
			days, err := BuildCalendar(tt.year, dataDir)
			if err != nil {
				t.Fatalf("BuildCalendar(%d) error: %v", tt.year, err)
			}
			day := findDay(days, tt.year, tt.month, tt.day)
			if day == nil {
				t.Fatalf("%04d-%02d-%02d not found", tt.year, tt.month, tt.day)
			}
			found := false
			for _, comm := range day.Commemorations {
				if comm.ID == "comm-extra-06-14-all-saints-of-antioch" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected All Saints of Antioch commemoration on %04d-%02d-%02d", tt.year, tt.month, tt.day)
			}
		})
	}
}

func TestBuildCalendarAWRVLateFebruaryFeastsStayFixedInLeapYear(t *testing.T) {
	days, err := BuildCalendar(2024, findDataDir(t))
	if err != nil {
		t.Fatalf("BuildCalendar(2024) error: %v", err)
	}

	feb26 := findDay(days, 2024, 2, 26)
	if feb26 == nil {
		t.Fatal("2024-02-26 not found")
	}
	foundAlexander := false
	for _, comm := range feb26.Commemorations {
		if comm.ID == "comm-extra-02-26-st-alexander-of-alexandria-bishop-and-confessor" {
			foundAlexander = true
			break
		}
	}
	if !foundAlexander {
		t.Fatal("expected St Alexander of Alexandria commemoration on 2024-02-26")
	}

	feb27 := findDay(days, 2024, 2, 27)
	if feb27 == nil || feb27.Celebration == nil {
		t.Fatal("2024-02-27 missing celebration")
	}
	if feb27.Celebration.ID != "st-raphael-of-brooklyn" {
		t.Fatalf("2024-02-27 celebration = %v, want st-raphael-of-brooklyn", feb27.Celebration.ID)
	}
}

func TestBuildCalendarEastertideSundays(t *testing.T) {
	days := buildCalendar2026(t)

	may10 := findDay(days, 2026, 5, 10)
	if may10 == nil || may10.Celebration == nil {
		t.Fatal("May 10 missing celebration")
	}
	if may10.Celebration.ID != "easter-sunday-4" {
		t.Errorf("May 10 = %v, want easter-sunday-4", may10.Celebration.ID)
	}

	may24 := findDay(days, 2026, 5, 24)
	if may24 == nil || may24.Celebration == nil {
		t.Fatal("May 24 missing celebration")
	}
	if may24.Celebration.ID != "ascension-sunday-within-octave" {
		t.Errorf("May 24 = %v, want ascension-sunday-within-octave", may24.Celebration.ID)
	}
}

func TestBuildCalendarVigils(t *testing.T) {
	days := buildCalendar2026(t)
	// Vigil of All Saints = Oct 31
	oct31 := findDay(days, 2026, 10, 31)
	if oct31 == nil || oct31.Celebration == nil {
		t.Fatal("Oct 31 missing celebration")
	}
	if oct31.Celebration.ID != "vigil-of-all-saints" {
		t.Errorf("Oct 31 = %v, want vigil-of-all-saints", oct31.Celebration.ID)
	}
}

func TestBuildCalendarOctaves(t *testing.T) {
	days := buildCalendar2026(t)

	// Octave Day of the Epiphany = Jan 13
	jan13 := findDay(days, 2026, 1, 13)
	if jan13 == nil || jan13.Celebration == nil {
		t.Fatal("Jan 13 missing celebration")
	}
	if jan13.Celebration.ID != "epiphany-octave-day" {
		t.Errorf("Jan 13 = %v, want epiphany-octave-day", jan13.Celebration.ID)
	}
	if jan13.Celebration.Rank != models.GreaterDouble {
		t.Errorf("octave day rank = %v, want greater-double", jan13.Celebration.Rank)
	}

	// Easter octave day 4 = April 15 (privileged, double-1st-class)
	apr15 := findDay(days, 2026, 4, 15)
	if apr15 == nil || apr15.Celebration == nil {
		t.Fatal("Apr 15 missing celebration")
	}
	if apr15.Celebration.Rank != models.Double1stClass {
		t.Errorf("Easter octave day rank = %v, want double-1st-class", apr15.Celebration.Rank)
	}
}

func TestBuildCalendarEmberDays(t *testing.T) {
	days := buildCalendar2026(t)

	// Lent Ember Wednesday = easter-39 = March 4
	mar4 := findDay(days, 2026, 3, 4)
	if mar4 == nil || mar4.Celebration == nil {
		t.Fatal("Mar 4 missing celebration")
	}
	if mar4.Celebration.ID != "lent-ember-wednesday" {
		t.Errorf("Mar 4 = %v, want lent-ember-wednesday", mar4.Celebration.ID)
	}

	// Sep 16 is a privileged Ember feria, with Ss. Cornelius & Cyprian commemorated.
	sep16 := findDay(days, 2026, 9, 16)
	if sep16 == nil || sep16.Celebration == nil {
		t.Fatal("Sep 16 missing celebration")
	}
	if sep16.Celebration.ID != "september-ember-wednesday" {
		t.Errorf("Sep 16 = %v, want september-ember-wednesday", sep16.Celebration.ID)
	}
	foundSaints := false
	for _, comm := range sep16.Commemorations {
		if comm.ID == "ss-cornelius-cyprian" {
			foundSaints = true
			break
		}
	}
	if !foundSaints {
		t.Error("expected ss-cornelius-cyprian as commemoration on Sep 16")
	}
}

func TestBuildCalendarPrivilegedRogationAndEmberFerias(t *testing.T) {
	days := buildCalendar2026(t)

	for _, tt := range []struct {
		month      int
		day        int
		wantID     string
		wantCommID string
	}{
		{5, 18, "rogation-monday", "st-venantius"},
		{9, 16, "september-ember-wednesday", "ss-cornelius-cyprian"},
		{9, 19, "september-ember-saturday", "st-januarius"},
		{12, 18, "advent-ember-friday", "expectation-bvm"},
	} {
		day := findDay(days, 2026, tt.month, tt.day)
		if day == nil || day.Celebration == nil {
			t.Fatalf("2026-%02d-%02d missing celebration", tt.month, tt.day)
		}
		if got := day.Celebration.ID; got != tt.wantID {
			t.Errorf("2026-%02d-%02d celebration = %q, want %q", tt.month, tt.day, got, tt.wantID)
		}
		found := false
		for _, comm := range day.Commemorations {
			if comm.ID == tt.wantCommID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("2026-%02d-%02d missing commemoration %q", tt.month, tt.day, tt.wantCommID)
		}
	}
}

func TestBuildCalendarPenitentialObservance2026(t *testing.T) {
	days := buildCalendar2026(t)

	tests := []struct {
		month      int
		day        int
		fast       bool
		abstinence bool
	}{
		{2, 25, true, true},    // Ash Wednesday
		{3, 13, true, true},    // Friday after Lent II
		{3, 15, false, true},   // III Sunday in Lent
		{3, 22, false, true},   // Laetare Sunday
		{4, 17, false, false},  // Friday in the Octave of Easter
		{6, 5, true, true},     // Whit Ember Friday
		{9, 16, true, true},    // September Ember Wednesday
		{11, 29, false, true},  // I Sunday of Advent
		{12, 25, false, false}, // Christmas on Friday
		{12, 24, true, true},   // Christmas Eve
	}

	for _, tt := range tests {
		day := findDay(days, 2026, tt.month, tt.day)
		if day == nil {
			t.Fatalf("%04d-%02d-%02d not found", 2026, tt.month, tt.day)
		}
		if day.Penitential.Fast != tt.fast || day.Penitential.Abstinence != tt.abstinence {
			t.Fatalf(
				"%04d-%02d-%02d penitential = fast:%t abstinence:%t, want fast:%t abstinence:%t",
				2026, tt.month, tt.day,
				day.Penitential.Fast, day.Penitential.Abstinence,
				tt.fast, tt.abstinence,
			)
		}
	}
}

func TestBuildCalendarPenitentialObservanceFutureYear(t *testing.T) {
	year := 2027
	days, err := BuildCalendar(year, findDataDir(t))
	if err != nil {
		t.Fatalf("BuildCalendar(%d) error: %v", year, err)
	}

	moveable := ComputeMoveableDates(year)
	advent1 := findDay(days, year, int(moveable.Advent1.Month()), moveable.Advent1.Day())
	if advent1 == nil {
		t.Fatalf("%s not found", moveable.Advent1.Format("2006-01-02"))
	}
	if advent1.Penitential.Fast || !advent1.Penitential.Abstinence {
		t.Fatalf("Advent I penitential = %#v, want abstinence only", advent1.Penitential)
	}

	nextMonday := moveable.Advent1.AddDate(0, 0, 1)
	adventMonday := findDay(days, year, int(nextMonday.Month()), nextMonday.Day())
	if adventMonday == nil {
		t.Fatalf("%s not found", nextMonday.Format("2006-01-02"))
	}
	if !adventMonday.Penitential.Fast || !adventMonday.Penitential.Abstinence {
		t.Fatalf("Advent Monday penitential = %#v, want fasting and abstinence", adventMonday.Penitential)
	}
}

func TestBuildCalendarNeverFastsOnSunday(t *testing.T) {
	year := 2023 // Dec 24 was a Sunday
	days, err := BuildCalendar(year, findDataDir(t))
	if err != nil {
		t.Fatalf("BuildCalendar(%d) error: %v", year, err)
	}

	day := findDay(days, year, 12, 24)
	if day == nil {
		t.Fatal("2023-12-24 not found")
	}
	if day.Penitential.Fast {
		t.Fatalf("2023-12-24 should not be a fast day: %#v", day.Penitential)
	}
	if !day.Penitential.Abstinence {
		t.Fatalf("2023-12-24 should remain an abstinence day: %#v", day.Penitential)
	}
}

func TestBuildCalendarSundayPrecedence(t *testing.T) {
	days := buildCalendar2026(t)
	// Jan 25 = 3rd Sunday after Epiphany vs Conversion of St. Paul (greater-double)
	// Sunday should win, Paul commemorated
	jan25 := findDay(days, 2026, 1, 25)
	if jan25 == nil || jan25.Celebration == nil {
		t.Fatal("Jan 25 missing celebration")
	}
	if jan25.Celebration.ID != "epiphany-sunday-3" {
		t.Errorf("Jan 25 = %v, want epiphany-sunday-3", jan25.Celebration.ID)
	}
	if len(jan25.Commemorations) == 0 {
		t.Error("expected Conversion of St. Paul as commemoration")
	}
}

func TestBuildCalendarTransfers(t *testing.T) {
	days := buildCalendar2026(t)
	// Holy Name of Jesus (Jan 2, 2cl) should appear somewhere — it's on Jan 2
	// Circumcision (Jan 1, 1cl) has an octave — Jan 2 is Christmas Octave Day (1cl)
	// Holy Name should actually appear on Jan 3 since Jan 2 gets Christmas octave day
	jan2 := findDay(days, 2026, 1, 2)
	if jan2 == nil || jan2.Celebration == nil {
		t.Fatal("Jan 2 missing celebration")
	}
	// In the Python output, Jan 2 shows "Octave Day of Christmas" [1cl]
	// and Jan 3 shows "Holy Name of Jesus" [2cl]
	// Let's just verify the ordo output matches later
}

func TestBuildCalendarVespersConcurrence2026(t *testing.T) {
	days := buildCalendar2026(t)

	tests := []struct {
		month     int
		day       int
		wantOwner models.VespersOwner
		desc      string
	}{
		// Jan 1 (Circumcision D2): II prec.
		{1, 1, models.VespersIIOfPreceding, "Circumcision D2 should have II prec."},
		// Jan 2 (Oct St Stephen S, followed by Oct St John S): both Simple, no concurrence
		{1, 2, models.VespersNotApplicable, "Two adjacent Simples have no concurrence"},
		// Jan 5 (Vigil of Epiphany SD): I fol. (Epiphany D1)
		{1, 5, models.VespersIOfFollowing, "Vigil of Epiphany yields to Epiphany D1"},
		// Jan 6 (Epiphany D1): II prec.
		{1, 6, models.VespersIIOfPreceding, "Epiphany D1 should have II prec."},
		// Jan 17 (St Anthony D): I fol. (II Sunday after Epiphany)
		{1, 17, models.VespersIOfFollowing, "St Anthony D yields to Sunday"},
		// Mar 21 (St Benedict D2): II prec. against Laetare Sunday (XIII.6)
		{3, 21, models.VespersIIOfPreceding, "St Benedict D2 retains Vespers against Laetare"},
	}

	for _, tt := range tests {
		day := findDay(days, 2026, tt.month, tt.day)
		if day == nil {
			t.Fatalf("2026-%02d-%02d not found", tt.month, tt.day)
		}
		if day.Vespers.Owner != tt.wantOwner {
			t.Errorf("2026-%02d-%02d vespers: got %d, want %d (%s)",
				tt.month, tt.day, day.Vespers.Owner, tt.wantOwner, tt.desc)
		}
	}
}

func TestBuildCalendarDec31VespersUsesCircumcision(t *testing.T) {
	days := buildCalendar2026(t)
	dec31 := findDay(days, 2026, 12, 31)
	if dec31 == nil {
		t.Fatal("2026-12-31 not found")
	}
	if dec31.Vespers.Owner != models.VespersIOfFollowing {
		t.Fatalf("Dec 31 vespers owner = %v, want I of following", dec31.Vespers.Owner)
	}
	if dec31.Vespers.Feast == nil || dec31.Vespers.Feast.ID != "circumcision" {
		t.Fatalf("Dec 31 vespers feast = %#v, want circumcision", dec31.Vespers.Feast)
	}
	if dec31.Vespers.Color != models.White {
		t.Fatalf("Dec 31 vespers color = %s, want white", dec31.Vespers.Color)
	}
	if dec31.Vespers.Season != models.Christmas {
		t.Fatalf("Dec 31 vespers season = %s, want christmas", dec31.Vespers.Season)
	}
}

func TestBuildCalendarFeriaCommemoration(t *testing.T) {
	days := buildCalendar2026(t)

	// A Double of the second class displaces the privileged feria, which is
	// then commemorated at Lauds with the governing Sunday's proper ID.
	gregory := findDay(days, 2026, 3, 12) // St. Gregory, Thursday after Lent II
	var feria *models.Feast
	for _, comm := range gregory.Commemorations {
		if comm.Rank == models.PrivilegedFeria {
			feria = comm
			break
		}
	}
	if feria == nil {
		t.Fatalf("St. Gregory: expected a privileged-feria commemoration")
	}
	if got, want := feria.Name, "Thursday after Lent II"; got != want {
		t.Errorf("feria name = %q, want %q", got, want)
	}
	if got, want := feria.ProperID, "lent-sunday-2"; got != want {
		t.Errorf("feria ProperID = %q, want %q", got, want)
	}

	// A Septuagesima-season weekday feast is likewise commemorated, named from
	// the governing Sunday since lentenFeriaName does not cover pre-Lent.
	matthias := findDay(days, 2026, 2, 24) // St. Matthias, Tuesday after Quinquagesima
	if matthias.FeriaCommemoration == nil {
		t.Fatalf("St. Matthias: expected a feria commemoration")
	}
	if got, want := matthias.FeriaCommemoration.Name, "Tuesday after Quinquagesima"; got != want {
		t.Errorf("feria name = %q, want %q", got, want)
	}

	// The Sacred Triduum is the temporal office of the day, not a displaced
	// feria — no commemoration.
	goodFriday := findDay(days, 2026, 4, 10)
	if goodFriday.FeriaCommemoration != nil {
		t.Errorf("Good Friday: unexpected feria commemoration %q", goodFriday.FeriaCommemoration.Name)
	}

	// A plain Lenten weekday is itself the privileged feria, so it carries no
	// additional feria commemoration.
	plainFeria := findDay(days, 2026, 3, 5)
	if plainFeria.Celebration == nil || plainFeria.Celebration.Rank != models.PrivilegedFeria {
		t.Fatalf("2026-03-05: expected privileged feria, got %#v", plainFeria.Celebration)
	}
	if plainFeria.FeriaCommemoration != nil {
		t.Errorf("plain feria: unexpected commemoration %q", plainFeria.FeriaCommemoration.Name)
	}

	// A Lenten Ember day is likewise a privileged feria. It takes the office,
	// with the occurring saint commemorated, and needs no synthetic duplicate.
	emberFriday := findDay(days, 2026, 3, 6) // St. Perpetua & Felicitas on Lent Ember Friday
	if emberFriday.FeriaCommemoration != nil {
		t.Errorf("Ember Friday: unexpected extra feria commemoration %q", emberFriday.FeriaCommemoration.Name)
	}
	if emberFriday.Celebration == nil || emberFriday.Celebration.ID != "lent-ember-friday" {
		t.Fatalf("Ember Friday: expected Lent Ember Friday to take the office, got %#v", emberFriday.Celebration)
	}
	hasSaintComm := false
	for _, c := range emberFriday.Commemorations {
		if c.ID == "st-perpetua-felicitas" {
			hasSaintComm = true
		}
	}
	if !hasSaintComm {
		t.Errorf("Ember Friday: expected St Perpetua & Felicitas to be commemorated")
	}
}

func TestBuildCalendarPrivilegedLentenFerias(t *testing.T) {
	days := buildCalendar2026(t)

	for _, tt := range []struct {
		date       time.Time
		wantID     string
		wantName   string
		wantCommID string
	}{
		{
			date:       time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC),
			wantID:     "privileged-lenten-feria",
			wantName:   "Friday after Ash Wednesday",
			wantCommID: "st-raphael-of-brooklyn",
		},
		{
			date:       time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC),
			wantID:     "lent-ember-friday",
			wantName:   "Lent Ember Friday",
			wantCommID: "st-perpetua-felicitas",
		},
		{
			date:       time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
			wantID:     "privileged-lenten-feria",
			wantName:   "Friday after Passion Sunday",
			wantCommID: "our-lady-of-sorrows-passion",
		},
	} {
		day := findDay(days, tt.date.Year(), int(tt.date.Month()), tt.date.Day())
		if day == nil || day.Celebration == nil {
			t.Fatalf("%s missing celebration", tt.date.Format("2006-01-02"))
		}
		if got := day.Celebration.ID; got != tt.wantID {
			t.Errorf("%s celebration = %q, want %q", tt.date.Format("2006-01-02"), got, tt.wantID)
		}
		if got := day.Celebration.Name; got != tt.wantName {
			t.Errorf("%s celebration name = %q, want %q", tt.date.Format("2006-01-02"), got, tt.wantName)
		}
		found := false
		for _, comm := range day.Commemorations {
			if comm.ID == tt.wantCommID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s missing commemoration %q", tt.date.Format("2006-01-02"), tt.wantCommID)
		}
	}
}

func findDay(days []models.CalendarDay, year, month, day int) *models.CalendarDay {
	target := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	for i, d := range days {
		if d.Date.Equal(target) {
			return &days[i]
		}
	}
	return nil
}

func TestSaturdayOfficeBVMSeasonalProperID(t *testing.T) {
	days := buildCalendar2026(t)

	tests := []struct {
		month, day   int
		wantProperID string
		desc         string
	}{
		{1, 31, "saturday-office-bvm-christmastide", "before the Purification"},
		{5, 16, "", "Paschaltide (handled by the -paschal proper tier)"},
		{6, 20, "", "per annum"},
	}
	for _, tt := range tests {
		d := findDay(days, 2026, tt.month, tt.day)
		if d == nil {
			t.Fatalf("2026-%02d-%02d not found", tt.month, tt.day)
		}
		if d.Celebration == nil || d.Celebration.ID != "saturday-office-bvm" {
			t.Fatalf("2026-%02d-%02d: expected Saturday Office of the BVM, got %v",
				tt.month, tt.day, d.Celebration)
		}
		if got := d.Celebration.ProperID; got != tt.wantProperID {
			t.Errorf("2026-%02d-%02d ProperID = %q, want %q (%s)",
				tt.month, tt.day, got, tt.wantProperID, tt.desc)
		}
	}
}

// TestVespersBoundaryCommemorations2026 pins the boundary-commemoration rules
// against printed 2026 ordo evenings: the perpetual Peter/Paul companion at
// II Vespers, a displaced Greater Double (but not a plain Double) carried to
// I Vespers of the following, an incoming Double admitted at a II Class
// II Vespers, and the octave exclusions at II Class I Vespers.
func TestVespersBoundaryCommemorations2026(t *testing.T) {
	days := buildCalendar2026(t)

	has := func(month, day int, name string) bool {
		d := findDay(days, 2026, month, day)
		if d == nil {
			t.Fatalf("2026-%02d-%02d not found", month, day)
		}
		for _, c := range d.Vespers.Commemorations {
			if c.Name == name {
				return true
			}
		}
		return false
	}

	tests := []struct {
		month, day int
		name       string
		want       bool
		desc       string
	}{
		{1, 18, "Commemoration of St Paul", true, "companion of the Chair of St Peter at II Vespers"},
		{1, 25, "Commemoration of St. Peter", true, "companion of the Conversion of St Paul at II Vespers"},
		{3, 24, "St. Gabriel the Archangel", true, "displaced Greater Double at the Annunciation's I Vespers"},
		{3, 19, "St. Cuthbert, Bishop & Confessor", true, "incoming Double at St Joseph's II Vespers"},
		{3, 20, "St. Cuthbert, Bishop & Confessor", false, "displaced plain Double not carried to St Benedict's I Vespers"},
		{7, 1, "Day IV within the Octave of Ss Peter & Paul", false, "octave day excluded at the Visitation's I Vespers"},
		{12, 12, "Day VI within Conception Octave", true, "octave day admitted at Gaudete Sunday's I Vespers"},
	}
	for _, tt := range tests {
		if got := has(tt.month, tt.day, tt.name); got != tt.want {
			t.Errorf("2026-%02d-%02d %q present=%t, want %t (%s)",
				tt.month, tt.day, tt.name, got, tt.want, tt.desc)
		}
	}
}
