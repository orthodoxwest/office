//! CSV for corpus and review ledgers: comma separator, no comments, strict quotes, and field count
//! fixed by the first record.

/// Parses every record, or returns an error at the first invalid record.
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
}

fn parse_error(line: usize, column: usize, err: ParseErr) -> String {
    let text = match err {
        ParseErr::Quote => "unterminated quoted field or stray quote after one",
        ParseErr::BareQuote => "quote inside an unquoted field",
    };
    format!("line {line}, column {column}: {text}")
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
                    err = Some(parse_error(self.num_line, pos_col + j, ParseErr::BareQuote));
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
                        err = Some(parse_error(self.num_line, pos_col - 1, ParseErr::Quote));
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
                    err = Some(parse_error(pos_line, pos_col, ParseErr::Quote));
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
                return Err(format!("line {rec_line}: expected {} fields, found {}", self.fields_per_record, record.len()));
            }
        } else {
            self.fields_per_record = record.len();
        }
        Ok(Some(record))
    }
}

/// Writes one CSV record with LF line endings.
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
    // Use the Unicode White_Space property.
    field.chars().next().is_some_and(char::is_whitespace)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reads_csv_records() {
        assert_eq!(read_all("a,b\r\n\n\"c,\"\"d\",e\n").unwrap(), vec![vec!["a", "b"], vec!["c,\"d", "e"]]);
        assert_eq!(read_all("a,\"x\ny\"\n").unwrap(), vec![vec!["a", "x\ny"]]);
        assert_eq!(read_all("a,b\r").unwrap(), vec![vec!["a", "b"]]);
        // Malformed records are rejected.
        assert_eq!(read_all("a,b\nc\n").unwrap_err(), "line 2: expected 2 fields, found 1");
        assert_eq!(read_all("a,b\"c\n").unwrap_err(), "line 1, column 4: quote inside an unquoted field");
        assert_eq!(read_all("\"a\"b\n").unwrap_err(), "line 1, column 3: unterminated quoted field or stray quote after one");
        assert_eq!(read_all("x\n\"a\nb").unwrap_err(), "line 3, column 2: unterminated quoted field or stray quote after one");
        assert!(read_all("").unwrap().is_empty());
    }

    #[test]
    fn writes_csv_records() {
        let mut out = String::new();
        write_record(&mut out, &["a", "", "b,c", "say \"hi\"", " lead", "\\.", "x\ny"]);
        assert_eq!(out, "a,,\"b,c\",\"say \"\"hi\"\"\",\" lead\",\"\\.\",\"x\ny\"\n");
    }
}
