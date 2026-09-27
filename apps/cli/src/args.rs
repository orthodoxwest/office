//! A small flag parser that accepts what Go's `flag` package accepts:
//! `-name value`, `-name=value`, `--name value`, boolean `-name`, and parsing
//! stops at the first non-flag. The error messages are our own.

use std::collections::HashMap;

pub struct Flags {
    values: HashMap<String, String>,
    pub rest: Vec<String>,
}

impl Flags {
    /// Parses `args` against the known flag names (all take a value).
    pub fn parse(args: &[String], known: &[&str]) -> Result<Flags, String> {
        Flags::parse_with_bools(args, known, &[])
    }

    /// Parses `args` against flags that take a value and boolean flags,
    /// which never consume the next argument (`-x` or `-x=false`).
    pub fn parse_with_bools(args: &[String], known: &[&str], bools: &[&str]) -> Result<Flags, String> {
        let mut values = HashMap::new();
        let mut i = 0;
        while i < args.len() {
            let arg = &args[i];
            if arg == "--" {
                i += 1;
                break;
            }
            let Some(body) = arg.strip_prefix("--").or_else(|| arg.strip_prefix('-')).filter(|b| !b.is_empty()) else {
                break;
            };
            let (name, value) = match body.split_once('=') {
                Some((n, v)) => (n.to_string(), v.to_string()),
                None if bools.contains(&body) => (body.to_string(), "true".to_string()),
                None => {
                    let v = args.get(i + 1).ok_or_else(|| format!("flag -{body} needs a value"))?;
                    i += 1;
                    (body.to_string(), v.clone())
                }
            };
            if bools.contains(&name.as_str()) {
                if parse_bool(&value).is_none() {
                    return Err(format!("flag -{name}: {} is not true or false", compat::quote(&value)));
                }
            } else if !known.contains(&name.as_str()) {
                return Err(format!("unknown flag -{name}"));
            }
            values.insert(name, value);
            i += 1;
        }
        Ok(Flags { values, rest: args[i..].to_vec() })
    }

    pub fn str(&self, name: &str) -> &str {
        self.values.get(name).map_or("", String::as_str)
    }

    /// A boolean flag's value (false when absent).
    pub fn bool(&self, name: &str) -> bool {
        self.values.get(name).and_then(|v| parse_bool(v)).unwrap_or(false)
    }

    /// A string flag with a default.
    pub fn str_or<'a>(&'a self, name: &str, default: &'a str) -> &'a str {
        self.values.get(name).map_or(default, String::as_str)
    }

    pub fn int(&self, name: &str, default: i32) -> Result<i32, String> {
        match self.values.get(name) {
            None => Ok(default),
            Some(v) => v.parse().map_err(|_| format!("flag -{name}: {} is not an integer", compat::quote(v))),
        }
    }
}

/// Splits a comma-separated list, trimming and dropping empty parts.
pub fn split_list(value: &str) -> Vec<String> {
    value.split(',').map(str::trim).filter(|p| !p.is_empty()).map(str::to_string).collect()
}

/// Go's `strconv.ParseBool`.
fn parse_bool(v: &str) -> Option<bool> {
    match v {
        "1" | "t" | "T" | "true" | "TRUE" | "True" => Some(true),
        "0" | "f" | "F" | "false" | "FALSE" | "False" => Some(false),
        _ => None,
    }
}
