package models

import "fmt"

// PrayerForm selects private recitation or an office led by clergy.
// It describes how this office is prayed, not the reader's ordination.
// Priest includes bishops for the forms currently supported by the app.
type PrayerForm string

const (
	PrayerPrivate PrayerForm = "private"
	PrayerDeacon  PrayerForm = "deacon"
	PrayerPriest  PrayerForm = "priest"
)

var PrayerForms = []PrayerForm{PrayerPrivate, PrayerDeacon, PrayerPriest}

func ParsePrayerForm(value string) (PrayerForm, error) {
	if value == "" {
		return PrayerPrivate, nil
	}
	for _, leader := range PrayerForms {
		if string(leader) == value {
			return leader, nil
		}
	}
	return "", fmt.Errorf("invalid prayer form %q: choose private, deacon, or priest", value)
}

func (l PrayerForm) Label() string {
	switch l {
	case PrayerDeacon:
		return "Deacon"
	case PrayerPriest:
		return "Priest or bishop"
	default:
		return "Private"
	}
}
