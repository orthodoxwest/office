//! Go-compatible parsing and formatting. While both engines exist
//! (RUST-PORT.md), output and findings must match the Go engine byte for
//! byte, so these helpers reproduce the Go library behaviors that shape
//! them: which lines and integers parse, how CSV splits, how a key is quoted
//! in a finding, and how report JSON is written. Error prose is not part of
//! that contract, and Rust words its own. Remove the crate after the cutover
//! (Phase 7).

pub mod csv;
pub mod json;

/// Quotes `s` as Go's `%q` does for the text in our data: backslash escapes
/// for quote, backslash, and ASCII control characters; everything else
/// literal. Go also escapes non-printing Unicode (U+00A0, U+200B, …); this
/// prints it literally, a deliberate difference (RUST-PORT.md).
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

/// Go's `bufio.Scanner` with `ScanLines`: split on `\n`, drop one trailing
/// `\r` from each line (including a final unterminated one), and produce no
/// empty final line after a trailing newline.
pub fn scan_lines(s: &str) -> impl Iterator<Item = &str> {
    let body = s.strip_suffix('\n').unwrap_or(s);
    let pieces = if s.is_empty() { None } else { Some(body.split('\n')) };
    pieces.into_iter().flatten().map(|line| line.strip_suffix('\r').unwrap_or(line))
}

/// The integers Go's `strconv.Atoi` accepts: an optional sign and ASCII
/// digits.
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
    fn quote_matches_go_on_ascii() {
        assert_eq!(quote("a\"b\\c\u{7}\u{1}é\u{7f}"), "\"a\\\"b\\\\c\\a\\x01é\\x7f\"");
        // Non-printing Unicode stays literal (Go would write \u00a0).
        assert_eq!(quote("a\u{a0}b"), "\"a\u{a0}b\"");
    }

    #[test]
    fn atoi_accepts_what_go_accepts() {
        assert_eq!(atoi("+5"), Ok(5));
        assert_eq!(atoi("-3"), Ok(-3));
        assert_eq!(atoi("07"), Ok(7));
        assert!(atoi("").is_err());
        assert!(atoi("1_0").is_err());
        assert!(atoi(" 1").is_err());
    }
}
