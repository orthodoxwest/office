package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func makeSuffrageDay(date time.Time, season models.Season, celebration *models.Feast, comms ...*models.Feast) *models.CalendarDay {
	return &models.CalendarDay{
		Date:           date,
		Season:         season,
		Celebration:    celebration,
		Commemorations: comms,
	}
}

func TestEvaluateConditionNegation(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		name      string
		condition string
		day       *models.CalendarDay
		want      bool
	}{
		{
			name:      "not-weekday-sunday on Sunday is false",
			condition: "not-weekday-sunday",
			day:       &models.CalendarDay{Date: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)}, // Sunday
			want:      false,
		},
		{
			name:      "not-weekday-sunday on Monday is true",
			condition: "not-weekday-sunday",
			day:       &models.CalendarDay{Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)}, // Monday
			want:      true,
		},
		{
			name:      "not-if-preces on ferial day is false",
			condition: "not-if-preces",
			day:       makeDay(2026, 3, 16, nil, nil, ""),
			want:      false,
		},
		{
			name:      "not-if-preces on double feast is true",
			condition: "not-if-preces",
			day:       makeDay(2026, 3, 15, &models.Feast{ID: "test", Rank: models.Double}, nil, ""),
			want:      true,
		},
		{
			name:      "not-feast-easter-sunday when not Easter",
			condition: "not-feast-easter-sunday",
			day:       makeDay(2026, 3, 16, &models.Feast{ID: "some-feast"}, nil, ""),
			want:      true,
		},
		{
			name:      "not-feast-easter-sunday on Easter",
			condition: "not-feast-easter-sunday",
			day:       makeDay(2026, 4, 12, &models.Feast{ID: "easter-sunday"}, nil, ""),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition(tt.condition, tt.day, moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestEvaluateConditionAND(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		name      string
		condition string
		day       *models.CalendarDay
		want      bool
	}{
		{
			name:      "sunday AND not-if-preces on double feast — true",
			condition: "weekday-sunday,not-if-preces",
			day:       makeDay(2026, 3, 15, &models.Feast{ID: "test", Rank: models.Double}, nil, ""),
			want:      true, // Sunday=true, double suppresses preces so not-if-preces=true
		},
		{
			name:      "sunday AND not-if-preces on ferial Sunday — false",
			condition: "weekday-sunday,not-if-preces",
			day:       makeDay(2026, 3, 15, nil, nil, ""),
			want:      false, // Sunday=true, but preces ARE said so not-if-preces=false
		},
		{
			name:      "monday AND monday — true",
			condition: "weekday-monday,weekday-monday",
			day:       &models.CalendarDay{Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
			want:      true,
		},
		{
			name:      "monday AND sunday — false",
			condition: "weekday-monday,weekday-sunday",
			day:       &models.CalendarDay{Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
			want:      false,
		},
		{
			name:      "feast match AND weekday match — both true",
			condition: "feast-easter-sunday,weekday-sunday",
			day:       makeDay(2026, 4, 12, &models.Feast{ID: "easter-sunday"}, nil, ""),
			want:      true,
		},
		{
			name:      "feast match AND weekday mismatch — false",
			condition: "feast-easter-sunday,weekday-monday",
			day:       makeDay(2026, 4, 12, &models.Feast{ID: "easter-sunday"}, nil, ""),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition(tt.condition, tt.day, moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestEvaluateConditionFeast(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		day       *models.CalendarDay
		want      bool
	}{
		{
			name:      "feast matches celebration ID",
			condition: "feast-christmas",
			day:       makeDay(2026, 12, 25, &models.Feast{ID: "christmas"}, nil, ""),
			want:      true,
		},
		{
			name:      "feast does not match different celebration",
			condition: "feast-christmas",
			day:       makeDay(2026, 12, 26, &models.Feast{ID: "st-stephen"}, nil, ""),
			want:      false,
		},
		{
			name:      "feast condition with no celebration",
			condition: "feast-christmas",
			day:       makeDay(2026, 3, 16, nil, nil, ""),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition(tt.condition, tt.day, nil)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestEvaluateConditionUnknown(t *testing.T) {
	day := &models.CalendarDay{Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)}
	if evaluateCondition("some-unknown-condition", day, nil) {
		t.Error("evaluateCondition(unknown) = true, want false")
	}
	if evaluateCondition("not-some-unknown-condition", day, nil) {
		t.Error("evaluateCondition(not-unknown) = true, want false")
	}
}

func TestShouldSaySuffrage(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		name string
		day  *models.CalendarDay
		want bool
	}{
		{
			name: "Septuagesima Sunday now says suffrage",
			day: makeSuffrageDay(
				time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC),
				models.Septuagesima,
				&models.Feast{ID: "septuagesima", Category: models.CategorySunday, Rank: models.Double2ndClass},
			),
			want: true,
		},
		{
			name: "Sexagesima Sunday with commemoration rank commemoration still says suffrage",
			day: makeSuffrageDay(
				time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
				models.Septuagesima,
				&models.Feast{ID: "sexagesima", Category: models.CategorySunday, Rank: models.Double2ndClass},
				&models.Feast{ID: "comm-02-15-ss-faustinus-and-jovita-martyrs", Rank: models.Commemoration},
			),
			want: true,
		},
		{
			name: "Lent Sunday with commemoration rank commemoration still says suffrage",
			day: makeSuffrageDay(
				time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
				models.Lent,
				&models.Feast{ID: "lent-sunday-1", Category: models.CategorySunday, Rank: models.Double1stClass},
				&models.Feast{ID: "comm-03-01-st-david-of-wales-bishop-and-confessor", Rank: models.Commemoration},
			),
			want: true,
		},
		{
			name: "Lent Sunday with no commemoration still says suffrage",
			day: makeSuffrageDay(
				time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
				models.Lent,
				&models.Feast{ID: "lent-sunday-3", Category: models.CategorySunday, Rank: models.Double1stClass},
			),
			want: true,
		},
		{
			name: "ordinary Pentecost Sunday says suffrage",
			day: makeSuffrageDay(
				time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC),
				models.Pentecost,
				&models.Feast{ID: "pentecost-sunday-6", Category: models.CategorySunday, Rank: models.SemiDouble},
				&models.Feast{ID: "comm-07-12-ss-nabor-and-felix-martyrs", Rank: models.Commemoration},
			),
			want: true,
		},
		{
			name: "Sunday with simplified double commemoration suppresses suffrage",
			day: makeSuffrageDay(
				time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC),
				models.Epiphany,
				&models.Feast{ID: "epiphany-sunday-2", Category: models.CategorySunday, Rank: models.SemiDouble},
				&models.Feast{ID: "chair-peter-rome", Rank: models.GreaterDouble},
			),
			want: false,
		},
		{
			name: "within octave still suppresses suffrage",
			day: &models.CalendarDay{
				Date:           time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
				Season:         models.Pentecost,
				Celebration:    &models.Feast{ID: "pentecost-sunday-5", Category: models.CategorySunday, Rank: models.SemiDouble},
				Commemorations: []*models.Feast{{ID: "ss-peter-paul-octave-day-7", Rank: models.SemiDouble}},
				WithinOctaveOf: "ss-peter-paul",
			},
			want: false,
		},
		{
			name: "feast office does not say suffrage just because rank is semi-double",
			day: makeSuffrageDay(
				time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
				models.Pentecost,
				&models.Feast{ID: "some-semidouble-feast", Category: models.CategoryMartyr, Rank: models.SemiDouble},
			),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSaySuffrage(tt.day, moveable)
			if got != tt.want {
				t.Errorf("shouldSaySuffrage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldSayCrossCommemoration(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		name string
		day  *models.CalendarDay
		want bool
	}{
		{
			name: "Easter feria after Low Sunday says cross commemoration",
			day: makeSuffrageDay(
				time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
				models.Easter,
				&models.Feast{ID: "feria", Category: models.CategoryFeria, Rank: models.Commemoration},
			),
			want: true,
		},
		{
			name: "Easter Sunday office says cross commemoration",
			day: makeSuffrageDay(
				time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
				models.Easter,
				&models.Feast{ID: "easter-sunday-5", Category: models.CategorySunday, Rank: models.SemiDouble},
			),
			want: true,
		},
		{
			name: "feast office in Easter season does not say cross commemoration",
			day: makeSuffrageDay(
				time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
				models.Easter,
				&models.Feast{ID: "some-semidouble-feast", Category: models.CategoryMartyr, Rank: models.SemiDouble},
			),
			want: false,
		},
		{
			name: "within octave suppresses cross commemoration",
			day: &models.CalendarDay{
				Date:           time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
				Season:         models.Easter,
				Celebration:    &models.Feast{ID: "easter-sunday-2", Category: models.CategorySunday, Rank: models.SemiDouble},
				WithinOctaveOf: "st-george",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSayCrossCommemoration(tt.day, moveable)
			if got != tt.want {
				t.Errorf("shouldSayCrossCommemoration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateConditionIsFerialAndSeason(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		day       *models.CalendarDay
		want      bool
	}{
		{
			name:      "is-ferial with feria category",
			condition: "is-ferial",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
				Celebration: &models.Feast{ID: "feria-lent-monday", Category: models.CategoryFeria},
			},
			want: true,
		},
		{
			name:      "is-ferial with feast category",
			condition: "is-ferial",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
				Celebration: &models.Feast{ID: "st-cyril", Category: models.CategoryConfessorDoctor},
			},
			want: false,
		},
		{
			name:      "is-ferial with Sunday category",
			condition: "is-ferial",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
				Celebration: &models.Feast{ID: "sunday-lent", Category: models.CategorySunday},
			},
			want: false,
		},
		{
			name:      "is-ferial with nil celebration",
			condition: "is-ferial",
			day:       &models.CalendarDay{Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
			want:      true,
		},
		{
			name:      "season-easter in Easter",
			condition: "season-easter",
			day:       &models.CalendarDay{Date: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), Season: models.Easter},
			want:      true,
		},
		{
			name:      "season-easter in Lent",
			condition: "season-easter",
			day:       &models.CalendarDay{Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), Season: models.Lent},
			want:      false,
		},
		{
			name:      "season-passiontide in Passiontide",
			condition: "season-passiontide",
			day:       &models.CalendarDay{Date: time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC), Season: models.Passiontide},
			want:      true,
		},
		{
			name:      "not-season-easter in Lent (negated)",
			condition: "not-season-easter",
			day:       &models.CalendarDay{Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), Season: models.Lent},
			want:      true,
		},
		{
			name:      "is-ferial,not-season-easter on Lent feria — both true",
			condition: "is-ferial,not-season-easter",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
				Season:      models.Lent,
				Celebration: &models.Feast{ID: "feria", Category: models.CategoryFeria},
			},
			want: true,
		},
		{
			name:      "is-ferial,season-easter on Easter feria — both true",
			condition: "is-ferial,season-easter",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
				Season:      models.Easter,
				Celebration: &models.Feast{ID: "feria", Category: models.CategoryFeria},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition(tt.condition, tt.day, nil)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}
