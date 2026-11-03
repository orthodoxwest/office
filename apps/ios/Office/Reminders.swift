import BackgroundTasks
import Combine
import CoreLocation
import Foundation
import UserNotifications

/// One hour on the reminder page: whether it is chosen, and its time.
struct HourReminder: Equatable {
    let hour: String
    let name: String
    var chosen: Bool
    var hourOfDay: Int
    var minute: Int
}

/// Minutes before the hour, as the web's "Remind me" offers them (without its alert-less option).
let reminderLeads: [(minutes: Int, label: String)] = [
    (0, "At the hour"), (5, "5 minutes before"), (10, "10 minutes before"), (15, "15 minutes before"), (30, "30 minutes before"),
]

/// Monday first, as the web's Days row; 1 is Sunday, as the calendar counts.
let reminderWeek = [2, 3, 4, 5, 6, 7, 1]

/// Where the phone last was, read while the app was open, and when: the sun's times are this place's.
struct StoredPlace: Equatable {
    var latitude: Double
    var longitude: Double
    var readAt: Date

    /// Whether the sun can time the hours here; past the limit the setting is greyed out.
    var sunAvailable: Bool { sunReaches(latitude) }
}

/// The core's latitude limit (`sunAvailable(latitude:)`), under a name the property above does not shadow.
private func sunReaches(_ latitude: Double) -> Bool { sunAvailable(latitude: latitude) }

/// The reader's reminders. Nothing is scheduled until they are turned on.
struct ReminderSettings: Equatable {
    var on: Bool
    var hours: [HourReminder]
    /// The weekdays chosen, 1 (Sunday) to 7.
    var days: Set<Int>
    var lead: Int
    /// Whether the hours follow the sun at `place` rather than the times chosen (all but Compline's).
    var followSun = false
    var place: StoredPlace?

    /// Sunday first, as the engine counts the week.
    var weekdays: [Bool] { (1...7).map { days.contains($0) } }

    /// The sun's times are in use: the setting is on, a place is known, and it is short of the limit.
    var sunActive: Bool { followSun && place?.sunAvailable == true }

    /// The place for the engine over `days` days from `from`'s day, each day with the zone's
    /// offset at its local noon; nil while the hours keep the chosen times.
    func sunPlace(from: Date, days: Int, calendar: Calendar = .current) -> SunPlace? {
        guard followSun, let place else { return nil }
        let start = calendar.startOfDay(for: from)
        let offsets = (0..<days).map { i -> Int32 in
            let day = calendar.date(byAdding: .day, value: i, to: start) ?? start
            let noon = calendar.date(bySettingHour: 12, minute: 0, second: 0, of: day) ?? day
            return Int32(calendar.timeZone.secondsFromGMT(for: noon) / 60)
        }
        return SunPlace(latitude: place.latitude, longitude: place.longitude, utcOffsets: offsets)
    }

    /// Today's time of every hour, chosen or not, as the sun sets them here (Compline's the
    /// reader's own, kept an hour after sunset); nil unless the sun's times are in use.
    func sunTimes(on day: Date, core: OfficeCore, calendar: Calendar = .current) -> [String: (hour: Int, minute: Int)]? {
        guard sunActive else { return nil }
        let all = hours.map { ReminderChoice(hour: $0.hour, hourOfDay: Int32($0.hourOfDay), minute: Int32($0.minute)) }
        let parts = calendar.dateComponents([.year, .month, .day], from: day)
        let from = CivilDate(year: Int32(parts.year ?? 2026), month: Int32(parts.month ?? 1), day: Int32(parts.day ?? 1))
        guard let times = try? core.reminders(from: from, days: 1, choices: all, weekdays: Array(repeating: true, count: 7), sun: sunPlace(from: day, days: 1, calendar: calendar)) else { return nil }
        return Dictionary(times.map { ($0.hour, (hour: Int($0.hourOfDay), minute: Int($0.minute))) }, uniquingKeysWith: { a, _ in a })
    }

    func with(_ h: HourReminder) -> ReminderSettings {
        var s = self
        s.hours = hours.map { $0.hour == h.hour ? h : $0 }
        return s
    }
}

/// The settings in the app's defaults, starting from the web's.
enum ReminderStore {
    private static let defaults = UserDefaults.standard

    static func load() -> ReminderSettings {
        let hours = reminderDefaults().map { d -> HourReminder in
            let saved = defaults.string(forKey: "reminders.hour.\(d.hour)")?.split(separator: ",").compactMap { Int($0) }
            if let saved, saved.count == 3 {
                return HourReminder(hour: d.hour, name: d.name, chosen: saved[0] == 1, hourOfDay: saved[1], minute: saved[2])
            }
            return HourReminder(hour: d.hour, name: d.name, chosen: d.chosen, hourOfDay: Int(d.hourOfDay), minute: Int(d.minute))
        }
        let days = defaults.string(forKey: "reminders.days").map { Set($0.split(separator: ",").compactMap { Int($0) }) }
        return ReminderSettings(
            on: defaults.bool(forKey: "reminders.on"),
            hours: hours,
            days: days ?? Set(1...7),
            lead: defaults.object(forKey: "reminders.lead") as? Int ?? 10,
            followSun: defaults.bool(forKey: "reminders.followSun"),
            place: place()
        )
    }

    private static func place() -> StoredPlace? {
        guard let latitude = defaults.object(forKey: "reminders.place.latitude") as? Double,
              let longitude = defaults.object(forKey: "reminders.place.longitude") as? Double else { return nil }
        let read = defaults.object(forKey: "reminders.place.read") as? Double ?? 0
        return StoredPlace(latitude: latitude, longitude: longitude, readAt: Date(timeIntervalSince1970: read))
    }

    static func save(_ s: ReminderSettings) {
        defaults.set(s.on, forKey: "reminders.on")
        defaults.set(s.lead, forKey: "reminders.lead")
        defaults.set(s.days.sorted().map(String.init).joined(separator: ","), forKey: "reminders.days")
        for h in s.hours {
            defaults.set("\(h.chosen ? 1 : 0),\(h.hourOfDay),\(h.minute)", forKey: "reminders.hour.\(h.hour)")
        }
        defaults.set(s.followSun, forKey: "reminders.followSun")
        if let place = s.place {
            defaults.set(place.latitude, forKey: "reminders.place.latitude")
            defaults.set(place.longitude, forKey: "reminders.place.longitude")
            defaults.set(place.readAt.timeIntervalSince1970, forKey: "reminders.place.read")
        } else {
            for key in ["reminders.place.latitude", "reminders.place.longitude", "reminders.place.read"] { defaults.removeObject(forKey: key) }
        }
    }
}

/// A reminder to be scheduled: its identity, when it rings, and its words.
struct PlannedReminder: Equatable {
    let id: String
    let hour: String
    let date: CivilDate
    let at: DateComponents
    let fireDate: Date
    /// When the office begins: the title's time, and an hour after it the reminder clears.
    let office: Date
    let title: String
    let feast: String
}

/**
 * Keeps the coming reminders scheduled with iOS. Each carries its notification's words, the
 * office and the day's feast, so it rings with no engine to load. iOS holds 64 pending
 * notifications for an app, so the schedule reaches as many days ahead as fit, up to four weeks;
 * every visit to the app, and a background refresh, move it on.
 */
enum ReminderScheduler {
    static let category = "hour"
    static let snoozeAction = "snooze"
    static let refreshTask = "org.orthodoxwest.office.reminders"
    static let snooze: TimeInterval = 10 * 60
    private static let reach = 28
    /// iOS's pending limit, less room for a snoozed reminder or two.
    static let capacity = 60

    /// The reminders due from `now`, earliest first, no more than fit.
    static func plan(_ settings: ReminderSettings, now: Date, core: OfficeCore, calendar: Calendar = .current) throws -> [PlannedReminder] {
        guard settings.on else { return [] }
        let chosen = settings.hours.filter(\.chosen).map { ReminderChoice(hour: $0.hour, hourOfDay: Int32($0.hourOfDay), minute: Int32($0.minute)) }
        if chosen.isEmpty || settings.days.isEmpty { return [] }
        let parts = calendar.dateComponents([.year, .month, .day], from: now)
        let from = CivilDate(year: Int32(parts.year ?? 2026), month: Int32(parts.month ?? 1), day: Int32(parts.day ?? 1))
        var out: [PlannedReminder] = []
        let sun = settings.sunPlace(from: now, days: reach, calendar: calendar)
        for r in try core.reminders(from: from, days: Int32(reach), choices: chosen, weekdays: settings.weekdays, sun: sun) {
            let office = DateComponents(year: Int(r.date.year), month: Int(r.date.month), day: Int(r.date.day), hour: Int(r.hourOfDay), minute: Int(r.minute))
            guard let officeDate = calendar.date(from: office),
                  let fire = calendar.date(byAdding: .minute, value: -settings.lead, to: officeDate),
                  fire > now else { continue }
            let at = calendar.dateComponents([.year, .month, .day, .hour, .minute], from: fire)
            out.append(PlannedReminder(id: "reminder.\(r.date.iso).\(r.hour)", hour: r.hour, date: r.date, at: at, fireDate: fire, office: officeDate, title: r.title, feast: r.feast))
            if out.count == capacity { break }
        }
        return out
    }

    /// How long after its office begins a reminder stays in Notification Center before a sync clears it (Android's `LINGER`).
    static let linger: TimeInterval = 60 * 60

    /// "Vespers · 6:00 PM": the clock time in the phone's 12- or 24-hour style, not an age that drifts and reads as past.
    static func title(_ name: String, office: Date) -> String {
        "\(name) · \(office.formatted(date: .omitted, time: .shortened))"
    }

    /// The notification a reminder posts: "Vespers · 6:00 PM", then the day it keeps, with the bell.
    static func content(_ r: PlannedReminder) -> UNMutableNotificationContent {
        let c = UNMutableNotificationContent()
        c.title = title(r.title, office: r.office)
        c.body = r.feast
        c.sound = UNNotificationSound(named: UNNotificationSoundName("bell.caf"))
        c.categoryIdentifier = category
        c.threadIdentifier = "hours"
        c.userInfo = ["hour": r.hour, "date": r.date.iso, "office": r.office.timeIntervalSince1970]
        return c
    }

    /// The reminder's category: its "Remind me in 10 min" action, handled without opening the app.
    static func register(_ center: UNUserNotificationCenter = .current()) {
        let later = UNNotificationAction(identifier: snoozeAction, title: "Remind me in 10 min", options: [])
        center.setNotificationCategories([UNNotificationCategory(identifier: category, actions: [later], intentIdentifiers: [], options: [])])
    }

    /// Replaces the scheduled reminders with the plan; a snoozed one is left to ring.
    static func sync(now: Date = Date(), done: (() -> Void)? = nil) {
        let center = UNUserNotificationCenter.current()
        DispatchQueue.global(qos: .utility).async {
            let planned = (try? Office.core.get()).flatMap { try? plan(ReminderStore.load(), now: now, core: $0) } ?? []
            // Reminders whose office began over an hour ago are cleared, as Android's time out.
            center.getDeliveredNotifications { delivered in
                let past = delivered.filter { n in
                    guard let office = n.request.content.userInfo["office"] as? Double else { return false }
                    return now.timeIntervalSince1970 > office + linger
                }
                center.removeDeliveredNotifications(withIdentifiers: past.map(\.request.identifier))
            }
            center.getPendingNotificationRequests { pending in
                let wanted = Set(planned.map(\.id))
                let stale = pending.map(\.identifier).filter { $0.hasPrefix("reminder.") && !wanted.contains($0) }
                center.removePendingNotificationRequests(withIdentifiers: stale)
                let group = DispatchGroup()
                for r in planned {
                    group.enter()
                    let trigger = UNCalendarNotificationTrigger(dateMatching: r.at, repeats: false)
                    // The same identity replaces a reminder already scheduled.
                    center.add(UNNotificationRequest(identifier: r.id, content: content(r), trigger: trigger)) { _ in group.leave() }
                }
                group.notify(queue: .main) { done?() }
            }
        }
    }

    /// "Remind me in 10 min": the reminder is put away and rings again, with the same words, after the wait.
    static func snooze(_ notification: UNNotification, done: @escaping () -> Void = {}) {
        let center = UNUserNotificationCenter.current()
        let request = notification.request
        center.removeDeliveredNotifications(withIdentifiers: [request.identifier])
        guard let again = request.content.mutableCopy() as? UNMutableNotificationContent else { return done() }
        // Kept a while once it rings again, as Android's lingers at least a snooze's length.
        again.userInfo["office"] = Date(timeIntervalSinceNow: snooze).timeIntervalSince1970
        let trigger = UNTimeIntervalNotificationTrigger(timeInterval: snooze, repeats: false)
        center.add(UNNotificationRequest(identifier: "snooze.\(request.identifier)", content: again, trigger: trigger)) { _ in done() }
    }

    /// Asks iOS to wake the app about twice a day to move the schedule on.
    static func scheduleRefresh() {
        let request = BGAppRefreshTaskRequest(identifier: refreshTask)
        request.earliestBeginDate = Date(timeIntervalSinceNow: 12 * 60 * 60)
        try? BGTaskScheduler.shared.submit(request)
    }

    static func registerRefresh() {
        _ = BGTaskScheduler.shared.register(forTaskWithIdentifier: refreshTask, using: nil) { task in
            scheduleRefresh()
            sync { task.setTaskCompleted(success: true) }
            task.expirationHandler = { task.setTaskCompleted(success: false) }
        }
    }
}

/// Whether iOS lets reminders through.
enum ReminderPermission: Equatable {
    case unknown, allowed, denied
}

/**
 * Where the phone is, for reminders that follow the sun: read to the kilometre, only while the
 * app is in front (when-in-use permission, never in the background), and kept on the phone. Each
 * place read is saved and the reminders rescheduled from it; `revision` tells the page.
 */
final class PhoneLocation: NSObject, ObservableObject, CLLocationManagerDelegate {
    static let shared = PhoneLocation()

    private let manager = CLLocationManager()
    @Published private(set) var authorization: CLAuthorizationStatus
    /// Moves on with every place saved, so the page reads its settings again.
    @Published private(set) var revision = 0

    override private init() {
        authorization = manager.authorizationStatus
        super.init()
        manager.delegate = self
        manager.desiredAccuracy = kCLLocationAccuracyKilometer
    }

    var denied: Bool { authorization == .denied || authorization == .restricted }
    private var allowed: Bool { authorization == .authorizedWhenInUse || authorization == .authorizedAlways }

    /// Reads the place again when the hours follow the sun, asking first if the reader has not been asked.
    func refresh() {
        guard ReminderStore.load().followSun else { return }
        switch authorization {
        case .notDetermined: manager.requestWhenInUseAuthorization()
        case .authorizedWhenInUse, .authorizedAlways: manager.requestLocation()
        default: break
        }
    }

    func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        authorization = manager.authorizationStatus
        if allowed && ReminderStore.load().followSun { manager.requestLocation() }
    }

    func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let l = locations.last else { return }
        var s = ReminderStore.load()
        guard s.followSun else { return }
        s.place = StoredPlace(latitude: l.coordinate.latitude, longitude: l.coordinate.longitude, readAt: Date())
        ReminderStore.save(s)
        ReminderScheduler.sync()
        revision += 1
    }

    func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        // No place this time: the one kept, or the chosen times, stand until the next visit.
    }
}
