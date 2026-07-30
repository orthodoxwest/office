// Package scaffold creates and extends proper text files so editors can fill
// in corpus entries without memorising section keys.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

// Action is one ensure result for a feast proper file.
type Action string

const (
	ActionCreate Action = "create" // new scaffold file
	ActionAppend Action = "append" // added commented keys to an existing file
	ActionOK     Action = "ok"     // file already has every catalog key (live or commented)
	ActionSkip   Action = "skip"   // feast not eligible
)

// Result describes what EnsurePropers did (or would do) for one feast.
type Result struct {
	FeastID    string
	Path       string // relative to dataDir, e.g. texts/proper/st-ambrose.txt
	Action     Action
	AddedKeys  []string // keys created or appended as comments
	SkipReason string
}

// Options control which feasts are considered and whether files are written.
type Options struct {
	// Write applies create/append. When false, Results still report what
	// would change (dry-run / check).
	Write bool

	// IncludeCommemorations also scaffolds rank=commemoration feasts with
	// the thinner CommemorationKeys catalog. Default is false (matches make audit).
	IncludeCommemorations bool

	// FeastID, when non-empty, limits work to that single feast ID.
	FeastID string
}

var (
	liveSectionRE      = regexp.MustCompile(`^\[([a-z0-9-]+)\]\s*$`)
	commentedSectionRE = regexp.MustCompile(`^#\s*\[([a-z0-9-]+)\]\s*$`)
)

// EnsurePropers creates missing proper files and appends missing commented
// keys to sparse existing files. Live (uncommented) sections are never
// modified.
func EnsurePropers(dataDir string, opts Options) ([]Result, error) {
	feasts, err := calendar.LoadFeasts(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading feasts: %w", err)
	}

	properDir := filepath.Join(dataDir, "texts", "proper")
	if opts.Write {
		if err := os.MkdirAll(properDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating proper dir: %w", err)
		}
	}

	var results []Result
	for _, feast := range feasts {
		if opts.FeastID != "" && feast.ID != opts.FeastID {
			continue
		}
		r, err := ensureOne(dataDir, properDir, feast, opts)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}

	if opts.FeastID != "" && len(results) == 0 {
		return nil, fmt.Errorf("unknown feast id %q", opts.FeastID)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FeastID < results[j].FeastID
	})
	return results, nil
}

func ensureOne(dataDir, properDir string, feast *models.Feast, opts Options) (Result, error) {
	rel := filepath.ToSlash(filepath.Join("texts", "proper", feast.ID+".txt"))
	path := filepath.Join(properDir, feast.ID+".txt")
	r := Result{FeastID: feast.ID, Path: rel}

	if feast.Rank == models.Commemoration && !opts.IncludeCommemorations {
		r.Action = ActionSkip
		r.SkipReason = "commemoration (pass -include-commemorations to scaffold)"
		return r, nil
	}

	keys := KeysFor(feast.Rank == models.Commemoration)
	existing, err := readFileIfExists(path)
	if err != nil {
		return r, err
	}

	live, commented := parsePresentKeys(existing)
	missing := missingKeys(keys, live, commented)

	if existing == nil {
		// No file yet.
		if len(missing) == 0 {
			// Empty catalog edge case.
			r.Action = ActionOK
			return r, nil
		}
		body := renderNewFile(feast, keys)
		r.Action = ActionCreate
		r.AddedKeys = keyNames(keys)
		if opts.Write {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return r, fmt.Errorf("writing %s: %w", rel, err)
			}
		}
		return r, nil
	}

	if len(missing) == 0 {
		r.Action = ActionOK
		return r, nil
	}

	block := renderKeyBlocks(missing)
	// Append after a blank line; preserve original content exactly.
	updated := strings.TrimRight(*existing, "\n") + "\n\n" + block
	r.Action = ActionAppend
	r.AddedKeys = keyNames(missing)
	if opts.Write {
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return r, fmt.Errorf("writing %s: %w", rel, err)
		}
	}
	return r, nil
}

func readFileIfExists(path string) (*string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := string(b)
	return &s, nil
}

// parsePresentKeys returns live section names and commented scaffold names.
func parsePresentKeys(content *string) (live, commented map[string]bool) {
	live = make(map[string]bool)
	commented = make(map[string]bool)
	if content == nil {
		return live, commented
	}
	for _, line := range strings.Split(*content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if m := commentedSectionRE.FindStringSubmatch(trimmed); m != nil {
				commented[m[1]] = true
			}
			continue
		}
		if m := liveSectionRE.FindStringSubmatch(trimmed); m != nil {
			live[m[1]] = true
		}
	}
	return live, commented
}

func missingKeys(catalog []Key, live, commented map[string]bool) []Key {
	var out []Key
	for _, k := range catalog {
		if live[k.Key] || commented[k.Key] {
			continue
		}
		out = append(out, k)
	}
	return out
}

func keyNames(keys []Key) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.Key
	}
	return out
}

func renderNewFile(feast *models.Feast, keys []Key) string {
	var b strings.Builder
	b.WriteString("# Proper scaffold for [" + feast.ID + "]\n")
	b.WriteString("# Name: " + feast.Name + "\n")
	b.WriteString(fmt.Sprintf("# Rank: %s  Category: %s\n", feast.Rank, feast.Category))
	if feast.ProperID != "" && feast.ProperID != feast.ID {
		b.WriteString("# ProperID: " + feast.ProperID + " (looked up before this file)\n")
	}
	b.WriteString("#\n")
	b.WriteString("# Uncomment a section header and put text (or @use …) beneath it to activate.\n")
	b.WriteString("# Commented sections are not loaded. Do not leave an empty uncommented [key] —\n")
	b.WriteString("# that silences make audit without providing text.\n")
	if feast.Category != "" {
		b.WriteString("# Commons fallback: commons/" + string(feast.Category) + "/{ref}\n")
	}
	b.WriteString("# Intentional commons-only: add this feast id to data/audit-ok.txt\n")
	b.WriteString("#\n")

	// Group core then optional.
	var core, optional []Key
	for _, k := range keys {
		if k.Tier == "optional" {
			optional = append(optional, k)
		} else {
			core = append(core, k)
		}
	}
	if len(core) > 0 {
		b.WriteString("# --- core ---\n")
		b.WriteString("#\n")
		b.WriteString(renderKeyBlocks(core))
	}
	if len(optional) > 0 {
		if len(core) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("# --- optional ---\n")
		b.WriteString("#\n")
		b.WriteString(renderKeyBlocks(optional))
	}
	return b.String()
}

func renderKeyBlocks(keys []Key) string {
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("#\n")
		}
		b.WriteString("# [" + k.Key + "]\n")
		b.WriteString("# " + k.Blurb + "\n")
	}
	return b.String()
}

// NeedsWork reports whether any create or append actions are pending.
func NeedsWork(results []Result) bool {
	for _, r := range results {
		if r.Action == ActionCreate || r.Action == ActionAppend {
			return true
		}
	}
	return false
}

// Summarize returns counts by action for human-readable CLI output.
func Summarize(results []Result) (created, appended, ok, skipped int) {
	for _, r := range results {
		switch r.Action {
		case ActionCreate:
			created++
		case ActionAppend:
			appended++
		case ActionOK:
			ok++
		case ActionSkip:
			skipped++
		}
	}
	return
}
