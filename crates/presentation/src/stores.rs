//! Where the native apps are found: the web's colophon tells a reader of them in one quiet line
//! ("Also an app for Android and iPhone", layout.html), each name linking to its store, and a
//! reader on a phone sees only their own phone's (the pre-paint script marks the platform).
//! Nothing here interrupts a page: no banner of our own, no prompt, nothing over the prayers.

/// The Android app's Google Play listing.
pub const PLAY_URL: &str = "https://play.google.com/store/apps/details?id=org.orthodoxwest.office";

/// The iOS app's numeric App Store ID (App Store Connect → App Information → Apple ID), or
/// `None` while the app is not yet on the App Store. Setting it is the whole of the iOS switch:
/// the colophon names iPhone beside Android, iPhone and iPad readers see their store's link in
/// place of none, and Safari offers its own Smart App Banner (`apple-itunes-app`, layout.html).
pub const APP_STORE_ID: Option<&str> = None;

/// The colophon's words before the platforms.
pub const APPS_LEAD: &str = "Also an app for";

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
