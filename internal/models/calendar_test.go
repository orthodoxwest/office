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
