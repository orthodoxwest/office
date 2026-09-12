package web

import (
	"slices"
	"sync"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/render"
)

type yearEntry struct {
	days       []models.CalendarDay
	moveable   *calendar.MoveableDates
	monthsOnce sync.Once
	months     []render.MonthData
}

type yearCache struct {
	mu      sync.Mutex
	entries map[int]*yearEntry
	dataDir string
	order   []int
}

func newYearCache(dataDir string) *yearCache {
	return &yearCache{
		entries: make(map[int]*yearEntry),
		dataDir: dataDir,
	}
}

func (c *yearCache) get(year int) ([]models.CalendarDay, *calendar.MoveableDates, error) {
	e, err := c.entry(year)
	if err != nil {
		return nil, nil, err
	}
	return e.days, e.moveable, nil
}

// getMonths reuses the immutable office summaries as well as calendar dates.
// Compose outside the cache lock: concurrent readers of this year share the
// work, while requests for an hour or another year remain free to proceed.
func (c *yearCache) getMonths(year int, eng *office.Engine) ([]render.MonthData, error) {
	e, err := c.entry(year)
	if err != nil {
		return nil, err
	}
	e.monthsOnce.Do(func() { e.months = buildMonthData(e.days, eng, e.moveable) })
	return e.months, nil
}

// Retain a small window of years, rather than an entire browsable archive of
// composed summaries. Evicted entries stay valid for any in-flight readers.
const maxCachedYears = 8

func (c *yearCache) entry(year int) (*yearEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[year]; ok {
		// A frequently read year should survive browsing older years.
		i := slices.Index(c.order, year)
		copy(c.order[i:], c.order[i+1:])
		c.order[len(c.order)-1] = year
		return e, nil
	}

	days, err := calendar.BuildCalendar(year, c.dataDir)
	if err != nil {
		return nil, err
	}

	moveable := calendar.ComputeMoveableDates(year)
	e := &yearEntry{days: days, moveable: moveable}
	if len(c.order) == maxCachedYears {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.entries[year] = e
	c.order = append(c.order, year)
	return e, nil
}
