//! The data directory, compiled into the library.

use calendar::DataSource;

mod files {
    include!(concat!(env!("OUT_DIR"), "/data_files.rs"));
}

/// The embedded corpus, calendar, and hour definitions, inflated. Build one to load the engine
/// and let it go: the engine keeps what it needs.
pub struct EmbeddedData {
    bytes: Vec<u8>,
}

impl EmbeddedData {
    /// Inflates the data compiled in by build.rs.
    pub fn new() -> Result<EmbeddedData, String> {
        let bytes = miniz_oxide::inflate::decompress_to_vec_with_limit(files::DEFLATED, files::LEN)
            .map_err(|e| format!("inflating the embedded data: {e:?}"))?;
        Ok(EmbeddedData { bytes })
    }

    fn files(&self) -> impl Iterator<Item = (&'static str, &[u8])> {
        files::FILES.iter().filter_map(|&(path, start, end)| self.bytes.get(start..end).map(|b| (path, b)))
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
