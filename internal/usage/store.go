// Package usage stores approximate daily browser counts, not request logs.
package usage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	_ "modernc.org/sqlite"
)

var Hours = []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}

// extraScopes are single-purpose scopes tracked alongside the hours: the
// ordo (calendar) page view, and the reminder-feed link actually being
// generated (not merely the /reminders page loading — see app.js).
var extraScopes = []string{"ordo", "reminders"}

// dimensionFamilies describe how a page was rendered rather than which page
// it was: the appearance actually on screen, and whether the phone layout was
// in force. A beacon may carry at most one token from each family, and each
// is counted like any other scope — once per browser, per day — so the
// families sum to at most the daily total and a reader who switches
// appearance or rotates a phone during a day counts on both sides.
var dimensionFamilies = [][2]string{{"nave", "apse"}, {"desktop", "mobile"}}

// Event is one beacon: the page scope plus whatever dimensions the browser
// reported about it.
type Event struct {
	Scope      string
	Dimensions []string
}

// ParseEvent reads a beacon body: the page scope, then at most one token per
// dimension family, separated by single spaces ("lauds apse mobile").
//
// Only the scope has to be understood. The service worker keeps app.js
// cached across deploys, so both directions of skew are ordinary: a client
// from before dimensions existed sends the bare scope, and a client from
// after a rollback sends tokens this build has never heard of. Either way
// the page still counts and the unreadable tokens are dropped, rather than
// a whole congregation of un-refreshed browsers vanishing from the report.
func ParseEvent(body string) (Event, bool) {
	fields := strings.Split(body, " ")
	if !ValidScope(fields[0]) {
		return Event{}, false
	}
	event := Event{Scope: fields[0]}
	seen := make(map[int]bool, len(dimensionFamilies))
	for _, field := range fields[1:] {
		family, ok := dimensionFamily(field)
		if !ok || seen[family] {
			continue
		}
		seen[family] = true
		event.Dimensions = append(event.Dimensions, field)
	}
	return event, true
}

func dimensionFamily(token string) (int, bool) {
	for i, family := range dimensionFamilies {
		for _, name := range family {
			if token == name {
				return i, true
			}
		}
	}
	return 0, false
}

var eastern = func() *time.Location {
	l, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return l
}()

func Day(t time.Time) string { return t.In(eastern).Format(time.DateOnly) }

func ValidScope(scope string) bool {
	if scope == "site" {
		return true
	}
	for _, h := range Hours {
		if scope == h {
			return true
		}
	}
	for _, s := range extraScopes {
		if scope == s {
			return true
		}
	}
	return false
}

type Store struct{ db *sql.DB }

type Daily struct {
	Day       string
	Users     int
	Hours     [7]int
	Ordo      int
	Reminders int
	// Dimensions of the same day's browsers: appearance rendered and screen.
	// Each pair overlaps the total rather than partitioning it.
	Nave, Apse      int
	Desktop, Mobile int
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA busy_timeout=1000;
PRAGMA journal_mode=WAL;
PRAGMA secure_delete=ON;
CREATE TABLE IF NOT EXISTS totals (
 day TEXT NOT NULL, scope TEXT NOT NULL, users INTEGER NOT NULL,
 PRIMARY KEY(day, scope)
);
CREATE TABLE IF NOT EXISTS seen (
 day TEXT NOT NULL, browser BLOB NOT NULL, scope TEXT NOT NULL,
 PRIMARY KEY(day, browser, scope)
);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Record updates the site total, the selected hour, and any reported
// dimensions atomically. The cookie itself is never stored; hashes differ
// each reporting day.
func (s *Store) Record(ctx context.Context, now time.Time, browser, scope string, dimensions ...string) error {
	if !ValidScope(scope) {
		return fmt.Errorf("invalid usage scope")
	}
	for _, d := range dimensions {
		if _, ok := dimensionFamily(d); !ok {
			return fmt.Errorf("invalid usage dimension")
		}
	}
	day := Day(now)
	hash := sha256.Sum256([]byte(day + "\x00" + browser))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM seen WHERE day < ?", Day(now.In(eastern).AddDate(0, 0, -2))); err != nil {
		return err
	}
	scopes := []string{"site"}
	if scope != "site" {
		scopes = append(scopes, scope)
	}
	scopes = append(scopes, dimensions...)
	for _, v := range scopes {
		res, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO seen VALUES (?, ?, ?)", day, hash[:], v)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO totals VALUES (?, ?, 1)
ON CONFLICT(day, scope) DO UPDATE SET users = users + 1`, day, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Daily returns a bounded, zero-filled window, newest day first. Aggregates
// survive indefinitely; deduplication rows expire on the next event or read.
func (s *Store) Daily(ctx context.Context, now time.Time, days int) ([]Daily, error) {
	if days < 1 || days > 366 {
		return nil, fmt.Errorf("invalid day window")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM seen WHERE day < ?", Day(now.In(eastern).AddDate(0, 0, -2))); err != nil {
		return nil, err
	}
	result := make([]Daily, days)
	indices := make(map[string]int, days)
	for i := range result {
		result[i].Day = Day(now.In(eastern).AddDate(0, 0, -i))
		indices[result[i].Day] = i
	}
	rows, err := s.db.QueryContext(ctx, "SELECT day, scope, users FROM totals WHERE day >= ? AND day <= ?", result[days-1].Day, result[0].Day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var day, scope string
		var n int
		if err := rows.Scan(&day, &scope, &n); err != nil {
			return nil, err
		}
		i, ok := indices[day]
		if !ok {
			continue
		}
		switch scope {
		case "site":
			result[i].Users = n
		case "ordo":
			result[i].Ordo = n
		case "reminders":
			result[i].Reminders = n
		case "nave":
			result[i].Nave = n
		case "apse":
			result[i].Apse = n
		case "desktop":
			result[i].Desktop = n
		case "mobile":
			result[i].Mobile = n
		default:
			for h, name := range Hours {
				if scope == name {
					result[i].Hours[h] = n
				}
			}
		}
	}
	return result, rows.Err()
}
