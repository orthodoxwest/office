package office

import (
	"testing"

	"github.com/orthodoxwest/office/internal/texts"
)

func TestIsValidCondition(t *testing.T) {
	tests := []struct {
		condition string
		want      bool
	}{
		// Named conditions
		{"if-preces", true},
		{"festal-lauds-psalmody", true},

		// feast- prefix
		{"feast-easter-sunday", true},
		{"feast-", false}, // empty ID

		// weekday- prefix
		{"weekday-sunday", true},
		{"weekday-monday", true},
		{"weekday-tuesday", true},
		{"weekday-wednesday", true},
		{"weekday-thursday", true},
		{"weekday-friday", true},
		{"weekday-saturday", true},
		{"weekday-invalid", false},
		{"weekday-", false},

		// season- prefix is validated against the modeled season names.
		{"season-lent", true},
		{"season-passiontide", true},
		{"season-lnt", false},
		{"season-", false},

		// not- negation
		{"not-if-preces", true},
		{"not-feast-easter-sunday", true},
		{"not-weekday-sunday", true},
		{"not-weekday-invalid", false},

		// AND (comma-separated)
		{"weekday-sunday,if-preces", true},
		{"feast-christmas,weekday-sunday", true},
		{"not-weekday-sunday,if-preces", true},
		{"weekday-sunday,weekday-invalid", false}, // one part invalid
		{"weekday-invalid,if-preces", false},      // first part invalid

		// Unknown
		{"", false},
		{"unknown", false},
		{"not-unknown", false},
		{"weekday-sunday,", false},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			got := isValidCondition(tt.condition)
			if got != tt.want {
				t.Errorf("isValidCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestValidationHours(t *testing.T) {
	tests := []struct {
		name     string
		hour     string
		elemType string
		ref      string
		want     []string
	}{
		{name: "terce collect validates against lauds", hour: "terce", elemType: "proper-collect", ref: "collect", want: []string{"lauds"}},
		{name: "sext collect validates against lauds", hour: "sext", elemType: "proper-collect", ref: "collect", want: []string{"lauds"}},
		{name: "none collect validates against lauds", hour: "none", elemType: "proper-collect", ref: "collect", want: []string{"lauds"}},
		{name: "prime collect stays local", hour: "prime", elemType: "proper-collect", ref: "collect", want: []string{"prime"}},
		{name: "indexed antiphon stays local", hour: "lauds", elemType: "proper-antiphon", ref: "psalm-antiphon-2", want: []string{"lauds"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validationHours(tt.hour, tt.elemType, tt.ref)
			if len(got) != len(tt.want) {
				t.Fatalf("len(validationHours(...)) = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("validationHours(...)[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHasAnyKeySuffix(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-mark/psalm-antiphon": "Antiphon",
	})

	if !hasAnyKeySuffix(corpus, []string{"psalm-antiphon-2", "psalm-antiphon"}) {
		t.Fatal("expected generic suffix match to satisfy indexed candidate set")
	}
	if hasAnyKeySuffix(corpus, []string{"collect-2", "collect"}) {
		t.Fatal("unexpected match for unrelated refs")
	}
}
