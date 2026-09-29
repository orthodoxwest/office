//! Build-time processing of the embedded stylesheet: its comments are
//! dropped and the assets it references are stamped with their content hash.
//!
//! style.css carries its design notes as comments, which were well over half
//! of its compressed bytes, and every page blocks on it. Its fonts and wall
//! images are relative `url()`s; stamped, they are cached as immutable like
//! every other static URL instead of refetched on each visit.
//!
//! Shared by build.rs (through `#[path]`) and the unit tests, so it depends
//! on nothing outside std.

/// Characters that fuse into one token when nothing separates them.
fn name_char(c: char) -> bool {
    c.is_ascii_alphanumeric() || c == '-' || c == '_' || !c.is_ascii()
}

/// Removes `/* … */` comments outside strings, then blank lines and
/// trailing spaces. A comment between two name characters (`1px/**/2px`)
/// leaves a space, as the tokenizer would; strings are never touched.
pub fn strip_comments(src: &str) -> String {
    let mut out = String::with_capacity(src.len());
    let mut chars = src.chars().peekable();
    let mut quote: Option<char> = None;
    while let Some(c) = chars.next() {
        if let Some(q) = quote {
            out.push(c);
            if c == '\\' {
                if let Some(escaped) = chars.next() {
                    out.push(escaped);
                }
            } else if c == q {
                quote = None;
            }
            continue;
        }
        match c {
            '"' | '\'' => {
                quote = Some(c);
                out.push(c);
            }
            '/' if chars.peek() == Some(&'*') => {
                chars.next();
                let mut prev = '\0';
                for c in chars.by_ref() {
                    if prev == '*' && c == '/' {
                        break;
                    }
                    prev = c;
                }
                if out.chars().next_back().is_some_and(name_char) && chars.peek().is_some_and(|&c| name_char(c)) {
                    out.push(' ');
                }
            }
            _ => out.push(c),
        }
    }
    let mut trimmed = String::with_capacity(out.len());
    for line in out.lines().map(str::trim_end).filter(|l| !l.trim().is_empty()) {
        trimmed.push_str(line);
        trimmed.push('\n');
    }
    trimmed
}

/// Appends `?v=STAMP` to every quoted relative `url()`. `stamp` receives the
/// path relative to the stylesheet; a reference it does not know, or an
/// unquoted relative one, is an error, so a renamed asset fails the build
/// rather than going unstamped.
pub fn stamp_urls(css: &str, stamp: impl Fn(&str) -> Option<String>) -> Result<String, String> {
    let mut out = String::with_capacity(css.len() + 256);
    let mut rest = css;
    while let Some(at) = rest.find("url(") {
        let (head, tail) = rest.split_at(at + 4);
        out.push_str(head);
        let quote = tail.chars().next().filter(|c| *c == '"' || *c == '\'');
        let Some(q) = quote else {
            let target = tail.split(')').next().unwrap_or("").trim();
            if target.starts_with("data:") || target.starts_with('#') || target.contains("://") || target.starts_with('/') {
                rest = tail;
                continue;
            }
            return Err(format!("unquoted relative url({target})"));
        };
        let end = tail[1..].find(q).ok_or("unterminated url()")? + 1;
        let target = &tail[1..end];
        out.push(q);
        out.push_str(target);
        if !(target.starts_with("data:") || target.starts_with('#') || target.contains("://") || target.starts_with('/')) {
            let v = stamp(target).ok_or_else(|| format!("url(\"{target}\") names no embedded file"))?;
            out.push_str("?v=");
            out.push_str(&v);
        }
        rest = &tail[end..];
    }
    out.push_str(rest);
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::{stamp_urls, strip_comments};

    #[test]
    fn drops_comments_and_the_lines_they_leave() {
        let src = "/* head */\n\na {\n  color: red; /* why */\n  /* note\n     spanning */\n  margin: 0;\n}\n";
        assert_eq!(strip_comments(src), "a {\n  color: red;\n  margin: 0;\n}\n");
    }

    #[test]
    fn keeps_strings_and_urls_intact() {
        let src = "a::after { content: \"/* not a comment */\"; }\nb { content: '\\'/*'; background: url(\"x/*y.png\"); }\n";
        assert_eq!(strip_comments(src), src);
    }

    #[test]
    fn a_comment_never_joins_the_tokens_around_it() {
        assert_eq!(strip_comments("a /* x */ .b {}\n"), "a  .b {}\n");
        assert_eq!(strip_comments("a/**/.b {}\n"), "a.b {}\n");
        assert_eq!(strip_comments("p { margin: 1px/**/2px; }\n"), "p { margin: 1px 2px; }\n");
        assert_eq!(strip_comments("a{b:c}/**/\n"), "a{b:c}\n");
    }

    #[test]
    fn stamps_relative_urls_and_leaves_the_rest() {
        let stamp = |p: &str| (p == "fonts/a.woff2" || p == "wall.jpg").then(|| format!("h-{}", p.len()));
        let css = "src: url(\"fonts/a.woff2\") format(\"woff2\");\n--w: url('wall.jpg');\nmask: url(data:image/svg+xml,x) url(\"#m\") url(/abs.png);\n";
        assert_eq!(
            stamp_urls(css, stamp).unwrap(),
            "src: url(\"fonts/a.woff2?v=h-13\") format(\"woff2\");\n--w: url('wall.jpg?v=h-8');\nmask: url(data:image/svg+xml,x) url(\"#m\") url(/abs.png);\n"
        );
        assert!(stamp_urls("a { b: url(\"gone.png\"); }", stamp).unwrap_err().contains("gone.png"));
        assert!(stamp_urls("a { b: url(wall.jpg); }", stamp).is_err());
    }

    #[test]
    fn the_served_stylesheet_is_trimmed_and_stamped() {
        let css = std::str::from_utf8(crate::pwa::file("static/style.css").unwrap()).unwrap();
        assert!(!css.contains("/*"), "style.css is served with its comments");
        let font = crate::pwa::stamp("static/fonts/eb-garamond-regular.woff2").unwrap();
        assert!(css.contains(&format!("url(\"fonts/eb-garamond-regular.woff2?v={font}\")")));
        assert!(!css.contains(".woff2\")") && !css.contains(".jpg\")"), "an asset url is unstamped");
    }
}
