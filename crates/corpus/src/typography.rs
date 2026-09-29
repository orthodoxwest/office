//! Display typography shared by the web and print renderers: curly quotes for the corpus's
//! typewriter punctuation, en dashes in verse ranges, and the softened ALL-CAPS opening a
//! drop cap takes. Display text only; the corpus itself keeps its plain punctuation.

/// Words that begin with an eliding apostrophe rather than an opening quote.
const ELIDED_WORDS: [&str; 9] = ["mid", "midst", "tis", "twas", "twere", "gainst", "neath", "tween", "twixt"];

// Typography uses Unicode Alphabetic, Uppercase, White_Space, and numeric properties.
fn is_letter(c: char) -> bool {
    c.is_alphabetic()
}

fn is_digit(c: char) -> bool {
    c.is_ascii_digit() || (!c.is_ascii() && c.is_numeric())
}

/// Curly quotes and apostrophes for the corpus's typewriter punctuation, and
/// an en dash between two digits (a verse range).
pub fn typeset(s: &str) -> String {
    if !s.contains(['\'', '"', '-']) {
        return s.to_string();
    }
    let mut b = String::with_capacity(s.len() + 8);
    let mut prev: Option<char> = None;
    for (i, r) in s.char_indices() {
        // The sentinel U+FFFD is neither a letter nor a digit.
        let next = s[i + r.len_utf8()..].chars().next().unwrap_or('\u{FFFD}');
        match r {
            '\'' => {
                let after_word = prev.is_some_and(|p| is_letter(p) || is_digit(p) || is_closing_punct(p));
                if after_word || (is_letter(next) && starts_elision(&s[i + 1..])) {
                    b.push('’');
                } else {
                    b.push('‘');
                }
            }
            '"' => {
                if prev.is_none_or(|p| p.is_whitespace() || is_opening_punct(p)) {
                    b.push('“');
                } else {
                    b.push('”');
                }
            }
            '-' if prev.is_some_and(is_digit) && is_digit(next) => b.push('–'),
            _ => b.push(r),
        }
        prev = Some(r);
    }
    b
}

fn starts_elision(rest: &str) -> bool {
    let end = rest.find(|c: char| !is_letter(c)).unwrap_or(rest.len());
    ELIDED_WORDS.contains(&rest[..end].to_lowercase().as_str())
}

fn is_opening_punct(c: char) -> bool {
    "([{‘“—–-/".contains(c)
}

fn is_closing_punct(c: char) -> bool {
    ".,;:!?)]}’”".contains(c)
}

/// Title-cases a leading run of ALL-CAPS words so a drop cap takes only the
/// initial: "GOD be merciful" → "God be merciful". A single capital ("O",
/// "I") is a whole word and stays as it is.
pub fn soften_drop_cap_opening(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    let mut changed = false;
    let mut rest = s;
    loop {
        let ws = rest.len() - rest.trim_start().len();
        out.push_str(&rest[..ws]);
        rest = &rest[ws..];
        if rest.is_empty() {
            break;
        }
        let end = rest.find(char::is_whitespace).unwrap_or(rest.len());
        let word = &rest[..end];
        let letter_end = word.char_indices().rev().find(|(_, c)| is_letter(*c)).map_or(0, |(i, c)| i + c.len_utf8());
        if letter_end == 0 {
            out.push_str(rest);
            return out;
        }
        let (letters, trail) = word.split_at(letter_end);
        if !letters.chars().all(|c| is_letter(c) && c.is_uppercase()) {
            out.push_str(rest);
            return out;
        }
        if letters.chars().count() >= 2 {
            let mut chars = letters.chars();
            out.push(chars.next().expect("non-empty"));
            for c in chars {
                // Use Unicode simple lowercase, mapping one character to one character.
                let mut lower = c.to_lowercase();
                match (lower.next(), lower.next()) {
                    (Some(l), None) => out.push(l),
                    _ => out.push(c),
                }
            }
            out.push_str(trail);
            changed = true;
        } else {
            out.push_str(word);
        }
        rest = &rest[end..];
    }
    if changed { out } else { s.to_string() }
}
