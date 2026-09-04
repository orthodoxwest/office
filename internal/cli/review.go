package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/review"
)

const reviewUsage = `Usage: office review <subcommand> [args]

Subcommands:
  manifest [-start YEAR] [-years N] [-base URL]
                                         Print the review-unit checklist as CSV
  status   [-start YEAR] [-years N]      Report coverage vs data/review/signoffs.txt
  provenance [-csv]                     Report structured corpus provenance
  provenance-queue [-start YEAR] [-years N] [-base URL] [-summary] [-include-verified] [-suspect-only]
                                         Rank atomic text review by dependency fan-out,
                                         suspect (pre-flagged) entries first
  zero-occurrences [-start YEAR] [-years N] [-summary]
                                         List unverified entries never selected in a sweep
  resolution-inventory [-start YEAR] [-years N] [-json] [-fallback-only] [-summary]
                                         List effective dynamic-proper resolutions and fallbacks
  attest [flags] KEY REVIEWER            Record a source attestation for one text
  flag [flags] KEY                       Record a prescreen suspicion for one text
  assurance [-markdown] [-update-baseline] Run release assurance gates and summary
  explain HOUR YYYY-MM-DD               Print a composition assurance manifest as JSON
  plan [-start YEAR] [-years N] [-base URL] [-summary] [-include-sources]
                                         Fan-out-weighted structural plan (default 28y; credits sign-offs)
  sign HOUR YYYY-MM-DD REVIEWER [note...] Record a structural sign-off for one page`

// cmdReview dispatches the review subcommands.
func cmdReview(e env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", reviewUsage)
	}

	name, rest := args[0], args[1:]
	switch name {
	case "manifest":
		return e.reviewManifest(rest)
	case "status":
		return e.reviewStatus(rest)
	case "provenance":
		return e.reviewProvenance(rest)
	case "provenance-queue":
		return e.reviewProvenanceQueue(rest)
	case "zero-occurrences":
		return e.reviewZeroOccurrences(rest)
	case "resolution-inventory":
		return e.reviewResolutionInventory(rest)
	case "attest":
		return e.reviewAttest(rest)
	case "flag":
		return e.reviewFlag(rest)
	case "assurance":
		return e.reviewAssurance(rest)
	case "explain":
		return e.reviewExplain(rest)
	case "plan":
		return e.reviewPlan(rest)
	case "sign":
		return e.reviewSign(rest)
	default:
		return fmt.Errorf("%s", reviewUsage)
	}
}

// sweepFlags are the year-range flags the sweep-based subcommands share.
func (e env) sweepFlags(name string, args []string) (start, years int, base string, err error) {
	fs := e.newFlagSet("review " + name)
	fs.IntVar(&start, "start", time.Now().Year(), "first calendar year of the sweep")
	fs.IntVar(&years, "years", 1, "number of calendar years to sweep")
	fs.StringVar(&base, "base", review.DefaultBaseURL, "base URL prefixed to checklist links")
	err = fs.Parse(args)
	return start, years, base, err
}

func (e env) buildManifest(start, years int) (*review.Manifest, error) {
	m, err := review.BuildManifest(e.dataDir, start, years)
	if err != nil {
		return nil, fmt.Errorf("building review manifest: %w", err)
	}
	return m, nil
}

func (e env) reviewManifest(args []string) error {
	start, years, base, err := e.sweepFlags("manifest", args)
	if err != nil {
		return err
	}
	m, err := e.buildManifest(start, years)
	if err != nil {
		return err
	}
	if err := review.WriteCSV(m, e.out, base); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	return nil
}

func (e env) reviewStatus(args []string) error {
	start, years, _, err := e.sweepFlags("status", args)
	if err != nil {
		return err
	}
	m, err := e.buildManifest(start, years)
	if err != nil {
		return err
	}
	signoffs, err := review.LoadSignoffs(e.dataDir)
	if err != nil {
		return fmt.Errorf("loading sign-offs: %w", err)
	}
	review.PrintStatus(review.Classify(m, signoffs), e.out)
	return nil
}

func (e env) reviewProvenance(args []string) error {
	fs := e.newFlagSet("review provenance")
	csvOutput := fs.Bool("csv", false, "write the complete provenance inventory as CSV")
	if err := fs.Parse(args); err != nil {
		return err
	}

	inventory, err := review.ScanProvenance(e.dataDir)
	if err != nil {
		return fmt.Errorf("scanning provenance: %w", err)
	}
	if !*csvOutput {
		review.PrintProvenanceSummary(inventory, e.out)
		return nil
	}
	if err := review.WriteProvenanceCSV(inventory, e.out); err != nil {
		return fmt.Errorf("writing provenance CSV: %w", err)
	}
	return nil
}

func (e env) reviewProvenanceQueue(args []string) error {
	fs := e.newFlagSet("review provenance-queue")
	start := fs.Int("start", time.Now().Year(), "first calendar year of the sweep")
	years := fs.Int("years", 1, "number of calendar years to sweep")
	base := fs.String("base", review.DefaultBaseURL, "base URL prefixed to representative links")
	summary := fs.Bool("summary", false, "print counts instead of the review queue CSV")
	includeVerified := fs.Bool("include-verified", false, "include already verified corpus entries")
	suspectOnly := fs.Bool("suspect-only", false, "keep only entries with a prescreen flag or advisory lint")
	if err := fs.Parse(args); err != nil {
		return err
	}

	queue, err := review.BuildProvenanceQueue(e.dataDir, *start, *years, *includeVerified)
	if err != nil {
		return fmt.Errorf("building provenance queue: %w", err)
	}
	if *suspectOnly {
		queue.FilterSuspect()
	}
	if *summary {
		review.PrintProvenanceQueueSummary(queue, e.out)
		return nil
	}
	if err := review.WriteProvenanceQueueCSV(queue, e.out, *base); err != nil {
		return fmt.Errorf("writing provenance queue: %w", err)
	}
	return nil
}

func (e env) reviewZeroOccurrences(args []string) error {
	fs := e.newFlagSet("review zero-occurrences")
	start := fs.Int("start", time.Now().Year(), "first calendar year of the sweep")
	years := fs.Int("years", 1, "number of calendar years to sweep")
	summary := fs.Bool("summary", false, "print category and ledger counts instead of CSV")
	if err := fs.Parse(args); err != nil {
		return err
	}

	report, err := review.BuildZeroOccurrenceReport(e.dataDir, *start, *years)
	if err != nil {
		return fmt.Errorf("building zero-occurrence report: %w", err)
	}
	if *summary {
		review.PrintZeroOccurrenceSummary(report, e.out)
		return nil
	}
	if err := review.WriteZeroOccurrenceCSV(report, e.out); err != nil {
		return fmt.Errorf("writing zero-occurrence CSV: %w", err)
	}
	return nil
}

func (e env) reviewAttest(args []string) error {
	fs := e.newFlagSet("review attest")
	source := fs.String("source", "", "source title or edition (required)")
	locator := fs.String("locator", "", "source section or other locator")
	page := fs.String("page", "", "visible source page number")
	note := fs.String("note", "", "short verification note")
	reviewedOn := fs.String("date", time.Now().Format("2006-01-02"), "review date (YYYY-MM-DD)")
	replace := fs.Bool("replace", false, "replace an existing attestation for this key")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) != 2 {
		return fmt.Errorf("usage: office review attest --source SOURCE [--page PAGE|--locator LOCATOR] [--note NOTE] [--date YYYY-MM-DD] [--replace] KEY REVIEWER")
	}

	entry, err := review.RecordAttestation(e.dataDir, review.AttestOptions{
		Key: rest[0], Reviewer: rest[1], Source: *source,
		Locator: *locator, Page: *page, ReviewedOn: *reviewedOn, Notes: *note, Replace: *replace,
	})
	if err != nil {
		return fmt.Errorf("recording attestation: %w", err)
	}
	fmt.Fprintf(e.out, "Verified %s (%s, %s)\n", entry.Key, entry.Reviewer, entry.ReviewedOn)
	return nil
}

func (e env) reviewFlag(args []string) error {
	fs := e.newFlagSet("review flag")
	severity := fs.String("severity", "high", "how likely the text is wrong: high or medium")
	reason := fs.String("reason", "", "short statement of the suspected defect (required)")
	flagged := fs.String("flagged", time.Now().Format("2006-01"), "prescreen batch identifier")
	issue := fs.String("issue", "", "related GitHub issue number")
	replace := fs.Bool("replace", false, "replace an existing flag for this key")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) != 1 || *reason == "" {
		return fmt.Errorf("usage: office review flag --reason REASON [--severity high|medium] [--flagged YYYY-MM] [--issue N] [--replace] KEY")
	}

	recorded, err := review.RecordPrescreenFlag(e.dataDir, review.PrescreenFlag{
		Key: rest[0], Severity: review.PrescreenSeverity(*severity),
		Reason: *reason, Flagged: *flagged, Issue: *issue,
	}, *replace)
	if err != nil {
		return fmt.Errorf("recording prescreen flag: %w", err)
	}
	fmt.Fprintf(e.out, "Flagged %s (%s): %s\n", recorded.Key, recorded.Severity, recorded.Reason)
	return nil
}

func (e env) reviewAssurance(args []string) error {
	fs := e.newFlagSet("review assurance")
	markdown := fs.Bool("markdown", false, "write a Markdown CI/release summary")
	updateBaseline := fs.Bool("update-baseline", false, "set the reviewable coverage floor to current counts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	baseline, err := review.LoadAssuranceBaseline(e.dataDir)
	if err != nil {
		return fmt.Errorf("loading assurance baseline: %w", err)
	}
	report, err := review.BuildAssuranceReport(e.dataDir, baseline.StartYear, baseline.Years)
	if err != nil {
		return fmt.Errorf("building assurance report: %w", err)
	}
	if *updateBaseline {
		if err := review.UpdateAssuranceBaseline(e.dataDir, report); err != nil {
			return fmt.Errorf("updating assurance baseline: %w", err)
		}
		baseline.VerifiedMinimum = report.Verified
		baseline.ModeledFeaturesMinimum = report.ModeledFeatures
	}

	failures := review.EvaluateAssurance(report, baseline)
	review.WriteAssuranceSummary(report, failures, e.out, *markdown)
	if len(failures) > 0 {
		return errReported
	}
	return nil
}

func (e env) reviewExplain(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: office review explain HOUR YYYY-MM-DD")
	}
	date, err := parseDate(args[1])
	if err != nil {
		return err
	}

	assurance, err := review.ExplainComposition(e.dataDir, args[0], date)
	if err != nil {
		return fmt.Errorf("explaining composition: %w", err)
	}
	enc := json.NewEncoder(e.out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(assurance); err != nil {
		return fmt.Errorf("writing assurance JSON: %w", err)
	}
	return nil
}

func (e env) reviewPlan(args []string) error {
	fs := e.newFlagSet("review plan")
	start := fs.Int("start", time.Now().Year(), "first calendar year of the sweep")
	// Default 28 years matches data/review/assurance-baseline.json so fan-out
	// ranks include rare concurrence/octave edges that miss a single year.
	years := fs.Int("years", 28, "number of calendar years to sweep")
	base := fs.String("base", review.DefaultBaseURL, "base URL prefixed to checklist links")
	summary := fs.Bool("summary", false, "print counts instead of the selected-page CSV")
	includeSources := fs.Bool("include-sources", false, "also cover every rendered corpus key; text provenance is separate by default")
	if err := fs.Parse(args); err != nil {
		return err
	}

	plan, err := review.BuildReviewPlan(e.dataDir, *start, *years, *includeSources)
	if err != nil {
		return fmt.Errorf("building review plan: %w", err)
	}
	if *summary {
		review.PrintReviewPlanSummary(plan, e.out)
		return nil
	}
	if err := review.WriteReviewPlanCSV(plan, e.out, *base); err != nil {
		return fmt.Errorf("writing review plan: %w", err)
	}
	// The CSV is the output; the residual summary goes to stderr so
	// `make review-plan > plan.csv` keeps the spreadsheet clean.
	fmt.Fprintf(e.err, "structural plan: residual %d pages (full cover %d); features %d (%d credited); residual impact %d; uncovered %d\n",
		len(plan.Selected), plan.FullCoverPages, plan.FeatureCount, plan.CreditedCount, plan.RemainingImpact, len(plan.Uncovered))
	return nil
}

func (e env) reviewSign(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: office review sign HOUR YYYY-MM-DD REVIEWER [note...]")
	}
	hourName, reviewer := args[0], args[2]
	date, err := parseDate(args[1])
	if err != nil {
		return err
	}
	note := strings.Join(args[3:], " ")

	s, unit, err := review.SignoffForPage(e.dataDir, hourName, date, reviewer, note)
	if err != nil {
		return fmt.Errorf("resolving reviewed page: %w", err)
	}
	if err := review.AppendSignoff(e.dataDir, *s); err != nil {
		return fmt.Errorf("writing sign-off: %w", err)
	}
	fmt.Fprintf(e.out, "Signed off: %s %s on %s by %s\n", unit.Hour, unit.Name, date.Format("2006-01-02"), reviewer)
	return nil
}

func (e env) reviewResolutionInventory(args []string) error {
	fs := e.newFlagSet("review resolution-inventory")
	start := fs.Int("start", time.Now().Year(), "first calendar year of the sweep")
	years := fs.Int("years", 28, "number of calendar years to sweep")
	jsonOutput := fs.Bool("json", false, "write the machine-readable inventory JSON")
	fallbackOnly := fs.Bool("fallback-only", false, "keep rows not resolved by the owning proper")
	summary := fs.Bool("summary", false, "print fallback-tier counts instead of rows")
	if err := fs.Parse(args); err != nil {
		return err
	}
	inventory, err := review.BuildResolutionInventory(e.dataDir, *start, *years)
	if err != nil {
		return fmt.Errorf("building resolution inventory: %w", err)
	}
	if *fallbackOnly {
		inventory.FilterFallbacks()
	}
	if *jsonOutput {
		enc := json.NewEncoder(e.out)
		enc.SetIndent("", "  ")
		return enc.Encode(inventory)
	}
	if *summary {
		fmt.Fprintf(e.out, "resolution inventory: %d rows\n", len(inventory.Rows))
		for _, tier := range []string{"proper", "proper-inherited", "temporal-week", "special", "common", "seasonal", "ordinary-weekday", "ordinary", "shared", "not-found"} {
			if n := review.ResolutionInventorySummary(inventory)[tier]; n != 0 {
				fmt.Fprintf(e.out, "%s: %d\n", tier, n)
			}
		}
		return nil
	}
	for _, row := range inventory.Rows {
		fmt.Fprintf(e.out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.Date, row.OwnerID, row.Hour, row.SlotRef, row.SelectedTier, row.SelectedRef, row.Reason)
	}
	return nil
}
