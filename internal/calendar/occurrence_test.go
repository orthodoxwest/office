package calendar

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

func TestResolveDayFeria(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	day, transfers := ResolveDay(date, nil, models.Epiphany, models.Green, m, nil)

	if day.Celebration != nil {
		t.Error("expected nil celebration for feria")
	}
	if day.Color != models.Green {
		t.Errorf("feria color = %v, want green", day.Color)
	}
	if len(transfers) != 0 {
		t.Error("expected no transfers for feria")
	}
}

func TestResolveDayPrivileged(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)

	easter := &models.Feast{
		ID:       "easter-sunday",
		Name:     "Easter Sunday",
		Rank:     models.Double1stClass,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter+0",
	}
	saint := &models.Feast{
		ID:       "some-saint",
		Name:     "Some Saint",
		Rank:     models.Double,
		Color:    models.Red,
		Category: models.CategoryMartyr,
		Month:    4,
		Day:      12,
	}

	day, transfers := ResolveDay(date, []*models.Feast{easter, saint}, models.Easter, models.White, m, nil)

	if day.Celebration != easter {
		t.Error("expected easter to win")
	}
	if len(transfers) != 0 {
		t.Error("saint is only double rank, should not transfer")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != saint {
		t.Error("saint should be commemorated")
	}
}

func TestResolveDaySundayPrecedence(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)

	sunday := &models.Feast{
		ID:       "epiphany-sunday-3",
		Name:     "3rd Sunday after Epiphany",
		Rank:     models.SemiDouble,
		Color:    models.Green,
		Category: models.CategorySunday,
		DateRule: "epiphany-sunday-3",
	}
	saint := &models.Feast{
		ID:       "conversion-st-paul",
		Name:     "Conversion of St. Paul",
		Rank:     models.GreaterDouble,
		Color:    models.White,
		Category: models.CategoryApostle,
		Month:    1,
		Day:      25,
	}

	day, _ := ResolveDay(date, []*models.Feast{sunday, saint}, models.Epiphany, models.Green, m, nil)

	// Sunday (boosted to greater-double) should beat greater-double saint via temporal bonus
	if day.Celebration != sunday {
		t.Errorf("expected Sunday to win, got %v", day.Celebration.Name)
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != saint {
		t.Error("saint should be commemorated")
	}
}

func TestResolveDayPrivilegedFeriaPrecedence(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)
	feria := &models.Feast{
		ID:       "privileged-lenten-feria",
		Name:     "Friday after Ash Wednesday",
		Rank:     models.PrivilegedFeria,
		Color:    models.Violet,
		Category: models.CategoryFeria,
	}
	greaterDouble := &models.Feast{
		ID:       "st-raphael-of-brooklyn",
		Name:     "St. Raphael of Brooklyn",
		Rank:     models.GreaterDouble,
		Color:    models.White,
		Category: models.CategoryConfessorBishop,
	}
	secondClass := &models.Feast{
		ID:       "st-joseph",
		Name:     "St. Joseph",
		Rank:     models.Double2ndClass,
		Color:    models.White,
		Category: models.CategoryConfessor,
	}

	day, transfers := ResolveDay(date, []*models.Feast{feria, greaterDouble}, models.Lent, models.Violet, m, nil)
	if day.Celebration != feria {
		t.Fatal("expected privileged feria to outrank Greater Double")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != greaterDouble {
		t.Fatal("expected Greater Double to be commemorated under privileged feria")
	}
	if len(transfers) != 0 {
		t.Fatal("expected no transfer for Greater Double")
	}

	day, transfers = ResolveDay(date, []*models.Feast{feria, secondClass}, models.Lent, models.Violet, m, nil)
	if day.Celebration != secondClass {
		t.Fatal("expected Double of the second class to outrank privileged feria")
	}
	if len(transfers) != 0 {
		t.Fatal("expected privileged feria to be commemorated, not transferred")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != feria {
		t.Fatal("expected privileged feria to be commemorated under Double of the second class")
	}
}

func TestResolveDayTransfer(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)

	corpus := &models.Feast{
		ID:       "corpus-christi",
		Name:     "Corpus Christi",
		Rank:     models.Double1stClass,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter+60",
	}
	barnabas := &models.Feast{
		ID:       "st-barnabas",
		Name:     "St. Barnabas, Apostle",
		Rank:     models.GreaterDouble,
		Color:    models.Red,
		Category: models.CategoryApostle,
		Month:    6,
		Day:      11,
	}

	day, _ := ResolveDay(date, []*models.Feast{corpus, barnabas}, models.Pentecost, models.Green, m, nil)

	if day.Celebration != corpus {
		t.Error("corpus christi should win")
	}
	// Barnabas is greater-double — commemorated, not transferred
	if len(day.Commemorations) != 1 || day.Commemorations[0] != barnabas {
		t.Error("barnabas should be commemorated")
	}
}

func TestResolveDayCorpusOctaveBeatsDouble(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)

	corpusDay := &models.Feast{
		ID:       "corpus-christi-octave-day-2",
		Name:     "Day II within the Octave of Corpus Christi",
		Rank:     models.SemiDouble,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter+61",
	}
	irenaeus := &models.Feast{
		ID:       "st-irenaeus",
		Name:     "St Irenaeus of Lyons, Bishop & Martyr",
		Rank:     models.Double,
		Color:    models.Red,
		Category: models.CategoryBishopMartyr,
		Month:    6,
		Day:      28,
	}

	day, _ := ResolveDay(date, []*models.Feast{corpusDay, irenaeus}, models.Pentecost, models.Green, m, nil)
	if day.Celebration != corpusDay {
		t.Fatal("expected corpus octave day to win over double feast")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != irenaeus {
		t.Fatal("expected double feast to be commemorated")
	}
}

func TestResolveDayCorpusOctaveBeatsSecondClassAndTransfersIt(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)

	corpusDay := &models.Feast{
		ID:       "corpus-christi-octave-day-6",
		Name:     "Day VI within the Octave of Corpus Christi",
		Rank:     models.SemiDouble,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter+65",
	}
	visitation := &models.Feast{
		ID:       "visitation-bvm",
		Name:     "Visitation of the Blessed Virgin Mary",
		Rank:     models.Double2ndClass,
		Color:    models.White,
		Category: models.CategoryBlessedVirgin,
		Month:    7,
		Day:      2,
	}

	day, transfers := ResolveDay(date, []*models.Feast{corpusDay, visitation}, models.Pentecost, models.Green, m, nil)
	if day.Celebration != corpusDay {
		t.Fatal("expected corpus octave day to win over second-class feast")
	}
	if len(transfers) != 1 || transfers[0] != visitation {
		t.Fatal("expected second-class feast to be transferred out")
	}
}

func TestResolveDaySundayBeatsCorpusOctave(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	sunday := &models.Feast{
		ID:       "pentecost-sunday-2",
		Name:     "II Sunday after Pentecost",
		Rank:     models.SemiDouble,
		Color:    models.Green,
		Category: models.CategorySunday,
		DateRule: "easter+63",
	}
	corpusDay := &models.Feast{
		ID:       "corpus-christi-octave-day-4",
		Name:     "Day IV within the Octave of Corpus Christi",
		Rank:     models.SemiDouble,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter+63",
	}

	day, _ := ResolveDay(date, []*models.Feast{sunday, corpusDay}, models.Pentecost, models.Green, m, nil)
	if day.Celebration != sunday {
		t.Fatal("expected Sunday to outrank corpus octave day")
	}
}

func TestResolveDayFirstClassBeatsCorpusOctave(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)

	ssPeterPaul := &models.Feast{
		ID:       "ss-peter-paul",
		Name:     "Ss. Peter and Paul, Apostles",
		Rank:     models.Double1stClass,
		Color:    models.Red,
		Category: models.CategoryApostle,
		Month:    6,
		Day:      29,
	}
	corpusDay := &models.Feast{
		ID:       "corpus-christi-octave-day-3",
		Name:     "Day III within the Octave of Corpus Christi",
		Rank:     models.SemiDouble,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter+62",
	}

	day, _ := ResolveDay(date, []*models.Feast{ssPeterPaul, corpusDay}, models.Pentecost, models.Green, m, nil)
	if day.Celebration != ssPeterPaul {
		t.Fatal("expected first-class feast to outrank corpus octave day")
	}
}

func TestResolveDayAllCommemorationsDedupeAndCap(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)

	dupeA := &models.Feast{
		ID:       "comm-a",
		Name:     "St. Marcellus I of Rome, Pope & Martyr",
		Rank:     models.Commemoration,
		Color:    models.Red,
		Category: models.CategoryBishopMartyr,
		Month:    1,
		Day:      16,
	}
	dupeB := &models.Feast{
		ID:       "comm-b",
		Name:     "St. Marcellus I of Rome, Bishop & Martyr",
		Rank:     models.Commemoration,
		Color:    models.Red,
		Category: models.CategoryBishopMartyr,
		Month:    1,
		Day:      16,
	}
	commC := &models.Feast{
		ID:       "comm-c",
		Name:     "Commemoration of St Peter, Apostle",
		Rank:     models.Commemoration,
		Color:    models.White,
		Category: models.CategoryApostle,
		Month:    1,
		Day:      16,
	}
	commD := &models.Feast{
		ID:       "comm-d",
		Name:     "St. Titus, Bishop & Confessor",
		Rank:     models.Commemoration,
		Color:    models.White,
		Category: models.CategoryConfessorBishop,
		Month:    1,
		Day:      16,
	}
	commE := &models.Feast{
		ID:       "comm-e",
		Name:     "St. Prisca of Rome, Virgin Martyr",
		Rank:     models.Commemoration,
		Color:    models.Red,
		Category: models.CategoryVirginMartyr,
		Month:    1,
		Day:      16,
	}

	day, transfers := ResolveDay(
		date,
		[]*models.Feast{dupeA, dupeB, commC, commD, commE},
		models.Epiphany,
		models.Green,
		m,
		nil,
	)

	if day.Celebration != nil {
		t.Fatal("expected no principal celebration when all candidates are commemorations")
	}
	if len(transfers) != 0 {
		t.Fatal("expected no transfers when all candidates are commemorations")
	}
	if len(day.Commemorations) != 4 {
		t.Fatalf("expected 4 commemorations after deduplication, got %d", len(day.Commemorations))
	}
	for _, comm := range day.Commemorations {
		if comm == dupeB {
			t.Fatal("expected duplicate commemoration name to be removed")
		}
	}
}

func TestResolveDaySuppressOnlyWithWhenAllCandidatesAreCommemorations(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)

	commStPaul := &models.Feast{
		ID:       "comm-extra-02-22-commemoration-of-st-paul",
		Name:     "Commemoration of St Paul",
		Rank:     models.Commemoration,
		Color:    models.White,
		Category: models.CategoryConfessor,
		Month:    2,
		Day:      22,
		OnlyWith: "chair-peter-antioch",
	}
	otherCommemoration := &models.Feast{
		ID:       "comm-02-22-test",
		Name:     "Test Commemoration",
		Rank:     models.Commemoration,
		Color:    models.White,
		Category: models.CategoryConfessor,
		Month:    2,
		Day:      22,
	}

	day, transfers := ResolveDay(
		date,
		[]*models.Feast{commStPaul, otherCommemoration},
		models.Septuagesima,
		models.Violet,
		m,
		nil,
	)

	if day.Celebration != nil {
		t.Fatal("expected no principal celebration when all candidates are commemorations")
	}
	if len(transfers) != 0 {
		t.Fatal("expected no transfers when all candidates are commemorations")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != otherCommemoration {
		t.Fatal("expected OnlyWith commemoration to be suppressed without matching winner")
	}
}

func TestResolveDaySuppressWhitEmberCommemorationWithinPentecostOctave(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

	octaveDay := &models.Feast{
		ID:       "pentecost-octave-day-7",
		Name:     "Day VII within the Octave of Pentecost",
		Rank:     models.Double1stClass,
		Color:    models.Red,
		Category: models.CategoryLord,
		DateRule: "easter+56",
	}
	whitEmber := &models.Feast{
		ID:       "whit-ember-saturday",
		Name:     "Whit Ember Saturday",
		Rank:     models.SemiDouble,
		Color:    models.Red,
		Category: models.CategoryFeria,
		DateRule: "easter+55",
	}
	otherCommemoration := &models.Feast{
		ID:       "comm-06-10-st-margaret-of-scotland",
		Name:     "St Margaret of Scotland, Queen & Widow",
		Rank:     models.Commemoration,
		Color:    models.White,
		Category: models.CategoryConfessor,
		Month:    6,
		Day:      10,
	}

	day, _ := ResolveDay(
		date,
		[]*models.Feast{octaveDay, whitEmber, otherCommemoration},
		models.Pentecost,
		models.Green,
		m,
		nil,
	)

	if day.Celebration != octaveDay {
		t.Fatal("expected Pentecost octave day to win")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != otherCommemoration {
		t.Fatal("expected Whit Ember commemoration to be suppressed")
	}
}

func TestResolveDaySuppressDayIVCommemorationOnSundayWithinOctave(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)

	sunday := &models.Feast{
		ID:       "pentecost-sunday-2",
		Name:     "II Sunday after Pentecost",
		Rank:     models.SemiDouble,
		Color:    models.Green,
		Category: models.CategorySunday,
		DateRule: "easter+63",
	}
	dayIVCorpus := &models.Feast{
		ID:       "corpus-christi-octave-day-4",
		Name:     "Day IV within the Octave of Corpus Christi",
		Rank:     models.SemiDouble,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter+63",
	}
	otherCommemoration := &models.Feast{
		ID:       "comm-06-14-st-basil",
		Name:     "St Basil the Great, Bishop, Confessor & Doctor",
		Rank:     models.Commemoration,
		Color:    models.White,
		Category: models.CategoryConfessorDoctor,
		Month:    6,
		Day:      14,
	}

	day, _ := ResolveDay(
		date,
		[]*models.Feast{sunday, dayIVCorpus, otherCommemoration},
		models.Pentecost,
		models.Green,
		m,
		nil,
	)

	if day.Celebration != sunday {
		t.Fatal("expected Sunday to win")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != otherCommemoration {
		t.Fatal("expected day IV octave commemoration to be suppressed on Sunday")
	}
}

func TestResolveDaySuppressCommemorationOfStPaulOutsideChairAtAntioch(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)

	peterDamian := &models.Feast{
		ID:       "st-peter-damian",
		Name:     "St Peter Damian, Bishop, Confessor & Doctor",
		Rank:     models.Double,
		Color:    models.White,
		Category: models.CategoryConfessorDoctor,
		Month:    2,
		Day:      23,
	}
	commStPaul := &models.Feast{
		ID:       "comm-extra-02-23-commemoration-of-st-paul",
		Name:     "Commemoration of St Paul",
		Rank:     models.Commemoration,
		Color:    models.White,
		Category: models.CategoryConfessor,
		Month:    2,
		Day:      23,
		OnlyWith: "chair-peter-antioch",
	}
	vigilMatthias := &models.Feast{
		ID:       "comm-extra-02-23-vigil-of-st-matthias",
		Name:     "Vigil of St. Matthias",
		Rank:     models.Commemoration,
		Color:    models.Violet,
		Category: models.CategoryFeria,
		Month:    2,
		Day:      23,
	}

	day, _ := ResolveDay(
		date,
		[]*models.Feast{peterDamian, commStPaul, vigilMatthias},
		models.Septuagesima,
		models.Violet,
		m,
		nil,
	)

	if day.Celebration != peterDamian {
		t.Fatal("expected St Peter Damian to win")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != vigilMatthias {
		t.Fatal("expected commemoration of St Paul to be suppressed outside Chair at Antioch")
	}
}

func TestResolveDayKeepCommemorationOfStPaulWithChairAtAntioch(t *testing.T) {
	m := ComputeMoveableDates(2026)
	date := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)

	chair := &models.Feast{
		ID:       "chair-peter-antioch",
		Name:     "Chair of St. Peter at Antioch",
		Rank:     models.Double2ndClass,
		Color:    models.White,
		Category: models.CategoryApostle,
		Month:    2,
		Day:      23,
	}
	commStPaul := &models.Feast{
		ID:       "comm-extra-02-23-commemoration-of-st-paul",
		Name:     "Commemoration of St Paul",
		Rank:     models.Commemoration,
		Color:    models.White,
		Category: models.CategoryConfessor,
		Month:    2,
		Day:      23,
		OnlyWith: "chair-peter-antioch",
	}

	day, _ := ResolveDay(
		date,
		[]*models.Feast{chair, commStPaul},
		models.Septuagesima,
		models.Violet,
		m,
		nil,
	)

	if day.Celebration != chair {
		t.Fatal("expected Chair at Antioch to win")
	}
	if len(day.Commemorations) != 1 || day.Commemorations[0] != commStPaul {
		t.Fatal("expected commemoration of St Paul to be retained with Chair at Antioch")
	}
}

func TestResolveDaySuppressStGeorgeOctaveCommemorationOnPrivilegedDay(t *testing.T) {
	m := ComputeMoveableDates(2025)
	date := time.Date(2025, 4, 25, 0, 0, 0, 0, time.UTC)

	holyThursday := &models.Feast{
		ID:       "holy-thursday",
		Name:     "Holy Thursday",
		Rank:     models.Double1stClass,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter-3",
	}
	stGeorgeDayIII := &models.Feast{
		ID:       "st-george-octave-day-3",
		Name:     "Day III within the Octave of St. George",
		Rank:     models.SemiDouble,
		Color:    models.Red,
		Category: models.CategoryMartyr,
		Month:    4,
		Day:      25,
	}

	day, _ := ResolveDay(
		date,
		[]*models.Feast{holyThursday, stGeorgeDayIII},
		models.Passiontide,
		models.Violet,
		m,
		nil,
	)

	if day.Celebration != holyThursday {
		t.Fatal("expected Holy Thursday to win")
	}
	if len(day.Commemorations) != 0 {
		t.Fatal("expected St George octave commemoration to be suppressed on privileged day")
	}
}

func TestResolveDaySuppressStGeorgeOctaveCommemorationOnEasterOctaveDay(t *testing.T) {
	m := ComputeMoveableDates(2025)
	date := time.Date(2025, 4, 25, 0, 0, 0, 0, time.UTC)

	easterDayVI := &models.Feast{
		ID:       "easter-sunday-octave-day-6",
		Name:     "Day VI within the Octave of Easter",
		Rank:     models.Double1stClass,
		Color:    models.White,
		Category: models.CategoryLord,
		DateRule: "easter+5",
	}
	stGeorgeDayIII := &models.Feast{
		ID:       "st-george-octave-day-3",
		Name:     "Day III within the Octave of St. George",
		Rank:     models.SemiDouble,
		Color:    models.Red,
		Category: models.CategoryMartyr,
		Month:    4,
		Day:      25,
	}

	day, _ := ResolveDay(
		date,
		[]*models.Feast{easterDayVI, stGeorgeDayIII},
		models.Easter,
		models.White,
		m,
		nil,
	)

	if day.Celebration != easterDayVI {
		t.Fatal("expected Easter octave day to win")
	}
	if len(day.Commemorations) != 0 {
		t.Fatal("expected St George octave commemoration to be suppressed during Easter octave")
	}
}
