//! Where the native apps are found, and the web's words for them: the colophon's quiet line
//! ("Also an app for Android, iPhone and iPad", layout.html), the About page's paragraph, and the
//! Reminders page, which offers the app first and the calendar feed as the fallback, since an
//! app's notifications are far better reminders than a subscribed calendar's. A reader on a phone
//! sees only their own phone's store (the pre-paint script marks the platform). Nothing here
//! interrupts a page: no banner of our own, no prompt, nothing over the prayers.

/// The Android app's Google Play listing.
pub const PLAY_URL: &str = "https://play.google.com/store/apps/details?id=org.orthodoxwest.office";

/// The iOS app's numeric App Store ID (App Store Connect → App Information → Apple ID), or
/// `None` while the app is not yet on the App Store. Setting it is the whole of the iOS switch:
/// the colophon names iPhone and iPad beside Android, iPhone and iPad readers see their store's link in
/// place of none, and Safari offers its own Smart App Banner (`apple-itunes-app`, layout.html).
pub const APP_STORE_ID: Option<&str> = Some("6828679748");

/// The colophon's words before the platforms, and its name for Apple's (the iOS app is built for
/// both, apps/ios/project.yml's TARGETED_DEVICE_FAMILY).
pub const APPS_LEAD: &str = "Also an app for";
pub const APPS_APPLE: &str = "iPhone and iPad";

/// The store buttons' words, as the stores' own badges word them.
pub const PLAY_LABEL: &str = "Get it on Google Play";
pub const APP_STORE_LABEL: &str = "Download on the App Store";

/// The About page's paragraph on the apps, after "This app" (web only: the apps have no need of it).
pub const ABOUT_APPS: &str = "The Daily Office is also available as a mobile app, with full offline access and prayer \
     reminders that open straight to the hour.";

/// The Reminders page's first word: the app.
pub const REMINDERS_APP: &str = "The Daily Office app is the best way to be reminded. At each hour you choose, a \
     notification names the office and the day it keeps, and a tap opens it, even without a connection.";

/// The fold that holds the calendar feed, shut on a phone that can have the app.
pub const REMINDERS_CALENDAR: &str = "Use a calendar instead";

/// The calendar feed's own introduction, inside that fold: its lead, said only where the app is
/// offered, then the rest.
pub const REMINDERS_CALENDAR_LEAD: &str = "If you can’t use the app, your calendar can remind you instead.";
pub const REMINDERS_CALENDAR_INTRO: &str = "Choose the hours you pray and their times. Each calendar event names \
     the office and feast of the day. Your schedule lives entirely in the private calendar link below; nothing \
     is stored on this site.";

/// The App Store listing for [`APP_STORE_ID`], once there is one.
#[must_use]
pub fn app_store_url() -> Option<String> {
    APP_STORE_ID.map(|id| format!("https://apps.apple.com/app/id{id}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn app_store_id_is_bare_digits() {
        // A pasted URL or "id123" here would make a dead link and a banner Safari ignores.
        if let Some(id) = APP_STORE_ID {
            assert!(!id.is_empty() && id.bytes().all(|b| b.is_ascii_digit()), "{id}");
        }
    }

    #[test]
    fn play_url_names_the_package() {
        assert!(PLAY_URL.ends_with("?id=org.orthodoxwest.office"));
    }
}
