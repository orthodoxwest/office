package dump

import (
	"errors"
	"strings"
	"testing"
)

func TestMarshalSortsKeysAndEscapesPerRFC8785(t *testing.T) {
	got, err := Marshal(Record{
		"z": int64(-7),
		"a": []any{true, false, nil, 0},
		// Only quote, backslash, and control characters are escaped; HTML
		// characters, U+2028, and liturgical symbols stay literal (Go's
		// encoding/json would escape <, >, &, and U+2028).
		"m": "q\" b\\ \b\f\n\r\t \x01\x1f\x7f <>& \u2028 ℣ ℟ ✠ ’",
		"e": Record{},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":[true,false,null,0],"e":{},"m":"q\" b\\ \b\f\n\r\t \u0001\u001f` + "\x7f" + ` <>& ` + "\u2028" + ` ℣ ℟ ✠ ’","z":-7}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestMarshalPrettyMatchesSerdeJSON(t *testing.T) {
	got, err := MarshalPretty(Record{
		"b": []any{1, Record{"y": nil, "x": "v"}},
		"a": []any{},
		"c": Record{},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "a": [],
  "b": [
    1,
    {
      "x": "v",
      "y": null
    }
  ],
  "c": {}
}
`
	if string(got) != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMarshalRejectsEmptyStringsWithTheirLocation(t *testing.T) {
	_, err := Marshal(Record{"sections": []any{Record{"label": ""}}})
	if !errors.Is(err, errEmptyString) {
		t.Fatalf("err = %v, want errEmptyString", err)
	}
	if !strings.HasPrefix(err.Error(), "/sections/0/label: ") {
		t.Fatalf("err = %q, want a JSON Pointer to the field", err)
	}
}

func TestMarshalRejectsValuesOutsideTheDomain(t *testing.T) {
	for name, v := range map[string]any{
		"invalid UTF-8": "\xff",
		"float":         1.5,
		"string slice":  []string{"a"},
	} {
		if _, err := Marshal(Record{"k": v}); err == nil {
			t.Errorf("%s: Marshal succeeded, want an error", name)
		}
	}
}

func TestPointerTokenEscapesPerRFC6901(t *testing.T) {
	if got := pointerToken("a/b~c"); got != "a~1b~0c" {
		t.Fatalf("pointerToken = %q", got)
	}
}

func TestParseCanonicalRejectsOtherEncodings(t *testing.T) {
	if _, err := ParseCanonical([]byte(`{"a":1,"b":"x"}`)); err != nil {
		t.Fatalf("canonical line rejected: %v", err)
	}
	for _, line := range []string{
		`{"b":1,"a":2}`,          // key order
		`{"a": 1}`,               // whitespace
		`{"a":"\u003c"}`,         // needless escape
		`{"a":1.0}`,              // not an integer
		`[1]`,                    // not an object
		`{"a":1} {"b":2}`,        // trailing data
		`{"a":"x","a":"y"}`,      // duplicate key
		`{"a":"` + "\xff" + `"}`, // invalid UTF-8
	} {
		if _, err := ParseCanonical([]byte(line)); err == nil {
			t.Errorf("ParseCanonical(%s) succeeded, want an error", line)
		}
	}
}
