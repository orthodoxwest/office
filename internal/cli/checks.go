package cli

import (
	"fmt"
	"time"

	"github.com/orthodoxwest/office/internal/audit"
	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/review"
	"github.com/orthodoxwest/office/internal/texts"
)

// cmdValidate runs every data-file validator. It exits non-zero when any
// validator reports an error, so it can gate a build.
func cmdValidate(e env, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: office validate")
	}

	var errs []string
	errs = append(errs, calendar.ValidateAll(e.dataDir)...)
	errs = append(errs, office.ValidateHourDefinitions(e.dataDir)...)
	errs = append(errs, texts.ValidateAll(e.dataDir)...)
	if inventory, err := review.ScanProvenance(e.dataDir); err != nil {
		errs = append(errs, "review provenance: "+err.Error())
	} else if _, err := review.LoadZeroClassifications(e.dataDir, inventory); err != nil {
		errs = append(errs, "review zero occurrences: "+err.Error())
	}

	if len(errs) > 0 {
		fmt.Fprintln(e.out, "Validation errors found:")
		fmt.Fprintln(e.out)
		for _, err := range errs {
			fmt.Fprintf(e.out, "  %s\n", err)
		}
		return errReported
	}

	fmt.Fprintln(e.out, "All data files valid.")
	return nil
}

// cmdAudit reports placeholder texts and missing propers, then sweeps a year's
// compositions for not-found markers and ordinary fallbacks.
func cmdAudit(e env, args []string) error {
	fs := e.newFlagSet("audit")
	year := fs.Int("year", time.Now().Year(), "calendar year for the composition sweep")
	if err := fs.Parse(args); err != nil {
		return err
	}

	r, err := audit.Run(e.dataDir)
	if err != nil {
		return err
	}
	audit.Print(r, e.out)

	sweep, err := audit.SweepYear(e.dataDir, *year)
	if err != nil {
		return fmt.Errorf("running sweep: %w", err)
	}
	audit.PrintSweep(sweep, e.out)
	return nil
}

// cmdLint runs the text-corpus lints. Mechanical findings fail; advisory ones
// are printed for triage.
func cmdLint(e env, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: office lint")
	}

	r, err := audit.Lint(e.dataDir)
	if err != nil {
		return err
	}
	if audit.PrintLint(r, e.out) {
		return errReported
	}
	return nil
}
