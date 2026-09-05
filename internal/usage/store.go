// Package usage stores approximate daily browser counts, not request logs.
package usage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
	_ "time/tzdata"

	_ "modernc.org/sqlite"
)

var Hours = []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}

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
	return false
}

type Store struct{ db *sql.DB }

type Daily struct {
	Day   string
	Users int
	Hours [7]int
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

// Record updates both the site total and the selected hour atomically. The
// cookie itself is never stored; hashes differ each reporting day.
func (s *Store) Record(ctx context.Context, now time.Time, browser, scope string) error {
	if !ValidScope(scope) {
		return fmt.Errorf("invalid usage scope")
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
		if scope == "site" {
			result[i].Users = n
		}
		for h, name := range Hours {
			if scope == name {
				result[i].Hours[h] = n
			}
		}
	}
	return result, rows.Err()
}
