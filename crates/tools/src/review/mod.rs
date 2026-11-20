//! Review tooling: text provenance and composition explanations.

pub mod explain;
pub mod provenance;
pub mod sweep;

use liturgy::OfficeHour;
use office::concurrence::VespersOwner;
use office::day::Day;
use sha2::{Digest, Sha256};

/// A short content hash of a composed hour, the date excluded so identical
/// compositions on different dates hash alike. Source keys and decision
/// traces are excluded; psalm incipits are content.
pub fn hash_hour(h: &OfficeHour) -> String {
    let mut b: Vec<u8> = Vec::new();
    let mut put = |s: &str| b.extend_from_slice(s.as_bytes());
    put(&h.hour);
    put("\x1f");
    put(&h.title);
    put("\x1f");
    put(h.season.map_or("", |s| s.as_str()));
    put("\x1f");
    put(&h.feast);
    put("\x1f");
    put(h.color.map_or("", |c| c.as_str()));
    put("\x1e");
    for s in &h.sections {
        put(&s.label);
        put("\x1f");
        put(if s.collapsible { "true" } else { "false" });
        put("\x1e");
        for e in &s.elements {
            put(e.kind.as_str());
            put("\x1f");
            put(&e.label);
            put("\x1f");
            put(&e.incipit);
            put("\x1f");
            put(&e.rubric);
            put("\x1f");
            put(&e.text);
            for v in &e.voice {
                put("\x1f");
                put(if v.spoken { "1" } else { "0" });
                put("\x1f");
                put(&v.text);
            }
            put("\x1e");
        }
    }
    hex12(&Sha256::digest(&b))
}

fn hex12(sum: &[u8]) -> String {
    sum[..6].iter().map(|b| format!("{b:02x}")).collect()
}

/// A stable key for the celebration owning the hour; Vespers keys on the
/// evening's owner.
pub fn unit_key(day: &Day, hour_name: &str) -> String {
    if hour_name == "vespers"
        && let Some(f) = day.vespers.feast()
    {
        if day.vespers.owner() == VespersOwner::IOfFollowing {
            return format!("{}-1v", f.id);
        }
        return f.id.clone();
    }
    if let Some(c) = &day.celebration {
        return c.id.clone();
    }
    if let Some(t) = day.tempora.as_deref().filter(|t| !t.is_empty()) {
        return slugify(t);
    }
    format!("feria-{}", day.season.as_str())
}

pub fn celebration_name(day: &Day) -> String {
    if let Some(c) = &day.celebration {
        return c.name.clone();
    }
    if let Some(t) = day.tempora.as_deref().filter(|t| !t.is_empty()) {
        return t.to_string();
    }
    let season = day.season.as_str();
    format!("{}{} feria", season[..1].to_uppercase(), &season[1..])
}

fn slugify(s: &str) -> String {
    let mut b = String::new();
    let mut prev_dash = false;
    for c in s.to_lowercase().chars() {
        if c.is_ascii_lowercase() || c.is_ascii_digit() {
            b.push(c);
            prev_dash = false;
        } else if !prev_dash && !b.is_empty() {
            b.push('-');
            prev_dash = true;
        }
    }
    b.trim_end_matches('-').to_string()
}
