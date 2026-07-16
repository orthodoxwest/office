package calendar

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestConcurrenceWinnerDoubleVsLesserSunday(t *testing.T) {
	// XIII.3: Non-I/II-Class Double vs Lesser Sunday — Sunday wins
	double := &models.Feast{
		ID: "some-double", Rank: models.Double, Category: models.CategoryMartyr,
	}
	sunday := &models.Feast{
		ID: "epiphany-sunday-2", Rank: models.SemiDouble, Category: models.CategorySunday,
	}
	if got := concurrenceWinner(double, sunday); got != models.VespersIOfFollowing {
		t.Errorf("Double vs Lesser Sunday: got %d, want VespersIOfFollowing", got)
	}
}

func TestConcurrenceWinnerDoubleVsGreaterSunday(t *testing.T) {
	// XIII.4: Non-I/II-Class Double vs Greater Sunday — Sunday wins
	double := &models.Feast{
		ID: "some-double", Rank: models.Double, Category: models.CategoryMartyr,
	}
	sunday := &models.Feast{
		ID: "advent-sunday-1", Rank: models.Double2ndClass, Category: models.CategorySunday,
	}
	if got := concurrenceWinner(double, sunday); got != models.VespersIOfFollowing {
		t.Errorf("Double vs Greater Sunday: got %d, want VespersIOfFollowing", got)
	}
}

func TestConcurrenceWinnerFirstClassDoubleVsGreaterSunday(t *testing.T) {
	// XIII.6: I Class Double vs Greater Sunday — feast wins
	feast := &models.Feast{
		ID: "christmas", Rank: models.Double1stClass, Category: models.CategoryLord,
	}
	sunday := &models.Feast{
		ID: "advent-sunday-4", Rank: models.Double2ndClass, Category: models.CategorySunday,
	}
	if got := concurrenceWinner(feast, sunday); got != models.VespersIIOfPreceding {
		t.Errorf("I Class Double vs Greater Sunday: got %d, want VespersIIOfPreceding", got)
	}
}

func TestConcurrenceWinnerSecondClassDoubleVsGreaterSunday(t *testing.T) {
	// XIII.6: a sanctoral II Class Double vs Greater Sunday — feast wins.
	feast := &models.Feast{
		ID: "st-benedict", Rank: models.Double2ndClass, Category: models.CategoryConfessor,
	}
	sunday := &models.Feast{
		ID: "advent-sunday-1", Rank: models.Double2ndClass, Category: models.CategorySunday,
	}
	if got := concurrenceWinner(feast, sunday); got != models.VespersIIOfPreceding {
		t.Errorf("II Class Double vs Greater Sunday: got %d, want VespersIIOfPreceding", got)
	}
	if got := concurrenceWinner(sunday, feast); got != models.VespersIOfFollowing {
		t.Errorf("Greater Sunday vs II Class Double: got %d, want VespersIOfFollowing", got)
	}
}

func TestConcurrenceWinnerLordDoubleVsLesserSunday(t *testing.T) {
	// XIII.2-5, as the 2026 ordo applies them: a feast below II Class —
	// even a Greater Double of the Lord — yields the concurrence to the
	// Sunday (the XV Sunday keeps its II Vespers against the Exaltation
	// of the Holy Cross, and its I Vespers likewise prevails).
	lord := &models.Feast{
		ID: "exaltation-holy-cross", Rank: models.GreaterDouble, Category: models.CategoryLord,
	}
	sunday := &models.Feast{
		ID: "pentecost-sunday-15", Rank: models.SemiDouble, Category: models.CategorySunday,
	}
	if got := concurrenceWinner(sunday, lord); got != models.VespersIIOfPreceding {
		t.Errorf("Lesser Sunday vs Lord Greater Double: got %d, want VespersIIOfPreceding", got)
	}
	if got := concurrenceWinner(lord, sunday); got != models.VespersIOfFollowing {
		t.Errorf("Lord Greater Double vs Lesser Sunday: got %d, want VespersIOfFollowing", got)
	}
}

func TestConcurrenceSimpleHasNoSecondVespers(t *testing.T) {
	// XIII.17: Simple has no II Vespers
	simple := &models.Feast{
		ID: "some-simple", Rank: models.Simple, Category: models.CategoryMartyr,
	}
	if hasSecondVespers(simple) {
		t.Error("Simple feast should have no II Vespers")
	}
}

func TestConcurrenceFeraCannotConcur(t *testing.T) {
	// XIII.18: Feria cannot concur
	feria := &models.Feast{
		ID: "some-feria", Rank: models.SemiDouble, Category: models.CategoryFeria,
	}
	if hasSecondVespers(feria) {
		t.Error("Feria should not have II Vespers")
	}
	if hasFirstVespers(feria) {
		// Ferias that are semi-double (e.g. ember days) do have I vespers by rank,
		// but they aren't relevant for concurrence per XIII.18.
		// hasFirstVespers checks rank, not category — the concurrence resolver
		// handles ferias via hasSecondVespers on the preceding day.
	}
}

func TestConcurrenceWinnerDoubleVsDayWithinOctave(t *testing.T) {
	// XIII.12: Double vs day-within-Octave — Double wins
	double := &models.Feast{
		ID: "some-double", Rank: models.Double, Category: models.CategoryMartyr,
	}
	octDay := &models.Feast{
		ID: "epiphany-octave-day-3", Rank: models.SemiDouble, Category: models.CategoryLord,
	}
	if got := concurrenceWinner(double, octDay); got != models.VespersIIOfPreceding {
		t.Errorf("Double vs day-within-Octave: got %d, want VespersIIOfPreceding", got)
	}
}

func TestConcurrenceWinnerOctaveDayVsDouble(t *testing.T) {
	// XIII.10: an Octave Day concurring with a Double below II Class takes
	// the concurrence in both directions, regardless of relative rank
	// (issue #22; 2026 ordo: Octave Day of the Epiphany over St Hilary,
	// Octave of John Baptist over the Commemoration of St Paul).
	double := &models.Feast{
		ID: "some-double", Rank: models.Double, Category: models.CategoryMartyr,
	}
	greaterOctDay := &models.Feast{
		ID: "epiphany-octave-day", Rank: models.GreaterDouble, Category: models.CategoryLord,
	}
	if got := concurrenceWinner(double, greaterOctDay); got != models.VespersIOfFollowing {
		t.Errorf("Double vs Octave Day: got %d, want VespersIOfFollowing", got)
	}
	if got := concurrenceWinner(greaterOctDay, double); got != models.VespersIIOfPreceding {
		t.Errorf("Octave Day vs Double: got %d, want VespersIIOfPreceding", got)
	}
	// A Greater Double does not displace even a lesser-ranked Octave Day:
	// XIII.10 excludes only I and II Class Doubles.
	greaterDouble := &models.Feast{
		ID: "some-greater-double", Rank: models.GreaterDouble, Category: models.CategoryMartyr,
	}
	lesserOctDay := &models.Feast{
		ID: "some-octave-day", Rank: models.Double, Category: models.CategoryConfessor,
	}
	if got := concurrenceWinner(greaterDouble, lesserOctDay); got != models.VespersIOfFollowing {
		t.Errorf("Greater Double vs lesser Octave Day: got %d, want VespersIOfFollowing", got)
	}
	if got := concurrenceWinner(lesserOctDay, greaterDouble); got != models.VespersIIOfPreceding {
		t.Errorf("lesser Octave Day vs Greater Double: got %d, want VespersIIOfPreceding", got)
	}
	// But a II Class Double prevails over an Octave Day (XIII.10 excludes
	// I and II Class Doubles).
	secondClass := &models.Feast{
		ID: "visitation-bvm", Rank: models.Double2ndClass, Category: models.CategoryBlessedVirgin,
	}
	if got := concurrenceWinner(greaterOctDay, secondClass); got != models.VespersIOfFollowing {
		t.Errorf("Octave Day vs II Class Double: got %d, want VespersIOfFollowing", got)
	}
}

func TestConcurrenceWinnerOctaveDayVsOctaveDay(t *testing.T) {
	// XIII.11: Octave Day vs Octave Day — worthier wins
	higherOct := &models.Feast{
		ID: "epiphany-octave-day", Rank: models.GreaterDouble, Category: models.CategoryLord,
	}
	lowerOct := &models.Feast{
		ID: "st-stephen-octave-day", Rank: models.GreaterDouble, Category: models.CategoryMartyr,
	}
	// Lord feast should beat non-Lord by precedence
	if got := concurrenceWinner(higherOct, lowerOct); got != models.VespersIIOfPreceding {
		t.Errorf("Higher Octave Day vs Lower: got %d, want VespersIIOfPreceding", got)
	}
}

func TestConcurrenceWinnerSaturdayBVMVsSunday(t *testing.T) {
	bvm := &models.Feast{
		ID: "saturday-office-bvm", Rank: models.Simple, Category: models.CategoryBlessedVirgin,
		Color: models.White,
	}
	sunday := &models.Feast{
		ID: "pentecost-sunday-5", Rank: models.SemiDouble, Category: models.CategorySunday,
		Color: models.Green,
	}
	// BVM is Simple, so no II Vespers — Sunday gets I Vespers by default
	if hasSecondVespers(bvm) {
		t.Error("Saturday BVM (Simple) should have no II Vespers")
	}
	day1 := &models.CalendarDay{Celebration: bvm, Color: models.White}
	day2 := &models.CalendarDay{Celebration: sunday, Color: models.Green}
	result := resolveConcurrence(day1, day2)
	if result.Owner != models.VespersIOfFollowing {
		t.Errorf("Saturday BVM vs Sunday: got owner %d, want VespersIOfFollowing", result.Owner)
	}
	if result.Feast != sunday {
		t.Error("expected Sunday to own vespers when BVM has no II Vespers")
	}
}

func TestConcurrenceWinnerPrivilegedDayAlwaysWins(t *testing.T) {
	easter := &models.Feast{
		ID: "easter-sunday", Rank: models.Double1stClass, Category: models.CategoryLord,
	}
	double := &models.Feast{
		ID: "some-double-1st", Rank: models.Double1stClass, Category: models.CategoryApostle,
	}
	if got := concurrenceWinner(easter, double); got != models.VespersIIOfPreceding {
		t.Errorf("Easter (prec) vs Double: got %d, want VespersIIOfPreceding", got)
	}
	if got := concurrenceWinner(double, easter); got != models.VespersIOfFollowing {
		t.Errorf("Double vs Easter (fol): got %d, want VespersIOfFollowing", got)
	}
	// A sanctoral I Class Double does supersede the other I Class Sundays
	// (2026 ordo: St Tikhon's I Vespers over Low Sunday).
	low := &models.Feast{
		ID: "low-sunday", Rank: models.Double1stClass, Category: models.CategoryLord,
	}
	if got := concurrenceWinner(low, double); got != models.VespersIOfFollowing {
		t.Errorf("Low Sunday vs sanctoral I Class Double: got %d, want VespersIOfFollowing", got)
	}
}

func TestResolveConcurrenceTwoFerias(t *testing.T) {
	current := &models.Feast{ID: "current-memorial", Name: "Current Memorial", Rank: models.Commemoration}
	incoming := &models.Feast{ID: "incoming-memorial", Name: "Incoming Memorial", Rank: models.Commemoration}
	day1 := &models.CalendarDay{Celebration: nil, Commemorations: []*models.Feast{current}}
	day2 := &models.CalendarDay{Celebration: nil, Commemorations: []*models.Feast{incoming}}
	result := resolveConcurrence(day1, day2)
	if result.Owner != models.VespersNotApplicable {
		t.Errorf("Two ferias: got owner %d, want VespersNotApplicable", result.Owner)
	}
	if len(result.Commemorations) != 1 || result.Commemorations[0] != incoming {
		t.Fatalf("commemorations = %#v, want following day's memorial", result.Commemorations)
	}
	assertTraceRule(t, result.Decisions, "commemoration:incoming-at-unowned-vespers")
}

func TestResolveConcurrenceSimplePrecedingYieldsToFollowing(t *testing.T) {
	simple := &models.Feast{
		ID: "some-simple", Rank: models.Simple, Category: models.CategoryConfessor,
		Color: models.White,
	}
	double := &models.Feast{
		ID: "some-double", Rank: models.Double, Category: models.CategoryMartyr,
		Color: models.Red,
	}
	day1 := &models.CalendarDay{Celebration: simple, Color: models.White}
	day2 := &models.CalendarDay{Celebration: double, Color: models.Red}
	result := resolveConcurrence(day1, day2)
	if result.Owner != models.VespersIOfFollowing {
		t.Errorf("Simple prec vs Double fol: got owner %d, want VespersIOfFollowing", result.Owner)
	}
	if result.Feast != double {
		t.Error("expected following feast to own vespers")
	}
}

func TestResolveConcurrenceNilPrecedingYieldsToFollowingDouble(t *testing.T) {
	// A plain feria (no Celebration) has no vespers rights of its own
	// (XIII.18) and must not be mistaken for "no concurrence to resolve":
	// the following Double's I Vespers still takes the evening.
	double := &models.Feast{
		ID: "st-anthony-egypt", Rank: models.Double, Category: models.CategoryConfessor,
		Color: models.White,
	}
	day1 := &models.CalendarDay{Celebration: nil, Color: models.Green}
	day2 := &models.CalendarDay{Celebration: double, Color: models.White}
	result := resolveConcurrence(day1, day2)
	if result.Owner != models.VespersIOfFollowing {
		t.Fatalf("nil preceding vs Double: got owner %d, want VespersIOfFollowing", result.Owner)
	}
	if result.Feast != double {
		t.Error("expected following feast to own vespers")
	}
	if len(result.Commemorations) != 0 {
		t.Errorf("expected no boundary commemoration (preceding has no Celebration), got %v", result.Commemorations)
	}
}

func TestResolveConcurrenceNilFollowingYieldsToPrecedingDouble(t *testing.T) {
	double := &models.Feast{
		ID: "some-double", Rank: models.Double, Category: models.CategoryMartyr,
	}
	day1 := &models.CalendarDay{Celebration: double}
	day2 := &models.CalendarDay{Celebration: nil}
	result := resolveConcurrence(day1, day2)
	if result.Owner != models.VespersIIOfPreceding {
		t.Fatalf("Double vs nil following: got owner %d, want VespersIIOfPreceding", result.Owner)
	}
	if len(result.Commemorations) != 0 {
		t.Errorf("expected no boundary commemoration (following has no Celebration), got %v", result.Commemorations)
	}
}

func TestResolveConcurrenceBoundaryCommemoration(t *testing.T) {
	simple := &models.Feast{
		ID: "some-simple", Rank: models.Simple, Category: models.CategoryConfessor,
		Color: models.White,
	}
	double := &models.Feast{
		ID: "some-double", Rank: models.Double, Category: models.CategoryMartyr,
		Color: models.Red,
	}
	day1 := &models.CalendarDay{Celebration: simple, Color: models.White}
	day2 := &models.CalendarDay{Celebration: double, Color: models.Red}

	// A Simple office ends at None and is not commemorated when the following
	// Double begins I Vespers (XIII.17, XIV.9).
	result := resolveConcurrence(day1, day2)
	if result.Owner != models.VespersIOfFollowing {
		t.Fatalf("got owner %d, want VespersIOfFollowing", result.Owner)
	}
	if len(result.Commemorations) != 0 {
		t.Errorf("expected outgoing Simple to be suppressed, got %v", result.Commemorations)
	}

	// II Vespers of preceding: an incoming Simple is explicitly excluded by
	// XIV.7-8 even though it is the following day's celebration.
	result = resolveConcurrence(day2, day1)
	if result.Owner != models.VespersIIOfPreceding {
		t.Fatalf("got owner %d, want VespersIIOfPreceding", result.Owner)
	}
	if len(result.Commemorations) != 0 {
		t.Errorf("expected incoming Simple to be suppressed, got %v", result.Commemorations)
	}
}

func TestOccurrenceCommemoratedAtFirstVespers(t *testing.T) {
	tests := []struct {
		name string
		comm *models.Feast
		want bool
	}{
		{"Memorial begins at I Vespers", &models.Feast{ID: "memorial", Rank: models.Commemoration}, true},
		{"simplified Double begins at I Vespers", &models.Feast{ID: "double", Rank: models.Double}, true},
		{"Ember day is Lauds only", &models.Feast{ID: "september-ember-wednesday", Rank: models.PrivilegedFeria, Category: models.CategoryFeria}, false},
		{"Rogation is Lauds only", &models.Feast{ID: "rogation-monday", Rank: models.PrivilegedFeria, Category: models.CategoryFeria}, false},
		{"common vigil is Lauds only", &models.Feast{ID: "comm-extra-08-22-vigil-of-st-bartholomew", Name: "Vigil of St. Bartholomew", Rank: models.Commemoration, IsVigil: true}, false},
		{"vigil-looking name without trait follows rank", &models.Feast{ID: "vigil-looking-memorial", Name: "Vigil of an Example", Rank: models.Commemoration}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := occurrenceCommemoratedAtFirstVespers(tt.comm)
			if got != tt.want {
				t.Fatalf("got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestOccurrenceCommemoratedAtSecondVespers(t *testing.T) {
	firstClass := &models.Feast{ID: "first-class", Rank: models.Double1stClass, Category: models.CategoryLord}
	secondClass := &models.Feast{ID: "second-class", Rank: models.Double2ndClass, Category: models.CategoryApostle}
	greaterDouble := &models.Feast{ID: "greater-double", Rank: models.GreaterDouble, Category: models.CategoryConfessor}

	tests := []struct {
		name   string
		winner *models.Feast
		comm   *models.Feast
		want   bool
	}{
		{"memorial is I Vespers and Lauds only", greaterDouble, &models.Feast{ID: "memorial", Rank: models.Commemoration}, false},
		{"simple is not at II Vespers", greaterDouble, &models.Feast{ID: "simple", Rank: models.Simple}, false},
		{"Ember day is Lauds only", greaterDouble, &models.Feast{ID: "september-ember-wednesday", Rank: models.PrivilegedFeria, Category: models.CategoryFeria}, false},
		{"Rogation is Lauds only", greaterDouble, &models.Feast{ID: "rogation-monday", Rank: models.PrivilegedFeria, Category: models.CategoryFeria}, false},
		{"common vigil is Lauds only", greaterDouble, &models.Feast{ID: "vigil-st-lawrence", Rank: models.Simple, Category: models.CategoryFeria, IsVigil: true}, false},
		{"apostolic companion remains at II Vespers", greaterDouble, &models.Feast{ID: "companion", Rank: models.Commemoration, IsApostolicCompanion: true}, true},
		{"apostolic companion excluded by first class", firstClass, &models.Feast{ID: "companion", Rank: models.Commemoration, IsApostolicCompanion: true}, false},
		{"Peter name without trait is an ordinary memorial", greaterDouble, &models.Feast{ID: "named-companion", Name: "Commemoration of St Peter", Rank: models.Commemoration}, false},
		{"Sunday remains at II Vespers of first class", firstClass, &models.Feast{ID: "sunday", Rank: models.SemiDouble, Category: models.CategorySunday}, true},
		{"Double excluded by first class winner", firstClass, &models.Feast{ID: "double", Rank: models.Double, Category: models.CategoryMartyr}, false},
		{"day within octave excluded by second class", secondClass, &models.Feast{ID: "epiphany-octave-day-3", Rank: models.SemiDouble, Category: models.CategoryMartyr}, false},
		{"Double retained under greater double", greaterDouble, &models.Feast{ID: "double", Rank: models.Double, Category: models.CategoryMartyr}, true},
		{"seasonal privileged feria retained", greaterDouble, &models.Feast{ID: "privileged-lenten-feria", Rank: models.PrivilegedFeria, Category: models.CategoryFeria}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := occurrenceCommemoratedAtSecondVespers(tt.winner, tt.comm)
			if got != tt.want {
				t.Fatalf("got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestResolveConcurrenceFiltersOccurrenceCommemorationsAtSecondVespers(t *testing.T) {
	winner := &models.Feast{ID: "winner", Rank: models.GreaterDouble, Category: models.CategoryConfessor}
	memorial := &models.Feast{ID: "memorial", Rank: models.Commemoration, Category: models.CategoryMartyr}
	simplifiedDouble := &models.Feast{ID: "simplified-double", Rank: models.Double, Category: models.CategoryMartyr}
	seasonalFeria := &models.Feast{ID: "lenten-feria", Rank: models.PrivilegedFeria, Category: models.CategoryFeria}

	result := resolveConcurrence(
		&models.CalendarDay{Celebration: winner, Commemorations: []*models.Feast{memorial, simplifiedDouble, seasonalFeria}},
		&models.CalendarDay{},
	)

	if result.Owner != models.VespersIIOfPreceding {
		t.Fatalf("got owner %d, want VespersIIOfPreceding", result.Owner)
	}
	if len(result.Commemorations) != 2 || result.Commemorations[0] != simplifiedDouble || result.Commemorations[1] != seasonalFeria {
		t.Fatalf("got commemorations %v, want simplified Double and seasonal feria", result.Commemorations)
	}
	foundSuppression := false
	for _, decision := range result.Decisions {
		if decision.Rule == "commemoration:second-vespers-memorial-or-simple" && decision.Outcome == "suppressed" && decision.Detail == memorial.ID {
			foundSuppression = true
			break
		}
	}
	if !foundSuppression {
		t.Fatalf("missing traced memorial suppression in decisions: %v", result.Decisions)
	}
}

func TestResolveConcurrenceNoOwnerCombinesHourEligibleCommemorations(t *testing.T) {
	currentMemorial := &models.Feast{ID: "current-memorial", Rank: models.Commemoration, Category: models.CategoryMartyr}
	currentDouble := &models.Feast{ID: "current-double", Rank: models.Double, Category: models.CategoryMartyr}
	incomingMemorial := &models.Feast{ID: "incoming-memorial", Rank: models.Commemoration, Category: models.CategoryMartyr}
	incomingVigil := &models.Feast{ID: "comm-extra-vigil", Name: "Vigil of an Apostle", Rank: models.Commemoration, Category: models.CategoryFeria, IsVigil: true}

	result := resolveConcurrence(
		&models.CalendarDay{Commemorations: []*models.Feast{currentMemorial, currentDouble}},
		&models.CalendarDay{Commemorations: []*models.Feast{incomingMemorial, incomingVigil}},
	)

	if result.Owner != models.VespersNotApplicable {
		t.Fatalf("got owner %d, want VespersNotApplicable", result.Owner)
	}
	if len(result.Commemorations) != 2 || result.Commemorations[0] != currentDouble || result.Commemorations[1] != incomingMemorial {
		t.Fatalf("got commemorations %v, want current Double then incoming Memorial", result.Commemorations)
	}
	assertTraceRule(t, result.Decisions, "commemoration:second-vespers-included")
	assertTraceRule(t, result.Decisions, "commemoration:incoming-at-unowned-vespers")
	assertTraceRule(t, result.Decisions, "commemoration:first-vespers-feria-or-vigil-lauds-only")
}

func TestResolveConcurrenceSuppressesSameOctaveBoundary(t *testing.T) {
	parent := &models.Feast{ID: "octave-feast", Name: "Octave Feast", Rank: models.Double1stClass, Category: models.CategoryLord, HasOctave: true}
	nextDay := &models.Feast{ID: "octave-feast-octave-day-2", Name: "Day II within the Octave Feast", Rank: models.Double1stClass, Category: models.CategoryLord}
	preceding := &models.CalendarDay{Celebration: parent, WithinOctaveOf: ""}
	following := &models.CalendarDay{Celebration: nextDay, WithinOctaveOf: parent.ID}

	result := resolveConcurrence(preceding, following)
	if len(result.Commemorations) != 0 {
		t.Fatalf("same-octave boundary commemorations = %v, want none", result.Commemorations)
	}
	assertTraceRule(t, result.Decisions, "commemoration:same-octave-boundary")
}

func TestResolveConcurrenceDoesNotConflateOccurrenceWithinOctaveWithOctaveOffice(t *testing.T) {
	parent := &models.Feast{ID: "octave-feast", Name: "Octave Feast", Rank: models.Double1stClass, Category: models.CategoryLord, HasOctave: true}
	saint := &models.Feast{ID: "saint", Name: "Saint", Rank: models.Double, Category: models.CategoryMartyr}
	sunday := &models.Feast{ID: "sunday", Name: "Sunday within the Octave", Rank: models.SemiDouble, Category: models.CategorySunday}
	preceding := &models.CalendarDay{Celebration: saint, WithinOctaveOf: parent.ID}
	following := &models.CalendarDay{Celebration: sunday, WithinOctaveOf: parent.ID}

	result := resolveConcurrence(preceding, following)
	if len(result.Commemorations) != 1 || result.Commemorations[0] != saint {
		t.Fatalf("commemorations = %v, want distinct occurring saint", result.Commemorations)
	}
	for _, decision := range result.Decisions {
		if decision.Rule == "commemoration:same-octave-boundary" {
			t.Fatalf("distinct occurring offices incorrectly treated as the same octave office: %v", result.Decisions)
		}
	}
}

func TestOutgoingCommemoratedAtFirstVespers(t *testing.T) {
	firstClass := &models.Feast{ID: "first-class", Rank: models.Double1stClass, Category: models.CategoryLord}
	secondClass := &models.Feast{ID: "second-class", Rank: models.Double2ndClass, Category: models.CategoryApostle}
	double := &models.Feast{ID: "double", Rank: models.Double, Category: models.CategoryMartyr}

	tests := []struct {
		name   string
		winner *models.Feast
		loser  *models.Feast
		want   bool
	}{
		{"Simple ends at None", double, &models.Feast{ID: "simple", Rank: models.Simple, Category: models.CategoryConfessor}, false},
		{"ordinary feria does not concur", double, &models.Feast{ID: "feria", Rank: models.SemiDouble, Category: models.CategoryFeria}, false},
		{"day within octave under ordinary Double", double, &models.Feast{ID: "epiphany-octave-day-3", Rank: models.SemiDouble, Category: models.CategoryMartyr}, true},
		{"day within octave excluded by second class", secondClass, &models.Feast{ID: "epiphany-octave-day-3", Rank: models.SemiDouble, Category: models.CategoryMartyr}, false},
		{"seasonal privileged feria retained", double, &models.Feast{ID: "privileged-lenten-feria", Rank: models.PrivilegedFeria, Category: models.CategoryFeria}, true},
		{"seasonal privileged feria excluded by first class", firstClass, &models.Feast{ID: "privileged-advent-feria", Rank: models.PrivilegedFeria, Category: models.CategoryFeria}, false},
		{"second class outgoing excluded by first class", firstClass, secondClass, false},
		{"Sunday outgoing excluded by first class", firstClass, &models.Feast{ID: "sunday", Rank: models.SemiDouble, Category: models.CategorySunday}, false},
		{"Christmas commemorates outgoing Sunday", &models.Feast{ID: "christmas", Rank: models.Double1stClass, Category: models.CategoryLord}, &models.Feast{ID: "sunday", Rank: models.SemiDouble, Category: models.CategorySunday}, true},
		{"ordinary Double outgoing under second class", secondClass, double, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := outgoingCommemoratedAtFirstVespers(tt.winner, tt.loser)
			if got != tt.want {
				t.Fatalf("got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestConcurrenceSecondClassDoubleVsLesserSunday(t *testing.T) {
	// XIII.6: II Class Double vs Lesser Sunday — feast wins
	feast := &models.Feast{
		ID: "holy-name-jesus", Rank: models.Double2ndClass, Category: models.CategoryLord,
	}
	sunday := &models.Feast{
		ID: "epiphany-sunday-2", Rank: models.SemiDouble, Category: models.CategorySunday,
	}
	if got := concurrenceWinner(feast, sunday); got != models.VespersIIOfPreceding {
		t.Errorf("II Class Double vs Lesser Sunday: got %d, want VespersIIOfPreceding", got)
	}
}
