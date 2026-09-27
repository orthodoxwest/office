package dump

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"unicode/utf8"
)

// DiffOptions bounds how much Diff prints.
type DiffOptions struct {
	MaxRecords int // differing records described in full; the rest are only counted
	MaxPaths   int // differing values listed per record
}

// DiffSummary counts what Diff found.
type DiffSummary struct {
	Records   int            // records compared
	Differing int            // records whose values differ
	ByKind    map[string]int // differing records per kind
	Diverged  bool           // the streams stopped lining up; later records were not compared
}

// Clean reports whether the two dumps matched completely.
func (s DiffSummary) Clean() bool { return s.Differing == 0 && !s.Diverged }

// Diff compares two dumps record by record. Both must list records in the
// canonical order, so a missing or extra record ends the comparison: past
// that point every pair would differ for the same reason.
func Diff(left, right io.Reader, out io.Writer, opts DiffOptions) (DiffSummary, error) {
	if opts.MaxRecords <= 0 {
		opts.MaxRecords = 20
	}
	if opts.MaxPaths <= 0 {
		opts.MaxPaths = 8
	}
	summary := DiffSummary{ByKind: map[string]int{}}
	lr, rr := NewReader(left), NewReader(right)
	for {
		l, lerr := lr.Next()
		r, rerr := rr.Next()
		if lerr != nil && !errors.Is(lerr, io.EOF) {
			return summary, fmt.Errorf("left: %w", lerr)
		}
		if rerr != nil && !errors.Is(rerr, io.EOF) {
			return summary, fmt.Errorf("right: %w", rerr)
		}
		lend, rend := errors.Is(lerr, io.EOF), errors.Is(rerr, io.EOF)
		if lend && rend {
			break
		}
		line := max(lr.Line(), rr.Line())
		if lend || rend {
			summary.Diverged = true
			side, next := "left", r
			if rend {
				side, next = "right", l
			}
			fmt.Fprintf(out, "line %d: %s dump ended; the other continues with %s\n", line, side, describeLine(next))
			break
		}
		summary.Records++
		if bytes.Equal(l, r) {
			continue
		}
		lrec, err := Parse(l)
		if err != nil {
			return summary, fmt.Errorf("left line %d: %w", line, err)
		}
		rrec, err := Parse(r)
		if err != nil {
			return summary, fmt.Errorf("right line %d: %w", line, err)
		}
		lkey, rkey := recordKey(lrec), recordKey(rrec)
		if lkey != rkey {
			summary.Diverged = true
			fmt.Fprintf(out, "line %d: record sequence diverges: left has %s, right has %s\n", line, lkey, rkey)
			break
		}
		summary.Differing++
		kind, _ := lrec["kind"].(string)
		summary.ByKind[kind]++
		if summary.Differing > opts.MaxRecords {
			continue
		}
		diffs := diffValues("", lrec, rrec, nil)
		if len(diffs) == 0 {
			fmt.Fprintf(out, "line %d: %s: same values, different encoding (%s)\n", line, lkey, nonCanonicalSide(lrec, l, r))
			continue
		}
		fmt.Fprintf(out, "line %d: %s\n", line, lkey)
		for i, d := range diffs {
			if i == opts.MaxPaths {
				fmt.Fprintf(out, "  … %d more\n", len(diffs)-i)
				break
			}
			fmt.Fprintf(out, "  %s\n    left:  %s\n    right: %s\n", d.path, d.left, d.right)
		}
	}
	if summary.Differing > opts.MaxRecords {
		fmt.Fprintf(out, "… %d more differing records not shown\n", summary.Differing-opts.MaxRecords)
	}
	kinds := make([]string, 0, len(summary.ByKind))
	for k := range summary.ByKind {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
	fmt.Fprintf(out, "%d records compared, %d differ", summary.Records, summary.Differing)
	for _, k := range kinds {
		fmt.Fprintf(out, ", %s %d", k, summary.ByKind[k])
	}
	if summary.Diverged {
		fmt.Fprint(out, "; comparison stopped where the streams diverged")
	}
	fmt.Fprintln(out)
	return summary, nil
}

// recordKey identifies a record by kind and position in the calendar.
func recordKey(r Record) string {
	kind, _ := r["kind"].(string)
	switch kind {
	case KindCalendarYear:
		y, _ := asInt(r["year"])
		return fmt.Sprintf("%s %d", kind, y)
	case KindCalendarDay, KindOfficeDay:
		return fmt.Sprintf("%s %v", kind, r["date"])
	case KindHour:
		return fmt.Sprintf("%s %v %v %v", kind, r["date"], r["hour"], r["form"])
	default:
		return kind
	}
}

func describeLine(line []byte) string {
	rec, err := Parse(line)
	if err != nil {
		return "an unparseable line"
	}
	return recordKey(rec)
}

func nonCanonicalSide(rec Record, l, r []byte) string {
	canonical, err := Marshal(rec)
	switch {
	case err != nil:
		return err.Error()
	case bytes.Equal(canonical, l):
		return "right is not canonical"
	case bytes.Equal(canonical, r):
		return "left is not canonical"
	default:
		return "neither side is canonical"
	}
}

type valueDiff struct {
	path, left, right string
}

// diffValues lists the JSON Pointers at which a and b differ.
func diffValues(path string, a, b any, out []valueDiff) []valueDiff {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			break
		}
		keys := make([]string, 0, len(x)+len(y))
		for k := range x {
			keys = append(keys, k)
		}
		for k := range y {
			if _, ok := x[k]; !ok {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		for _, k := range keys {
			child := path + "/" + pointerToken(k)
			av, aok := x[k]
			bv, bok := y[k]
			if !aok || !bok {
				out = append(out, valueDiff{child, presence(av, aok), presence(bv, bok)})
				continue
			}
			out = diffValues(child, av, bv, out)
		}
		return out
	case []any:
		y, ok := b.([]any)
		if !ok {
			break
		}
		for i := range min(len(x), len(y)) {
			out = diffValues(path+"/"+strconv.Itoa(i), x[i], y[i], out)
		}
		if len(x) != len(y) {
			out = append(out, valueDiff{path, fmt.Sprintf("%d items", len(x)), fmt.Sprintf("%d items", len(y))})
		}
		return out
	case string:
		if y, ok := b.(string); ok {
			if x != y {
				l, r := excerpt(x, y)
				out = append(out, valueDiff{path, l, r})
			}
			return out
		}
	default:
		if a == b {
			return out
		}
	}
	out = append(out, valueDiff{path, render(a), render(b)})
	return out
}

func presence(v any, ok bool) string {
	if !ok {
		return "(absent)"
	}
	return render(v)
}

func render(v any) string {
	b, err := Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	const limit = 160
	if len(b) > limit {
		return string(b[:limit]) + "…"
	}
	return string(b)
}

// excerpt shows two differing strings around their first difference.
func excerpt(a, b string) (string, string) {
	const before, after = 40, 80
	i := firstDifference([]byte(a), []byte(b))
	// The common prefix is identical, so a rune boundary in a is one in b.
	for i > 0 && i < len(a) && !utf8.RuneStart(a[i]) {
		i--
	}
	cut := func(s string) string {
		start := max(0, i-before)
		for start > 0 && !utf8.RuneStart(s[start]) {
			start--
		}
		end := min(len(s), i+after)
		for end < len(s) && !utf8.RuneStart(s[end]) {
			end++
		}
		out := strconv.Quote(s[start:end])
		if start > 0 {
			out = "…" + out
		}
		if end < len(s) {
			out += "…"
		}
		return out
	}
	return cut(a), cut(b)
}
