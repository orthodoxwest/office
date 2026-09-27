//! Hour definition files (`data/office/<hour>.txt`) and their condition
//! language. Ported from Go's `hourdef.go` and `condition.go`.

use calendar::{MoveableDates, Season, Weekday};
use compat::{quote, scan_lines};

use crate::day::Day;
use crate::texts::OfficeTexts;

/// One `Type`/`Ref` pair.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct HourElement {
    pub kind: String,
    pub reference: String,
}

impl HourElement {
    pub fn new(kind: &str, reference: &str) -> HourElement {
        HourElement { kind: kind.to_string(), reference: reference.to_string() }
    }
}

/// A named section with an optional condition.
#[derive(Clone, Debug, PartialEq)]
pub struct HourSection {
    pub name: String,
    pub condition: String,
    parsed: Option<Condition>,
    pub collapsible: bool,
    pub label: String,
    pub elements: Vec<HourElement>,
}

impl HourSection {
    pub fn new(name: &str) -> HourSection {
        HourSection {
            name: name.to_string(),
            condition: String::new(),
            parsed: None,
            collapsible: false,
            label: String::new(),
            elements: Vec::new(),
        }
    }

    /// Whether the section's condition holds. Hand-built sections (tests)
    /// parse on demand; an unparsable condition never holds.
    pub fn condition_holds(&self, day: &Day, moveable: Option<&MoveableDates>, t: &OfficeTexts) -> bool {
        match &self.parsed {
            Some(c) => c.evaluate(day, moveable, t),
            None => Condition::parse(&self.condition).is_ok_and(|c| c.evaluate(day, moveable, t)),
        }
    }

    pub fn has_element_type(&self, kinds: &[&str]) -> bool {
        self.elements.iter().any(|e| kinds.contains(&e.kind.as_str()))
    }
}

/// Parses an hour definition. Errors match Go's `ParseHourDefinition`.
pub fn parse_hour_definition(path: &str, content: &str) -> Result<Vec<HourSection>, String> {
    let mut sections = Vec::new();
    let mut current: Option<HourSection> = None;
    let mut pending_type = String::new();
    let mut condition_line = 0;
    let finish = |section: &mut HourSection, condition_line: usize| -> Result<(), String> {
        if condition_line != 0 {
            let parsed = Condition::parse(&section.condition)
                .map_err(|e| format!("{path}:{condition_line}: invalid condition in section [{}]: {e}", section.name))?;
            section.parsed = Some(parsed);
        }
        Ok(())
    };
    for (i, raw) in scan_lines(content).enumerate() {
        let n = i + 1;
        let line = raw.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        if line.len() >= 2 && line.starts_with('[') && line.ends_with(']') {
            if let Some(mut section) = current.take() {
                if !pending_type.is_empty() {
                    return Err(format!("{path}:{n}: Type without matching Ref in section [{}]", section.name));
                }
                finish(&mut section, condition_line)?;
                sections.push(section);
            }
            current = Some(HourSection::new(&line[1..line.len() - 1]));
            pending_type.clear();
            condition_line = 0;
            continue;
        }
        let Some((key, value)) = line.split_once('=') else {
            return Err(format!("{path}:{n}: expected Key = Value, got {}", quote(line)));
        };
        let (key, value) = (key.trim(), value.trim());
        let Some(section) = current.as_mut() else {
            return Err(format!("{path}:{n}: key-value before any section"));
        };
        match key {
            "Condition" => {
                if condition_line != 0 {
                    return Err(format!("{path}:{n}: duplicate Condition in section [{}]", section.name));
                }
                section.condition = value.to_string();
                condition_line = n;
            }
            "Collapsible" => section.collapsible = value == "true",
            "Label" => section.label = value.to_string(),
            "Type" => {
                if !pending_type.is_empty() {
                    return Err(format!("{path}:{n}: consecutive Type without Ref in section [{}]", section.name));
                }
                pending_type = value.to_string();
            }
            "Ref" => {
                if pending_type.is_empty() {
                    return Err(format!("{path}:{n}: Ref without preceding Type in section [{}]", section.name));
                }
                section.elements.push(HourElement { kind: std::mem::take(&mut pending_type), reference: value.to_string() });
            }
            _ => return Err(format!("{path}:{n}: unknown key {} in section [{}]", quote(key), section.name)),
        }
    }
    if let Some(mut section) = current {
        if !pending_type.is_empty() {
            return Err(format!("{path}: Type without matching Ref at end of section [{}]", section.name));
        }
        finish(&mut section, condition_line)?;
        sections.push(section);
    }
    Ok(sections)
}

#[derive(Clone, Debug, PartialEq, Eq)]
enum Atom {
    Preces,
    Suffrage,
    BvmSuffrageForm,
    CrossCommemoration,
    IsFeast,
    FestalLaudsPsalmody,
    DeclaredLaudsPsalmody,
    FestalWeekdayLaudsCanticle,
    FestalVespersPsalmody,
    IsFerial,
    OfficeOfTheDead,
    AppendedOfficeOfTheDead,
    Triduum,
    Weekday(Weekday),
    Feast(String),
    Season(Season),
}

/// Comma-separated conjunctions of optionally negated (`not-`) atoms.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Condition {
    clauses: Vec<(Atom, bool)>,
}

impl Condition {
    pub fn parse(raw: &str) -> Result<Condition, String> {
        if raw.trim().is_empty() {
            return Err("condition is empty".to_string());
        }
        let mut clauses = Vec::new();
        for part in raw.split(',') {
            let mut atom = part.trim();
            if atom.is_empty() {
                return Err("condition contains an empty clause".to_string());
            }
            let mut negated = false;
            while let Some(rest) = atom.strip_prefix("not-") {
                negated = !negated;
                atom = rest;
            }
            clauses.push((parse_atom(atom)?, negated));
        }
        Ok(Condition { clauses })
    }

    pub fn evaluate(&self, day: &Day, moveable: Option<&MoveableDates>, t: &OfficeTexts) -> bool {
        self.clauses.iter().all(|(atom, negated)| evaluate_atom(atom, day, moveable, t) != *negated)
    }
}

fn parse_atom(atom: &str) -> Result<Atom, String> {
    Ok(match atom {
        "if-preces" => Atom::Preces,
        "if-suffrage" => Atom::Suffrage,
        "bvm-suffrage-form" => Atom::BvmSuffrageForm,
        "if-cross-commemoration" => Atom::CrossCommemoration,
        "is-feast" => Atom::IsFeast,
        "festal-weekday-lauds-canticle" => Atom::FestalWeekdayLaudsCanticle,
        "declared-lauds-psalmody" => Atom::DeclaredLaudsPsalmody,
        "festal-lauds-psalmody" => Atom::FestalLaudsPsalmody,
        "festal-vespers-psalmody" => Atom::FestalVespersPsalmody,
        "is-ferial" => Atom::IsFerial,
        "office-of-the-dead" => Atom::OfficeOfTheDead,
        "appended-office-of-the-dead" => Atom::AppendedOfficeOfTheDead,
        "triduum" => Atom::Triduum,
        _ => {
            if let Some(value) = atom.strip_prefix("weekday-") {
                let day = Weekday::ALL.into_iter().find(|d| d.name().to_lowercase() == value);
                return day.map(Atom::Weekday).ok_or_else(|| format!("invalid weekday condition {}", quote(atom)));
            }
            if let Some(value) = atom.strip_prefix("feast-") {
                if value.is_empty() {
                    return Err("feast condition has an empty ID".to_string());
                }
                return Ok(Atom::Feast(value.to_string()));
            }
            if let Some(value) = atom.strip_prefix("season-") {
                return Season::parse(value).map(Atom::Season).map_err(|_| format!("invalid season condition {}", quote(atom)));
            }
            return Err(format!("unknown condition {}", quote(atom)));
        }
    })
}

fn evaluate_atom(atom: &Atom, day: &Day, moveable: Option<&MoveableDates>, t: &OfficeTexts) -> bool {
    use crate::{lauds_psalmody, preces, psalmody};
    match atom {
        Atom::Preces => preces::should_say_preces(Some(day), moveable),
        Atom::Suffrage => preces::should_say_suffrage(Some(day)),
        Atom::BvmSuffrageForm => preces::uses_bvm_suffrage_form(day),
        Atom::CrossCommemoration => preces::should_say_cross_commemoration(day, moveable),
        Atom::IsFeast => day
            .celebration
            .as_deref()
            .is_some_and(|c| !c.is_category(calendar::Category::Feria) && !c.is_category(calendar::Category::Sunday)),
        Atom::DeclaredLaudsPsalmody => lauds_psalmody::uses_declared_lauds_psalmody(day, t),
        Atom::FestalLaudsPsalmody => preces::uses_festal_lauds_psalmody(day, t),
        Atom::FestalWeekdayLaudsCanticle => lauds_psalmody::uses_festal_weekday_lauds_canticle(day, t),
        Atom::FestalVespersPsalmody => psalmody::uses_festal_vespers_psalmody(day, t),
        Atom::IsFerial => day.is_ferial(),
        Atom::OfficeOfTheDead => psalmody::is_office_of_the_dead(day),
        Atom::AppendedOfficeOfTheDead => day.vespers.appended_office_of_the_dead,
        Atom::Triduum => is_triduum(day),
        Atom::Weekday(w) => day.civil_weekday() == *w,
        Atom::Feast(id) => day.celebration_is(id),
        Atom::Season(s) => day.season == *s,
    }
}

/// The Sacred Triduum's celebrations. Use [`uses_triduum_form`] for rules
/// whose scope ends at None of Holy Saturday.
pub fn is_triduum(day: &Day) -> bool {
    matches!(day.celebration_id(), Some("holy-thursday" | "good-friday" | "holy-saturday"))
}

/// Whether this hour follows the common Triduum rubrics, which end at None
/// of Holy Saturday (Diurnal p. 313).
pub fn uses_triduum_form(day: &Day, hour_name: &str) -> bool {
    is_triduum(day) && (!day.celebration_is("holy-saturday") || (hour_name != "vespers" && hour_name != "compline"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_definitions() {
        let sections = parse_hour_definition(
            "h.txt",
            "# c\n[Opening]\nCondition = not-triduum\nLabel = Open\nCollapsible = true\n\nType = versicle\nRef = a/b\n[Next]\nType = psalm\nRef = psalms/004\n",
        )
        .unwrap();
        assert_eq!(sections.len(), 2);
        assert_eq!(sections[0].label, "Open");
        assert!(sections[0].collapsible);
        assert!(sections[0].parsed.is_some());
        assert_eq!(sections[1].elements, vec![HourElement::new("psalm", "psalms/004")]);
    }

    #[test]
    fn definition_errors() {
        let err = |s| parse_hour_definition("h.txt", s).unwrap_err();
        assert_eq!(err("Type = x\n"), "h.txt:1: key-value before any section");
        assert_eq!(err("[A]\nnope\n"), "h.txt:2: expected Key = Value, got \"nope\"");
        assert_eq!(err("[A]\nType = x\n[B]\n"), "h.txt:3: Type without matching Ref in section [A]");
        assert_eq!(err("[A]\nType = x\nType = y\n"), "h.txt:3: consecutive Type without Ref in section [A]");
        assert_eq!(err("[A]\nRef = y\n"), "h.txt:2: Ref without preceding Type in section [A]");
        assert_eq!(err("[A]\nColor = y\n"), "h.txt:2: unknown key \"Color\" in section [A]");
        assert_eq!(err("[A]\nType = x\n"), "h.txt: Type without matching Ref at end of section [A]");
        assert_eq!(err("[A]\nCondition = a\nCondition = b\n"), "h.txt:3: duplicate Condition in section [A]");
        assert_eq!(
            err("[A]\nCondition = weekday-funday\n"),
            "h.txt:2: invalid condition in section [A]: invalid weekday condition \"weekday-funday\""
        );
    }

    #[test]
    fn condition_language() {
        assert!(Condition::parse("").is_err());
        assert_eq!(Condition::parse("if-preces,,triduum").unwrap_err(), "condition contains an empty clause");
        assert_eq!(Condition::parse("a,,b").unwrap_err(), "unknown condition \"a\"");
        assert_eq!(Condition::parse("feast-").unwrap_err(), "feast condition has an empty ID");
        assert_eq!(Condition::parse("season-winter").unwrap_err(), "invalid season condition \"season-winter\"");
        assert_eq!(Condition::parse("bogus").unwrap_err(), "unknown condition \"bogus\"");
        let c = Condition::parse("not-not-triduum, not-weekday-sunday").unwrap();
        assert_eq!(c.clauses, vec![(Atom::Triduum, false), (Atom::Weekday(Weekday::Sunday), true)]);
    }
}
