//! JSON output for review reports: two-space indentation and HTML escaping. Object fields keep
//! insertion order.

/// A JSON value. `omitempty` is the builder's job: leave the field out.
#[derive(Clone, Debug, PartialEq)]
pub enum Json {
    Null,
    Bool(bool),
    Int(i64),
    Str(String),
    Arr(Vec<Json>),
    Obj(Vec<(String, Json)>),
}

impl Json {
    pub fn str(s: impl Into<String>) -> Json {
        Json::Str(s.into())
    }

    /// An array of strings, including an empty array.
    pub fn strings(values: &[String]) -> Json {
        Json::Arr(values.iter().map(|v| Json::Str(v.clone())).collect())
    }
}

/// An object under construction, with helpers for omitting empty fields.
#[derive(Default)]
pub struct Obj(Vec<(String, Json)>);

impl Obj {
    pub fn new() -> Obj {
        Obj(Vec::new())
    }

    pub fn field(mut self, name: &str, value: Json) -> Obj {
        self.0.push((name.to_string(), value));
        self
    }

    pub fn str(self, name: &str, value: &str) -> Obj {
        self.field(name, Json::str(value))
    }

    pub fn str_omitempty(self, name: &str, value: &str) -> Obj {
        if value.is_empty() { self } else { self.str(name, value) }
    }

    pub fn strings_omitempty(self, name: &str, values: &[String]) -> Obj {
        if values.is_empty() { self } else { self.field(name, Json::strings(values)) }
    }

    pub fn bool_omitempty(self, name: &str, value: bool) -> Obj {
        if value { self.field(name, Json::Bool(true)) } else { self }
    }

    pub fn int(self, name: &str, value: i64) -> Obj {
        self.field(name, Json::Int(value))
    }

    pub fn build(self) -> Json {
        Json::Obj(self.0)
    }
}

/// `json.NewEncoder(w).SetIndent("", "  ")` then `Encode(v)`: indented, with
/// a trailing newline.
pub fn encode_indent(v: &Json) -> String {
    let mut out = String::new();
    write_value(&mut out, v, 0);
    out.push('\n');
    out
}

/// `json.Marshal(v)`: compact, with the same escaping.
pub fn encode_compact(v: &Json) -> String {
    let mut out = String::new();
    write_compact(&mut out, v);
    out
}

fn write_compact(out: &mut String, v: &Json) {
    match v {
        Json::Arr(items) => {
            out.push('[');
            for (i, item) in items.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                write_compact(out, item);
            }
            out.push(']');
        }
        Json::Obj(fields) => {
            out.push('{');
            for (i, (name, value)) in fields.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                write_string(out, name);
                out.push(':');
                write_compact(out, value);
            }
            out.push('}');
        }
        Json::Null | Json::Bool(_) | Json::Int(_) | Json::Str(_) => write_value(out, v, 0),
    }
}

fn indent(out: &mut String, depth: usize) {
    out.push('\n');
    for _ in 0..depth {
        out.push_str("  ");
    }
}

fn write_value(out: &mut String, v: &Json, depth: usize) {
    match v {
        Json::Null => out.push_str("null"),
        Json::Bool(b) => out.push_str(if *b { "true" } else { "false" }),
        Json::Int(n) => out.push_str(&n.to_string()),
        Json::Str(s) => write_string(out, s),
        Json::Arr(items) => {
            if items.is_empty() {
                out.push_str("[]");
                return;
            }
            out.push('[');
            for (i, item) in items.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                indent(out, depth + 1);
                write_value(out, item, depth + 1);
            }
            indent(out, depth);
            out.push(']');
        }
        Json::Obj(fields) => {
            if fields.is_empty() {
                out.push_str("{}");
                return;
            }
            out.push('{');
            for (i, (name, value)) in fields.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                indent(out, depth + 1);
                write_string(out, name);
                out.push_str(": ");
                write_value(out, value, depth + 1);
            }
            indent(out, depth);
            out.push('}');
        }
    }
}

/// String encoding with HTML escaping: `<`, `>`, `&`, U+2028, and U+2029 as `\u` escapes; `\b`,
/// `\f`, `\n`, `\r`, `\t` short forms.
fn write_string(out: &mut String, s: &str) {
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '\u{8}' => out.push_str("\\b"),
            '\u{c}' => out.push_str("\\f"),
            '<' | '>' | '&' | '\u{2028}' | '\u{2029}' => out.push_str(&format!("\\u{:04x}", c as u32)),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn compact_like_marshal() {
        let v = Json::Arr(vec![
            Obj::new().str("Key", "a<b").field("Points", Json::Null).field("N", Json::Arr(vec![Json::Int(1), Json::Int(2)])).build(),
        ]);
        assert_eq!(encode_compact(&v), "[{\"Key\":\"a\\u003cb\",\"Points\":null,\"N\":[1,2]}]");
    }

    #[test]
    fn encodes_report_json() {
        let v = Obj::new()
            .str("a", "x<y>&\u{2028}\u{1}\u{8}é")
            .field("empty", Json::Arr(Vec::new()))
            .field("none", Json::Null)
            .field("list", Json::strings(&["p".to_string()]))
            .field("obj", Obj::new().int("n", -3).bool_omitempty("f", false).build())
            .build();
        assert_eq!(
            encode_indent(&v),
            "{\n  \"a\": \"x\\u003cy\\u003e\\u0026\\u2028\\u0001\\bé\",\n  \"empty\": [],\n  \"none\": null,\n  \"list\": [\n    \"p\"\n  ],\n  \"obj\": {\n    \"n\": -3\n  }\n}\n"
        );
    }
}
