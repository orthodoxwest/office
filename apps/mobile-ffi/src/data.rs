//! The data directory, compiled into the library.

use calendar::DataSource;

mod files {
    include!(concat!(env!("OUT_DIR"), "/data_files.rs"));
}

/// The embedded corpus, calendar, and hour definitions, inflated. Build one to load the engine
/// and let it go: the engine keeps what it needs.
pub struct EmbeddedData {
    files: Vec<(&'static str, Vec<u8>)>,
}

impl EmbeddedData {
    /// Inflates the data compiled in by build.rs, each file from its own stream.
    pub fn new() -> Result<EmbeddedData, String> {
        let inflate = |&(path, start, end, len): &(&'static str, usize, usize, usize)| {
            let stream = files::DEFLATED.get(start..end).ok_or_else(|| format!("data/{path}: outside the embedded data"))?;
            let bytes =
                miniz_oxide::inflate::decompress_to_vec_with_limit(stream, len).map_err(|e| format!("inflating data/{path}: {e:?}"))?;
            Ok((path, bytes))
        };
        Ok(EmbeddedData { files: files::FILES.iter().map(inflate).collect::<Result<_, String>>()? })
    }

    fn files(&self) -> impl Iterator<Item = (&'static str, &[u8])> {
        self.files.iter().map(|(path, bytes)| (*path, bytes.as_slice()))
    }
}

impl DataSource for EmbeddedData {
    fn display_path(&self, rel: &str) -> String {
        format!("data/{rel}")
    }

    fn read(&self, rel: &str) -> Result<Option<String>, String> {
        match self.files().find(|(path, _)| *path == rel) {
            Some((_, bytes)) => {
                String::from_utf8(bytes.to_vec()).map(Some).map_err(|_| format!("{}: invalid UTF-8", self.display_path(rel)))
            }
            None => Ok(None),
        }
    }

    fn walk(&self, rel: &str) -> Result<Vec<(String, Vec<u8>)>, String> {
        let prefix = format!("{}/", rel.trim_end_matches('/'));
        Ok(self.files().filter_map(|(path, bytes)| path.strip_prefix(&prefix).map(|p| (p.to_string(), bytes.to_vec()))).collect())
    }
}
