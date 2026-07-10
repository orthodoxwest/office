package main

import (
	"encoding/json"
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
	case "rubrics":
		cmdRubrics(dataDir)
	case "validate":
		cmdValidate(dataDir)
	case "audit":
		cmdAudit(dataDir)
	case "lint":
		cmdLint(dataDir)
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
	moveable := calendar.ComputeMoveableDates(year)

	engine, err := office.NewEngine(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating office engine: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(output.FormatCalendar(days, engine, moveable))
}

// cmdRubrics prints a per-day TSV of composed rubric flags (preces, suffrage,
// commemorations) for cross-checking against a printed ordo.
func cmdRubrics(dataDir string) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: office rubrics YEAR")
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
	moveable := calendar.ComputeMoveableDates(year)

	engine, err := office.NewEngine(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating office engine: %v\n", err)
		os.Exit(1)
	}

	flags := func(hour *models.OfficeHour) (preces, suffrage bool, comms []string, gospelAnt string) {
		for _, sec := range hour.Sections {
			if strings.Contains(sec.Label, "Suffrage") {
				suffrage = true
			}
			for _, el := range sec.Elements {
				if el.Type == models.Preces {
					preces = true
				}
				if el.Type == models.Heading && strings.HasPrefix(el.Text, "Commemoration of ") {
					comms = append(comms, strings.TrimPrefix(el.Text, "Commemoration of "))
				}
				if gospelAnt == "" && (el.SlotRef == "benedictus-antiphon" || el.SlotRef == "magnificat-antiphon") {
					gospelAnt = strings.ReplaceAll(el.Text, "\n", " ")
				}
			}
		}
		return
	}

	fmt.Println("date\tcelebration\tlauds_preces\tlauds_suffrage\tlauds_comms\thours_preces\tvespers_preces\tvespers_suffrage\tvespers_comms\tbenedictus_ant\tmagnificat_ant")
	for i := range days {
		day := &days[i]
		celebration := "Feria"
		if day.Celebration != nil {
			celebration = day.Celebration.Name
		}

		row := []string{day.Date.Format("2006-01-02"), celebration}
		lauds, err := engine.ComposeHour("lauds", day, moveable)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error composing lauds for %s: %v\n", row[0], err)
			os.Exit(1)
		}
		lp, ls, lc, benAnt := flags(lauds)

		prime, err := engine.ComposeHour("prime", day, moveable)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error composing prime for %s: %v\n", row[0], err)
			os.Exit(1)
		}
		hp, _, _, _ := flags(prime)

		vespers, err := engine.ComposeHour("vespers", day, moveable)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error composing vespers for %s: %v\n", row[0], err)
			os.Exit(1)
		}
		vp, vs, vc, magAnt := flags(vespers)

		row = append(row,
			fmt.Sprintf("%t", lp), fmt.Sprintf("%t", ls), strings.Join(lc, "; "),
			fmt.Sprintf("%t", hp),
			fmt.Sprintf("%t", vp), fmt.Sprintf("%t", vs), strings.Join(vc, "; "),
			benAnt, magAnt,
		)
		fmt.Println(strings.Join(row, "\t"))
	}
}

func cmdValidate(dataDir string) {
	var errors []string
	errors = append(errors, calendar.ValidateAll(dataDir)...)
	errors = append(errors, office.ValidateHourDefinitions(dataDir)...)
	errors = append(errors, texts.ValidateAll(dataDir)...)
	if _, err := review.ScanProvenance(dataDir); err != nil {
		errors = append(errors, "review provenance: "+err.Error())
	}
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
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	year := fs.Int("year", time.Now().Year(), "calendar year for the composition sweep")
	fs.Parse(os.Args[2:])

	r, err := audit.Run(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	audit.Print(r, os.Stdout)

	sweep, err := audit.SweepYear(dataDir, *year)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running sweep: %v\n", err)
		os.Exit(1)
	}
	audit.PrintSweep(sweep, os.Stdout)
}

func cmdLint(dataDir string) {
	r, err := audit.Lint(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if audit.PrintLint(r, os.Stdout) {
		os.Exit(1)
	}
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
  provenance [-csv]                     Report structured corpus provenance
  provenance-queue [-start YEAR] [-years N] [-base URL] [-summary] [-include-verified]
                                         Rank atomic text review by dependency fan-out
  attest [flags] KEY REVIEWER              Record a source attestation for one text
  assurance [-markdown] [-update-baseline] Run release assurance gates and summary
  explain HOUR YYYY-MM-DD               Print a composition assurance manifest as JSON
  plan [-start YEAR] [-years N] [-base URL] [-summary] [-include-sources]
                                         Select a minimal coverage-oriented review set
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

	case "provenance":
		fs := flag.NewFlagSet("review provenance", flag.ExitOnError)
		csvOutput := fs.Bool("csv", false, "write the complete provenance inventory as CSV")
		fs.Parse(os.Args[3:])
		inventory, err := review.ScanProvenance(dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning provenance: %v\n", err)
			os.Exit(1)
		}
		if *csvOutput {
			if err := review.WriteProvenanceCSV(inventory, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing provenance CSV: %v\n", err)
				os.Exit(1)
			}
		} else {
			review.PrintProvenanceSummary(inventory, os.Stdout)
		}

	case "provenance-queue":
		fs := flag.NewFlagSet("review provenance-queue", flag.ExitOnError)
		start := fs.Int("start", time.Now().Year(), "first calendar year of the sweep")
		years := fs.Int("years", 1, "number of calendar years to sweep")
		base := fs.String("base", review.DefaultBaseURL, "base URL prefixed to representative links")
		summary := fs.Bool("summary", false, "print counts instead of the review queue CSV")
		includeVerified := fs.Bool("include-verified", false, "include already verified corpus entries")
		fs.Parse(os.Args[3:])
		queue, err := review.BuildProvenanceQueue(dataDir, *start, *years, *includeVerified)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building provenance queue: %v\n", err)
			os.Exit(1)
		}
		if *summary {
			review.PrintProvenanceQueueSummary(queue, os.Stdout)
		} else if err := review.WriteProvenanceQueueCSV(queue, os.Stdout, *base); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing provenance queue: %v\n", err)
			os.Exit(1)
		}

	case "attest":
		fs := flag.NewFlagSet("review attest", flag.ExitOnError)
		source := fs.String("source", "", "source title or edition (required)")
		locator := fs.String("locator", "", "source section or other locator")
		page := fs.String("page", "", "visible source page number")
		note := fs.String("note", "", "short verification note")
		reviewedOn := fs.String("date", time.Now().Format("2006-01-02"), "review date (YYYY-MM-DD)")
		replace := fs.Bool("replace", false, "replace an existing attestation for this key")
		fs.Parse(os.Args[3:])
		args := fs.Args()
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: office review attest --source SOURCE [--page PAGE|--locator LOCATOR] [--note NOTE] [--date YYYY-MM-DD] [--replace] KEY REVIEWER")
			os.Exit(1)
		}
		entry, err := review.RecordAttestation(dataDir, review.AttestOptions{
			Key: args[0], Reviewer: args[1], Source: *source,
			Locator: *locator, Page: *page, ReviewedOn: *reviewedOn, Notes: *note, Replace: *replace,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error recording attestation: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Verified %s (%s, %s)\n", entry.Key, entry.Reviewer, entry.ReviewedOn)

	case "assurance":
		fs := flag.NewFlagSet("review assurance", flag.ExitOnError)
		markdown := fs.Bool("markdown", false, "write a Markdown CI/release summary")
		updateBaseline := fs.Bool("update-baseline", false, "set the reviewable coverage floor to current counts")
		fs.Parse(os.Args[3:])
		baseline, err := review.LoadAssuranceBaseline(dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading assurance baseline: %v\n", err)
			os.Exit(1)
		}
		report, err := review.BuildAssuranceReport(dataDir, baseline.StartYear, baseline.Years)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building assurance report: %v\n", err)
			os.Exit(1)
		}
		if *updateBaseline {
			if err := review.UpdateAssuranceBaseline(dataDir, report); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating assurance baseline: %v\n", err)
				os.Exit(1)
			}
			baseline.VerifiedMinimum = report.Verified
			baseline.ModeledFeaturesMinimum = report.ModeledFeatures
		}
		failures := review.EvaluateAssurance(report, baseline)
		review.WriteAssuranceSummary(report, failures, os.Stdout, *markdown)
		if len(failures) > 0 {
			os.Exit(1)
		}

	case "explain":
		if len(os.Args) != 5 {
			fmt.Fprintln(os.Stderr, "Usage: office review explain HOUR YYYY-MM-DD")
			os.Exit(1)
		}
		date, err := time.Parse("2006-01-02", os.Args[4])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid date: %s\n", os.Args[4])
			os.Exit(1)
		}
		assurance, err := review.ExplainComposition(dataDir, os.Args[3], date)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error explaining composition: %v\n", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(assurance); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing assurance JSON: %v\n", err)
			os.Exit(1)
		}

	case "plan":
		fs := flag.NewFlagSet("review plan", flag.ExitOnError)
		start := fs.Int("start", time.Now().Year(), "first calendar year of the sweep")
		years := fs.Int("years", 1, "number of calendar years to sweep")
		base := fs.String("base", review.DefaultBaseURL, "base URL prefixed to checklist links")
		summary := fs.Bool("summary", false, "print counts instead of the selected-page CSV")
		includeSources := fs.Bool("include-sources", false, "also cover every rendered corpus key; text provenance is separate by default")
		fs.Parse(os.Args[3:])
		plan, err := review.BuildReviewPlan(dataDir, *start, *years, *includeSources)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building review plan: %v\n", err)
			os.Exit(1)
		}
		if *summary {
			review.PrintReviewPlanSummary(plan, os.Stdout)
		} else if err := review.WriteReviewPlanCSV(plan, os.Stdout, *base); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing review plan: %v\n", err)
			os.Exit(1)
		}

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
