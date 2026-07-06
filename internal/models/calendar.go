// Package models defines shared types for the liturgical calendar.
package models

import (
	"fmt"
	"time"
)

// Rank represents the pre-1962 ranking system for liturgical observances.
type Rank string

const (
	Double1stClass Rank = "double-1st-class"
	Double2ndClass Rank = "double-2nd-class"
	GreaterDouble  Rank = "greater-double"
	Double         Rank = "double"
	SemiDouble     Rank = "semi-double"
	Simple         Rank = "simple"
	Commemoration  Rank = "commemoration"
)

var rankWeights = map[Rank]int{
	Double1stClass: 7,
	Double2ndClass: 6,
	GreaterDouble:  5,
	Double:         4,
	SemiDouble:     3,
	Simple:         2,
	Commemoration:  1,
}

var rankAbbreviations = map[Rank]string{
	Double1stClass: "1cl",
	Double2ndClass: "2cl",
	GreaterDouble:  "gd",
	Double:         "d",
	SemiDouble:     "sd",
	Simple:         "s",
	Commemoration:  "com",
}

// Weight returns the numeric weight for rank comparison (higher = more important).
func (r Rank) Weight() int {
	if w, ok := rankWeights[r]; ok {
		return w
	}
	return 0
}

// Abbrev returns the short abbreviation for display.
func (r Rank) Abbrev() string {
	if a, ok := rankAbbreviations[r]; ok {
		return a
	}
	return "?"
}

var rankDisplayNames = map[Rank]string{
	Double1stClass: "Double of the 1st Class",
	Double2ndClass: "Double of the 2nd Class",
	GreaterDouble:  "Greater Double",
	Double:         "Double",
	SemiDouble:     "Semi-double",
	Simple:         "Simple",
	Commemoration:  "Commemoration",
}

// DisplayName returns the full human-readable rank name.
func (r Rank) DisplayName() string {
	if n, ok := rankDisplayNames[r]; ok {
		return n
	}
	return string(r)
}

// Valid returns true if the rank is a known value.
func (r Rank) Valid() bool {
	_, ok := rankWeights[r]
	return ok
}

// ParseRank converts a string to a Rank, returning an error if invalid.
func ParseRank(s string) (Rank, error) {
	r := Rank(s)
	if !r.Valid() {
		return "", fmt.Errorf("invalid rank: %q", s)
	}
	return r, nil
}

// Color represents a liturgical color.
type Color string

const (
	White  Color = "white"
	Red    Color = "red"
	Green  Color = "green"
	Violet Color = "violet"
	Black  Color = "black"
	Rose   Color = "rose"
)

var colorAbbreviations = map[Color]string{
	White:  "w",
	Red:    "r",
	Green:  "g",
	Violet: "v",
	Black:  "b",
	Rose:   "p",
}

var validColors = map[Color]bool{
	White: true, Red: true, Green: true,
	Violet: true, Black: true, Rose: true,
}

// Abbrev returns the single-character abbreviation for display.
func (c Color) Abbrev() string {
	if a, ok := colorAbbreviations[c]; ok {
		return a
	}
	return "?"
}

// Valid returns true if the color is a known value.
func (c Color) Valid() bool {
	return validColors[c]
}

// ParseColor converts a string to a Color, returning an error if invalid.
func ParseColor(s string) (Color, error) {
	c := Color(s)
	if !c.Valid() {
		return "", fmt.Errorf("invalid color: %q", s)
	}
	return c, nil
}

// FeastCategory represents the category of a feast for common texts and precedence.
type FeastCategory string

const (
	CategoryLord            FeastCategory = "lord"
	CategoryBlessedVirgin   FeastCategory = "blessed-virgin"
	CategoryAngel           FeastCategory = "angel"
	CategoryApostle         FeastCategory = "apostle"
	CategoryEvangelist      FeastCategory = "evangelist"
	CategoryMartyr          FeastCategory = "martyr"
	CategoryBishopMartyr    FeastCategory = "bishop-martyr"
	CategoryVirginMartyr    FeastCategory = "virgin-martyr"
	CategoryConfessorBishop FeastCategory = "confessor-bishop"
	CategoryConfessorDoctor FeastCategory = "confessor-doctor"
	CategoryConfessor       FeastCategory = "confessor"
	CategoryVirgin          FeastCategory = "virgin"
	CategoryHolyWoman       FeastCategory = "holy-woman"
	CategoryDedication      FeastCategory = "dedication"
	CategoryFeria           FeastCategory = "feria"
	CategorySunday          FeastCategory = "sunday"
)

var validCategories = map[FeastCategory]bool{
	CategoryLord: true, CategoryBlessedVirgin: true, CategoryAngel: true,
	CategoryApostle: true, CategoryEvangelist: true, CategoryMartyr: true,
	CategoryBishopMartyr: true, CategoryVirginMartyr: true,
	CategoryConfessorBishop: true, CategoryConfessorDoctor: true,
	CategoryConfessor: true, CategoryVirgin: true, CategoryHolyWoman: true,
	CategoryDedication: true, CategoryFeria: true, CategorySunday: true,
}

// Valid returns true if the category is a known value.
func (c FeastCategory) Valid() bool {
	return validCategories[c]
}

// ParseFeastCategory converts a string to a FeastCategory, returning an error if invalid.
func ParseFeastCategory(s string) (FeastCategory, error) {
	c := FeastCategory(s)
	if !c.Valid() {
		return "", fmt.Errorf("invalid feast category: %q", s)
	}
	return c, nil
}

// Season represents a liturgical season.
type Season string

const (
	Advent       Season = "advent"
	Christmas    Season = "christmas"
	Epiphany     Season = "epiphany"
	Septuagesima Season = "septuagesima"
	Lent         Season = "lent"
	Passiontide  Season = "passiontide"
	Easter       Season = "easter"
	Pentecost    Season = "pentecost"
)

var validSeasons = map[Season]bool{
	Advent: true, Christmas: true, Epiphany: true, Septuagesima: true,
	Lent: true, Passiontide: true, Easter: true, Pentecost: true,
}

// Valid returns true if the season is a known value.
func (s Season) Valid() bool {
	return validSeasons[s]
}

// ParseSeason converts a string to a Season, returning an error if invalid.
func ParseSeason(s string) (Season, error) {
	season := Season(s)
	if !season.Valid() {
		return "", fmt.Errorf("invalid season: %q", s)
	}
	return season, nil
}

// FeastSource indicates the origin of a feast entry.
type FeastSource string

const (
	SourceBase FeastSource = "base"
	SourceAWRV FeastSource = "awrv"
)

// Feast represents a liturgical feast or observance.
type Feast struct {
	ID       string
	Name     string
	Rank     Rank
	Color    Color
	Category FeastCategory

	// ProperName is the liturgical given name used for "N." substitution
	// in common and ordinary texts (e.g., "Ambrose", "Agatha").
	ProperName string

	// ProperID redirects feast-specific proper lookup to another feast ID while
	// preserving this feast's own calendar identity and displayed name.
	ProperID string

	// For temporal (moveable) feasts: offset from Easter or a named rule.
	DateRule string

	// For sanctoral (fixed) feasts: month and day.
	Month int
	Day   int

	HasOctave bool
	HasVigil  bool

	// OnlyWith restricts this feast/commemoration to days where the winning
	// celebration has the given feast ID.
	OnlyWith string

	// SkipRomanLeapShift keeps a fixed late-February feast on its civil date in
	// leap years instead of applying the Roman bissextile one-day shift.
	SkipRomanLeapShift bool

	Source FeastSource
	Notes  string
}

// IsFixed returns true if the feast has a fixed date (month/day).
func (f *Feast) IsFixed() bool {
	return f.Month != 0 && f.Day != 0
}

// IsTemporal returns true if the feast is moveable (has a date rule).
func (f *Feast) IsTemporal() bool {
	return f.DateRule != ""
}

// SeasonDefinition defines a liturgical season with its default color.
type SeasonDefinition struct {
	ID    Season
	Name  string
	Color Color
	Notes string
}

// PenitentialObservance captures fasting and abstinence obligations for a day.
type PenitentialObservance struct {
	Fast       bool
	Abstinence bool
}

// Empty reports whether no penitential observance applies.
func (p PenitentialObservance) Empty() bool {
	return !p.Fast && !p.Abstinence
}

// Marker returns the compact ordo marker used in text output.
func (p PenitentialObservance) Marker() string {
	var marker string
	if p.Abstinence {
		marker += "L"
	}
	if p.Fast {
		marker += "§"
	}
	return marker
}

// Labels returns human-readable labels for display.
func (p PenitentialObservance) Labels() []string {
	labels := make([]string, 0, 2)
	if p.Fast {
		labels = append(labels, "Fasting")
	}
	if p.Abstinence {
		labels = append(labels, "Abstinence")
	}
	return labels
}

// VespersOwner indicates which office owns vespers on a given evening.
type VespersOwner int

const (
	VespersNotApplicable VespersOwner = iota // feria/simple with no concurrence
	VespersIIOfPreceding                     // II Vespers of today's celebration
	VespersIOfFollowing                      // I Vespers of tomorrow's celebration
	// TODO(phase3): VespersSplit
)

// VespersDesignation records which office owns vespers on a given evening.
type VespersDesignation struct {
	Owner  VespersOwner
	Feast  *Feast // celebration that owns vespers (may differ from day's Celebration)
	Color  Color  // liturgical color for vespers
	Season Season

	// Commemorations holds the celebration that lost the vespers concurrence
	// (the outgoing office at I Vespers of a following feast, or the incoming
	// office at II Vespers of a preceding feast), commemorated per XIII.2-17.
	Commemorations []*Feast
}

// CalendarDay represents the resolved calendar for a single day.
type CalendarDay struct {
	Date           time.Time
	Season         Season
	Tempora        string
	Celebration    *Feast
	Commemorations []*Feast
	Color          Color
	Notes          string

	// WithinOctaveOf is the parent feast ID if this day falls within an octave
	// (days 2-8), or empty string if not. Set by the calendar builder.
	WithinOctaveOf string

	// MarianAntiphon is the corpus subkey for ordinary/marian/ that applies on
	// this date (e.g. "salve-regina"). Set by the calendar builder.
	MarianAntiphon string

	Penitential PenitentialObservance

	Vespers VespersDesignation
}
