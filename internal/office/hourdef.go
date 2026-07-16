package office

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// HourElement represents a single element in an hour definition (a Type/Ref pair).
type HourElement struct {
	Type string
	Ref  string
}

// HourSection represents a named section of an hour definition with an optional condition.
type HourSection struct {
	Name            string
	Condition       string
	parsedCondition *parsedCondition
	Collapsible     bool
	Label           string
	Elements        []HourElement
}

// ParseHourDefinition reads an hour definition file and returns ordered sections
// with their elements. The format is INI-like with repeated Type/Ref pairs:
//
//	[SectionName]
//	Condition = if-preces
//
//	Type = psalm
//	Ref = psalms/004
//
//	Type = psalm
//	Ref = psalms/091
func ParseHourDefinition(path string) ([]HourSection, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening hour definition: %w", err)
	}
	defer f.Close()

	var sections []HourSection
	var current *HourSection
	pendingType := ""
	conditionLine := 0

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if current != nil {
				if pendingType != "" {
					return nil, fmt.Errorf("%s:%d: Type without matching Ref in section [%s]", path, lineNum, current.Name)
				}
				if conditionLine != 0 {
					if err := compileSectionCondition(path, conditionLine, current); err != nil {
						return nil, err
					}
				}
				sections = append(sections, *current)
			}
			name := line[1 : len(line)-1]
			current = &HourSection{Name: name}
			pendingType = ""
			conditionLine = 0
			continue
		}

		// Key = Value
		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("%s:%d: expected Key = Value, got %q", path, lineNum, line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if current == nil {
			return nil, fmt.Errorf("%s:%d: key-value before any section", path, lineNum)
		}

		switch key {
		case "Condition":
			if conditionLine != 0 {
				return nil, fmt.Errorf("%s:%d: duplicate Condition in section [%s]", path, lineNum, current.Name)
			}
			current.Condition = value
			conditionLine = lineNum
		case "Collapsible":
			current.Collapsible = value == "true"
		case "Label":
			current.Label = value
		case "Type":
			if pendingType != "" {
				return nil, fmt.Errorf("%s:%d: consecutive Type without Ref in section [%s]", path, lineNum, current.Name)
			}
			pendingType = value
		case "Ref":
			if pendingType == "" {
				return nil, fmt.Errorf("%s:%d: Ref without preceding Type in section [%s]", path, lineNum, current.Name)
			}
			current.Elements = append(current.Elements, HourElement{Type: pendingType, Ref: value})
			pendingType = ""
		default:
			return nil, fmt.Errorf("%s:%d: unknown key %q in section [%s]", path, lineNum, key, current.Name)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading hour definition: %w", err)
	}

	if current != nil {
		if pendingType != "" {
			return nil, fmt.Errorf("%s: Type without matching Ref at end of section [%s]", path, current.Name)
		}
		if conditionLine != 0 {
			if err := compileSectionCondition(path, conditionLine, current); err != nil {
				return nil, err
			}
		}
		sections = append(sections, *current)
	}

	return sections, nil
}

func compileSectionCondition(path string, line int, section *HourSection) error {
	parsed, err := parseCondition(section.Condition)
	if err != nil {
		return fmt.Errorf("%s:%d: invalid condition in section [%s]: %w", path, line, section.Name, err)
	}
	section.parsedCondition = &parsed
	return nil
}
