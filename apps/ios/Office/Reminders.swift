import BackgroundTasks
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

/// The reader's reminders. Nothing is scheduled until they are turned on.
struct ReminderSettings: Equatable {
    var on: Bool
    var hours: [HourReminder]
    /// The weekdays chosen, 1 (Sunday) to 7.
    var days: Set<Int>
    var lead: Int

    /// Sunday first, as the engine counts the week.
    var weekdays: [Bool] { (1...7).map { days.contains($0) } }

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
            lead: defaults.object(forKey: "reminders.lead") as? Int ?? 10
        )
    }

    static func save(_ s: ReminderSettings) {
        defaults.set(s.on, forKey: "reminders.on")
        defaults.set(s.lead, forKey: "reminders.lead")
        defaults.set(s.days.sorted().map(String.init).joined(separator: ","), forKey: "reminders.days")
        for h in s.hours {
            defaults.set("\(h.chosen ? 1 : 0),\(h.hourOfDay),\(h.minute)", forKey: "reminders.hour.\(h.hour)")
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
        for r in try core.reminders(from: from, days: Int32(reach), choices: chosen, weekdays: settings.weekdays) {
            let office = DateComponents(year: Int(r.date.year), month: Int(r.date.month), day: Int(r.date.day), hour: Int(r.hourOfDay), minute: Int(r.minute))
            guard let officeDate = calendar.date(from: office),
                  let fire = calendar.date(byAdding: .minute, value: -settings.lead, to: officeDate),
                  fire > now else { continue }
            let at = calendar.dateComponents([.year, .month, .day, .hour, .minute], from: fire)
            out.append(PlannedReminder(id: "reminder.\(r.date.iso).\(r.hour)", hour: r.hour, date: r.date, at: at, fireDate: fire, title: r.title, feast: r.feast))
            if out.count == capacity { break }
        }
        return out
    }

    /// The notification a reminder posts: "Vespers", then the day it keeps, with the bell.
    static func content(_ r: PlannedReminder) -> UNMutableNotificationContent {
        let c = UNMutableNotificationContent()
        c.title = r.title
        c.body = r.feast
        c.sound = UNNotificationSound(named: UNNotificationSoundName("bell.caf"))
        c.categoryIdentifier = category
        c.threadIdentifier = "hours"
        c.userInfo = ["hour": r.hour, "date": r.date.iso]
        return c
    }

    /// The reminder's category: its "In 10 minutes" action, handled without opening the app.
    static func register(_ center: UNUserNotificationCenter = .current()) {
        let later = UNNotificationAction(identifier: snoozeAction, title: "In 10 minutes", options: [])
        center.setNotificationCategories([UNNotificationCategory(identifier: category, actions: [later], intentIdentifiers: [], options: [])])
    }

    /// Replaces the scheduled reminders with the plan; a snoozed one is left to ring.
    static func sync(now: Date = Date(), done: (() -> Void)? = nil) {
        let center = UNUserNotificationCenter.current()
        DispatchQueue.global(qos: .utility).async {
            let planned = (try? Office.core.get()).flatMap { try? plan(ReminderStore.load(), now: now, core: $0) } ?? []
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

    /// "In 10 minutes": the reminder is put away and rings again, with the same words, after the wait.
    static func snooze(_ notification: UNNotification) {
        let center = UNUserNotificationCenter.current()
        let request = notification.request
        center.removeDeliveredNotifications(withIdentifiers: [request.identifier])
        guard let again = request.content.mutableCopy() as? UNMutableNotificationContent else { return }
        let trigger = UNTimeIntervalNotificationTrigger(timeInterval: snooze, repeats: false)
        center.add(UNNotificationRequest(identifier: "snooze.\(request.identifier)", content: again, trigger: trigger))
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
