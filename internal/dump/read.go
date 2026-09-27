package dump

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Reader reads a dump one line at a time. Lines can be far longer than
// bufio.Scanner's default limit, so it reads with ReadBytes.
type Reader struct {
	r    *bufio.Reader
	line int
}

// NewReader wraps r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReaderSize(r, 1<<20)}
}

// Line is the number of the line most recently returned by Next.
func (r *Reader) Line() int { return r.line }

// Next returns the next line without its newline, or io.EOF.
func (r *Reader) Next() ([]byte, error) {
	b, err := r.r.ReadBytes('\n')
	if len(b) == 0 && err != nil {
		return nil, err
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	r.line++
	return bytes.TrimSuffix(b, []byte{'\n'}), nil
}

// Parse decodes one dump line into a Record, with integers as int64.
func Parse(line []byte) (Record, error) {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("trailing data after the record")
	}
	rec, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("record is not a JSON object")
	}
	normalized, err := normalizeNumbers(rec)
	if err != nil {
		return nil, err
	}
	return normalized.(map[string]any), nil
}

func normalizeNumbers(v any) (any, error) {
	switch x := v.(type) {
	case json.Number:
		n, err := strconv.ParseInt(string(x), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("number %s is not an integer", x)
		}
		return n, nil
	case []any:
		for i, e := range x {
			n, err := normalizeNumbers(e)
			if err != nil {
				return nil, err
			}
			x[i] = n
		}
	case map[string]any:
		for k, e := range x {
			n, err := normalizeNumbers(e)
			if err != nil {
				return nil, err
			}
			x[k] = n
		}
	}
	return v, nil
}

// ParseCanonical decodes line and confirms it is the canonical encoding of
// the record it holds, so a dump from another implementation cannot pass on
// encoding differences that happen to decode alike.
func ParseCanonical(line []byte) (Record, error) {
	rec, err := Parse(line)
	if err != nil {
		return nil, err
	}
	canonical, err := Marshal(rec)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, line) {
		return nil, fmt.Errorf("not canonically encoded (first difference at byte %d)", firstDifference(canonical, line))
	}
	return rec, nil
}

func firstDifference(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// Digest reads a dump and returns its parity snapshot.
func Digest(r io.Reader) (*ParitySnapshot, error) {
	reader := NewReader(r)
	d := NewDigester()
	for {
		line, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return d.Snapshot(), nil
		}
		if err != nil {
			return nil, err
		}
		rec, err := ParseCanonical(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", reader.Line(), err)
		}
		if err := d.Add(rec); err != nil {
			return nil, fmt.Errorf("line %d: %w", reader.Line(), err)
		}
	}
}
