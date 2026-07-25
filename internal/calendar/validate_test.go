package calendar

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestValidateSemanticsRejectsGeneratedAndExplicitVigilOwnership(t *testing.T) {
	owner := &models.Feast{
		ID:       "st-example",
		Name:     "St. Example, Apostle",
		Rank:     models.Double2ndClass,
		Category: models.CategoryApostle,
		HasVigil: true,
	}
	explicit := &models.Feast{
		ID:       "comm-extra-vigil-of-st-example",
		Name:     "Our Example Vigil",
		Rank:     models.Commemoration,
		Category: models.CategoryFeria,
		IsVigil:  true,
		VigilOf:  owner.ID,
	}

	errs := validateSemantics([]*models.Feast{owner, explicit})
	if got := strings.Join(errs, "\n"); !strings.Contains(got, "defines the same observance") {
		t.Fatalf("validateSemantics errors = %q, want duplicate-vigil error", got)
	}
}

func TestValidateSemanticsAcceptsDistinctExplicitVigilOwner(t *testing.T) {
	generatedOwner := &models.Feast{ID: "st-john", HasVigil: true}
	explicitOwner := &models.Feast{ID: "st-john-baptist"}
	explicit := &models.Feast{
		ID:       "vigil-st-john-baptist",
		Name:     "Vigil of St John",
		Category: models.CategoryFeria,
		IsVigil:  true,
		VigilOf:  explicitOwner.ID,
	}

	errs := validateSemantics([]*models.Feast{generatedOwner, explicitOwner, explicit})
	if got := strings.Join(errs, "\n"); strings.Contains(got, "defines the same observance") {
		t.Fatalf("validateSemantics errors = %q, want distinct owner IDs accepted", got)
	}
}

func TestValidateSemanticsRejectsDuplicateExplicitVigils(t *testing.T) {
	owner := &models.Feast{ID: "st-james"}
	first := &models.Feast{
		ID: "vigil-st-james", Category: models.CategoryFeria,
		IsVigil: true, VigilOf: owner.ID,
	}
	second := &models.Feast{
		ID: "alternate-vigil-st-james", Category: models.CategoryFeria,
		IsVigil: true, VigilOf: owner.ID,
	}

	errs := validateSemantics([]*models.Feast{owner, first, second})
	if got := strings.Join(errs, "\n"); !strings.Contains(got, "define the same observance for 'st-james'") {
		t.Fatalf("validateSemantics errors = %q, want duplicate-explicit-vigil error", got)
	}
}

func TestValidateSemanticsRejectsVigilTarget(t *testing.T) {
	first := &models.Feast{
		ID: "first-vigil", Category: models.CategoryFeria,
		IsVigil: true, VigilOf: "second-vigil",
	}
	second := &models.Feast{
		ID: "second-vigil", Category: models.CategoryFeria,
		IsVigil: true, VigilOf: "first-vigil",
	}

	errs := validateSemantics([]*models.Feast{first, second})
	if got := strings.Join(errs, "\n"); !strings.Contains(got, "invalid vigil target") {
		t.Fatalf("validateSemantics errors = %q, want vigil-target error", got)
	}
}

func TestValidateSemanticsRejectsMalformedDateRule(t *testing.T) {
	feast := &models.Feast{
		ID:       "bad-moveable-feast",
		DateRule: "easter++1",
	}

	errs := validateSemantics([]*models.Feast{feast})
	if got := strings.Join(errs, "\n"); !strings.Contains(got, `Feast 'bad-moveable-feast' has unrecognized DateRule: "easter++1"`) {
		t.Fatalf("validateSemantics errors = %q, want malformed DateRule error", got)
	}
}

func TestIsValidDateRule(t *testing.T) {
	tests := []struct {
		rule string
		want bool
	}{
		{"easter+0", true},
		{"easter-3", true},
		{"epiphany-sunday-1", true},
		{"epiphany-sunday-8", true},
		{"advent-sunday-1", true},
		{"advent-sunday-4", true},
		{"pentecost-sunday-1", true},
		{"holy-name", true},
		{"last-sunday-october", true},
		{"", false},
		{"easter", false},
		{"easter+", false},
		{"epiphany-sunday-0", false},
		{"epiphany-sunday-x", false},
		{"advent-sunday-0", false},
		{"advent-sunday-5", false},
		{"advent-sunday-x", false},
		{"pentecost-sunday-0", false},
		{"pentecost-sunday-x", false},
		{"holy-name-extra", false},
	}

	for _, tt := range tests {
		t.Run(tt.rule, func(t *testing.T) {
			if got := isValidDateRule(tt.rule); got != tt.want {
				t.Fatalf("isValidDateRule(%q) = %t, want %t", tt.rule, got, tt.want)
			}
		})
	}
}
