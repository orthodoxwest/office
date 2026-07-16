package office

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestEnginePreloadsHourDefinitions(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	eng, err := NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, hourName := range hourNames {
		want, err := ParseHourDefinition(filepath.Join(dataDir, "office", hourName+".txt"))
		if err != nil {
			t.Fatalf("ParseHourDefinition(%s): %v", hourName, err)
		}
		if !reflect.DeepEqual(eng.definitions[hourName], want) {
			t.Errorf("preloaded %s definition differs from direct parse", hourName)
		}
	}
}

func TestEngineConcurrentCompositionIsDeterministicAndDoesNotMutateDays(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	eng, err := NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	type testCase struct {
		hour     string
		day      *models.CalendarDay
		moveable *calendar.MoveableDates
		want     string
		before   []byte
	}
	inputs := []struct {
		hour string
		date time.Time
	}{
		{hour: "lauds", date: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)},
		{hour: "vespers", date: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)},
		{hour: "prime", date: time.Date(2027, 4, 17, 0, 0, 0, 0, time.UTC)},
		{hour: "compline", date: time.Date(2027, 12, 25, 0, 0, 0, 0, time.UTC)},
	}
	calendars := map[int][]models.CalendarDay{}
	moveables := map[int]*calendar.MoveableDates{}
	var cases []testCase
	for _, input := range inputs {
		year := input.date.Year()
		if calendars[year] == nil {
			calendars[year], err = calendar.BuildCalendar(year, dataDir)
			if err != nil {
				t.Fatalf("BuildCalendar(%d): %v", year, err)
			}
			moveables[year] = calendar.ComputeMoveableDates(year)
		}
		day := &calendars[year][input.date.YearDay()-1]
		hour, err := eng.ComposeHour(input.hour, day, moveables[year])
		if err != nil {
			t.Fatalf("ComposeHour(%s, %s): %v", input.hour, input.date.Format("2006-01-02"), err)
		}
		before, err := json.Marshal(day)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, testCase{
			hour: input.hour, day: day, moveable: moveables[year],
			want: officeHourDigest(hour), before: before,
		})
	}

	errCh := make(chan error, len(cases)*8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for _, tc := range cases {
			tc := tc
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					hour, err := eng.ComposeHour(tc.hour, tc.day, tc.moveable)
					if err != nil {
						errCh <- err
						return
					}
					if got := officeHourDigest(hour); got != tc.want {
						errCh <- fmt.Errorf("%s %s digest = %s, want %s", tc.hour, tc.day.Date.Format("2006-01-02"), got, tc.want)
						return
					}
				}
			}()
		}
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	for _, tc := range cases {
		after, err := json.Marshal(tc.day)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(after, tc.before) {
			t.Errorf("ComposeHour mutated cached CalendarDay for %s %s", tc.hour, tc.day.Date.Format("2006-01-02"))
		}
	}
}

func officeHourDigest(hour *models.OfficeHour) string {
	b, err := json.Marshal(hour)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
