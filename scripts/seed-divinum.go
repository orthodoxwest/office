//go:build ignore

// Seed proper and common texts from a local Divinum Officium checkout.
//
// For each allowlisted feast (and each common), the script reads the
// corresponding DO English file (falling back through DO's @file:section
// indirection, including Latin files whose body is pure indirection),
// converts DO markup to this project's INI conventions, and:
//
//   - appends sections we lack (chapter/hymn/versicle per hour, antiphons,
//     collect, commemoration texts)
//   - replaces flat psalm-antiphon-1..5 sets (all five identical) when DO
//     has five distinct antiphons
//
// Every emitted section carries a "# SOURCE: divinum-officium" comment so
// seeded texts can be grepped and checked against the diurnal. Sections
// already present in our data are never modified (except flat antiphon sets).
//
// Usage:
//
//	go run scripts/seed-divinum.go -do ~/code/DivinumOfficium/divinum-officium [-feast id] [-write]
//
// Without -write it prints the planned changes.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Feast → DO file mapping
// ---------------------------------------------------------------------------

// temporalDOFiles maps moveable feasts (and specials) to DO file paths
// relative to the language root. Sanctoral feasts are derived from the
// proper file name's feast Month/Day via data/feasts, but since this script
// operates on text files directly, fixed-date mappings are listed here too.
var feastDOFiles = map[string]string{
	// --- the seven feasts with no proper file at all ---
	"christ-the-king":       "Sancti/10-DU",
	"exaltation-holy-cross": "Sancti/09-14",
	"vigil-pentecost":       "Tempora/Pasc6-6",
	"vigil-nativity":        "Sancti/12-24",
	"vigil-epiphany":        "Sancti/01-05",
	"vigil-st-james":        "Sancti/07-24",
	"vigil-ascension":       "Tempora/Pasc5-3",
	"advent-sunday-1":       "Tempora/Adv1-0",
	"advent-sunday-2":       "Tempora/Adv2-0",
	"advent-sunday-3":       "Tempora/Adv3-0",
	"advent-sunday-4":       "Tempora/Adv4-0",

	// --- temporal feasts with existing proper files ---
	"septuagesima":         "Tempora/Quadp1-0",
	"sexagesima":           "Tempora/Quadp2-0",
	"quinquagesima":        "Tempora/Quadp3-0",
	"ash-wednesday":        "Tempora/Quadp3-3",
	"lent-sunday-1":        "Tempora/Quad1-0",
	"lent-ember-wednesday": "Tempora/Quad1-3",
	"lent-ember-friday":    "Tempora/Quad1-5",
	"lent-ember-saturday":  "Tempora/Quad1-6",
	"lent-sunday-2":        "Tempora/Quad2-0",
	"lent-sunday-3":        "Tempora/Quad3-0",
	"laetare-sunday":       "Tempora/Quad4-0",
	"passion-sunday":       "Tempora/Quad5-0",
	"palm-sunday":          "Tempora/Quad6-0",
	"low-sunday":           "Tempora/Pasc1-0",
	"ascension":            "Tempora/Pasc5-4",
	"pentecost":            "Tempora/Pasc7-0",
	"whit-ember-wednesday": "Tempora/Pasc7-3",
	"whit-ember-friday":    "Tempora/Pasc7-5",
	"whit-ember-saturday":  "Tempora/Pasc7-6",
	"trinity-sunday":       "Tempora/Pent01-0",
	"corpus-christi":       "Tempora/Pent01-4",
	"holy-name-jesus":      "Tempora/Nat2-0",

	// --- sanctoral feasts with existing proper files ---
	"all-saints":             "Sancti/11-01",
	"all-souls":              "Sancti/11-02",
	"annunciation":           "Sancti/03-25",
	"apparition-st-michael":  "Sancti/05-08",
	"assumption-bvm":         "Sancti/08-15",
	"beheading-john-baptist": "Sancti/08-29",
	"chains-st-peter":        "Sancti/08-01",
	"chair-peter-antioch":    "Sancti/02-22",
	"chair-peter-rome":       "Sancti/01-18",
	"christmas":              "Sancti/12-25",
	"circumcision":           "Sancti/01-01",
	"conception-bvm":         "Sancti/12-08",
	"conversion-st-paul":     "Sancti/01-25",
	"dedication-st-michael":  "Sancti/09-29",
	"epiphany":               "Sancti/01-06",
	"finding-holy-cross":     "Sancti/05-03",
	"guardian-angels":        "Sancti/10-02",
	"holy-innocents":         "Sancti/12-28",
	"nativity-bvm":           "Sancti/09-08",
	"nativity-john-baptist":  "Sancti/06-24",
	"presentation-bvm":       "Sancti/11-21",
	"purification-bvm":       "Sancti/02-02",
	"ss-peter-paul":          "Sancti/06-29",
	"ss-philip-james":        "Sancti/05-01",
	"st-agatha":              "Sancti/02-05",
	"st-andrew":              "Sancti/11-30",
	"st-anne":                "Sancti/07-26",
	"st-bartholomew":         "Sancti/08-24",
	"st-cecilia":             "Sancti/11-22",
	"st-clement-i":           "Sancti/11-23",
	"st-gabriel-archangel":   "Sancti/03-24",
	"st-george":              "Sancti/04-23",
	"st-john-evangelist":     "Sancti/12-27",
	"st-joseph":              "Sancti/03-19",
	"st-lawrence":            "Sancti/08-10",
	"st-mark":                "Sancti/04-25",
	"st-martin-tours":        "Sancti/11-11",
	"st-mary-magdalene":      "Sancti/07-22",
	"st-matthew":             "Sancti/09-21",
	"st-raphael-archangel":   "Sancti/10-24",
	"st-stephen":             "Sancti/12-26",
	"transfiguration":        "Sancti/08-06",
	"visitation-bvm":         "Sancti/07-02",
}

// weekDOFiles maps temporal week IDs (the governing Sunday's office, as set
// in CalendarDay.TemporalWeekID) to DO Tempora file prefixes. The weekday
// files <prefix>-1..6 supply the per-day gospel-canticle antiphons that the
// office distributes through the week, seeded into the Sunday's proper file
// as benedictus-antiphon-<weekday> / magnificat-antiphon-<weekday>. The
// Sunday file <prefix>-0 additionally seeds the Sunday's own antiphons
// (Ant 1 = I Vespers Magnificat, Ant 2 = Benedictus, Ant 3 = II Vespers
// Magnificat) when our file lacks them. Saturday Magnificat antiphons are
// not seeded: Saturday evening is I Vespers of the Sunday.
//
// Per-annum weeks (after Epiphany and Pentecost) and most Paschal weeks have
// no weekday antiphons in DO — the monastic weekly distribution must come
// from the diurnal — but their Sunday files still carry the Sunday's own
// gospel antiphons (Ant 2/Ant 3), so they are listed and the weekday lookups
// simply find nothing. pentecost-sunday-1 does not exist as a proper:
// Trinity Sunday occupies that week with its own office.
var weekDOFiles = map[string]string{
	"advent-sunday-1":                "Tempora/Adv1",
	"advent-sunday-2":                "Tempora/Adv2",
	"advent-sunday-3":                "Tempora/Adv3",
	"advent-sunday-4":                "Tempora/Adv4",
	"septuagesima":                   "Tempora/Quadp1",
	"sexagesima":                     "Tempora/Quadp2",
	"quinquagesima":                  "Tempora/Quadp3",
	"lent-sunday-1":                  "Tempora/Quad1",
	"lent-sunday-2":                  "Tempora/Quad2",
	"lent-sunday-3":                  "Tempora/Quad3",
	"laetare-sunday":                 "Tempora/Quad4",
	"passion-sunday":                 "Tempora/Quad5",
	"low-sunday":                     "Tempora/Pasc1",
	"easter-sunday-2":                "Tempora/Pasc2",
	"easter-sunday-3":                "Tempora/Pasc3",
	"easter-sunday-4":                "Tempora/Pasc4",
	"easter-sunday-5":                "Tempora/Pasc5",
	"ascension-sunday-within-octave": "Tempora/Pasc6",
	"epiphany-sunday-1":              "Tempora/Epi1",
	"epiphany-sunday-2":              "Tempora/Epi2",
	"epiphany-sunday-3":              "Tempora/Epi3",
	"epiphany-sunday-4":              "Tempora/Epi4",
	"epiphany-sunday-5":              "Tempora/Epi5",
	"epiphany-sunday-6":              "Tempora/Epi6",
	"pentecost-sunday-2":             "Tempora/Pent02",
	"pentecost-sunday-3":             "Tempora/Pent03",
	"pentecost-sunday-4":             "Tempora/Pent04",
	"pentecost-sunday-5":             "Tempora/Pent05",
	"pentecost-sunday-6":             "Tempora/Pent06",
	"pentecost-sunday-7":             "Tempora/Pent07",
	"pentecost-sunday-8":             "Tempora/Pent08",
	"pentecost-sunday-9":             "Tempora/Pent09",
	"pentecost-sunday-10":            "Tempora/Pent10",
	"pentecost-sunday-11":            "Tempora/Pent11",
	"pentecost-sunday-12":            "Tempora/Pent12",
	"pentecost-sunday-13":            "Tempora/Pent13",
	"pentecost-sunday-14":            "Tempora/Pent14",
	"pentecost-sunday-15":            "Tempora/Pent15",
	"pentecost-sunday-16":            "Tempora/Pent16",
	"pentecost-sunday-17":            "Tempora/Pent17",
	"pentecost-sunday-18":            "Tempora/Pent18",
	"pentecost-sunday-19":            "Tempora/Pent19",
	"pentecost-sunday-20":            "Tempora/Pent20",
	"pentecost-sunday-21":            "Tempora/Pent21",
	"pentecost-sunday-22":            "Tempora/Pent22",
	"pentecost-sunday-23":            "Tempora/Pent23",
	"pentecost-sunday-24":            "Tempora/Pent24",
}

// noFirstVespersMagWeeks lists weeks whose DO Sunday file's Ant 1 must NOT be
// seeded as magnificat-antiphon-first. For the Epiphany weeks DO stores the
// ferial Saturday Magnificat antiphon there (the engine already falls back to
// ordinary/vespers/magnificat-antiphon-saturday); seeding it would shadow the
// Coverdale wording. pentecost-sunday-2's eve belongs to the Corpus Christi
// octave. For the remaining Pentecost weeks Ant 1 (or the conditional
// Ant 1_) is the summer historia antiphon, which the engine uses only until
// the August scripture-month begins.
var noFirstVespersMagWeeks = map[string]bool{
	"epiphany-sunday-1":  true,
	"epiphany-sunday-2":  true,
	"epiphany-sunday-3":  true,
	"epiphany-sunday-4":  true,
	"epiphany-sunday-5":  true,
	"epiphany-sunday-6":  true,
	"pentecost-sunday-2": true,
}

// noWeekdayAntWeeks lists weeks whose DO weekday files must not seed the
// per-day antiphons. DO's Pent02-5 is the feast of the Sacred Heart (the
// Friday after the Corpus Christi octave), which the parish does not observe
// (2026 ordo: a green feria); seeding it would put its antiphons on that
// Friday via the weekly-temporal tier.
var noWeekdayAntWeeks = map[string]bool{
	"pentecost-sunday-2": true,
}

// specialSundayDOFiles supplies Sunday offices that do not share the
// ordinary numbered-Sunday source. The Sunday within the Epiphany octave is
// Epi1-0a, distinct from the Holy Family material in Epi1-0.
var specialSundayDOFiles = map[string]string{
	"epiphany-sunday-within-octave": "Tempora/Epi1-0a",
}

// historiaDOFiles maps our proper/historia-<month>-<week> antiphon files to
// the DO month-week Tempora prefixes. The Sunday file's Ant 1 is the
// Magnificat antiphon of the scripture historia, sung at the Saturday
// I Vespers of the week's Sunday (calendar.HistoriaWeekID picks the week).
var historiaDOFiles = map[string]string{
	"historia-august-1":    "Tempora/081",
	"historia-august-2":    "Tempora/082",
	"historia-august-3":    "Tempora/083",
	"historia-august-4":    "Tempora/084",
	"historia-august-5":    "Tempora/085",
	"historia-september-1": "Tempora/091",
	"historia-september-2": "Tempora/092",
	"historia-september-3": "Tempora/093",
	"historia-september-4": "Tempora/094",
	"historia-september-5": "Tempora/095",
	"historia-october-1":   "Tempora/101",
	"historia-october-2":   "Tempora/102",
	"historia-october-3":   "Tempora/103",
	"historia-october-4":   "Tempora/104",
	"historia-october-5":   "Tempora/105",
	"historia-november-1":  "Tempora/111",
	"historia-november-2":  "Tempora/112",
	"historia-november-3":  "Tempora/113",
	"historia-november-4":  "Tempora/114",
	"historia-november-5":  "Tempora/115",
}

// octaveDayDOFiles maps per-day octave feast IDs to the DO file carrying that
// day's proper antiphons, for octaves where DO has a distinct office per day
// (unlike the in-course …-octave-set-N mechanism).
var octaveDayDOFiles = map[string]string{
	"easter-sunday-octave-day-4": "Tempora/Pasc0-3",
	"easter-sunday-octave-day-5": "Tempora/Pasc0-4",
	"easter-sunday-octave-day-6": "Tempora/Pasc0-5",
	"easter-sunday-octave-day-7": "Tempora/Pasc0-6",
	"pentecost-octave-day-2":     "Tempora/Pasc7-1",
	"pentecost-octave-day-3":     "Tempora/Pasc7-2",
	"pentecost-octave-day-4":     "Tempora/Pasc7-3",
	"pentecost-octave-day-5":     "Tempora/Pasc7-4",
	"pentecost-octave-day-6":     "Tempora/Pasc7-5",
	"pentecost-octave-day-7":     "Tempora/Pasc7-6",
}

// paschalCommonsAnts maps our paschal commons variant files to the DO file
// carrying the shared Paschaltide gospel-canticle antiphons. In Paschaltide
// apostles, evangelists and martyrs share one common (DO C1p: "Daughters of
// Jerusalem" at the Benedictus, "Saints and just" at the Magnificat); DO's
// C2p/C3p carry no antiphon sections of their own.
var paschalCommonsAnts = map[string]string{
	"apostle-paschal":       "Commune/C1p",
	"evangelist-paschal":    "Commune/C1p",
	"martyr-paschal":        "Commune/C1p",
	"bishop-martyr-paschal": "Commune/C1p",
}

// commonsDOFiles maps our commons files to DO Commune files.
var commonsDOFiles = map[string]string{
	"apostle":          "Commune/C1",
	"evangelist":       "Commune/C1a",
	"bishop-martyr":    "Commune/C2",
	"martyr":           "Commune/C2a",
	"confessor-bishop": "Commune/C4",
	"confessor-doctor": "Commune/C4a",
	"confessor":        "Commune/C5",
	"virgin-martyr":    "Commune/C6",
	"virgin":           "Commune/C6a",
	"holy-woman":       "Commune/C7a",
	"blessed-virgin":   "Commune/C11",
	"dedication":       "Commune/C8",
}

// antiphonDOSources overrides where psalm antiphons resolve from, for days
// whose own DO file carries no Ant Laudes/Ant Vespera section. The Whitsun
// Ember days repeat the Pentecost octave antiphons, All Souls takes the
// Office of the Dead, and the Chair feasts use the common of a Confessor
// Bishop as DO does. (Ash Wednesday and the Lent Ember days carry no proper
// antiphons at all: as ferias they take the weekday psalter antiphons in
// data/texts/ordinary/.) An empty section means the default
// "Ant Laudes"/"Ant Vespera" resolution against the override file.
var antiphonDOSources = map[string]struct{ file, section string }{
	"all-souls":            {"Commune/C9", "Ant Laudes_"},
	"whit-ember-wednesday": {"Tempora/Pasc7-0", ""},
	"whit-ember-friday":    {"Tempora/Pasc7-0", ""},
	"whit-ember-saturday":  {"Tempora/Pasc7-0", ""},
	"st-joseph":            {"Sancti/03-19", "Ant Laudes"},
	"chair-peter-antioch":  {"Commune/C4", ""},
	"chair-peter-rome":     {"Commune/C4", ""},
}

// createFor lists feast IDs for which a new proper file may be created
// (the audit's "missing propers" feasts). All other feasts only have
// existing files extended.
var createFor = map[string]bool{
	"christ-the-king":       true,
	"exaltation-holy-cross": true,
	"vigil-pentecost":       true,
	"vigil-nativity":        true,
	"vigil-epiphany":        true,
	"vigil-st-james":        true,
	"vigil-ascension":       true,
	"advent-sunday-1":       true,
	"advent-sunday-2":       true,
	"advent-sunday-3":       true,
	"advent-sunday-4":       true,
}

// auditedRefs are the refs `make audit` requires per feast; missing ones
// in newly created files get a TODO(diurnal) scaffold comment.
var auditedRefs = []string{
	"psalm-antiphon",
	"benedictus-antiphon",
	"magnificat-antiphon",
	"collect",
	"commemoration-antiphon",
	"commemoration-versicle",
}

// ---------------------------------------------------------------------------
// DO file parsing
// ---------------------------------------------------------------------------

type doTree struct {
	root  string // e.g. <do>/web/www/horas/English
	cache map[string]map[string][]string
}

var (
	sectionRE = regexp.MustCompile(`^\[(.+?)\]\s*(\(.*\))?\s*$`)
	// body-level conditional marker like "(sed rubrica cisterciensis ...)".
	// Alone on a line, the remainder of the section applies to other rubric
	// versions only; prefixed to a line, that single line is the variant.
	condMarkRE = regexp.MustCompile(`^\([^)]*rubrica[^)]*\)`)
	atRefRE    = regexp.MustCompile(`^@([^:]*)(?::([^:]*))?(?::(.*))?$`)
	// substChunkRE matches one s/// substitution inside an @-ref's third
	// field (chunks may be separated by ";;" or whitespace, and the pattern
	// itself may contain ";;", so the field cannot simply be split).
	substChunkRE = regexp.MustCompile(`s/((?:[^/\\]|\\.)*)/((?:[^/\\]|\\.)*)/([gims]*)`)
	// selectorRE matches a leading line selector like "1" or "2-4,7".
	selectorRE = regexp.MustCompile(`^([0-9]+(?:-[0-9]+)?(?:,[0-9]+(?:-[0-9]+)?)*)\s*(.*)$`)
)

func newDOTree(root string) *doTree {
	return &doTree{root: root, cache: map[string]map[string][]string{}}
}

// inheritKey is the pseudo-section under which a file-level inheritance
// target is stored: a bare "@Other/File" line before any [section] means
// sections not defined locally come from that file.
const inheritKey = "\x00inherit"

// file parses a DO data file into base sections (rubric-conditional section
// variants are dropped; bodies are truncated at rubric-conditional lines).
func (t *doTree) file(rel string) map[string][]string {
	if f, ok := t.cache[rel]; ok {
		return f
	}
	data, err := os.ReadFile(filepath.Join(t.root, rel+".txt"))
	if err != nil {
		t.cache[rel] = nil
		return nil
	}
	sections := map[string][]string{}
	var cur string
	skip := false
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if m := sectionRE.FindStringSubmatch(line); m != nil {
			cur = m[1]
			skip = m[2] != "" // conditional section variant: keep base only
			if _, exists := sections[cur]; !exists && !skip {
				sections[cur] = []string{}
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if cur == "" {
			// File-level inheritance: a bare @File line before any section.
			if strings.HasPrefix(trimmed, "@") && !strings.Contains(trimmed, ":") {
				sections[inheritKey] = []string{strings.TrimPrefix(trimmed, "@")}
			}
			continue
		}
		if skip {
			continue
		}
		if mark := condMarkRE.FindString(trimmed); mark != "" {
			if mark == trimmed {
				skip = true // rest of section applies to other rubric versions
			}
			continue // prefixed marker: drop the single variant line
		}
		sections[cur] = append(sections[cur], line)
	}
	t.cache[rel] = sections
	return sections
}

// rawSection returns the unexpanded lines of a section, following file-level
// inheritance. The returned rel is the file the section was actually found in.
func (t *doTree) rawSection(rel, name string, depth int) ([]string, string, bool) {
	if depth > 6 {
		return nil, "", false
	}
	f := t.file(rel)
	if f == nil {
		return nil, "", false
	}
	if body, ok := f[name]; ok {
		return body, rel, true
	}
	if inh, ok := f[inheritKey]; ok && len(inh) == 1 {
		return t.rawSection(inh[0], name, depth+1)
	}
	return nil, "", false
}

// resolved text provenance
type provenance struct {
	fromCommune bool
	viaLatin    bool
}

// section returns the raw (untransformed) lines of a DO section, following
// @file:section indirection within this tree. Returns nil if unresolvable.
func (t *doTree) section(rel, name string, depth int, prov *provenance) []string {
	if depth > 6 {
		return nil
	}
	body, foundRel, ok := t.rawSection(rel, name, 0)
	if !ok {
		return nil
	}
	var out []string
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if m := atRefRE.FindStringSubmatch(trimmed); m != nil && strings.HasPrefix(trimmed, "@") {
			refFile, refSect, subst := m[1], m[2], m[3]
			if refFile == "" {
				refFile = foundRel
			}
			if refSect == "" {
				refSect = name
			}
			if strings.HasPrefix(refFile, "Commune/") {
				prov.fromCommune = true
			}
			inc := t.section(refFile, refSect, depth+1, prov)
			if inc == nil {
				return nil // unresolvable indirection: give up on section
			}
			if subst != "" {
				inc = applySubstitutions(inc, subst)
				if inc == nil {
					return nil
				}
			}
			out = append(out, inc...)
			continue
		}
		out = append(out, line)
	}
	return out
}

// applySubstitutions applies an @-ref's third field to resolved lines.
// The field is an optional 1-based line selector ("1", "2-4,7") followed by
// zero or more s/// substitutions. Returns nil on unsupported syntax.
func applySubstitutions(lines []string, subst string) []string {
	subst = strings.TrimSpace(subst)
	if m := selectorRE.FindStringSubmatch(subst); m != nil && m[1] != "" {
		lines = selectLines(lines, m[1])
		if lines == nil {
			return nil
		}
		subst = m[2]
	}

	chunks := substChunkRE.FindAllStringSubmatch(subst, -1)
	// Verify the field contains nothing but the matched chunks and separators.
	leftover := substChunkRE.ReplaceAllString(subst, "")
	leftover = strings.ReplaceAll(leftover, ";;", "")
	if strings.TrimSpace(leftover) != "" {
		return nil // unsupported substitution syntax
	}

	text := strings.Join(lines, "\n")
	for _, m := range chunks {
		pat, repl, flags := m[1], m[2], m[3]
		var sb strings.Builder
		if strings.Contains(flags, "i") {
			sb.WriteString("(?i)")
		}
		if strings.Contains(flags, "s") {
			sb.WriteString("(?s)")
		}
		if strings.Contains(flags, "m") {
			sb.WriteString("(?m)")
		}
		sb.WriteString(pat)
		re, err := regexp.Compile(sb.String())
		if err != nil {
			return nil
		}
		repl = strings.ReplaceAll(repl, "$", "$$")
		repl = regexp.MustCompile(`\\(\d)`).ReplaceAllString(repl, "$$$1")
		if strings.Contains(flags, "g") {
			text = re.ReplaceAllString(text, repl)
		} else if loc := re.FindStringIndex(text); loc != nil {
			text = text[:loc[0]] + re.ReplaceAllString(text[loc[0]:loc[1]], repl) + text[loc[1]:]
		}
	}
	return strings.Split(text, "\n")
}

// selectLines applies a 1-based line selector ("1", "2-4,7") counting only
// non-blank lines, matching DO's @File:Section:N convention.
func selectLines(lines []string, selector string) []string {
	var content []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			content = append(content, l)
		}
	}
	var out []string
	for _, part := range strings.Split(selector, ",") {
		lo, hi := 0, 0
		if _, err := fmt.Sscanf(part, "%d-%d", &lo, &hi); err != nil {
			if _, err := fmt.Sscanf(part, "%d", &lo); err != nil {
				return nil
			}
			hi = lo
		}
		for i := lo; i <= hi; i++ {
			if i < 1 || i > len(content) {
				return nil
			}
			out = append(out, content[i-1])
		}
	}
	return out
}

// resolveSection looks up a section in English; if absent there, it consults
// the Latin file and accepts it only when the body is pure indirection that
// resolves through the English tree (DO stores shared structure in Latin).
func resolveSection(eng, lat *doTree, rel, name string) ([]string, provenance) {
	var prov provenance
	if lines := eng.section(rel, name, 0, &prov); lines != nil {
		return lines, prov
	}
	body, foundRel, ok := lat.rawSection(rel, name, 0)
	if !ok {
		return nil, prov
	}
	// Accept only if every content line is an @-indirection.
	var out []string
	prov = provenance{viaLatin: true}
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "@") {
			return nil, prov // genuine untranslated Latin text
		}
		m := atRefRE.FindStringSubmatch(trimmed)
		if m == nil {
			return nil, prov
		}
		refFile, refSect, subst := m[1], m[2], m[3]
		if refFile == "" {
			refFile = foundRel
		}
		if refSect == "" {
			refSect = name
		}
		if strings.HasPrefix(refFile, "Commune/") {
			prov.fromCommune = true
		}
		inc := eng.section(refFile, refSect, 0, &prov)
		if inc == nil {
			return nil, prov
		}
		if subst != "" {
			inc = applySubstitutions(inc, subst)
			if inc == nil {
				return nil, prov
			}
		}
		out = append(out, inc...)
	}
	if out == nil {
		return nil, prov
	}
	return out, prov
}

// ---------------------------------------------------------------------------
// DO markup → our text conventions
// ---------------------------------------------------------------------------

var inlineTagRE = regexp.MustCompile(`\{[^}]*\}`)

// vrSpaceRE fixes DO typos like "R.Blessed" (missing space after V./R.).
var vrSpaceRE = regexp.MustCompile(`^([VR])\.(\S)`)

// quoteNormalizer maps DO's typographic punctuation to the corpus house
// style (straight quotes).
var quoteNormalizer = strings.NewReplacer("’", "'", "‘", "'", "“", `"`, "”", `"`)

// latinFolder folds accented Latin to the plain spelling used by existing
// hymn incipit lines (e.g. "Jam Christe, sol justitiae").
var latinFolder = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ý", "y",
	"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U",
	"æ", "ae", "Æ", "Ae", "œ", "oe", "Œ", "Oe",
)

const gloriaLine = "Glory be to the Father, and to the Son, and to the Holy Ghost."

// transform converts DO body lines into our format. kind selects
// kind-specific handling (chapter keeps the !citation line, hymn keeps
// stanza breaks, etc.). Returns nil if a line cannot be converted.
func transform(lines []string, kind string) []string {
	var out []string
	pendingJoin := ""
	push := func(s string) {
		if pendingJoin != "" {
			s = pendingJoin + " " + strings.TrimSpace(s)
			pendingJoin = ""
		}
		out = append(out, s)
	}
	for i, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		line = inlineTagRE.ReplaceAllString(line, "")
		line = quoteNormalizer.Replace(line)
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "":
			if kind == "hymn" && len(out) > 0 {
				out = append(out, "")
			}
			continue
		case trimmed == "_":
			if len(out) > 0 {
				out = append(out, "")
			}
			continue
		case strings.HasPrefix(trimmed, "!"):
			// Citation line: keep for chapters (same syntax as our files),
			// drop rubric lines elsewhere.
			if kind == "chapter" && i == 0 {
				push(trimmed)
			}
			continue
		case strings.HasPrefix(trimmed, "$"):
			macro := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(trimmed, "$")), ".")
			switch {
			case strings.EqualFold(macro, "Deo gratias"):
				push("R. Thanks be to God.")
			case strings.EqualFold(macro, "Amen"):
				push("R. Amen.")
			default:
				// collect conclusions ($Per Dominum etc.) and similar: drop
			}
			continue
		case strings.HasPrefix(trimmed, "&"):
			if strings.HasPrefix(trimmed, "&Gloria") || strings.HasPrefix(trimmed, "&gloria") {
				push(gloriaLine)
			}
			continue
		}

		// strip psalm-assignment suffixes used by the monastic files
		// (e.g. "Stetit Angelus ... manu sua.;;98")
		if idx := strings.Index(trimmed, ";;"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
			if trimmed == "" {
				continue
			}
		}

		// strip DO reader-part prefixes
		trimmed = strings.TrimPrefix(trimmed, "v. ")
		trimmed = strings.TrimPrefix(trimmed, "r. ")
		trimmed = strings.ReplaceAll(trimmed, "R.br.", "R.")
		trimmed = vrSpaceRE.ReplaceAllString(trimmed, "$1. $2")

		if strings.HasSuffix(trimmed, "~") {
			pendingJoin += strings.TrimSpace(strings.TrimSuffix(trimmed, "~")) + " "
			pendingJoin = strings.TrimSuffix(pendingJoin, " ")
			continue
		}
		push(trimmed)
	}
	if pendingJoin != "" {
		out = append(out, pendingJoin)
	}
	// trim trailing blank lines
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// hymnIncipit fetches the Latin first line of a hymn section, for use as
// the title line our hymn format expects.
func hymnIncipit(lat *doTree, rel, name string) string {
	var prov provenance
	lines := lat.section(rel, name, 0, &prov)
	for _, raw := range lines {
		t := strings.TrimSpace(inlineTagRE.ReplaceAllString(raw, ""))
		if t == "" || t == "_" || strings.HasPrefix(t, "!") || strings.HasPrefix(t, "@") ||
			strings.HasPrefix(t, "$") || strings.HasPrefix(t, "&") || strings.HasPrefix(t, "(") {
			continue
		}
		t = strings.TrimPrefix(t, "v. ")
		t = strings.TrimRight(t, " ,.;:*")
		return latinFolder.Replace(t)
	}
	return ""
}

// ---------------------------------------------------------------------------
// Seed extraction
// ---------------------------------------------------------------------------

type seed struct {
	key   string // our section name
	cite  string // DO file + section it came from, e.g. `SanctiM/09-29 [Ant Laudes]`
	lines []string
	prov  provenance
}

// relCandidates orders DO files monastic-first: this is a Benedictine
// office, and DO keeps monastic variants in the *M trees (which inherit
// from the Roman files for everything they don't override).
func relCandidates(rel string) []string {
	for _, dir := range []string{"Sancti", "Tempora", "Commune"} {
		if rest, ok := strings.CutPrefix(rel, dir+"/"); ok {
			return []string{dir + "M/" + rest, rel}
		}
	}
	return []string{rel}
}

// extractSeeds builds the candidate sections for one DO file.
func extractSeeds(eng, lat *doTree, rel, id string) []seed {
	rels := relCandidates(rel)
	var seeds []seed

	// resolveIn tries each DO section name across the given candidate files.
	resolveIn := func(files []string, doSections ...string) ([]string, string, string, provenance) {
		for _, r := range files {
			for _, name := range doSections {
				if lines, prov := resolveSection(eng, lat, r, name); lines != nil {
					return lines, r + " [" + name + "]", name, prov
				}
			}
		}
		return nil, "", "", provenance{}
	}
	resolve := func(doSections ...string) ([]string, string, string, provenance) {
		return resolveIn(rels, doSections...)
	}

	add := func(key, kind string, doSections ...string) {
		lines, cite, matched, prov := resolve(doSections...)
		if lines == nil {
			return
		}
		body := transform(lines, kind)
		if len(body) == 0 {
			return
		}
		if kind == "hymn" {
			for _, r := range rels {
				if inc := hymnIncipit(lat, r, matched); inc != "" {
					body = append([]string{inc, ""}, body...)
					break
				}
			}
		}
		seeds = append(seeds, seed{key: key, cite: cite, lines: body, prov: prov})
	}

	// Psalm antiphons: prefer Lauds, fall back to Vespers antiphons.
	// Monastic Lauds has four psalm antiphons plus the Laudate antiphon;
	// Roman files carry five. Accept either.
	antRels, antSections := rels, []string{"Ant Laudes", "Ant Vespera"}
	if ov, ok := antiphonDOSources[id]; ok {
		antRels = []string{ov.file}
		if ov.section != "" {
			antSections = []string{ov.section}
		}
	}
	antLines, antCite, _, antProv := resolveIn(antRels, antSections...)
	if antLines != nil {
		ants := transform(antLines, "antiphon")
		var nonEmpty []string
		for _, a := range ants {
			// Drop inline rubric-conditional variants ("(sed rubrica 1960) …").
			if strings.TrimSpace(a) != "" && !strings.HasPrefix(strings.TrimSpace(a), "(") {
				nonEmpty = append(nonEmpty, a)
			}
		}
		if n := len(nonEmpty); n == 4 || n == 5 {
			for i, a := range nonEmpty {
				seeds = append(seeds, seed{
					key:   fmt.Sprintf("psalm-antiphon-%d", i+1),
					cite:  antCite,
					lines: []string{a},
					prov:  antProv,
				})
			}
			seeds = append(seeds, seed{key: "psalm-antiphon", cite: antCite, lines: []string{nonEmpty[0]}, prov: antProv})
		} else if n > 0 {
			fmt.Printf("    note: %s has %d antiphons (want 4 or 5), skipped\n", antCite, n)
		}
	}

	add("benedictus-antiphon", "antiphon", "Ant 2")
	add("magnificat-antiphon", "antiphon", "Ant 3")
	add("collect", "collect", "Oratio")
	add("chapter-lauds", "chapter", "Capitulum Laudes")
	add("chapter-vespers", "chapter", "Capitulum Vespera")
	add("chapter-terce", "chapter", "Capitulum Tertia")
	add("chapter-sext", "chapter", "Capitulum Sexta")
	add("chapter-none", "chapter", "Capitulum Nona")
	add("hymn-lauds", "hymn", "HymnusM Laudes", "Hymnus Laudes")
	add("hymn-vespers", "hymn", "HymnusM Vespera", "Hymnus Vespera")
	add("versicle-lauds", "versicle", "Versum 2")
	add("versicle-vespers", "versicle", "Versum 1")
	add("short-responsory-terce", "responsory", "Responsory Breve Tertia")
	add("short-responsory-sext", "responsory", "Responsory Breve Sexta")
	add("short-responsory-none", "responsory", "Responsory Breve Nona")
	add("commemoration-antiphon", "antiphon", "Ant 2")
	add("commemoration-versicle", "versicle", "Versum 2")

	// Vespers and Terce chapters default to the Lauds chapter in the
	// Roman office when the feast supplies none of their own.
	have := map[string]bool{}
	for _, s := range seeds {
		have[s.key] = true
	}
	for _, hour := range []string{"vespers", "terce"} {
		if have["chapter-lauds"] && !have["chapter-"+hour] {
			for _, s := range seeds {
				if s.key == "chapter-lauds" {
					seeds = append(seeds, seed{key: "chapter-" + hour, cite: s.cite + " (= Lauds chapter)", lines: s.lines, prov: s.prov})
					break
				}
			}
		}
	}
	return seeds
}

// antiphonSeed resolves one DO antiphon section (monastic-first, English
// then Latin-indirection) and returns it as a seed for our key, or nil.
func antiphonSeed(eng, lat *doTree, rel, doSection, key string) *seed {
	for _, r := range relCandidates(rel) {
		lines, prov := resolveSection(eng, lat, r, doSection)
		if lines == nil {
			continue
		}
		body := transform(lines, "antiphon")
		if len(body) == 0 {
			return nil
		}
		return &seed{key: key, cite: r + " [" + doSection + "]", lines: body, prov: prov}
	}
	return nil
}

// extractWeekSeeds builds the weekday (and, for files we lack, Sunday)
// antiphon sections for one temporal week from DO's <prefix>-0..6 files.
// seedFirst controls whether the Sunday file's Ant 1 (or the conditional
// Ant 1_) is seeded as the I Vespers magnificat-antiphon-first; seedWeekdays
// whether the per-day antiphons are taken from the weekday files.
func extractWeekSeeds(eng, lat *doTree, prefix string, seedFirst, seedWeekdays bool) []seed {
	weekdays := []string{"", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	var seeds []seed
	for d := 1; seedWeekdays && d <= 6; d++ {
		rel := fmt.Sprintf("%s-%d", prefix, d)
		if sd := antiphonSeed(eng, lat, rel, "Ant 2", "benedictus-antiphon-"+weekdays[d]); sd != nil {
			seeds = append(seeds, *sd)
		}
		// Saturday evening is I Vespers of the Sunday, so a Saturday file's
		// Ant 3 (where present) is not this week's ferial Magnificat.
		if d < 6 {
			if sd := antiphonSeed(eng, lat, rel, "Ant 3", "magnificat-antiphon-"+weekdays[d]); sd != nil {
				seeds = append(seeds, *sd)
			}
		}
	}

	return append(seeds, extractSundaySeeds(eng, lat, prefix+"-0", seedFirst)...)
}

// extractSundaySeeds builds a Sunday's gospel-canticle antiphons from its
// office file. Some Sunday files label the Lauds antiphon Ant 1 rather than
// Ant 2, so fall back accordingly.
func extractSundaySeeds(eng, lat *doTree, sunday string, seedFirst bool) []seed {
	var seeds []seed
	ben := antiphonSeed(eng, lat, sunday, "Ant 2", "benedictus-antiphon")
	if ben == nil {
		// Sundays whose DO file carries only Ant 1 use it at the Benedictus.
		ben = antiphonSeed(eng, lat, sunday, "Ant 1", "benedictus-antiphon")
	}
	if ben != nil {
		seeds = append(seeds, *ben)
	}
	mag := antiphonSeed(eng, lat, sunday, "Ant 3", "magnificat-antiphon")
	if mag != nil {
		seeds = append(seeds, *mag)
	}
	if seedFirst {
		first := antiphonSeed(eng, lat, sunday, "Ant 1", "magnificat-antiphon-first")
		if first == nil {
			// Conditional variant: the summer historia antiphon of the
			// numbered Pentecost weeks, displaced once the August
			// scripture-month begins.
			first = antiphonSeed(eng, lat, sunday, "Ant 1_", "magnificat-antiphon-first")
		}
		if first != nil &&
			(mag == nil || strings.Join(first.lines, "\n") != strings.Join(mag.lines, "\n")) {
			seeds = append(seeds, *first)
		}
	}
	return seeds
}

// extractOAntiphonSeeds pulls the Greater ("O") Antiphons of December 17-23
// from DO's Major Special file, keyed by date for the Advent seasonal tier.
func extractOAntiphonSeeds(eng, lat *doTree) []seed {
	const rel = "Psalterium/Special/Major Special"
	var seeds []seed
	for day := 17; day <= 23; day++ {
		doSection := fmt.Sprintf("Adv Ant %d", day)
		key := fmt.Sprintf("magnificat-antiphon-december-%d", day)
		if sd := antiphonSeed(eng, lat, rel, doSection, key); sd != nil {
			seeds = append(seeds, *sd)
		}
	}
	return seeds
}

// ---------------------------------------------------------------------------
// Our INI files: parse + merge
// ---------------------------------------------------------------------------

type iniSection struct {
	name string
	span []string // raw lines including header
}

func parseINI(path string) (head []string, sections []iniSection, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var cur *iniSection
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inner := trimmed[1 : len(trimmed)-1]
			if len(inner) > 0 && !strings.ContainsAny(inner, " :\t") {
				sections = append(sections, iniSection{name: inner, span: []string{line}})
				cur = &sections[len(sections)-1]
				continue
			}
		}
		if cur == nil {
			head = append(head, line)
		} else {
			cur.span = append(cur.span, line)
		}
	}
	return head, sections, nil
}

func sectionBody(s iniSection) string {
	var lines []string
	for _, l := range s.span[1:] {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "#") {
			continue
		}
		lines = append(lines, l)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func sourceComment(cite string) string {
	return "# SOURCE: divinum-officium " + cite + " — check against diurnal"
}

// mergeFile applies seeds to one of our INI files. Returns the new content
// (or "" if no changes) plus human-readable change notes. forceAnts allows
// appending psalm-antiphon sections to an existing file (used when an
// explicit antiphon source override exists for the day).
func mergeFile(path string, seeds []seed, skipCommuneDerived, forceAnts bool) (string, []string) {
	var head []string
	var sections []iniSection
	exists := true
	if h, s, err := parseINI(path); err == nil {
		head, sections = h, s
	} else if os.IsNotExist(err) {
		exists = false
	} else {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
		return "", nil
	}

	byName := map[string]*iniSection{}
	for i := range sections {
		byName[sections[i].name] = &sections[i]
	}

	var notes []string

	// Flat antiphon replacement: psalm-antiphon-1..5 all present and identical.
	flat := exists
	var flatText string
	for i := 1; i <= 5; i++ {
		s, ok := byName[fmt.Sprintf("psalm-antiphon-%d", i)]
		if !ok {
			flat = false
			break
		}
		b := sectionBody(*s)
		if i == 1 {
			flatText = b
		} else if b != flatText {
			flat = false
			break
		}
	}
	seedAnts := map[string]seed{}
	distinct := map[string]bool{}
	for _, s := range seeds {
		if strings.HasPrefix(s.key, "psalm-antiphon-") {
			seedAnts[s.key] = s
			distinct[strings.Join(s.lines, "\n")] = true
		}
	}
	replacedFlat := false
	if flat && len(seedAnts) > 0 && len(seedAnts) != 5 {
		notes = append(notes, fmt.Sprintf("note: only %d antiphons seeded (want 5); flat set left in place", len(seedAnts)))
	}
	if flat && len(seedAnts) == 5 && len(distinct) >= 4 {
		for i := 1; i <= 5; i++ {
			key := fmt.Sprintf("psalm-antiphon-%d", i)
			sec := byName[key]
			sd := seedAnts[key]
			sec.span = append([]string{sec.span[0], sourceComment(sd.cite)}, sd.lines...)
			sec.span = append(sec.span, "")
		}
		if base, ok := byName["psalm-antiphon"]; ok && sectionBody(*base) == flatText {
			sd := seedAnts["psalm-antiphon-1"]
			base.span = append([]string{base.span[0], sourceComment(sd.cite)}, sd.lines...)
			base.span = append(base.span, "")
		}
		replacedFlat = true
		notes = append(notes, "replaced flat psalm-antiphon-1..5")
	}

	// Append missing sections.
	var appended []seed
	for _, sd := range seeds {
		if _, ok := byName[sd.key]; ok {
			continue
		}
		if strings.HasPrefix(sd.key, "psalm-antiphon") && exists && !replacedFlat && !forceAnts {
			continue // don't bolt antiphons onto files that manage their own
		}
		if forceAnts && strings.HasPrefix(sd.key, "psalm-antiphon") {
			// An explicit antiphon source override names a Commune file on
			// purpose (e.g. the Chair feasts taking the Confessor Bishop
			// antiphons); don't defer to the category commons tier.
			appended = append(appended, sd)
			continue
		}
		if skipCommuneDerived && sd.prov.fromCommune {
			notes = append(notes, fmt.Sprintf("skip %s (resolves to Commune — covered by commons tier)", sd.key))
			continue
		}
		appended = append(appended, sd)
	}

	if !replacedFlat && len(appended) == 0 {
		return "", notes
	}

	var out []string
	out = append(out, head...)
	for _, s := range sections {
		out = append(out, s.span...)
	}
	// normalize: single blank line between everything
	text := strings.TrimRight(strings.Join(out, "\n"), "\n")
	for _, sd := range appended {
		if text != "" {
			text += "\n\n"
		}
		text += "[" + sd.key + "]\n" + sourceComment(sd.cite) + "\n" + strings.Join(sd.lines, "\n")
		notes = append(notes, "added "+sd.key)
	}
	text += "\n"
	return text, notes
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	doRoot := flag.String("do", "", "path to DivinumOfficium checkout root")
	dataDir := flag.String("data", "data", "path to data dir")
	only := flag.String("feast", "", "process a single feast or common id")
	write := flag.Bool("write", false, "write changes (default: dry run)")
	flag.Parse()
	if *doRoot == "" {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/seed-divinum.go -do <path-to-DO-checkout> [-feast id] [-write]")
		os.Exit(1)
	}

	horas := filepath.Join(*doRoot, "web", "www", "horas")
	eng := newDOTree(filepath.Join(horas, "English"))
	lat := newDOTree(filepath.Join(horas, "Latin"))

	process := func(id, doRel, ourPath string, allowCreate, skipCommune bool) {
		if *only != "" && *only != id {
			return
		}
		if _, err := os.Stat(ourPath); err != nil && !allowCreate {
			return
		}
		seeds := extractSeeds(eng, lat, doRel, id)
		_, forceAnts := antiphonDOSources[id]
		newText, notes := mergeFile(ourPath, seeds, skipCommune, forceAnts)
		if newText == "" && len(notes) == 0 {
			fmt.Printf("%-28s %-18s no changes\n", id, doRel)
			return
		}
		fmt.Printf("%-28s %-18s\n", id, doRel)
		for _, n := range notes {
			fmt.Printf("    %s\n", n)
		}
		if newText != "" && *write {
			if err := os.WriteFile(ourPath, []byte(newText), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing %s: %v\n", ourPath, err)
				os.Exit(1)
			}
		}

		// Scaffold TODOs for newly created files still missing audited refs.
		if allowCreate && newText != "" {
			var missing []string
			for _, ref := range auditedRefs {
				found := false
				for _, sd := range seeds {
					if sd.key == ref || (ref == "psalm-antiphon" && strings.HasPrefix(sd.key, "psalm-antiphon")) {
						found = true
						break
					}
				}
				if !found {
					missing = append(missing, ref)
				}
			}
			if len(missing) > 0 {
				var sb strings.Builder
				sb.WriteString("\n")
				for _, ref := range missing {
					fmt.Fprintf(&sb, "# TODO(diurnal): [%s] — not found in DO %s; supply from diurnal or suppress in audit-ok.txt\n", ref, doRel)
				}
				fmt.Printf("    scaffold TODOs: %s\n", strings.Join(missing, ", "))
				if *write {
					f, err := os.OpenFile(ourPath, os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						f.WriteString(sb.String())
						f.Close()
					}
				}
			}
		}
	}

	// applySeeds merges pre-built seeds into one of our files (creating it
	// if absent) under the same dry-run/write and -feast filter conventions.
	applySeeds := func(id, label, ourPath string, seeds []seed) {
		if *only != "" && *only != id {
			return
		}
		newText, notes := mergeFile(ourPath, seeds, true, false)
		if newText == "" && len(notes) == 0 {
			fmt.Printf("%-28s %-18s no changes\n", id, label)
			return
		}
		fmt.Printf("%-28s %-18s\n", id, label)
		for _, n := range notes {
			fmt.Printf("    %s\n", n)
		}
		if newText != "" && *write {
			if err := os.WriteFile(ourPath, []byte(newText), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing %s: %v\n", ourPath, err)
				os.Exit(1)
			}
		}
	}

	var weekIDs []string
	for id := range weekDOFiles {
		weekIDs = append(weekIDs, id)
	}
	sort.Strings(weekIDs)
	for _, id := range weekIDs {
		prefix := weekDOFiles[id]
		applySeeds(id, prefix+"-*", filepath.Join(*dataDir, "texts", "proper", id+".txt"),
			extractWeekSeeds(eng, lat, prefix, !noFirstVespersMagWeeks[id], !noWeekdayAntWeeks[id]))
	}

	var specialSundayIDs []string
	for id := range specialSundayDOFiles {
		specialSundayIDs = append(specialSundayIDs, id)
	}
	sort.Strings(specialSundayIDs)
	for _, id := range specialSundayIDs {
		rel := specialSundayDOFiles[id]
		applySeeds(id, rel, filepath.Join(*dataDir, "texts", "proper", id+".txt"),
			extractSundaySeeds(eng, lat, rel, true))
	}

	var historiaIDs []string
	for id := range historiaDOFiles {
		historiaIDs = append(historiaIDs, id)
	}
	sort.Strings(historiaIDs)
	for _, id := range historiaIDs {
		rel := historiaDOFiles[id] + "-0"
		var seeds []seed
		if sd := antiphonSeed(eng, lat, rel, "Ant 1", "magnificat-antiphon-first"); sd != nil {
			seeds = append(seeds, *sd)
		}
		applySeeds(id, rel, filepath.Join(*dataDir, "texts", "proper", id+".txt"), seeds)
	}

	var octaveIDs []string
	for id := range octaveDayDOFiles {
		octaveIDs = append(octaveIDs, id)
	}
	sort.Strings(octaveIDs)
	for _, id := range octaveIDs {
		rel := octaveDayDOFiles[id]
		var seeds []seed
		if sd := antiphonSeed(eng, lat, rel, "Ant 2", "benedictus-antiphon"); sd != nil {
			seeds = append(seeds, *sd)
		}
		if sd := antiphonSeed(eng, lat, rel, "Ant 3", "magnificat-antiphon"); sd != nil {
			seeds = append(seeds, *sd)
		}
		applySeeds(id, rel, filepath.Join(*dataDir, "texts", "proper", id+".txt"), seeds)
	}

	var paschalIDs []string
	for id := range paschalCommonsAnts {
		paschalIDs = append(paschalIDs, id)
	}
	sort.Strings(paschalIDs)
	for _, id := range paschalIDs {
		rel := paschalCommonsAnts[id]
		var seeds []seed
		if sd := antiphonSeed(eng, lat, rel, "Ant 2", "benedictus-antiphon"); sd != nil {
			seeds = append(seeds, *sd)
		}
		mag := antiphonSeed(eng, lat, rel, "Ant 3", "magnificat-antiphon")
		if mag != nil {
			seeds = append(seeds, *mag)
		}
		if first := antiphonSeed(eng, lat, rel, "Ant 1", "magnificat-antiphon-first"); first != nil &&
			(mag == nil || strings.Join(first.lines, "\n") != strings.Join(mag.lines, "\n")) {
			seeds = append(seeds, *first)
		}
		applySeeds(id, rel, filepath.Join(*dataDir, "texts", "commons", id+".txt"), seeds)
	}

	applySeeds("advent-o-antiphons", "Major Special",
		filepath.Join(*dataDir, "texts", "seasonal", "advent.txt"),
		extractOAntiphonSeeds(eng, lat))

	var feastIDs []string
	for id := range feastDOFiles {
		feastIDs = append(feastIDs, id)
	}
	sort.Strings(feastIDs)
	for _, id := range feastIDs {
		process(id, feastDOFiles[id], filepath.Join(*dataDir, "texts", "proper", id+".txt"), createFor[id], true)
	}

	var commonIDs []string
	for id := range commonsDOFiles {
		commonIDs = append(commonIDs, id)
	}
	sort.Strings(commonIDs)
	for _, id := range commonIDs {
		process(id, commonsDOFiles[id], filepath.Join(*dataDir, "texts", "commons", id+".txt"), false, false)
	}
}
