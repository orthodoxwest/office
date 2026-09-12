package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// The repair reporter shares the Python ordo comparator and triage loader.
// Run it from this data checkout and pass the current binary explicitly, so
// it cannot silently compare another checkout's engine or a saved report.
func (e env) reviewRepairQueue(args []string) error {
	fs := e.newFlagSet("review repair-queue")
	year := fs.Int("year", time.Now().Year(), "calendar year to assess")
	resources := fs.String("resources", "", "external resources directory (defaults to ../resources)")
	ledger := fs.String("ledger", "", "source-requirement ledger (defaults to data/review/repair-targets.json)")
	discovery := fs.String("discovery", "", "optional discovery results.jsonl; evidence only, never applied")
	output := fs.String("output", "", "artifact directory beneath this checkout's output/")
	scope := fs.String("scope", "all", "all, ordinary-year, or triduum")
	jsonOutput := fs.Bool("json", false, "print JSON instead of CSV")
	markdown := fs.Bool("markdown", false, "print the Markdown report instead of CSV")
	summary := fs.Bool("summary", false, "print only the queue summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("repair-queue takes flags, not positional arguments")
	}
	format, formats := "csv", 0
	for _, choice := range []struct {
		on   bool
		name string
	}{{*jsonOutput, "json"}, {*markdown, "markdown"}, {*summary, "summary"}} {
		if choice.on {
			format, formats = choice.name, formats+1
		}
	}
	if formats > 1 {
		return fmt.Errorf("choose only one of -json, -markdown, or -summary")
	}
	data, err := filepath.Abs(e.dataDir)
	if err != nil {
		return err
	}
	root := filepath.Dir(data)
	script := filepath.Join(root, "scripts", "repair-queue.py")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("repair-queue requires the repository's scripts/repair-queue.py: %w", err)
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	command := []string{script, "--office", binary, "--year", fmt.Sprint(*year), "--format", format, "--scope", *scope}
	for _, option := range []struct{ name, value string }{
		{"resources", *resources}, {"ledger", *ledger}, {"discovery", *discovery}, {"output", *output},
	} {
		if option.value != "" {
			// Resolve user paths before changing to the data checkout.
			path, err := filepath.Abs(option.value)
			if err != nil {
				return err
			}
			command = append(command, "--"+option.name, path)
		}
	}
	cmd := exec.Command("python3", command...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = root, e.out, e.err
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repair queue failed (requires Python 3 and pdftotext): %w", err)
	}
	return nil
}
