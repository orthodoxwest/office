package models

import "testing"

func TestPrayerFormVocabulary(t *testing.T) {
	for _, tc := range []struct {
		value string
		form  PrayerForm
		label string
	}{
		{"", PrayerPrivate, "Private"},
		{"private", PrayerPrivate, "Private"},
		{"deacon", PrayerDeacon, "Deacon"},
		{"priest", PrayerPriest, "Priest"},
	} {
		form, err := ParsePrayerForm(tc.value)
		if err != nil || form != tc.form || form.Label() != tc.label {
			t.Errorf("parse %q = %q (%s), %v", tc.value, form, form.Label(), err)
		}
	}
	for _, value := range []string{"lay", "bishop", "Private", "deacon ", "invalid"} {
		if form, err := ParsePrayerForm(value); err == nil || form != "" {
			t.Errorf("accepted invalid form %q", value)
		}
	}
}
