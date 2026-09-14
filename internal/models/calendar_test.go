package models

import "testing"

func TestFeastDateClassification(t *testing.T) {
	fixedTemporalCycleFeast := &Feast{ID: "christmas", Month: 12, Day: 25}
	if !fixedTemporalCycleFeast.IsFixed() {
		t.Fatal("fixed Christmas feast should report IsFixed")
	}
	if fixedTemporalCycleFeast.IsMoveable() {
		t.Fatal("fixed Christmas feast should not report IsMoveable")
	}

	moveableFeast := &Feast{ID: "easter-sunday", DateRule: "easter+0"}
	if moveableFeast.IsFixed() {
		t.Fatal("moveable Easter feast should not report IsFixed")
	}
	if !moveableFeast.IsMoveable() {
		t.Fatal("moveable Easter feast should report IsMoveable")
	}
}

func TestRankIsDouble(t *testing.T) {
	doubles := []Rank{Double1stClass, Double2ndClass, GreaterDouble, Double}
	for _, r := range doubles {
		if !r.IsDouble() {
			t.Errorf("%s should be Double", r)
		}
	}
	not := []Rank{SemiDouble, PrivilegedFeria, Simple, Commemoration, ""}
	for _, r := range not {
		if r.IsDouble() {
			t.Errorf("%q should not be Double", r)
		}
	}
}

func TestCalendarDayIsFerial(t *testing.T) {
	// A synthesized office-day carries no Celebration at all, so the nil
	// Celebration case is a real ferial day rather than missing data.
	if !(&CalendarDay{}).IsFerial() {
		t.Error("day without a Celebration should be ferial")
	}

	feria := &CalendarDay{Celebration: &Feast{ID: "feria", Category: CategoryFeria}}
	if !feria.IsFerial() {
		t.Error("day celebrating a feria should be ferial")
	}

	feast := &CalendarDay{Celebration: &Feast{ID: "christmas", Category: CategoryLord}}
	if feast.IsFerial() {
		t.Error("day celebrating a Lord feast should not be ferial")
	}

	// Callers reach this through a possibly-absent office day.
	var absent *CalendarDay
	if absent.IsFerial() {
		t.Error("nil day should not be ferial")
	}
}
