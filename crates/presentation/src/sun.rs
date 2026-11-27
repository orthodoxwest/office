//! Reminders that follow the sun, for the native apps: sunrise, solar noon and sunset for a
//! place and day, and each office's time from them. The day from sunrise to sunset is divided
//! into twelve hours: Lauds at sunrise, Prime at the first hour, Terce at the third, Sext at
//! solar noon, None at the ninth, Vespers at sunset. Compline keeps the reader's own time, but
//! never sooner than an hour after sunset. The sun's position follows NOAA's solar calculator
//! (after Meeus), good to about a minute away from the poles.

use calendar::Date;

/// Past this latitude, north or south, the days are too long and too short for the sun to time
/// the hours: the reminders keep the reader's own times.
pub const LATITUDE_LIMIT: f64 = 60.0;

/// Why the setting is unavailable past the limit, under the greyed-out switch.
pub const UNAVAILABLE_NOTE: &str =
    "Not available this far from the equator: the summer days are too long and the winter days too short to time the hours by the sun.";

/// Compline's least distance after sunset, in minutes.
pub const COMPLINE_AFTER_SUNSET: f64 = 60.0;

/// Whether the hours can follow the sun at a latitude.
pub fn available(latitude: f64) -> bool {
    latitude.is_finite() && latitude.abs() < LATITUDE_LIMIT
}

/// One day's sun at a place, in minutes after the local midnight (fractional).
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct SunDay {
    pub sunrise: f64,
    pub noon: f64,
    pub sunset: f64,
}

/// The sun on `date` at a place (degrees, east and north positive), in a zone `utc_offset`
/// minutes east of UTC. None when the sun does not rise or set that day.
pub fn sun_day(date: Date, latitude: f64, longitude: f64, utc_offset: i32) -> Option<SunDay> {
    // Julian day at 0h UTC.
    let jd0 = 2_440_587.5 + f64::from(date.days_since(Date::new(1970, 1, 1)));
    let at = |utc_minutes: f64| Position::at(jd0 + utc_minutes / 1440.0);
    let noon_at = |p: &Position| 720.0 - 4.0 * longitude - p.equation_of_time;
    let mut noon = noon_at(&at(720.0 - 4.0 * longitude));
    noon = noon_at(&at(noon));
    // Each event at the sun's position for its own moment: twice is within seconds.
    let event = |sign: f64| -> Option<f64> {
        let mut t = noon;
        for _ in 0..2 {
            let p = at(t);
            t = noon_at(&p) + sign * 4.0 * p.hour_angle(latitude)?;
        }
        Some(t)
    };
    let offset = f64::from(utc_offset);
    Some(SunDay { sunrise: event(-1.0)? + offset, noon: noon + offset, sunset: event(1.0)? + offset })
}

/// The sun's declination and the equation of time at a Julian day.
struct Position {
    declination: f64,
    equation_of_time: f64,
}

impl Position {
    fn at(jd: f64) -> Position {
        let t = (jd - 2_451_545.0) / 36_525.0;
        let mean_long = (280.46646 + t * (36_000.769_83 + t * 0.000_303_2)).rem_euclid(360.0);
        let anomaly = 357.52911 + t * (35_999.050_29 - 0.000_153_7 * t);
        let ecc = 0.016_708_634 - t * (0.000_042_037 + 0.000_000_126_7 * t);
        let m = anomaly.to_radians();
        let centre = m.sin() * (1.914_602 - t * (0.004_817 + 0.000_014 * t))
            + (2.0 * m).sin() * (0.019_993 - 0.000_101 * t)
            + (3.0 * m).sin() * 0.000_289;
        let omega = (125.04 - 1934.136 * t).to_radians();
        let apparent = (mean_long + centre - 0.005_69 - 0.004_78 * omega.sin()).to_radians();
        let mean_obliquity = 23.0 + (26.0 + (21.448 - t * (46.815 + t * (0.000_59 - t * 0.001_813))) / 60.0) / 60.0;
        let obliquity = (mean_obliquity + 0.002_56 * omega.cos()).to_radians();
        let y = (obliquity / 2.0).tan().powi(2);
        let l = mean_long.to_radians();
        let eot = y * (2.0 * l).sin() - 2.0 * ecc * m.sin() + 4.0 * ecc * y * m.sin() * (2.0 * l).cos()
            - 0.5 * y * y * (4.0 * l).sin()
            - 1.25 * ecc * ecc * (2.0 * m).sin();
        Position { declination: (obliquity.sin() * apparent.sin()).asin(), equation_of_time: 4.0 * eot.to_degrees() }
    }

    /// The hour angle of sunrise and sunset in degrees (the sun's upper limb at the horizon,
    /// with refraction), or None in polar day or night.
    fn hour_angle(&self, latitude: f64) -> Option<f64> {
        let lat = latitude.to_radians();
        let c = 90.833_f64.to_radians().cos() / (lat.cos() * self.declination.cos()) - lat.tan() * self.declination.tan();
        (-1.0..=1.0).contains(&c).then(|| c.acos().to_degrees())
    }
}

/// An office's time on a day, in minutes after midnight, from the sun; `fixed` is the reader's
/// own time for it. A time the sun would put outside the day keeps the reader's.
pub fn office_minute(hour: &str, sun: &SunDay, fixed: u32) -> u32 {
    let unit = (sun.sunset - sun.sunrise) / 12.0;
    let at = match hour {
        "lauds" => sun.sunrise,
        "prime" => sun.sunrise + unit,
        "terce" => sun.sunrise + 3.0 * unit,
        "sext" => sun.noon,
        "none" => sun.sunrise + 9.0 * unit,
        "vespers" => sun.sunset,
        // The reader's own evening time, moved later only when it would come too soon after
        // Vespers; a time after midnight is already late enough.
        "compline" if fixed >= 12 * 60 => f64::from(fixed).max(sun.sunset + COMPLINE_AFTER_SUNSET),
        _ => f64::from(fixed),
    };
    let minute = at.round();
    if (0.0..1440.0).contains(&minute) { minute as u32 } else { fixed }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn hm(minutes: f64) -> String {
        let m = minutes.round() as i64;
        format!("{:02}:{:02}", m / 60, m % 60)
    }

    /// Within a minute of PyEphem's figures (upper limb, 34' refraction), in minutes.
    fn near(got: f64, h: f64, m: f64, s: f64) {
        let want = h * 60.0 + m + s / 60.0;
        assert!((got - want).abs() < 1.0, "{} against {:02}:{:02}:{:02}", hm(got), h, m, s);
    }

    #[test]
    fn sun_times_agree_with_an_ephemeris() {
        // Philadelphia (39.95 N, 75.17 W), at the solstices, EDT and EST.
        let june = sun_day(Date::new(2026, 6, 21), 39.9526, -75.1652, -240).unwrap();
        near(june.sunrise, 5.0, 32.0, 12.0);
        near(june.noon, 13.0, 2.0, 31.0);
        near(june.sunset, 20.0, 32.0, 49.0);
        let december = sun_day(Date::new(2026, 12, 21), 39.9526, -75.1652, -300).unwrap();
        near(december.sunrise, 7.0, 18.0, 49.0);
        near(december.noon, 11.0, 58.0, 49.0);
        near(december.sunset, 16.0, 38.0, 49.0);
        // The southern summer: Sydney (33.87 S, 151.21 E), AEDT.
        let sydney = sun_day(Date::new(2026, 12, 21), -33.8688, 151.2093, 660).unwrap();
        near(sydney.sunrise, 5.0, 40.0, 37.0);
        near(sydney.noon, 12.0, 53.0, 1.0);
        near(sydney.sunset, 20.0, 5.0, 25.0);
    }

    #[test]
    fn polar_day_and_night_have_no_sunrise() {
        assert!(sun_day(Date::new(2026, 6, 21), 71.29, -156.79, -480).is_none());
        assert!(sun_day(Date::new(2026, 12, 21), 71.29, -156.79, -540).is_none());
        assert!(sun_day(Date::new(2026, 3, 20), 71.29, -156.79, -480).is_some());
    }

    #[test]
    fn the_setting_stops_short_of_the_long_days() {
        // Juneau keeps it; Anchorage and Fairbanks keep their own times; south as north.
        assert!(available(58.30));
        assert!(!available(61.22));
        assert!(!available(64.84));
        assert!(!available(-60.0));
        assert!(available(-33.87));
        assert!(!available(f64::NAN));
    }

    #[test]
    fn the_offices_divide_the_day_into_twelve_hours() {
        let sun = SunDay { sunrise: 360.0, noon: 735.0, sunset: 1080.0 };
        let at = |hour: &str, fixed: u32| office_minute(hour, &sun, fixed);
        // An hour of an hour: 6:00 to 18:00.
        assert_eq!(
            [at("lauds", 0), at("prime", 0), at("terce", 0), at("sext", 0), at("none", 0), at("vespers", 0)],
            [360, 420, 540, 735, 900, 1080]
        );
        // Compline keeps 9 PM, but follows a late sunset by an hour.
        assert_eq!(at("compline", 21 * 60), 21 * 60);
        let summer = SunDay { sunrise: 300.0, noon: 780.0, sunset: 1290.0 };
        assert_eq!(office_minute("compline", &summer, 21 * 60), 1350);
        // A Compline after midnight stays where the reader set it.
        assert_eq!(office_minute("compline", &summer, 30), 30);
        // A time the sun would push past midnight keeps the reader's.
        let late = SunDay { sunrise: 300.0, noon: 900.0, sunset: 1420.0 };
        assert_eq!(office_minute("compline", &late, 21 * 60), 21 * 60);
    }
}
