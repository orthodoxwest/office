//! The data directory, compiled into the library.

use calendar::DataSource;

mod files {
    include!(concat!(env!("OUT_DIR"), "/data_files.rs"));
}

/// The embedded corpus, calendar, and hour definitions.
pub struct EmbeddedData;

impl DataSource for EmbeddedData {
    fn display_path(&self, rel: &str) -> String {
        format!("data/{rel}")
    }

    fn read(&self, rel: &str) -> Result<Option<String>, String> {
        match files::FILES.iter().find(|(path, _)| *path == rel) {
            Some((_, bytes)) => {
                String::from_utf8(bytes.to_vec()).map(Some).map_err(|_| format!("{}: invalid UTF-8", self.display_path(rel)))
            }
            None => Ok(None),
        }
    }

    fn walk(&self, rel: &str) -> Result<Vec<(String, Vec<u8>)>, String> {
        let prefix = format!("{}/", rel.trim_end_matches('/'));
        Ok(files::FILES.iter().filter_map(|(path, bytes)| path.strip_prefix(&prefix).map(|p| (p.to_string(), bytes.to_vec()))).collect())
    }
}
