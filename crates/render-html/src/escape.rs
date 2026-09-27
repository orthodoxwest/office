//! Escaping for composed HTML fragments and URL attributes. MiniJinja handles template
//! autoescaping; URL-valued attributes also use the `url`, `urlnorm`, or `urlpart` filter.

/// Escapes `"`, `'`, `&`, `<`, `>`, and NUL.
pub fn html_escape_string(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        match c {
            '"' => out.push_str("&#34;"),
            '\'' => out.push_str("&#39;"),
            '&' => out.push_str("&amp;"),
            '<' => out.push_str("&lt;"),
            '>' => out.push_str("&gt;"),
            '\0' => out.push('\u{FFFD}'),
            c => out.push(c),
        }
    }
    out
}

/// Replaces URL schemes other than http, https, or mailto with `#invalid-url`.
fn url_filter(s: &str) -> String {
    if let Some((protocol, _)) = s.split_once(':')
        && !protocol.contains('/')
        && !["http", "https", "mailto"].iter().any(|p| protocol.eq_ignore_ascii_case(p))
    {
        return "#invalid-url".to_string();
    }
    s.to_string()
}

/// Percent-encodes what a URL may not
/// carry; when normalizing, reserved characters and valid escapes stay.
fn process_url(s: &str, norm: bool) -> String {
    let mut out = String::with_capacity(s.len() + 16);
    let bytes = s.as_bytes();
    for (i, &c) in bytes.iter().enumerate() {
        let keep = match c {
            b'!' | b'#' | b'$' | b'&' | b'*' | b'+' | b',' | b'/' | b':' | b';' | b'=' | b'?' | b'@' | b'[' | b']' => norm,
            b'-' | b'.' | b'_' | b'~' => true,
            b'%' => norm && i + 2 < bytes.len() && bytes[i + 1].is_ascii_hexdigit() && bytes[i + 2].is_ascii_hexdigit(),
            c => c.is_ascii_alphanumeric(),
        };
        if keep {
            out.push(c as char);
        } else {
            out.push_str(&format!("%{c:02x}"));
        }
    }
    out
}

/// A value that starts a URL attribute (`href="{{ x|url }}"`): filtered and
/// normalized, left for the formatter's attribute escaping.
pub fn url_start(s: &str) -> String {
    process_url(&url_filter(s), true)
}

/// A value inside a URL after the path has begun (`action="/{{ x|urlnorm }}"`).
pub fn url_norm(s: &str) -> String {
    process_url(s, true)
}

/// A value in a URL's query or fragment (`href="#{{ x|urlpart }}"`).
pub fn url_part(s: &str) -> String {
    process_url(s, false)
}

/// Shortest float formatting: decimal notation, or an exponent below 1e-4 and from 1e6.
pub fn format_float(f: f64) -> String {
    if f == 0.0 || !f.is_finite() {
        return if f.is_nan() {
            "NaN".into()
        } else if f.is_infinite() {
            if f > 0.0 { "+Inf".into() } else { "-Inf".into() }
        } else {
            "0".into()
        };
    }
    let sci = format!("{f:e}");
    let (mantissa, exp) = sci.split_once('e').expect("exponent");
    let exp: i32 = exp.parse().expect("integer exponent");
    if (-4..6).contains(&exp) {
        return format!("{f}");
    }
    let sign = if exp < 0 { '-' } else { '+' };
    format!("{mantissa}e{sign}{:02}", exp.abs())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn escapes_composed_text_and_attribute_values() {
        assert_eq!(html_escape_string("a+b & \"c\" 'd' <e>"), "a+b &amp; &#34;c&#34; &#39;d&#39; &lt;e&gt;");
    }

    #[test]
    fn urls_filter_schemes_and_escape_components() {
        assert_eq!(url_start("/lauds/2026-01-01"), "/lauds/2026-01-01");
        assert_eq!(
            url_start("https://x.org/new?title=%5Breview%5D+a&labels=review"),
            "https://x.org/new?title=%5Breview%5D+a&labels=review"
        );
        assert_eq!(url_start("javascript:alert(1)"), "#invalid-url");
        assert_eq!(url_start("/a b\"c"), "/a%20b%22c");
        assert_eq!(url_part("a/b c"), "a%2fb%20c");
    }

    #[test]
    fn floats_use_compact_decimal_or_exponent_notation() {
        assert_eq!(format_float(160.0), "160");
        assert_eq!(format_float(2.16), "2.16");
        assert_eq!(format_float(0.00001), "1e-05");
        assert_eq!(format_float(1e20), "1e+20");
        assert_eq!(format_float(123456789.125), "1.23456789125e+08");
        assert_eq!(format_float(0.0001), "0.0001");
        assert_eq!(format_float(999999.5), "999999.5");
        assert_eq!(format_float(-3.5), "-3.5");
    }
}
