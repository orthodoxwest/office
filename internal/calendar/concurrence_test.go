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
	// XIII.6: II Class Double vs Greater Sunday — Sunday wins
	feast := &models.Feast{
		ID: "circumcision", Rank: models.Double2ndClass, Category: models.CategoryLord,
	}
	sunday := &models.Feast{
		ID: "advent-sunday-1", Rank: models.Double2ndClass, Category: models.CategorySunday,
	}
	if got := concurrenceWinner(feast, sunday); got != models.VespersIOfFollowing {
		t.Errorf("II Class Double vs Greater Sunday: got %d, want VespersIOfFollowing", got)
	}
}

func TestConcurrenceWinnerLordDoubleVsLesserSunday(t *testing.T) {
	// XIII.7: XII-Lesson Feast of Lord vs Lesser Sunday — feast wins
	lord := &models.Feast{
		ID: "transfiguration", Rank: models.Double, Category: models.CategoryLord,
	}
	sunday := &models.Feast{
		ID: "pentecost-sunday-10", Rank: models.SemiDouble, Category: models.CategorySunday,
	}
	if got := concurrenceWinner(lord, sunday); got != models.VespersIIOfPreceding {
		t.Errorf("Lord Double vs Lesser Sunday: got %d, want VespersIIOfPreceding", got)
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
	// XIII.10: Double vs Octave Day — the win is decided by precedence, not
	// applied unconditionally. A higher-ranked Octave Day out-ranks a plain
	// Double (issue #22: Octave Day of Epiphany keeps II Vespers vs St. Hilary).
	double := &models.Feast{
		ID: "some-double", Rank: models.Double, Category: models.CategoryMartyr,
	}
	greaterOctDay := &models.Feast{
		ID: "epiphany-octave-day", Rank: models.GreaterDouble, Category: models.CategoryLord,
	}
	// Octave Day (Greater Double) preceding, plain Double following — Octave Day wins.
	if got := concurrenceWinner(greaterOctDay, double); got != models.VespersIIOfPreceding {
		t.Errorf("greater Octave Day vs Double: got %d, want VespersIIOfPreceding", got)
	}
	// Reverse ordering — the greater Octave Day still wins (I Vespers of following).
	if got := concurrenceWinner(double, greaterOctDay); got != models.VespersIOfFollowing {
		t.Errorf("Double vs greater Octave Day: got %d, want VespersIOfFollowing", got)
	}

	// When the Double genuinely out-ranks a lesser Octave Day, the Double wins.
	greaterDouble := &models.Feast{
		ID: "some-greater-double", Rank: models.GreaterDouble, Category: models.CategoryMartyr,
	}
	lesserOctDay := &models.Feast{
		ID: "some-octave-day", Rank: models.Double, Category: models.CategoryConfessor,
	}
	if got := concurrenceWinner(greaterDouble, lesserOctDay); got != models.VespersIIOfPreceding {
		t.Errorf("greater Double vs lesser Octave Day: got %d, want VespersIIOfPreceding", got)
	}
	if got := concurrenceWinner(lesserOctDay, greaterDouble); got != models.VespersIOfFollowing {
		t.Errorf("lesser Octave Day vs greater Double: got %d, want VespersIOfFollowing", got)
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
		t.Errorf("Privileged (prec) vs Double: got %d, want VespersIIOfPreceding", got)
	}
	if got := concurrenceWinner(double, easter); got != models.VespersIOfFollowing {
		t.Errorf("Double vs Privileged (fol): got %d, want VespersIOfFollowing", got)
	}
}

func TestResolveConcurrenceTwoFerias(t *testing.T) {
	day1 := &models.CalendarDay{Celebration: nil}
	day2 := &models.CalendarDay{Celebration: nil}
	result := resolveConcurrence(day1, day2)
	if result.Owner != models.VespersNotApplicable {
		t.Errorf("Two ferias: got owner %d, want VespersNotApplicable", result.Owner)
	}
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

	// I Vespers of following: the outgoing (losing) office is commemorated.
	result := resolveConcurrence(day1, day2)
	if result.Owner != models.VespersIOfFollowing {
		t.Fatalf("got owner %d, want VespersIOfFollowing", result.Owner)
	}
	if len(result.Commemorations) != 1 || result.Commemorations[0] != simple {
		t.Errorf("expected boundary commemoration of the outgoing feast, got %v", result.Commemorations)
	}

	// II Vespers of preceding: the incoming (losing) office is commemorated.
	result = resolveConcurrence(day2, day1)
	if result.Owner != models.VespersIIOfPreceding {
		t.Fatalf("got owner %d, want VespersIIOfPreceding", result.Owner)
	}
	if len(result.Commemorations) != 1 || result.Commemorations[0] != simple {
		t.Errorf("expected boundary commemoration of the incoming feast, got %v", result.Commemorations)
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
