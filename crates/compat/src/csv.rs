//! Go's `encoding/csv` with its default settings (comma separator, no
//! comments, strict quotes, field count fixed by the first record), so the
//! review ledgers read, fail, and write byte for byte as in Go.

/// `csv.NewReader(r).ReadAll()`. The error is Go's `ParseError` text.
pub fn read_all(input: &str) -> Result<Vec<Vec<String>>, String> {
    let mut reader = Reader { input: input.as_bytes(), pos: 0, num_line: 0, fields_per_record: 0 };
    let mut records = Vec::new();
    loop {
        match reader.read_record() {
            Ok(Some(record)) => records.push(record),
            Ok(None) => return Ok(records),
            Err(e) => return Err(e),
        }
    }
}

struct Reader<'a> {
    input: &'a [u8],
    pos: usize,
    num_line: usize,
    fields_per_record: usize,
}

enum ParseErr {
    Quote,
    BareQuote,
    FieldCount,
}

fn parse_error(start_line: usize, line: usize, column: usize, err: ParseErr) -> String {
    let text = match err {
        ParseErr::FieldCount => return format!("record on line {line}: wrong number of fields"),
        ParseErr::Quote => "extraneous or missing \" in quoted-field",
        ParseErr::BareQuote => "bare \" in non-quoted-field",
    };
    if start_line != line {
        return format!("record on line {start_line}; parse error on line {line}, column {column}: {text}");
    }
    format!("parse error on line {line}, column {column}: {text}")
}

fn length_nl(b: &[u8]) -> usize {
    usize::from(b.last() == Some(&b'\n'))
}

impl Reader<'_> {
    /// One line with `\r\n` normalized to `\n`; `eof` when nothing was left.
    fn read_line(&mut self) -> (Vec<u8>, bool) {
        let rest = &self.input[self.pos..];
        let (mut line, hit_eof) = match rest.iter().position(|&c| c == b'\n') {
            Some(i) => (rest[..=i].to_vec(), false),
            None => (rest.to_vec(), true),
        };
        self.pos += line.len();
        let eof = hit_eof && line.is_empty();
        if hit_eof && line.last() == Some(&b'\r') {
            line.pop();
        }
        self.num_line += 1;
        let n = line.len();
        if n >= 2 && line[n - 2] == b'\r' && line[n - 1] == b'\n' {
            line[n - 2] = b'\n';
            line.pop();
        }
        (line, eof)
    }

    fn read_record(&mut self) -> Result<Option<Vec<String>>, String> {
        let (line, eof) = loop {
            let (line, eof) = self.read_line();
            if !eof && line.len() == length_nl(&line) {
                continue;
            }
            break (line, eof);
        };
        if eof {
            return Ok(None);
        }
        let rec_line = self.num_line;
        let mut buffer: Vec<u8> = Vec::new();
        let mut indexes: Vec<usize> = Vec::new();
        let (mut pos_line, mut pos_col) = (self.num_line, 1usize);
        let mut err = None;
        let mut owned = line;
        let mut offset = 0usize;
        'parse: loop {
            let cur = &owned[offset..];
            if cur.first() != Some(&b'"') {
                let i = cur.iter().position(|&c| c == b',');
                let field = match i {
                    Some(i) => &cur[..i],
                    None => &cur[..cur.len() - length_nl(cur)],
                };
                if let Some(j) = field.iter().position(|&c| c == b'"') {
                    err = Some(parse_error(rec_line, self.num_line, pos_col + j, ParseErr::BareQuote));
                    break 'parse;
                }
                buffer.extend_from_slice(field);
                indexes.push(buffer.len());
                match i {
                    Some(i) => {
                        offset += i + 1;
                        pos_col += i + 1;
                        continue 'parse;
                    }
                    None => break 'parse,
                }
            }
            // Quoted field.
            offset += 1;
            pos_col += 1;
            loop {
                let cur = &owned[offset..];
                if let Some(i) = cur.iter().position(|&c| c == b'"') {
                    buffer.extend_from_slice(&cur[..i]);
                    offset += i + 1;
                    pos_col += i + 1;
                    let rest = &owned[offset..];
                    if rest.first() == Some(&b'"') {
                        buffer.push(b'"');
                        offset += 1;
                        pos_col += 1;
                    } else if rest.first() == Some(&b',') {
                        offset += 1;
                        pos_col += 1;
                        indexes.push(buffer.len());
                        continue 'parse;
                    } else if length_nl(rest) == rest.len() {
                        indexes.push(buffer.len());
                        break 'parse;
                    } else {
                        err = Some(parse_error(rec_line, self.num_line, pos_col - 1, ParseErr::Quote));
                        break 'parse;
                    }
                } else if !cur.is_empty() {
                    buffer.extend_from_slice(cur);
                    pos_col += cur.len();
                    let (next, _) = self.read_line();
                    if !next.is_empty() {
                        pos_line += 1;
                        pos_col = 1;
                    }
                    owned = next;
                    offset = 0;
                } else {
                    err = Some(parse_error(rec_line, pos_line, pos_col, ParseErr::Quote));
                    break 'parse;
                }
            }
        }
        if let Some(e) = err {
            return Err(e);
        }
        let text = String::from_utf8_lossy(&buffer).into_owned();
        let mut record = Vec::with_capacity(indexes.len());
        let mut prev = 0;
        for idx in indexes {
            record.push(text[prev..idx].to_string());
            prev = idx;
        }
        if self.fields_per_record > 0 {
            if record.len() != self.fields_per_record {
                return Err(parse_error(rec_line, rec_line, 1, ParseErr::FieldCount));
            }
        } else {
            self.fields_per_record = record.len();
        }
        Ok(Some(record))
    }
}

/// One record as Go's `csv.Writer.Write` writes it (LF line endings).
pub fn write_record(out: &mut String, record: &[&str]) {
    for (n, field) in record.iter().enumerate() {
        if n > 0 {
            out.push(',');
        }
        if !needs_quotes(field) {
            out.push_str(field);
            continue;
        }
        out.push('"');
        out.push_str(&field.replace('"', "\"\""));
        out.push('"');
    }
    out.push('\n');
}

fn needs_quotes(field: &str) -> bool {
    if field.is_empty() {
        return false;
    }
    if field == "\\." || field.bytes().any(|c| matches!(c, b'\n' | b'\r' | b'"' | b',')) {
        return true;
    }
    // Go's unicode.IsSpace is the White_Space property, as here.
    field.chars().next().is_some_and(char::is_whitespace)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reads_like_go() {
        assert_eq!(read_all("a,b\r\n\n\"c,\"\"d\",e\n").unwrap(), vec![vec!["a", "b"], vec!["c,\"d", "e"]]);
        assert_eq!(read_all("a,\"x\ny\"\n").unwrap(), vec![vec!["a", "x\ny"]]);
        assert_eq!(read_all("a,b\r").unwrap(), vec![vec!["a", "b"]]);
        assert_eq!(read_all("a,b\nc\n").unwrap_err(), "record on line 2: wrong number of fields");
        assert_eq!(read_all("a,b\"c\n").unwrap_err(), "parse error on line 1, column 4: bare \" in non-quoted-field");
        assert_eq!(read_all("\"a\"b\n").unwrap_err(), "parse error on line 1, column 3: extraneous or missing \" in quoted-field");
        assert_eq!(
            read_all("x\n\"a\nb").unwrap_err(),
            "record on line 2; parse error on line 3, column 2: extraneous or missing \" in quoted-field"
        );
        assert!(read_all("").unwrap().is_empty());
    }

    #[test]
    fn writes_like_go() {
        let mut out = String::new();
        write_record(&mut out, &["a", "", "b,c", "say \"hi\"", " lead", "\\.", "x\ny"]);
        assert_eq!(out, "a,,\"b,c\",\"say \"\"hi\"\"\",\" lead\",\"\\.\",\"x\ny\"\n");
    }
}
