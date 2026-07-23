package office

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseHourDefinition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `[Opening]
Type = versicle
Ref = ordinary/compline/opening-versicle

Type = rubric
Ref = ordinary/compline/examination

[Psalmody]
Type = psalm
Ref = psalms/004

Type = psalm
Ref = psalms/091

[Preces]
Condition = if-preces

Type = preces
Ref = ordinary/compline/preces

[Closing]
Type = marian
Ref = seasonal
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sections, err := ParseHourDefinition(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}

	// Opening section
	if sections[0].Name != "Opening" {
		t.Errorf("section 0 name = %q, want Opening", sections[0].Name)
	}
	if sections[0].Condition != "" {
		t.Errorf("section 0 condition = %q, want empty", sections[0].Condition)
	}
	if len(sections[0].Elements) != 2 {
		t.Fatalf("section 0 elements = %d, want 2", len(sections[0].Elements))
	}
	if sections[0].Elements[0].Type != "versicle" || sections[0].Elements[0].Ref != "ordinary/compline/opening-versicle" {
		t.Errorf("section 0 element 0 = %+v", sections[0].Elements[0])
	}

	// Psalmody section
	if len(sections[1].Elements) != 2 {
		t.Fatalf("section 1 elements = %d, want 2", len(sections[1].Elements))
	}
	if sections[1].Elements[0].Type != "psalm" || sections[1].Elements[0].Ref != "psalms/004" {
		t.Errorf("section 1 element 0 = %+v", sections[1].Elements[0])
	}

	// Preces section with condition
	if sections[2].Condition != "if-preces" {
		t.Errorf("section 2 condition = %q, want if-preces", sections[2].Condition)
	}
	if len(sections[2].Elements) != 1 {
		t.Fatalf("section 2 elements = %d, want 1", len(sections[2].Elements))
	}

	// Closing section with marian type
	if sections[3].Elements[0].Type != "marian" || sections[3].Elements[0].Ref != "seasonal" {
		t.Errorf("section 3 element 0 = %+v", sections[3].Elements[0])
	}
}

func TestParseHourDefinitionErrors(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
	}{
		{"type without ref", "[S]\nType = psalm\n"},
		{"ref without type", "[S]\nRef = foo\n"},
		{"consecutive types", "[S]\nType = a\nType = b\nRef = c\n"},
		{"kv before section", "Type = foo\n"},
		{"unknown key", "[S]\nFoo = bar\n"},
		{"unknown condition", "[S]\nCondition = season-lnt\nType = rubric\nRef = foo\n"},
		{"empty condition", "[S]\nCondition =\nType = rubric\nRef = foo\n"},
		{"empty condition clause", "[S]\nCondition = weekday-sunday,\nType = rubric\nRef = foo\n"},
		{"duplicate condition", "[S]\nCondition = season-lent\nCondition = weekday-sunday\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name+".txt")
			os.WriteFile(path, []byte(tt.content), 0644)
			_, err := ParseHourDefinition(path)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestOfficeDataUsesExpectedPreCollectSections(t *testing.T) {
	tests := []struct {
		file     string
		elements []HourElement
		noIfPre  bool
	}{
		{file: "lauds.txt", elements: []HourElement{{Type: "prayer", Ref: "ordinary/shared/kyrie"}, {Type: "prayer", Ref: "ordinary/shared/our-father"}, {Type: "prayer", Ref: "ordinary/lauds/pre-collect-versicles"}}, noIfPre: true},
		{file: "vespers.txt", elements: []HourElement{{Type: "prayer", Ref: "ordinary/shared/kyrie"}, {Type: "prayer", Ref: "ordinary/shared/our-father"}, {Type: "prayer", Ref: "ordinary/vespers/pre-collect-versicles"}}, noIfPre: true},
		{file: "terce.txt", elements: []HourElement{{Type: "prayer", Ref: "ordinary/shared/kyrie"}, {Type: "rubric", Ref: "shared/formulas/our-father-partly-secret-rubric"}, {Type: "partly-secret-prayer", Ref: "ordinary/shared/our-father"}, {Type: "prayer", Ref: "ordinary/terce/pre-collect-versicles"}}, noIfPre: true},
		{file: "sext.txt", elements: []HourElement{{Type: "prayer", Ref: "ordinary/shared/kyrie"}, {Type: "rubric", Ref: "shared/formulas/our-father-partly-secret-rubric"}, {Type: "partly-secret-prayer", Ref: "ordinary/shared/our-father"}, {Type: "prayer", Ref: "ordinary/sext/pre-collect-versicles"}}, noIfPre: true},
		{file: "none.txt", elements: []HourElement{{Type: "prayer", Ref: "ordinary/shared/kyrie"}, {Type: "rubric", Ref: "shared/formulas/our-father-partly-secret-rubric"}, {Type: "partly-secret-prayer", Ref: "ordinary/shared/our-father"}, {Type: "prayer", Ref: "ordinary/none/pre-collect-versicles"}}, noIfPre: true},
		{file: "prime.txt", elements: []HourElement{{Type: "proper-versicle", Ref: "pre-collect-versicle"}, {Type: "prayer", Ref: "ordinary/shared/kyrie"}, {Type: "rubric", Ref: "shared/formulas/our-father-partly-secret-rubric"}, {Type: "partly-secret-prayer", Ref: "ordinary/shared/our-father"}}, noIfPre: false},
		// secret-prayer conversions for opening/closing are covered in TestOfficeDataExpandsModeledSecretPrayers
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "data", "office", tt.file)
			sections, err := ParseHourDefinition(path)
			if err != nil {
				t.Fatalf("ParseHourDefinition(%s): %v", tt.file, err)
			}

			foundPreCollect := false
			for _, section := range sections {
				if tt.noIfPre && section.Condition == "if-preces" {
					t.Fatalf("%s still has an if-preces section: %s", tt.file, section.Name)
				}
				if section.Name != "Pre-Collect" {
					continue
				}
				if len(section.Elements) != len(tt.elements) {
					t.Fatalf("%s Pre-Collect elements = %d, want %d", tt.file, len(section.Elements), len(tt.elements))
				}
				for i, want := range tt.elements {
					if section.Elements[i] != want {
						t.Fatalf("%s Pre-Collect[%d] = %+v, want %+v", tt.file, i, section.Elements[i], want)
					}
				}
				foundPreCollect = true
			}

			if !foundPreCollect {
				t.Fatalf("%s missing Pre-Collect section", tt.file)
			}
		})
	}
}

func TestOfficeDataUsesPrintedClosingSequence(t *testing.T) {
	tests := []struct {
		file     string
		start    string
		sections []HourSection
	}{
		{
			file:  "lauds.txt",
			start: "Closing",
			sections: []HourSection{
				{Name: "Closing", Elements: []HourElement{
					{Type: "blessing", Ref: "ordinary/lauds/blessing"},
					{Type: "versicle", Ref: "shared/formulas/faithful-departed"},
					{Type: "rubric", Ref: "shared/formulas/closing-our-father"},
					{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
					{Type: "versicle", Ref: "shared/formulas/closing-peace"},
				}},
				{Name: "Marian", Elements: []HourElement{{Type: "marian", Ref: "seasonal"}}},
				{Name: "Conclusion", Elements: []HourElement{{Type: "versicle", Ref: "shared/formulas/divine-help"}}},
				{Name: "Post-Office", Elements: []HourElement{
					{Type: "prayer", Ref: "ordinary/session/sacrosanctae"},
					{Type: "rubric", Ref: "ordinary/session/closing-rubric"},
					{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
					{Type: "secret-prayer", Ref: "ordinary/shared/hail-mary"},
				}},
			},
		},
		{
			file:  "vespers.txt",
			start: "Closing",
			sections: []HourSection{
				{Name: "Closing", Elements: []HourElement{
					{Type: "blessing", Ref: "ordinary/vespers/blessing"},
					{Type: "rubric", Ref: "shared/formulas/closing-our-father"},
					{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
					{Type: "versicle", Ref: "shared/formulas/closing-peace"},
				}},
				{Name: "Marian", Elements: []HourElement{{Type: "marian", Ref: "seasonal"}}},
				{Name: "Conclusion", Elements: []HourElement{{Type: "versicle", Ref: "shared/formulas/divine-help"}}},
				{Name: "Post-Office", Elements: []HourElement{
					{Type: "prayer", Ref: "ordinary/session/sacrosanctae"},
					{Type: "rubric", Ref: "ordinary/session/closing-rubric"},
					{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
					{Type: "secret-prayer", Ref: "ordinary/shared/hail-mary"},
				}},
			},
		},
		{
			file:  "compline.txt",
			start: "Marian",
			sections: []HourSection{
				{Name: "Marian", Elements: []HourElement{{Type: "marian", Ref: "seasonal"}}},
				{Name: "Closing", Elements: []HourElement{
					{Type: "blessing", Ref: "ordinary/compline/conclusion"},
					{Type: "rubric", Ref: "ordinary/compline/concluding-rubric"},
					{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
					{Type: "secret-prayer", Ref: "ordinary/shared/hail-mary"},
					{Type: "secret-prayer", Ref: "ordinary/shared/apostles-creed"},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "data", "office", tt.file)
			sections, err := ParseHourDefinition(path)
			if err != nil {
				t.Fatalf("ParseHourDefinition(%s): %v", tt.file, err)
			}

			start := -1
			for i, section := range sections {
				if section.Name == tt.start {
					start = i
					break
				}
			}
			if start < 0 {
				t.Fatalf("%s missing %s section", tt.file, tt.start)
			}
			got := sections[start:]
			if len(got) != len(tt.sections) {
				t.Fatalf("%s closing section count = %d, want %d", tt.file, len(got), len(tt.sections))
			}
			for i, want := range tt.sections {
				if got[i].Name != want.Name {
					t.Fatalf("%s closing section %d = %q, want %q", tt.file, i, got[i].Name, want.Name)
				}
				if !reflect.DeepEqual(got[i].Elements, want.Elements) {
					t.Fatalf("%s %s elements = %+v, want %+v", tt.file, got[i].Name, got[i].Elements, want.Elements)
				}
			}
		})
	}
}

func TestMinorHourDataUsesParishStructure(t *testing.T) {
	for _, file := range []string{"terce.txt", "sext.txt", "none.txt"} {
		t.Run(file, func(t *testing.T) {
			path := filepath.Join("..", "..", "data", "office", file)
			sections, err := ParseHourDefinition(path)
			if err != nil {
				t.Fatalf("ParseHourDefinition(%s): %v", file, err)
			}

			byName := make(map[string]HourSection, len(sections))
			for _, section := range sections {
				byName[section.Name] = section
				for _, elem := range section.Elements {
					if elem.Type == "proper-responsory" {
						t.Fatalf("%s still renders a Little Hours responsory: %+v", file, elem)
					}
				}
			}

			if got := byName["Pre-Office"].Elements; !reflect.DeepEqual(got, []HourElement{
				{Type: "rubric", Ref: "ordinary/session/little-hours-opening-rubric"},
				{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
				{Type: "secret-prayer", Ref: "ordinary/shared/hail-mary"},
			}) {
				t.Fatalf("%s Pre-Office = %+v", file, got)
			}
			if got := byName["Chapter"].Elements; !reflect.DeepEqual(got, []HourElement{
				{Type: "proper-chapter", Ref: "chapter"},
				{Type: "proper-versicle", Ref: "versicle"},
			}) {
				t.Fatalf("%s Chapter = %+v", file, got)
			}
			hour := strings.TrimSuffix(file, ".txt")
			if got := byName["Closing"].Elements; !reflect.DeepEqual(got, []HourElement{
				{Type: "blessing", Ref: "ordinary/" + hour + "/blessing"},
				{Type: "versicle", Ref: "shared/formulas/faithful-departed"},
			}) {
				t.Fatalf("%s Closing = %+v", file, got)
			}
			if got := byName["Post-Office"].Elements; !reflect.DeepEqual(got, []HourElement{
				{Type: "rubric", Ref: "shared/formulas/closing-our-father"},
				{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
			}) {
				t.Fatalf("%s Post-Office = %+v", file, got)
			}
		})
	}
}

func TestPrimeDataKeepsOptionalPrecesBlock(t *testing.T) {
	path := filepath.Join("..", "..", "data", "office", "prime.txt")
	sections, err := ParseHourDefinition(path)
	if err != nil {
		t.Fatalf("ParseHourDefinition(prime.txt): %v", err)
	}

	var foundPreces bool
	var foundCollectIntro bool
	for _, section := range sections {
		switch section.Name {
		case "Preces":
			foundPreces = true
			if section.Condition != "if-preces" {
				t.Fatalf("Prime Preces condition = %q, want if-preces", section.Condition)
			}
			wantRefs := []string{
				"ordinary/shared/apostles-creed",
				"ordinary/prime/preces-our-help",
				"ordinary/shared/confiteor",
				"ordinary/prime/preces-vouchsafe",
			}
			wantTypes := []string{"preces", "prayer", "prayer", "prayer"}
			if len(section.Elements) != len(wantRefs) {
				t.Fatalf("Prime Preces elements = %d, want %d", len(section.Elements), len(wantRefs))
			}
			for i, wantRef := range wantRefs {
				if section.Elements[i].Type != wantTypes[i] || section.Elements[i].Ref != wantRef {
					t.Fatalf("Prime Preces[%d] = %+v, want %s %q", i, section.Elements[i], wantTypes[i], wantRef)
				}
			}
		case "Collect-Intro":
			foundCollectIntro = true
			if len(section.Elements) != 1 || section.Elements[0].Ref != "ordinary/prime/collect-intro" {
				t.Fatalf("Prime Collect-Intro = %+v", section.Elements)
			}
		}
	}

	if !foundPreces {
		t.Fatal("prime.txt missing conditional Preces section")
	}
	if !foundCollectIntro {
		t.Fatal("prime.txt missing Collect-Intro section")
	}
}

func TestPrimeUsesParishOpeningAndClosingStructure(t *testing.T) {
	path := filepath.Join("..", "..", "data", "office", "prime.txt")
	sections, err := ParseHourDefinition(path)
	if err != nil {
		t.Fatalf("ParseHourDefinition(prime.txt): %v", err)
	}

	byName := make(map[string]HourSection, len(sections))
	for _, section := range sections {
		byName[section.Name] = section
	}

	if got := byName["Opening"].Elements; !reflect.DeepEqual(got, []HourElement{
		{Type: "versicle", Ref: "ordinary/prime/opening-versicle"},
		{Type: "proper-antiphon", Ref: "alleluia"},
	}) {
		t.Fatalf("Prime Opening = %+v", got)
	}
	if got := byName["Closing"].Elements; !reflect.DeepEqual(got, []HourElement{
		{Type: "blessing", Ref: "ordinary/prime/blessing"},
		{Type: "versicle", Ref: "shared/formulas/faithful-departed"},
	}) {
		t.Fatalf("Prime Closing = %+v", got)
	}
	if got := byName["Pre-Office"].Elements; !reflect.DeepEqual(got, []HourElement{
		{Type: "rubric", Ref: "ordinary/session/opening-rubric"},
		{Type: "prayer", Ref: "ordinary/session/open-my-mouth"},
		{Type: "rubric", Ref: "ordinary/session/prime-opening-secret-prayers-rubric"},
		{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
		{Type: "secret-prayer", Ref: "ordinary/shared/hail-mary"},
		{Type: "secret-prayer", Ref: "ordinary/shared/apostles-creed"},
	}) {
		t.Fatalf("Prime Pre-Office = %+v", got)
	}
	if got := byName["Post-Office"].Elements; !reflect.DeepEqual(got, []HourElement{
		{Type: "prayer", Ref: "ordinary/session/sacrosanctae"},
		{Type: "rubric", Ref: "ordinary/session/closing-rubric"},
		{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
		{Type: "secret-prayer", Ref: "ordinary/shared/hail-mary"},
	}) {
		t.Fatalf("Prime Post-Office = %+v", got)
	}
}

func TestOfficeDataExpandsModeledSecretPrayers(t *testing.T) {
	for _, file := range []string{"lauds.txt", "vespers.txt"} {
		t.Run(file, func(t *testing.T) {
			path := filepath.Join("..", "..", "data", "office", file)
			sections, err := ParseHourDefinition(path)
			if err != nil {
				t.Fatalf("ParseHourDefinition(%s): %v", file, err)
			}
			byName := make(map[string]HourSection, len(sections))
			for _, section := range sections {
				byName[section.Name] = section
			}
			if got := byName["Pre-Office"].Elements; !reflect.DeepEqual(got, []HourElement{
				{Type: "rubric", Ref: "ordinary/session/opening-rubric"},
				{Type: "prayer", Ref: "ordinary/session/open-my-mouth"},
				{Type: "rubric", Ref: "ordinary/session/our-father-hail-mary-rubric"},
				{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
				{Type: "secret-prayer", Ref: "ordinary/shared/hail-mary"},
			}) {
				t.Fatalf("%s Pre-Office = %+v", file, got)
			}
		})
	}

	path := filepath.Join("..", "..", "data", "office", "compline.txt")
	sections, err := ParseHourDefinition(path)
	if err != nil {
		t.Fatalf("ParseHourDefinition(compline.txt): %v", err)
	}
	byName := make(map[string]HourSection, len(sections))
	for _, section := range sections {
		byName[section.Name] = section
	}
	if got := byName["Opening"].Elements[:5]; !reflect.DeepEqual(got, []HourElement{
		{Type: "versicle", Ref: "ordinary/compline/opening-versicle"},
		{Type: "chapter", Ref: "ordinary/compline/short-lesson"},
		{Type: "rubric", Ref: "ordinary/compline/confiteor-rubric"},
		{Type: "secret-prayer", Ref: "ordinary/shared/our-father"},
		{Type: "prayer", Ref: "ordinary/shared/confiteor"},
	}) {
		t.Fatalf("Compline Opening private prayers = %+v", got)
	}
	if got := byName["Chapter"].Elements; !reflect.DeepEqual(got[len(got)-3:], []HourElement{
		{Type: "prayer", Ref: "ordinary/shared/kyrie"},
		{Type: "rubric", Ref: "shared/formulas/our-father-partly-secret-rubric"},
		{Type: "partly-secret-prayer", Ref: "ordinary/shared/our-father"},
	}) {
		t.Fatalf("Compline Chapter private prayers = %+v", got)
	}
	if got := byName["Chapter-Easter-Eve"].Elements; !reflect.DeepEqual(got, []HourElement{
		{Type: "prayer", Ref: "ordinary/shared/kyrie"},
	}) {
		t.Fatalf("Holy Saturday Compline private prayers = %+v", got)
	}
}

func TestSecretPrayerRubricsAreFollowedByFullTexts(t *testing.T) {
	wantAfter := map[string][]string{
		"ordinary/session/our-father-hail-mary-rubric":         {"ordinary/shared/our-father", "ordinary/shared/hail-mary"},
		"ordinary/session/prime-opening-secret-prayers-rubric": {"ordinary/shared/our-father", "ordinary/shared/hail-mary", "ordinary/shared/apostles-creed"},
		"ordinary/session/little-hours-opening-rubric":         {"ordinary/shared/our-father", "ordinary/shared/hail-mary"},
		"ordinary/session/closing-rubric":                      {"ordinary/shared/our-father", "ordinary/shared/hail-mary"},
		"shared/formulas/closing-our-father":                   {"ordinary/shared/our-father"},
		"shared/formulas/our-father-partly-secret-rubric":      {"ordinary/shared/our-father"},
		"ordinary/compline/confiteor-rubric":                   {"ordinary/shared/our-father"},
		"ordinary/compline/concluding-rubric":                  {"ordinary/shared/our-father", "ordinary/shared/hail-mary", "ordinary/shared/apostles-creed"},
	}

	seen := make(map[string]int, len(wantAfter))
	for _, file := range []string{"lauds.txt", "vespers.txt", "prime.txt", "terce.txt", "sext.txt", "none.txt", "compline.txt"} {
		path := filepath.Join("..", "..", "data", "office", file)
		sections, err := ParseHourDefinition(path)
		if err != nil {
			t.Fatalf("ParseHourDefinition(%s): %v", file, err)
		}
		for _, section := range sections {
			for i, elem := range section.Elements {
				want, ok := wantAfter[elem.Ref]
				if !ok {
					continue
				}
				seen[elem.Ref]++
				if i+len(want) >= len(section.Elements) {
					t.Errorf("%s %s %q is not followed by %d full prayer(s)", file, section.Name, elem.Ref, len(want))
					continue
				}
				for j, ref := range want {
					got := section.Elements[i+1+j]
					wantType := "secret-prayer"
					if elem.Ref == "shared/formulas/our-father-partly-secret-rubric" {
						wantType = "partly-secret-prayer"
					}
					if got.Type != wantType || got.Ref != ref {
						t.Errorf("%s %s after %q[%d] = %+v, want %s %q", file, section.Name, elem.Ref, j, got, wantType, ref)
					}
				}
			}
		}
	}
	for ref := range wantAfter {
		if seen[ref] == 0 {
			t.Errorf("secret-prayer rubric %q is not modeled in an office definition", ref)
		}
	}
}

func TestOfficeDataUsesOpeningAlleluia(t *testing.T) {
	files := []string{"lauds.txt", "vespers.txt", "prime.txt", "terce.txt", "sext.txt", "none.txt"}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			path := filepath.Join("..", "..", "data", "office", file)
			sections, err := ParseHourDefinition(path)
			if err != nil {
				t.Fatalf("ParseHourDefinition(%s): %v", file, err)
			}

			found := false
			for _, section := range sections {
				if section.Name != "Opening" {
					continue
				}
				if len(section.Elements) != 2 {
					t.Fatalf("%s Opening elements = %d, want 2", file, len(section.Elements))
				}
				if section.Elements[1].Type != "proper-antiphon" || section.Elements[1].Ref != "alleluia" {
					t.Fatalf("%s Opening[1] = %+v, want proper-antiphon alleluia", file, section.Elements[1])
				}
				found = true
			}

			if !found {
				t.Fatalf("%s missing Opening section", file)
			}
		})
	}
}

func TestMinorHoursUseIndexedLaudsAntiphons(t *testing.T) {
	tests := []struct {
		file string
		ref  string
	}{
		{file: "terce.txt", ref: "psalm-antiphon-2"},
		{file: "sext.txt", ref: "psalm-antiphon-3"},
		{file: "none.txt", ref: "psalm-antiphon-5"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "data", "office", tt.file)
			sections, err := ParseHourDefinition(path)
			if err != nil {
				t.Fatalf("ParseHourDefinition(%s): %v", tt.file, err)
			}

			for _, section := range sections {
				if !strings.HasPrefix(section.Name, "Psalmody-") {
					continue
				}
				for _, elem := range section.Elements {
					if !strings.Contains(elem.Type, "antiphon") {
						continue
					}
					if elem.Type != "proper-antiphon" || elem.Ref != tt.ref {
						t.Fatalf("%s %s antiphon = %+v, want proper-antiphon %q", tt.file, section.Name, elem, tt.ref)
					}
				}
			}
		})
	}
}

func TestPrimeUsesFixedCollectAndSplitPsalm9(t *testing.T) {
	path := filepath.Join("..", "..", "data", "office", "prime.txt")
	sections, err := ParseHourDefinition(path)
	if err != nil {
		t.Fatalf("ParseHourDefinition(prime.txt): %v", err)
	}

	byName := make(map[string]HourSection, len(sections))
	for _, section := range sections {
		byName[section.Name] = section
	}

	collect := byName["Collect"]
	wantCollect := []HourElement{{Type: "collect", Ref: "ordinary/prime/collect"}}
	if !reflect.DeepEqual(collect.Elements, wantCollect) {
		t.Fatalf("Prime Collect = %+v, want %+v", collect.Elements, wantCollect)
	}

	tuesday := byName["Psalmody-Tuesday"]
	wednesday := byName["Psalmody-Wednesday"]
	for name, section := range byName {
		if !strings.HasPrefix(name, "Psalmody-") {
			continue
		}
		for _, elem := range section.Elements {
			if strings.Contains(elem.Type, "antiphon") && elem.Ref != "psalm-antiphon-1" {
				t.Fatalf("%s antiphon = %+v, want Prime psalm-antiphon-1 slot", name, elem)
			}
		}
	}
	if !hasHourRef(tuesday.Elements, "psalms/009a") || hasHourRef(tuesday.Elements, "psalms/009") {
		t.Fatalf("Tuesday psalmody = %+v, want split psalms/009a only", tuesday.Elements)
	}
	if !adjacentHourRefs(wednesday.Elements, "psalms/009b", "psalms/010") {
		t.Fatalf("Wednesday psalmody = %+v, want Psalm 9:19-20 immediately followed by Psalm 10", wednesday.Elements)
	}
	for i, elem := range wednesday.Elements {
		if elem.Ref == "psalms/009b" && i+1 < len(wednesday.Elements) && wednesday.Elements[i+1].Type == "gloria-patri" {
			t.Fatal("Wednesday inserts a doxology between Psalm 9:19-20 and Psalm 10")
		}
	}
}

func TestLittleHoursUseParishPsalm119Sections(t *testing.T) {
	tests := []struct {
		file      string
		condition string
		want      []string
	}{
		{file: "prime.txt", condition: "weekday-sunday", want: []string{"psalms/119-i", "psalms/119-ii", "psalms/119-iii", "psalms/119-iv"}},
		{file: "terce.txt", condition: "weekday-sunday", want: []string{"psalms/119-v", "psalms/119-vi", "psalms/119-vii"}},
		{file: "sext.txt", condition: "weekday-sunday", want: []string{"psalms/119-viii", "psalms/119-ix", "psalms/119-x"}},
		{file: "none.txt", condition: "weekday-sunday", want: []string{"psalms/119-xi", "psalms/119-xii", "psalms/119-xiii"}},
		{file: "terce.txt", condition: "weekday-monday", want: []string{"psalms/119-xiv", "psalms/119-xv", "psalms/119-xvi"}},
		{file: "sext.txt", condition: "weekday-monday", want: []string{"psalms/119-xvii", "psalms/119-xviii", "psalms/119-xix"}},
		{file: "none.txt", condition: "weekday-monday", want: []string{"psalms/119-xx", "psalms/119-xxi", "psalms/119-xxii"}},
	}

	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.condition, func(t *testing.T) {
			path := filepath.Join("..", "..", "data", "office", tt.file)
			sections, err := ParseHourDefinition(path)
			if err != nil {
				t.Fatalf("ParseHourDefinition(%s): %v", tt.file, err)
			}

			var got []string
			for _, section := range sections {
				if section.Condition != tt.condition {
					continue
				}
				for _, elem := range section.Elements {
					if elem.Type == "psalm" {
						got = append(got, elem.Ref)
					}
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("%s %s psalms = %v, want %v", tt.file, tt.condition, got, tt.want)
			}
		})
	}
}

func hasHourRef(elements []HourElement, ref string) bool {
	for _, elem := range elements {
		if elem.Ref == ref {
			return true
		}
	}
	return false
}

func adjacentHourRefs(elements []HourElement, first, second string) bool {
	for i := 0; i+1 < len(elements); i++ {
		if elements[i].Ref == first && elements[i+1].Ref == second {
			return true
		}
	}
	return false
}

func TestLaudsAndVespersUseIndexedPsalmAntiphons(t *testing.T) {
	tests := []struct {
		file      string
		laudate   bool
		maxRefNum string
	}{
		{file: "lauds.txt", laudate: true, maxRefNum: "5"},
		{file: "vespers.txt", laudate: false, maxRefNum: "5"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "data", "office", tt.file)
			sections, err := ParseHourDefinition(path)
			if err != nil {
				t.Fatalf("ParseHourDefinition(%s): %v", tt.file, err)
			}

			for _, section := range sections {
				if !strings.HasPrefix(section.Name, "Psalmody-") && !(tt.laudate && strings.HasPrefix(section.Name, "Laudate")) {
					continue
				}
				for _, elem := range section.Elements {
					if !strings.Contains(elem.Type, "antiphon") {
						continue
					}
					if elem.Ref == "psalm-antiphon" {
						t.Fatalf("%s %s still uses generic psalm-antiphon", tt.file, section.Name)
					}
					if strings.HasPrefix(elem.Ref, "ordinary/") && strings.Contains(elem.Ref, "psalm-antiphon") {
						t.Fatalf("%s %s still uses ordinary psalm-antiphon ref %q", tt.file, section.Name, elem.Ref)
					}
					if strings.HasPrefix(elem.Ref, "psalm-antiphon-") {
						suffix := strings.TrimPrefix(elem.Ref, "psalm-antiphon-")
						if suffix < "1" || suffix > tt.maxRefNum {
							t.Fatalf("%s %s antiphon ref %q out of expected range", tt.file, section.Name, elem.Ref)
						}
					}
				}
			}
		})
	}
}

func TestLaudsPsalm67RemainsUnantiphoned(t *testing.T) {
	path := filepath.Join("..", "..", "data", "office", "lauds.txt")
	sections, err := ParseHourDefinition(path)
	if err != nil {
		t.Fatalf("ParseHourDefinition(lauds.txt): %v", err)
	}

	for _, sectionName := range []string{
		"Psalmody-Sunday",
		"Psalmody-Festal",
		"Psalmody-Monday",
		"Psalmody-Tuesday",
		"Psalmody-Wednesday",
		"Psalmody-Thursday",
		"Psalmody-Friday",
		"Psalmody-Saturday",
	} {
		var section *HourSection
		for i := range sections {
			if sections[i].Name == sectionName {
				section = &sections[i]
				break
			}
		}
		if section == nil {
			t.Fatalf("missing %s", sectionName)
		}
		if len(section.Elements) < 3 {
			t.Fatalf("%s elements = %d, want at least 3", sectionName, len(section.Elements))
		}
		if section.Elements[0].Type != "psalm" || section.Elements[0].Ref != "psalms/067" {
			t.Fatalf("%s first element = %+v, want psalm 067", sectionName, section.Elements[0])
		}
		if section.Elements[2].Type != "proper-antiphon" || section.Elements[2].Ref != "psalm-antiphon-1" {
			t.Fatalf("%s third element = %+v, want first indexed antiphon after Psalm 67", sectionName, section.Elements[2])
		}
	}
}

func TestLaudsSaturdayUsesPsalm143(t *testing.T) {
	path := filepath.Join("..", "..", "data", "office", "lauds.txt")
	sections, err := ParseHourDefinition(path)
	if err != nil {
		t.Fatalf("ParseHourDefinition(lauds.txt): %v", err)
	}

	var saturday *HourSection
	for i := range sections {
		if sections[i].Name == "Psalmody-Saturday" {
			saturday = &sections[i]
			break
		}
	}
	if saturday == nil {
		t.Fatal("missing Psalmody-Saturday section")
	}

	want := []HourElement{
		{Type: "psalm", Ref: "psalms/067"},
		{Type: "gloria-patri", Ref: "ordinary/shared/gloria-patri"},
		{Type: "proper-antiphon", Ref: "psalm-antiphon-1"},
		{Type: "psalm", Ref: "psalms/051"},
		{Type: "gloria-patri", Ref: "ordinary/shared/gloria-patri"},
		{Type: "proper-antiphon", Ref: "psalm-antiphon-1"},
		{Type: "proper-antiphon", Ref: "psalm-antiphon-2"},
		{Type: "psalm", Ref: "psalms/143"},
		{Type: "gloria-patri", Ref: "ordinary/shared/gloria-patri"},
		{Type: "proper-antiphon", Ref: "psalm-antiphon-2"},
		{Type: "proper-antiphon", Ref: "psalm-antiphon-3"},
		{Type: "canticle", Ref: "canticles/deuteronomy-32"},
		{Type: "gloria-patri", Ref: "ordinary/shared/gloria-patri"},
		{Type: "proper-antiphon", Ref: "psalm-antiphon-3"},
	}

	if len(saturday.Elements) != len(want) {
		t.Fatalf("Psalmody-Saturday elements = %d, want %d", len(saturday.Elements), len(want))
	}
	for i, wantElem := range want {
		if saturday.Elements[i] != wantElem {
			t.Fatalf("Psalmody-Saturday[%d] = %+v, want %+v", i, saturday.Elements[i], wantElem)
		}
	}
}

func TestLaudsSaturdayBVMUsesFestalSaturdayPsalmody(t *testing.T) {
	path := filepath.Join("..", "..", "data", "office", "lauds.txt")
	sections, err := ParseHourDefinition(path)
	if err != nil {
		t.Fatalf("ParseHourDefinition(lauds.txt): %v", err)
	}

	var bvm *HourSection
	for i := range sections {
		if sections[i].Name == "Psalmody-Saturday-BVM" {
			bvm = &sections[i]
			break
		}
	}
	if bvm == nil {
		t.Fatal("missing Psalmody-Saturday-BVM section")
	}

	wantRefs := []string{
		"psalms/067",
		"ordinary/shared/gloria-patri",
		"saturday-psalm-antiphon-1",
		"psalms/051",
		"ordinary/shared/gloria-patri",
		"saturday-psalm-antiphon-1",
		"saturday-psalm-antiphon-2",
		"psalms/143a",
		"ordinary/shared/gloria-patri",
		"psalms/143b",
		"ordinary/shared/gloria-patri",
		"saturday-psalm-antiphon-2",
		"saturday-psalm-antiphon-3",
		"canticles/sirach-36",
		"ordinary/shared/gloria-patri",
		"saturday-psalm-antiphon-3",
	}
	if len(bvm.Elements) != len(wantRefs) {
		t.Fatalf("Psalmody-Saturday-BVM elements = %d, want %d", len(bvm.Elements), len(wantRefs))
	}
	for i, wantRef := range wantRefs {
		if bvm.Elements[i].Ref != wantRef {
			t.Fatalf("Psalmody-Saturday-BVM[%d] ref = %q, want %q", i, bvm.Elements[i].Ref, wantRef)
		}
	}
}

func TestProperAndCommonTextsExposeIndexedPsalmAntiphons(t *testing.T) {
	roots := []string{
		filepath.Join("..", "..", "data", "texts", "proper"),
		filepath.Join("..", "..", "data", "texts", "commons"),
	}

	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".txt") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(content)
			if !strings.Contains(text, "[psalm-antiphon]") {
				return nil
			}

			for i := 1; i <= 5; i++ {
				header := "[psalm-antiphon-" + string(rune('0'+i)) + "]"
				if !strings.Contains(text, header) {
					t.Fatalf("%s missing %s", path, header)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Walk(%s): %v", root, err)
		}
	}
}

func TestEasterTuesdayTextDoesNotContainRawDivinumOfficiumDirectives(t *testing.T) {
	path := filepath.Join("..", "..", "data", "texts", "proper", "easter-tuesday.txt")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "@Tempora/") || strings.HasPrefix(strings.TrimSpace(line), "ex ") {
			t.Fatalf("%s contains raw Divinum Officium directive %q", path, strings.TrimSpace(line))
		}
	}
}
