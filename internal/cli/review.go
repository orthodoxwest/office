package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/orthodoxwest/office/internal/review"
)

const reviewUsage = `Usage: office review <subcommand> [args]

Subcommands:
  manifest [-start YEAR] [-years N] [-base URL]
                                         Inventory distinct rendered compositions as CSV
  provenance [-csv] [-start YEAR] [-years N]
                                         Report structured corpus provenance, plus (unless -csv)
                                         verified % of that sweep's rendered text weighted by
                                         how often each entry is actually prayed
  provenance-queue [-start YEAR] [-years N] [-base URL] [-summary] [-include-verified] [-suspect-only]
                                         Rank atomic text review by dependency fan-out,
                                         suspect (pre-flagged) entries first
  zero-occurrences [-start YEAR] [-years N] [-summary]
                                         List unverified entries never selected in a sweep
  resolution-inventory [-start YEAR] [-years N] [-json] [-fallback-only] [-summary]
                                         List effective dynamic-proper resolutions and fallbacks
  repair-queue [-year YEAR] [-resources DIR] [-discovery FILE] [-json|-markdown|-summary]
                                         Generate source-backed repair candidates and checks
  attest [flags] KEY REVIEWER            Record a source attestation for one text
  flag [flags] KEY                       Record a prescreen suspicion for one text
  assurance [-markdown] [-update-baseline] Check text-provenance floor and print summary
  explain HOUR YYYY-MM-DD               Print a composition assurance manifest as JSON
  plan [-start YEAR] [-years N] [-base URL] [-summary] [-include-sources]
                                         Sample observed engine behavior (default 28y); no completion score`

// cmdReview dispatches the review subcommands.
func cmdReview(e env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", reviewUsage)
	}

	name, rest := args[0], args[1:]
	switch name {
	case "manifest":
		return e.reviewManifest(rest)
	case "status", "sign":
		return fmt.Errorf("office review %s is retired; record source-backed composition checks in tests and issues (see REVIEWING.md)", name)
	case "provenance":
		return e.reviewProvenance(rest)
	case "provenance-queue":
		return e.reviewProvenanceQueue(rest)
	case "zero-occurrences":
		return e.reviewZeroOccurrences(rest)
	case "resolution-inventory":
		return e.reviewResolutionInventory(rest)
	case "repair-queue":
		return e.reviewRepairQueue(rest)
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

func (e env) reviewProvenance(args []string) error {
	fs := e.newFlagSet("review provenance")
	csvOutput := fs.Bool("csv", false, "write the complete provenance inventory as CSV")
	start := fs.Int("start", time.Now().Year(), "first calendar year of the usage-weighted sweep")
	years := fs.Int("years", 1, "number of calendar years to sweep for usage weighting")
	if err := fs.Parse(args); err != nil {
		return err
	}

	inventory, err := review.ScanProvenance(e.dataDir)
	if err != nil {
		return fmt.Errorf("scanning provenance: %w", err)
	}
	if *csvOutput {
		if err := review.WriteProvenanceCSV(inventory, e.out); err != nil {
			return fmt.Errorf("writing provenance CSV: %w", err)
		}
		return nil
	}

	review.PrintProvenanceSummary(inventory, e.out)

	// The flat count above weights every corpus entry equally, including
	// ones a year of prayer never touches. Sweep every hour of every day to
	// show what share of *actually rendered* text is verified, weighted by
	// how often each entry recurs — a daily psalm counts far more than a
	// once-a-year collect.
	weighted, err := review.BuildUsageWeightedProvenance(e.dataDir, *start, *years)
	if err != nil {
		return fmt.Errorf("building usage-weighted provenance: %w", err)
	}
	fmt.Fprintln(e.out)
	review.PrintUsageWeightedProvenance(weighted, e.out)
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
	}

	failures := review.EvaluateAssurance(report, baseline)
	review.WriteAssuranceSummary(report, failures, e.out, *markdown)
	if len(failures) > 0 {
		return errReported
	}
	return nil
}

func (e env) reviewExplain(args []string) error {
	args, form, err := takePrayerForm(args)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: office review explain HOUR YYYY-MM-DD [--form private|deacon|priest]")
	}
	date, err := parseDate(args[1])
	if err != nil {
		return err
	}

	assurance, err := review.ExplainComposition(e.dataDir, args[0], date, form)
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
	// Include examples of rare concurrence/octave edges absent from one year.
	years := fs.Int("years", 28, "number of calendar years to sweep")
	base := fs.String("base", review.DefaultBaseURL, "base URL prefixed to checklist links")
	summary := fs.Bool("summary", false, "print counts instead of the selected-page CSV")
	includeSources := fs.Bool("include-sources", false, "also sample every rendered corpus key; text provenance remains separate")
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
	// Keep the CSV machine-readable and describe its limited purpose on stderr.
	fmt.Fprintf(e.err, "composition samples: %d pages, %d observed features; not a rubric checklist or correctness measure\n", len(plan.Selected), len(plan.Features))
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
