//! The liturgical calendar shared by every product: computus and moveable
//! dates, seasons, the feast catalog, occurrence, the temporal week, octaves,
//! and fasting. Ported from Go's `internal/calendar` (see RUST-PORT.md).
//!
//! The crate does no file, network, or clock access: callers supply data
//! files through [`loader::DataSource`] and pass dates explicitly.

pub mod builder;
pub mod commemoration;
pub mod computus;
pub mod date;
pub mod loader;
pub mod model;
pub mod occurrence;
pub mod penitential;
pub mod traits;

pub use builder::{YearCalendar, build_calendar};
pub use computus::{MoveableDates, Tabula};
pub use date::{Date, Weekday};
pub use loader::{CalendarData, DataSource};
pub use model::{CalendarDay, Category, Color, Decision, Feast, FeastRef, Rank, Season};

#[cfg(test)]
mod behavior_tests;
