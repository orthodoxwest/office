package cli

import (
	"fmt"
	"strings"

	"github.com/orthodoxwest/office/internal/scaffold"
)

// cmdScaffold dispatches scaffold subcommands.
func cmdScaffold(e env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: office scaffold <propers> [flags]\n\nSubcommands:\n  propers   ensure proper text files exist with commented key catalogs")
	}
	switch args[0] {
	case "propers":
		return cmdScaffoldPropers(e, args[1:])
	default:
		return fmt.Errorf("unknown scaffold subcommand %q\nusage: office scaffold propers [flags]", args[0])
	}
}

// cmdScaffoldPropers creates missing proper files and appends missing
// commented keys to sparse existing files without overwriting live sections.
func cmdScaffoldPropers(e env, args []string) error {
	fs := e.newFlagSet("scaffold propers")
	check := fs.Bool("check", false, "do not write; exit non-zero if any file would be created or extended")
	dryRun := fs.Bool("dry-run", false, "print the plan without writing files")
	includeComms := fs.Bool("include-commemorations", false, "also scaffold rank=commemoration feasts (thin key set)")
	feastID := fs.String("feast", "", "limit to a single feast id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("usage: office scaffold propers [-check] [-dry-run] [-include-commemorations] [-feast ID]")
	}
	if *check && *dryRun {
		return fmt.Errorf("use only one of -check and -dry-run")
	}

	write := !*check && !*dryRun
	results, err := scaffold.EnsurePropers(e.dataDir, scaffold.Options{
		Write:                 write,
		IncludeCommemorations: *includeComms,
		FeastID:               *feastID,
	})
	if err != nil {
		return err
	}

	created, appended, ok, skipped := scaffold.Summarize(results)
	mode := "wrote"
	if *check {
		mode = "check"
	} else if *dryRun {
		mode = "dry-run"
	}

	// Detail lines for create/append only (skip noise of ok/skip unless -feast).
	for _, r := range results {
		switch r.Action {
		case scaffold.ActionCreate:
			fmt.Fprintf(e.out, "create  %s  (+%d keys)\n", r.Path, len(r.AddedKeys))
		case scaffold.ActionAppend:
			fmt.Fprintf(e.out, "append  %s  (+%d keys: %s)\n", r.Path, len(r.AddedKeys), strings.Join(r.AddedKeys, ", "))
		case scaffold.ActionSkip:
			if *feastID != "" {
				fmt.Fprintf(e.out, "skip    %s  (%s)\n", r.FeastID, r.SkipReason)
			}
		case scaffold.ActionOK:
			if *feastID != "" {
				fmt.Fprintf(e.out, "ok      %s\n", r.Path)
			}
		}
	}

	fmt.Fprintf(e.out, "\nscaffold propers (%s): create=%d append=%d ok=%d skip=%d\n",
		mode, created, appended, ok, skipped)

	if *check && scaffold.NeedsWork(results) {
		return errReported
	}
	return nil
}
