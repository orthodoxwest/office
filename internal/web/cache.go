package web

import (
	"sync"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

type yearEntry struct {
	days     []models.CalendarDay
	moveable *calendar.MoveableDates
}

type yearCache struct {
	mu      sync.Mutex
	entries map[int]*yearEntry
	dataDir string
}

func newYearCache(dataDir string) *yearCache {
	return &yearCache{
		entries: make(map[int]*yearEntry),
		dataDir: dataDir,
	}
}

func (c *yearCache) get(year int) ([]models.CalendarDay, *calendar.MoveableDates, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[year]; ok {
		return e.days, e.moveable, nil
	}

	days, err := calendar.BuildCalendar(year, c.dataDir)
	if err != nil {
		return nil, nil, err
	}

	moveable := calendar.ComputeMoveableDates(year)
	c.entries[year] = &yearEntry{days: days, moveable: moveable}
	return days, moveable, nil
}
