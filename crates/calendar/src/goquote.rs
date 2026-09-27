//! Go's `%q` for strings, so error messages match the Go engine byte for byte.

/// Quotes `s` as Go's `strconv.Quote` does for printable text: backslash
/// escapes for quote, backslash, and control characters; everything else
/// literal.
pub fn quote(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\u{7}' => out.push_str("\\a"),
            '\u{8}' => out.push_str("\\b"),
            '\u{c}' => out.push_str("\\f"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '\u{b}' => out.push_str("\\v"),
            c if (c as u32) < 0x20 || c == '\u{7f}' => out.push_str(&format!("\\x{:02x}", c as u32)),
            // PORT(inherited): Go also escapes non-printable Unicode (unicode.IsPrint);
            // the data files contain none.
            c => out.push(c),
        }
    }
    out.push('"');
    out
}
