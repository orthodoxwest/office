//! Data tooling over any corpus: the filesystem [`fs::FsData`] source and
//! the validators. Unlike the core crates, tools may touch the filesystem.

pub mod audit;
pub mod corpus_edit;
pub mod fs;
pub mod review;
pub mod scaffold;
pub mod validate;
pub mod year;
