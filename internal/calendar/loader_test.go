package calendar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

func TestParseINISections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := `# Comment line
[section-1]
Name = Test Feast
Rank = double

[section-2]
Name = Another Feast
Rank = simple
Color = red
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sections, err := parseINISections(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}

	if sections[0]["_id"] != "section-1" {
		t.Errorf("expected section-1, got %q", sections[0]["_id"])
	}
	if sections[0]["Name"] != "Test Feast" {
		t.Errorf("expected 'Test Feast', got %q", sections[0]["Name"])
	}
	if sections[1]["Color"] != "red" {
		t.Errorf("expected 'red', got %q", sections[1]["Color"])
	}
}

func TestParseINISectionsErrors(t *testing.T) {
	dir := t.TempDir()

	// Key-value outside section
	path := filepath.Join(dir, "no-section.txt")
	if err := os.WriteFile(path, []byte("Name = bad\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseINISections(path); err == nil {
		t.Error("expected error for key-value outside section")
	}

	// Missing equals
	path = filepath.Join(dir, "bad-line.txt")
	if err := os.WriteFile(path, []byte("[test]\nbad line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseINISections(path); err == nil {
		t.Error("expected error for missing equals")
	}
}

func TestLoadFeasts(t *testing.T) {
	// Use the real data directory
	dataDir := findDataDir(t)
	feasts, err := LoadFeasts(dataDir)
	if err != nil {
		t.Fatalf("LoadFeasts error: %v", err)
	}

	if len(feasts) == 0 {
		t.Fatal("expected feasts to be loaded")
	}

	// Check a known temporal feast
	var easter *models.Feast
	for _, f := range feasts {
		if f.ID == "easter-sunday" {
			easter = f
			break
		}
	}
	if easter == nil {
		t.Fatal("easter-sunday not found")
	}
	if easter.Rank != models.Double1stClass {
		t.Errorf("easter rank = %v, want double-1st-class", easter.Rank)
	}
	if easter.Color != models.White {
		t.Errorf("easter color = %v, want white", easter.Color)
	}
	if !easter.HasOctave {
		t.Error("easter should have octave")
	}

	// Check a known sanctoral feast
	var christmas *models.Feast
	for _, f := range feasts {
		if f.ID == "christmas" {
			christmas = f
			break
		}
	}
	if christmas == nil {
		t.Fatal("christmas not found")
	}
	if christmas.Month != 12 || christmas.Day != 25 {
		t.Errorf("christmas date = %d/%d, want 12/25", christmas.Month, christmas.Day)
	}

	// Check AWRV feast
	var raphael *models.Feast
	for _, f := range feasts {
		if f.ID == "st-raphael-of-brooklyn" {
			raphael = f
			break
		}
	}
	if raphael == nil {
		t.Fatal("st-raphael-of-brooklyn not found")
	}
	if raphael.Source != models.SourceAWRV {
		t.Errorf("raphael source = %v, want awrv", raphael.Source)
	}
}

func TestLoadFeastsVigilTraits(t *testing.T) {
	feasts, err := LoadFeasts(findDataDir(t))
	if err != nil {
		t.Fatalf("LoadFeasts error: %v", err)
	}
	want := map[string]bool{
		"vigil-ascension": true, "vigil-pentecost": true,
		"vigil-epiphany": true, "vigil-st-james": true, "vigil-nativity": true,
		"comm-extra-12-07-vigil-of-the-conception": true,
		"comm-extra-02-23-vigil-of-st-matthias":    true,
		"comm-extra-08-22-vigil-of-st-bartholomew": true,
	}
	for _, feast := range feasts {
		if !feast.IsVigil {
			continue
		}
		if !want[feast.ID] {
			t.Errorf("unexpected IsVigil trait on %q", feast.ID)
		}
		delete(want, feast.ID)
	}
	for id := range want {
		t.Errorf("missing IsVigil trait on %q", id)
	}
}

func TestLoadPenitentialRules(t *testing.T) {
	dataDir := findDataDir(t)
	rules, err := loadPenitentialRules(dataDir)
	if err != nil {
		t.Fatalf("loadPenitentialRules error: %v", err)
	}

	if len(rules) == 0 {
		t.Fatal("expected penitential rules to be loaded")
	}

	foundFriday := false
	for _, rule := range rules {
		if rule.ID == "friday-abstinence" {
			foundFriday = true
			if rule.Abstinence == nil || !*rule.Abstinence {
				t.Fatal("friday-abstinence should set abstinence")
			}
			if len(rule.Weekdays) == 0 || !rule.Weekdays[time.Friday] {
				t.Fatal("friday-abstinence should apply on Fridays")
			}
		}
	}
	if !foundFriday {
		t.Fatal("friday-abstinence rule not found")
	}
}

func TestSectionToFeastOnlyWith(t *testing.T) {
	feast, err := sectionToFeast(map[string]string{
		"_id":      "comm-test",
		"Name":     "Test Commemoration",
		"Rank":     "commemoration",
		"Color":    "white",
		"Category": "confessor",
		"Month":    "2",
		"Day":      "22",
		"OnlyWith": "chair-peter-antioch",
	}, "test.txt")
	if err != nil {
		t.Fatalf("sectionToFeast returned error: %v", err)
	}
	if feast.OnlyWith != "chair-peter-antioch" {
		t.Fatalf("OnlyWith = %q, want %q", feast.OnlyWith, "chair-peter-antioch")
	}
}

func TestSectionToFeastSkipRomanLeapShift(t *testing.T) {
	feast, err := sectionToFeast(map[string]string{
		"_id":                "st-raphael-of-brooklyn",
		"Name":               "St. Raphael of Brooklyn",
		"Rank":               "greater-double",
		"Color":              "white",
		"Category":           "confessor-bishop",
		"Month":              "2",
		"Day":                "27",
		"SkipRomanLeapShift": "true",
	}, "test.txt")
	if err != nil {
		t.Fatalf("sectionToFeast returned error: %v", err)
	}
	if !feast.SkipRomanLeapShift {
		t.Fatal("SkipRomanLeapShift = false, want true")
	}
}

func TestSectionToFeastIsVigil(t *testing.T) {
	feast, err := sectionToFeast(map[string]string{
		"_id": "example-vigil", "Name": "Example Vigil", "Rank": "simple",
		"Color": "violet", "Category": "feria", "Month": "6", "Day": "1",
		"IsVigil": "true",
	}, "test.txt")
	if err != nil {
		t.Fatalf("sectionToFeast returned error: %v", err)
	}
	if !feast.IsVigil {
		t.Fatal("IsVigil = false, want true")
	}
}

func TestSectionToFeastRejectsInvalidScalarValues(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{name: "invalid boolean", key: "HasOctave", value: "yes", wantErr: "expected true or false"},
		{name: "invalid vigil boolean", key: "IsVigil", value: "yes", wantErr: "expected true or false"},
		{name: "invalid month", key: "Month", value: "13", wantErr: "invalid fixed date"},
		{name: "invalid day", key: "Day", value: "30", wantErr: "invalid fixed date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]string{
				"_id": "test-feast", "Name": "Test Feast", "Rank": "simple",
				"Color": "white", "Category": "confessor", "Month": "2", "Day": "28",
			}
			data[tt.key] = tt.value
			_, err := sectionToFeast(data, "test.txt")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("sectionToFeast error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSectionToFeastRequiresCompleteFixedDate(t *testing.T) {
	_, err := sectionToFeast(map[string]string{
		"_id": "test-feast", "Name": "Test Feast", "Rank": "simple",
		"Color": "white", "Category": "confessor", "Month": "2",
	}, "test.txt")
	if err == nil || !strings.Contains(err.Error(), "Month and Day together") {
		t.Fatalf("sectionToFeast error = %v, want incomplete-date error", err)
	}
}

func TestSectionToFeastProperID(t *testing.T) {
	feast, err := sectionToFeast(map[string]string{
		"_id":      "epiphany-sunday-7",
		"Name":     "VII Sunday after Epiphany",
		"Rank":     "semi-double",
		"Color":    "green",
		"Category": "sunday",
		"DateRule": "epiphany-sunday-7",
		"ProperID": "pentecost-sunday-23",
	}, "test")
	if err != nil {
		t.Fatalf("sectionToFeast returned error: %v", err)
	}
	if feast.ProperID != "pentecost-sunday-23" {
		t.Fatalf("ProperID = %q, want %q", feast.ProperID, "pentecost-sunday-23")
	}
}

// findDataDir locates the project data directory from the test file location.
func findDataDir(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find the project root (contains go.mod)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "data")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}
