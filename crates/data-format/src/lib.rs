//! Parsing and formatting contracts shared by the corpus, review ledgers, and reports: lines,
//! integers, CSV, quoted diagnostic keys, and JSON.

pub mod csv;
pub mod json;

/// Quotes `s` with backslash escapes for quote, backslash, and ASCII control characters. All other
/// characters, including non-printing Unicode, remain literal.
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
            c => out.push(c),
        }
    }
    out.push('"');
    out
}

/// Splits on `\n`, drops one trailing `\r` from each line (including a final unterminated one), and
/// produces no empty final line after a trailing newline.
pub fn scan_lines(s: &str) -> impl Iterator<Item = &str> {
    let body = s.strip_suffix('\n').unwrap_or(s);
    let pieces = if s.is_empty() { None } else { Some(body.split('\n')) };
    pieces.into_iter().flatten().map(|line| line.strip_suffix('\r').unwrap_or(line))
}

/// Parses an optional sign followed by ASCII digits.
pub fn atoi(s: &str) -> Result<i64, String> {
    let digits = s.strip_prefix(['+', '-']).unwrap_or(s);
    if digits.is_empty() || !digits.bytes().all(|b| b.is_ascii_digit()) {
        return Err(format!("{} is not an integer", quote(s)));
    }
    s.parse::<i64>().map_err(|_| format!("{} is out of range", quote(s)))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn scan_lines_matches_bufio() {
        let lines = |s| scan_lines(s).collect::<Vec<_>>();
        assert_eq!(lines(""), Vec::<&str>::new());
        assert_eq!(lines("\n"), vec![""]);
        assert_eq!(lines("a\r\nb"), vec!["a", "b"]);
        assert_eq!(lines("a\n\nb\n"), vec!["a", "", "b"]);
        assert_eq!(lines("a\r"), vec!["a"]);
    }

    #[test]
    fn quotes_ascii_controls() {
        assert_eq!(quote("a\"b\\c\u{7}\u{1}é\u{7f}"), "\"a\\\"b\\\\c\\a\\x01é\\x7f\"");
        // Non-printing Unicode stays literal.
        assert_eq!(quote("a\u{a0}b"), "\"a\u{a0}b\"");
    }

    #[test]
    fn parses_signed_ascii_integers() {
        assert_eq!(atoi("+5"), Ok(5));
        assert_eq!(atoi("-3"), Ok(-3));
        assert_eq!(atoi("07"), Ok(7));
        assert!(atoi("").is_err());
        assert!(atoi("1_0").is_err());
        assert!(atoi(" 1").is_err());
    }
}
