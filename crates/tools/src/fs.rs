//! The data directory on disk, as the core crates' [`DataSource`].

use std::path::{Component, Path, PathBuf};

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
            for candidate in [clean(&dir.join("data")), clean(&dir.join("..").join("..").join("data"))] {
                if candidate.is_dir() {
                    return Some(FsData { dir: candidate });
                }
            }
        }
        Path::new("data").is_dir().then(|| FsData { dir: PathBuf::from("data") })
    }
}

/// Replaces `path` atomically, as the Go review tools do: a temporary file
/// beside it, synced, made 0644, then renamed over it.
pub fn write_atomic(path: &Path, contents: &[u8]) -> Result<(), String> {
    use std::io::Write as _;
    let dir = path.parent().unwrap_or(Path::new("."));
    std::fs::create_dir_all(dir).map_err(|e| io_error("mkdir", dir, &e))?;
    let name = path.file_name().map(|n| n.to_string_lossy().into_owned()).unwrap_or_default();
    let tmp = dir.join(format!(".{name}.{}.tmp", std::process::id()));
    let result = (|| {
        let mut f = std::fs::File::create(&tmp).map_err(|e| io_error("open", &tmp, &e))?;
        f.write_all(contents).map_err(|e| io_error("write", &tmp, &e))?;
        f.sync_all().map_err(|e| io_error("sync", &tmp, &e))?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt as _;
            std::fs::set_permissions(&tmp, std::fs::Permissions::from_mode(0o644)).map_err(|e| io_error("chmod", &tmp, &e))?;
        }
        std::fs::rename(&tmp, path).map_err(|e| io_error("rename", &tmp, &e))
    })();
    if result.is_err() {
        let _ = std::fs::remove_file(&tmp);
    }
    result
}

/// Go's `filepath.Clean`: the shortest lexically equivalent path.
pub fn clean(path: &Path) -> PathBuf {
    let mut out: Vec<Component> = Vec::new();
    for c in path.components() {
        match c {
            Component::CurDir => {}
            Component::ParentDir => match out.last() {
                Some(Component::Normal(_)) => {
                    out.pop();
                }
                Some(Component::RootDir | Component::Prefix(_)) => {}
                Some(Component::ParentDir | Component::CurDir) | None => out.push(c),
            },
            Component::Normal(_) | Component::RootDir | Component::Prefix(_) => out.push(c),
        }
    }
    if out.is_empty() {
        return PathBuf::from(".");
    }
    out.iter().collect()
}

/// Go's `filepath.Rel` for two absolute paths: `target` relative to `base`.
pub fn rel(base: &Path, target: &Path) -> PathBuf {
    let (base, target) = (clean(base), clean(target));
    let b: Vec<Component> = base.components().collect();
    let t: Vec<Component> = target.components().collect();
    let common = b.iter().zip(&t).take_while(|(x, y)| x == y).count();
    let mut out = PathBuf::new();
    for _ in common..b.len() {
        out.push("..");
    }
    for c in &t[common..] {
        out.push(c);
    }
    if out.as_os_str().is_empty() { PathBuf::from(".") } else { out }
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn clean_and_rel_follow_go() {
        assert_eq!(clean(Path::new("/a/b/../../data")), PathBuf::from("/data"));
        assert_eq!(clean(Path::new("./data/")), PathBuf::from("data"));
        assert_eq!(clean(Path::new("../x/./y/..")), PathBuf::from("../x"));
        assert_eq!(clean(Path::new("")), PathBuf::from("."));
        assert_eq!(rel(Path::new("/home/u/office"), Path::new("/home/u/office/data")), PathBuf::from("data"));
        assert_eq!(rel(Path::new("/tmp/x"), Path::new("/home/u/data")), PathBuf::from("../../home/u/data"));
        assert_eq!(rel(Path::new("/a"), Path::new("/a")), PathBuf::from("."));
    }
}
