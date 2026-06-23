package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/orthodoxwest/office/internal/audit"
	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/output"
	"github.com/orthodoxwest/office/internal/review"
	"github.com/orthodoxwest/office/internal/texts"
	"github.com/orthodoxwest/office/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: office <command> [args]")
		fmt.Fprintln(os.Stderr, "Commands: ordo, validate, audit, review, lauds, prime, terce, sext, none, vespers, compline, tex, serve")
		os.Exit(1)
	}

	dataDir := findDataDir()

	switch os.Args[1] {
	case "ordo":
		cmdOrdo(dataDir)
	case "validate":
		cmdValidate(dataDir)
	case "audit":
		cmdAudit(dataDir)
	case "lauds":
		cmdHour(dataDir, "lauds", "Lauds")
	case "terce":
		cmdHour(dataDir, "terce", "Terce")
	case "sext":
		cmdHour(dataDir, "sext", "Sext")
	case "none":
		cmdHour(dataDir, "none", "None")
	case "vespers":
		cmdHour(dataDir, "vespers", "Vespers")
	case "compline":
		cmdHour(dataDir, "compline", "Compline")
	case "prime":
		cmdHour(dataDir, "prime", "Prime")
	case "tex":
		cmdTeX(dataDir)
	case "review":
		cmdReview(dataDir)
	case "serve":
		cmdServe(dataDir)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdOrdo(dataDir string) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: office ordo YEAR")
		os.Exit(1)
	}

	year, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid year: %s\n", os.Args[2])
		os.Exit(1)
	}

	days, err := calendar.BuildCalendar(year, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building calendar: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(output.FormatCalendar(days))
}

func cmdValidate(dataDir string) {
	var errors []string
	errors = append(errors, calendar.ValidateAll(dataDir)...)
	errors = append(errors, office.ValidateHourDefinitions(dataDir)...)
	errors = append(errors, texts.ValidateAll(dataDir)...)
	if len(errors) > 0 {
		fmt.Println("Validation errors found:")
		fmt.Println()
		for _, err := range errors {
			fmt.Printf("  %s\n", err)
		}
		os.Exit(1)
	}
	fmt.Println("All data files valid.")
}

func cmdAudit(dataDir string) {
	r, err := audit.Run(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	audit.Print(r, os.Stdout)
}

// composeHour builds the calendar and engine for the given date and composes the named hour.
func composeHour(dataDir, hourName string, date time.Time) (*models.OfficeHour, error) {
	year := date.Year()
	moveable := calendar.ComputeMoveableDates(year)

	days, err := calendar.BuildCalendar(year, dataDir)
	if err != nil {
		return nil, fmt.Errorf("building calendar: %w", err)
	}

	dayIndex := date.YearDay() - 1
	if dayIndex < 0 || dayIndex >= len(days) {
		return nil, fmt.Errorf("date out of range")
	}

	engine, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}

	return engine.ComposeHour(hourName, &days[dayIndex], moveable)
}

func cmdHour(dataDir, hourName, displayName string) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: office %s YYYY-MM-DD\n", hourName)
		os.Exit(1)
	}

	date, err := time.Parse("2006-01-02", os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid date (use YYYY-MM-DD): %s\n", os.Args[2])
		os.Exit(1)
	}

	hour, err := composeHour(dataDir, hourName, date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error composing %s: %v\n", displayName, err)
		os.Exit(1)
	}

	fmt.Print(output.FormatOfficeHour(hour))
}

func cmdTeX(dataDir string) {
	const usage = "Usage: office tex [--chant] HOUR [YYYY-MM-DD]"

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		fmt.Fprintln(os.Stderr, "Example: office tex lauds 2026-03-11 > lauds.tex")
		fmt.Fprintln(os.Stderr, "         office tex --chant compline > compline.tex")
		os.Exit(1)
	}

	// Parse optional --chant flag; collect remaining positional args.
	chant := false
	var args []string
	for _, a := range os.Args[2:] {
		if a == "--chant" {
			chant = true
		} else {
			args = append(args, a)
		}
	}

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	hourName := args[0]
	date := time.Now()
	if len(args) >= 2 {
		var err error
		date, err = time.Parse("2006-01-02", args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid date (use YYYY-MM-DD): %s\n", args[1])
			os.Exit(1)
		}
	}

	hour, err := composeHour(dataDir, hourName, date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error composing %s: %v\n", hourName, err)
		os.Exit(1)
	}

	fmt.Print(output.FormatOfficeHourTeX(hour, dataDir, chant))
}

func cmdReview(dataDir string) {
	const usage = `Usage: office review <subcommand> [args]

Subcommands:
  manifest [-start YEAR] [-years N] [-base URL]
                                         Print the review-unit checklist as CSV
  status   [-start YEAR] [-years N]      Report coverage vs data/review/signoffs.txt
  sign HASH REVIEWER [note...]           Record a sign-off for the unit with HASH`

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	parseSweepFlags := func(name string, args []string) (start, years int, base string) {
		fs := flag.NewFlagSet("review "+name, flag.ExitOnError)
		fs.IntVar(&start, "start", time.Now().Year(), "first calendar year of the sweep")
		fs.IntVar(&years, "years", 1, "number of calendar years to sweep")
		fs.StringVar(&base, "base", review.DefaultBaseURL, "base URL prefixed to checklist links")
		fs.Parse(args)
		return start, years, base
	}

	buildManifest := func(start, years int) *review.Manifest {
		m, err := review.BuildManifest(dataDir, start, years)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building review manifest: %v\n", err)
			os.Exit(1)
		}
		return m
	}

	switch os.Args[2] {
	case "manifest":
		start, years, base := parseSweepFlags("manifest", os.Args[3:])
		m := buildManifest(start, years)
		if err := review.WriteCSV(m, os.Stdout, base); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing manifest: %v\n", err)
			os.Exit(1)
		}

	case "status":
		start, years, _ := parseSweepFlags("status", os.Args[3:])
		m := buildManifest(start, years)
		signoffs, err := review.LoadSignoffs(dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading sign-offs: %v\n", err)
			os.Exit(1)
		}
		review.PrintStatus(review.Classify(m, signoffs), os.Stdout)

	case "sign":
		if len(os.Args) < 5 {
			fmt.Fprintln(os.Stderr, "Usage: office review sign HASH REVIEWER [note...]")
			os.Exit(1)
		}
		hash, reviewer := os.Args[3], os.Args[4]
		note := strings.Join(os.Args[5:], " ")
		m := buildManifest(time.Now().Year(), 3)
		var unit *review.Unit
		for i := range m.Units {
			if strings.HasPrefix(m.Units[i].Hash, hash) {
				if unit != nil {
					fmt.Fprintf(os.Stderr, "Hash prefix %q is ambiguous\n", hash)
					os.Exit(1)
				}
				unit = &m.Units[i]
			}
		}
		if unit == nil {
			fmt.Fprintf(os.Stderr, "No review unit with hash %q in the current sweep\n", hash)
			os.Exit(1)
		}
		s := review.Signoff{
			Hash:     unit.Hash,
			Hour:     unit.Hour,
			UnitKey:  unit.UnitKey,
			Reviewer: reviewer,
			Date:     time.Now().Format("2006-01-02"),
			Note:     note,
		}
		if err := review.AppendSignoff(dataDir, s); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing sign-off: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Signed off: %s %s (%s) by %s\n", unit.Hour, unit.Name, unit.Hash, reviewer)

	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
}

func cmdServe(dataDir string) {
	addr := ":8080"
	if len(os.Args) >= 3 {
		addr = os.Args[2]
	}

	srv, err := web.New(dataDir, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Listening on http://localhost%s\n", addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func findDataDir() string {
	// Look for data/ relative to the executable, then relative to cwd
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		// Try alongside the executable
		candidate := filepath.Join(dir, "data")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		// Try two levels up (for cmd/server/main.go -> project root)
		candidate = filepath.Join(dir, "..", "..", "data")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	// Try relative to cwd
	if info, err := os.Stat("data"); err == nil && info.IsDir() {
		return "data"
	}

	fmt.Fprintln(os.Stderr, "Cannot find data directory")
	os.Exit(1)
	return ""
}
