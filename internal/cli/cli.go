// Package cli implements the office command-line interface.
//
// The commands live here rather than in package main so they can be tested:
// each one writes to an injected io.Writer and returns an error instead of
// calling os.Exit, and cmd/server is a thin shim over Run. Several of these
// commands are load-bearing outside the app — `rubrics` and `ordo` feed the
// ordo cross-check tooling in scripts/ — so their output is worth pinning.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// env carries what every command needs: where the data lives and where its
// output goes. out is the command's result (redirect it to a file and you get
// the ordo, the TSV, the CSV); err is diagnostics, usage, and progress.
type env struct {
	dataDir string
	out     io.Writer
	err     io.Writer
}

// errReported marks a command that finished normally but must fail the build:
// validate found errors, the linter found mechanical findings, an assurance
// gate did not hold. The findings are already on out, so Run exits non-zero
// without adding a message.
var errReported = errors.New("findings reported")

const usage = `Usage: office <command> [args]
Commands: ordo, rubrics, validate, audit, lint, review, scaffold, lauds, prime, terce, sext, none, vespers, compline, tex, serve`

// Run executes one command. args excludes the program name. It returns the
// process exit status: 0 on success, 1 on any failure.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 1
	}

	name, rest := args[0], args[1:]
	command, ok := lookup(name)
	if !ok {
		fmt.Fprintf(stderr, "Unknown command: %s\n", name)
		return 1
	}

	// Resolved after the command name so a typo is reported as a typo, not as
	// a missing data directory.
	dataDir := FindDataDir()
	if dataDir == "" {
		fmt.Fprintln(stderr, "Cannot find data directory")
		return 1
	}

	err := command(env{dataDir: dataDir, out: stdout, err: stderr}, rest)

	switch {
	case err == nil:
		return 0
	case errors.Is(err, errReported), errors.Is(err, flag.ErrHelp):
		return 1
	default:
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
}

// lookup resolves a command name to its implementation.
func lookup(name string) (func(env, []string) error, bool) {
	switch name {
	case "ordo":
		return cmdOrdo, true
	case "rubrics":
		return cmdRubrics, true
	case "validate":
		return cmdValidate, true
	case "audit":
		return cmdAudit, true
	case "lint":
		return cmdLint, true
	case "lauds", "prime", "terce", "sext", "none", "vespers", "compline":
		return func(e env, args []string) error { return cmdHour(e, name, args) }, true
	case "tex":
		return cmdTeX, true
	case "review":
		return cmdReview, true
	case "scaffold":
		return cmdScaffold, true
	case "serve":
		return cmdServe, true
	default:
		return nil, false
	}
}

// newFlagSet builds a flag set that reports parse errors instead of exiting,
// so a bad flag surfaces as an error from the command.
func (e env) newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(e.err)
	return fs
}

// parseYear reads the single YEAR argument shared by ordo and rubrics.
func parseYear(args []string, command string) (int, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("usage: office %s YEAR", command)
	}
	year, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("invalid year: %s", args[0])
	}
	return year, nil
}

// parseDate reads a YYYY-MM-DD argument.
func parseDate(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date (use YYYY-MM-DD): %s", value)
	}
	return date, nil
}

// FindDataDir locates the data/ directory: alongside the executable, two
// levels up from it (a `go run ./cmd/server` build), then relative to the
// working directory. Returns "" when none of those exist.
func FindDataDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(dir, "data"),
			filepath.Join(dir, "..", "..", "data"),
		} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}

	if info, err := os.Stat("data"); err == nil && info.IsDir() {
		return "data"
	}

	return ""
}
