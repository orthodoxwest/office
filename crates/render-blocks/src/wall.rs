//! The wall: the plaster's average colour in each look, which every front shows before it has drawn
//! anything else. A cold start is the wall, then its painting: the web's toolbar and installed
//! app, the iOS launch screen and the Android window and splash all open on this colour, the
//! reader's chosen look where the platform lets a launch follow it, so an Apse reader never sees
//! the Nave's cream first. The tests below read each front's copy of it.
//!
//! Where a launch cannot follow a chosen look (the web manifest's one `background_color`, the iOS
//! launch screen, Android 12's splash), it follows the phone's light or dark, as the Default look
//! does.

/// The wall in each look, as `#rrggbb`.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Wall {
    /// The Nave: the limewash by day.
    Nave,
    /// The Apse: the night.
    Apse,
}

impl Wall {
    pub const ALL: [Wall; 2] = [Wall::Nave, Wall::Apse];

    /// The colour, lower-case hex.
    pub fn hex(self) -> &'static str {
        match self {
            Wall::Nave => "#faf3e9",
            Wall::Apse => "#121c28",
        }
    }

    /// The colour's three channels.
    pub fn rgb(self) -> [u8; 3] {
        match self {
            Wall::Nave => [0xfa, 0xf3, 0xe9],
            Wall::Apse => [0x12, 0x1c, 0x28],
        }
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;

    fn read(path: &str) -> String {
        std::fs::read_to_string(Path::new(env!("CARGO_MANIFEST_DIR")).join("../..").join(path)).unwrap_or_else(|e| panic!("{path}: {e}"))
    }

    /// The two spellings of the colour agree.
    #[test]
    fn the_hex_is_the_channels() {
        for w in Wall::ALL {
            let [r, g, b] = w.rgb();
            assert_eq!(w.hex(), format!("#{r:02x}{g:02x}{b:02x}"));
        }
    }

    fn upper(w: Wall) -> String {
        w.hex()[1..].to_uppercase()
    }

    /// The web: style.css's `--bg` in both looks, the toolbar's `theme-color` before the first paint
    /// (layout.html, both the metas and the pre-paint script that follows a chosen look) and after a
    /// change (app.js), the installed app's manifest and the offline page.
    #[test]
    fn the_web_opens_on_the_wall() {
        let (nave, apse) = (Wall::Nave.hex(), Wall::Apse.hex());
        let css = read("apps/office-web/static/style.css");
        assert!(css.contains(&format!("--bg: {nave};")), "style.css's Nave --bg");
        assert_eq!(css.matches(&format!("--bg: {apse};")).count(), 2, "style.css's Apse --bg, by the phone and by choice");
        let layout = read("crates/render-html/templates/layout.html");
        assert!(layout.contains(&format!(r#"<meta name="theme-color" content="{nave}" media="(prefers-color-scheme: light)">"#)));
        assert!(layout.contains(&format!(r#"<meta name="theme-color" content="{apse}" media="(prefers-color-scheme: dark)">"#)));
        assert!(layout.contains(&format!(r#"t === "dark" ? "{apse}" : "{nave}""#)), "the pre-paint script's theme-color");
        let js = read("apps/office-web/static/app.js");
        assert!(js.contains(&format!(r#"var THEME_LIGHT_COLOR = "{nave}";"#)));
        assert!(js.contains(&format!(r#"var THEME_DARK_COLOR = "{apse}";"#)));
        let manifest = read("apps/office-web/static/manifest.webmanifest");
        assert!(manifest.contains(&format!(r#""background_color": "{nave}""#)), "the manifest's launch");
        assert!(manifest.contains(&format!(r#""theme_color": "{nave}""#)));
        let sw = read("apps/office-web/static/sw.js");
        assert!(sw.contains(&format!(r#"<meta name=\"theme-color\" content=\"{nave}\">"#)), "the offline page");
    }

    /// iOS: the launch screen's colour (the phone's light or dark) and the palettes' `bg`.
    #[test]
    fn the_ios_app_opens_on_the_wall() {
        let launch = read("apps/ios/Office/Assets.xcassets/LaunchBackground.colorset/Contents.json");
        for w in Wall::ALL {
            let [r, g, b] = w.rgb();
            let line = format!(r#""blue" : "0x{b:02X}", "green" : "0x{g:02X}", "red" : "0x{r:02X}""#);
            assert!(launch.contains(&line), "LaunchBackground lacks {w:?}: `{line}`");
        }
        let theme = read("apps/ios/Office/Theme.swift");
        for w in Wall::ALL {
            assert!(theme.contains(&format!("bg: Color(hex: 0x{}),", upper(w))), "Theme.swift's {w:?} bg");
        }
    }

    /// Android: the window's colour in each look and by the phone's, which is the splash's ground
    /// (values-v31), and the palettes' `bg`.
    #[test]
    fn the_android_app_opens_on_the_wall() {
        let res = "apps/android/app/src/main/res";
        let day = read(&format!("{res}/values/colors.xml"));
        let night = read(&format!("{res}/values-night/colors.xml"));
        let (nave, apse) = (upper(Wall::Nave), upper(Wall::Apse));
        assert!(day.contains(&format!(r##"<color name="page">#{nave}</color>"##)));
        assert!(day.contains(&format!(r##"<color name="nave_page">#{nave}</color>"##)));
        assert!(day.contains(&format!(r##"<color name="apse_page">#{apse}</color>"##)));
        assert!(night.contains(&format!(r##"<color name="page">#{apse}</color>"##)));
        let splash = read(&format!("{res}/values-v31/themes.xml"));
        for (style, color) in [("Theme.Office", "page"), ("Theme.Office.Nave", "nave_page"), ("Theme.Office.Apse", "apse_page")] {
            let at = splash.find(&format!(r#"<style name="{style}""#)).unwrap_or_else(|| panic!("values-v31 lacks {style}"));
            let body = &splash[at..splash[at..].find("</style>").unwrap() + at];
            assert!(
                body.contains(&format!(r#"<item name="android:windowSplashScreenBackground">@color/{color}</item>"#)),
                "{style}'s splash ground"
            );
            assert!(
                body.contains(r#"<item name="android:windowSplashScreenAnimatedIcon">@drawable/wall</item>"#),
                "{style}'s splash shows only the wall"
            );
        }
        let theme = read("apps/android/app/src/main/kotlin/org/orthodoxwest/office/Theme.kt");
        for w in Wall::ALL {
            assert!(theme.contains(&format!("bg = Color(0xFF{}),", upper(w))), "Theme.kt's {w:?} bg");
        }
    }
}
