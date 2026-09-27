//! HTML rendering of the Office: the text-to-HTML conversion of composed elements, the view models,
//! and the page templates. It touches neither the calendar nor the office engine, and review
//! metadata arrives as plain data.

pub mod escape;
pub mod html;
pub mod leader;
pub mod links;
pub mod pages;
pub mod usage;
pub mod view;

pub use pages::Pages;
