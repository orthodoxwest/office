//! The data directory on disk, as the core crates' [`DataSource`].

use std::path::{Path, PathBuf};

use calendar::DataSource;

pub struct FsData {
    pub dir: PathBuf,
}

impl FsData {
    /// Finds `data/` beside the executable, two levels above it (a cargo
    /// `target/<profile>/` build), or in the working directory, as Go's
    /// `FindDataDir` does.
    pub fn find() -> Option<FsData> {
        if let Ok(exe) = std::env::current_exe()
            && let Some(dir) = exe.parent()
        {
            for candidate in [dir.join("data"), dir.join("..").join("..").join("data")] {
                if candidate.is_dir() {
                    return Some(FsData { dir: candidate });
                }
            }
        }
        Path::new("data").is_dir().then(|| FsData { dir: PathBuf::from("data") })
    }
}

impl DataSource for FsData {
    fn display_path(&self, rel: &str) -> String {
        self.dir.join(rel).display().to_string()
    }

    fn read(&self, rel: &str) -> Result<Option<String>, String> {
        match std::fs::read_to_string(self.dir.join(rel)) {
            Ok(s) => Ok(Some(s)),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(None),
            Err(e) => Err(format!("open {}: {e}", self.display_path(rel))),
        }
    }
}
