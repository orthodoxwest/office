//! The data directory on disk, as the core crates' [`DataSource`].

use std::path::{Path, PathBuf};

use calendar::DataSource;

pub struct FsData {
    pub dir: PathBuf,
}

impl FsData {
    pub fn new(dir: impl Into<PathBuf>) -> FsData {
        FsData { dir: dir.into() }
    }

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

    fn walk(&self, rel: &str) -> Result<Vec<(String, Vec<u8>)>, String> {
        let root = self.dir.join(rel);
        let mut out = Vec::new();
        walk_dir(&root, "", &mut out)?;
        Ok(out)
    }
}

fn io_error(op: &str, path: &Path, e: &std::io::Error) -> String {
    let msg = match e.kind() {
        std::io::ErrorKind::NotFound => "no such file or directory".to_string(),
        std::io::ErrorKind::PermissionDenied => "permission denied".to_string(),
        _ => e.to_string(),
    };
    format!("{op} {}: {msg}", path.display())
}

/// Go's `filepath.Walk`: lstat the root, then visit directory entries in
/// byte order, depth first. Anything that is not a directory is a file.
fn walk_dir(path: &Path, rel: &str, out: &mut Vec<(String, Vec<u8>)>) -> Result<(), String> {
    let meta = std::fs::symlink_metadata(path).map_err(|e| io_error("lstat", path, &e))?;
    if !meta.is_dir() {
        let bytes = std::fs::read(path).map_err(|e| format!("reading {}: {}", path.display(), io_error("open", path, &e)))?;
        out.push((rel.to_string(), bytes));
        return Ok(());
    }
    let mut names: Vec<std::ffi::OsString> = std::fs::read_dir(path)
        .map_err(|e| io_error("open", path, &e))?
        .map(|entry| entry.map(|e| e.file_name()))
        .collect::<Result<_, _>>()
        .map_err(|e| io_error("readdirent", path, &e))?;
    names.sort_by(|a, b| a.as_encoded_bytes().cmp(b.as_encoded_bytes()));
    for name in names {
        let name_str = name.to_string_lossy();
        let child_rel = if rel.is_empty() { name_str.to_string() } else { format!("{rel}/{name_str}") };
        walk_dir(&path.join(&name), &child_rel, out)?;
    }
    Ok(())
}
