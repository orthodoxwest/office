package office

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestOfficeOfTheDeadHours(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	engine, err := NewEngine(dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	compose := func(t *testing.T, year int, month time.Month, day int, hourName string) (*models.CalendarDay, *models.OfficeHour) {
		t.Helper()
		days, err := calendar.BuildCalendar(year, dataDir)
		if err != nil {
			t.Fatalf("BuildCalendar(%d): %v", year, err)
		}
		var calDay *models.CalendarDay
		want := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		for i := range days {
			if days[i].Date.Equal(want) {
				calDay = &days[i]
				break
			}
		}
		if calDay == nil {
			t.Fatalf("%s not found", want.Format("2006-01-02"))
		}
		hour, err := engine.ComposeHour(hourName, calDay, calendar.ComputeMoveableDates(year))
		if err != nil {
			t.Fatalf("ComposeHour(%s): %v", hourName, err)
		}
		return calDay, hour
	}
	joined := func(hour *models.OfficeHour) string {
		var b strings.Builder
		for _, sec := range hour.Sections {
			if sec.Label != "" {
				b.WriteString(sec.Label)
				b.WriteByte('\n')
			}
			for _, el := range sec.Elements {
				b.WriteString(el.Text)
				b.WriteByte('\n')
			}
		}
		return b.String()
	}

	t.Run("2026-11-01 vespers appends the Dead after Let us bless the Lord", func(t *testing.T) {
		_, hour := compose(t, 2026, time.November, 1, "vespers")
		text := joined(hour)
		bless := strings.Index(text, "Let us bless the Lord")
		dead := strings.Index(text, "Vespers of the Dead")
		walk := strings.Index(text, "I will walk before the Lord")
		if bless < 0 || dead < 0 || walk < 0 {
			t.Fatalf("missing Dead vespers sequence: bless=%d dead=%d walk=%d", bless, dead, walk)
		}
		if !(bless < dead && dead < walk) {
			t.Fatalf("order bless=%d dead=%d walk=%d, want bless < heading < I will walk", bless, dead, walk)
		}
		if strings.Contains(text[dead:], "Salve Regina") || strings.Contains(text[dead:], "Mary we hail thee") {
			t.Fatal("Vespers of the Dead should end without the Marian antiphon")
		}
		if !strings.Contains(text[dead:], "All that the Father giveth me") {
			t.Fatal("Vespers of the Dead missing Magnificat antiphon")
		}
	})

	t.Run("2026-11-01 compline is of the Dead", func(t *testing.T) {
		_, hour := compose(t, 2026, time.November, 1, "compline")
		if hour.Color != models.Black {
			t.Errorf("Compline color = %s, want black", hour.Color)
		}
		text := joined(hour)
		if !strings.Contains(text, "Look down, we beseech thee, O Lord") {
			t.Fatal("Compline of the Dead missing Look down collect")
		}
		if strings.Contains(text, "Lord, grant a blessing") {
			t.Fatal("Compline of the Dead should omit Sir, ask a blessing")
		}
		if strings.Contains(text, "Salve Regina") || strings.Contains(text, "Mary we hail thee") {
			t.Fatal("Compline of the Dead should omit the Marian antiphon")
		}
		if !strings.Contains(text, "Lord, now lettest thou") && !strings.Contains(text, "lettest thou thy servant") {
			t.Fatal("Compline of the Dead missing Nunc dimittis")
		}
	})

	t.Run("2026-11-02 lauds of the Dead has no occurrent commemorations", func(t *testing.T) {
		_, hour := compose(t, 2026, time.November, 2, "lauds")
		text := joined(hour)
		if strings.Contains(text, "Commemoration of") {
			t.Fatal("All Souls Lauds should not commemorate the octave or Winifred")
		}
		if strings.Contains(text, "Salve Regina") || strings.Contains(text, "Mary we hail thee") {
			t.Fatal("Lauds of the Dead should end without the Marian antiphon")
		}
		if !strings.Contains(text, "The bones which thou hast broken") {
			t.Fatal("Lauds of the Dead missing first antiphon")
		}
		if !strings.Contains(text, "May they rest in peace") {
			t.Fatal("Lauds of the Dead missing closing versicle")
		}
	})

	t.Run("2025-11-02 vespers marks Dead office optional after Sunday", func(t *testing.T) {
		day, hour := compose(t, 2025, time.November, 2, "vespers")
		if !day.Vespers.AppendedOfficeOfTheDead {
			t.Fatal("2025-11-02 should append Vespers of the Dead (All Souls on Monday)")
		}
		text := joined(hour)
		if !strings.Contains(text, "Optional:") {
			t.Fatal("2025 Sunday eve of transferred All Souls should mark Vespers of the Dead optional")
		}
		if !strings.Contains(text, "I will walk before the Lord") {
			t.Fatal("optional Vespers of the Dead missing I will walk")
		}
	})
}
