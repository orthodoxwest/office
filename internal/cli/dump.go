package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/orthodoxwest/office/internal/dump"
	"github.com/orthodoxwest/office/internal/models"
)

const dumpUsage = `usage: office dump -start YEAR [-years N] [-hours LIST] [-forms LIST] [-groups LIST]
       office dump -dates YYYY-MM-DD,... [-hours LIST] [-forms LIST] [-groups LIST]
       office dump diff [-max-records N] [-max-paths N] LEFT RIGHT
       office dump digest [FILE|-]`

// cmdDump writes the canonical record stream the Go and Rust engines are
// compared on (RUST-PORT.md), or compares and fingerprints such streams.
func cmdDump(e env, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "diff":
			return cmdDumpDiff(e, args[1:])
		case "digest":
			return cmdDumpDigest(e, args[1:])
		}
	}

	fs := e.newFlagSet("dump")
	fs.Usage = func() { fmt.Fprintln(e.err, dumpUsage); fs.PrintDefaults() }
	start := fs.Int("start", 0, "first civil year of the window")
	years := fs.Int("years", 1, "number of civil years in the window")
	dates := fs.String("dates", "", "comma-separated dates instead of a year window")
	hours := fs.String("hours", "", "comma-separated hours (default: all)")
	forms := fs.String("forms", "", "comma-separated prayer forms (default: all)")
	groups := fs.String("groups", "", "comma-separated record groups: corpus, calendar, office, hours (default: all)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%s", dumpUsage)
	}

	sel := dump.Selection{
		Hours:  splitList(*hours),
		Groups: splitList(*groups),
	}
	for _, f := range splitList(*forms) {
		sel.Forms = append(sel.Forms, models.PrayerForm(f))
	}
	if *dates != "" {
		if *start != 0 {
			return fmt.Errorf("use -dates or -start, not both")
		}
		for _, value := range splitList(*dates) {
			date, err := parseDate(value)
			if err != nil {
				return err
			}
			sel.Dates = append(sel.Dates, date)
		}
	} else {
		sel.StartYear, sel.Years = *start, *years
	}

	gen, err := dump.NewGenerator(e.dataDir)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(e.out, 1<<20)
	err = gen.Generate(sel, func(r dump.Record) error {
		line, err := dump.Marshal(r)
		if err != nil {
			return fmt.Errorf("%s: %w", describeRecord(r), err)
		}
		w.Write(line)
		return w.WriteByte('\n')
	})
	if err != nil {
		return err
	}
	return w.Flush()
}

func describeRecord(r dump.Record) string {
	parts := []string{fmt.Sprint(r["kind"])}
	for _, k := range []string{"year", "date", "hour", "form"} {
		if v, ok := r[k]; ok {
			parts = append(parts, fmt.Sprint(v))
		}
	}
	return strings.Join(parts, " ")
}

func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func cmdDumpDiff(e env, args []string) error {
	fs := e.newFlagSet("dump diff")
	maxRecords := fs.Int("max-records", 20, "differing records to describe in full")
	maxPaths := fs.Int("max-paths", 8, "differing values to list per record")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: office dump diff [-max-records N] [-max-paths N] LEFT RIGHT")
	}
	left, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer left.Close()
	right, err := os.Open(fs.Arg(1))
	if err != nil {
		return err
	}
	defer right.Close()

	summary, err := dump.Diff(left, right, e.out, dump.DiffOptions{MaxRecords: *maxRecords, MaxPaths: *maxPaths})
	if err != nil {
		return err
	}
	if !summary.Clean() {
		return errReported
	}
	return nil
}

func cmdDumpDigest(e env, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: office dump digest [FILE|-]")
	}
	var in io.Reader = os.Stdin
	if len(args) == 1 && args[0] != "-" {
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	snapshot, err := dump.Digest(in)
	if err != nil {
		return err
	}
	return dump.WriteParitySnapshot(snapshot, e.out)
}
