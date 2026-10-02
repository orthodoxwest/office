import SwiftUI
import UIKit

/// The engine, loaded once per process: the app, its reminders and their background refresh share it.
enum Office {
    static let core: Result<OfficeCore, Error> = Result { try OfficeCore() }
}

/// The prayer forms: value, the control's label, and the chooser's phrase (hour.html).
let prayerForms: [(value: String, label: String, phrase: String)] = [
    ("private", "Private", "Praying privately"),
    ("deacon", "Deacon", "With others, led by a deacon"),
    ("priest", "Priest", "With others, led by a priest"),
]

/// Capitalizes an hour's name: "lauds" → "Lauds".
func hourLabel(_ hour: String) -> String { hour.prefix(1).uppercased() + hour.dropFirst() }

/// The civil calendar the Office counts in: Gregorian, in the phone's time zone, named in English.
let civil: Calendar = {
    var c = Calendar(identifier: .gregorian)
    c.locale = Locale(identifier: "en_US_POSIX")
    return c
}()

extension CivilDate {
    static func of(_ date: Date) -> CivilDate {
        let parts = civil.dateComponents([.year, .month, .day], from: date)
        return CivilDate(year: Int32(parts.year ?? 2026), month: Int32(parts.month ?? 1), day: Int32(parts.day ?? 1))
    }

    /// "2026-03-15".
    static func parse(_ s: String) -> CivilDate? {
        let f = s.split(separator: "-").compactMap { Int32($0) }
        guard f.count == 3, (1...12).contains(f[1]), (1...31).contains(f[2]) else { return nil }
        return CivilDate(year: f[0], month: f[1], day: f[2])
    }

    /// Noon on this day, clear of any time-zone edge.
    var date: Date {
        civil.date(from: DateComponents(year: Int(year), month: Int(month), day: Int(day), hour: 12)) ?? Date()
    }

    func adding(days: Int) -> CivilDate { .of(civil.date(byAdding: .day, value: days, to: date) ?? date) }

    /// 1 for Sunday through 7 for Saturday.
    var weekday: Int { civil.component(.weekday, from: date) }

    var iso: String { String(format: "%04d-%02d-%02d", year, month, day) }
}

/// The month's name: 3 → "March".
func monthName(_ month: Int) -> String { DateFormatter.english.monthSymbols[(month - 1 + 12) % 12] }
func monthShort(_ month: Int) -> String { DateFormatter.english.shortMonthSymbols[(month - 1 + 12) % 12] }

extension DateFormatter {
    static let english: DateFormatter = {
        let f = DateFormatter()
        f.calendar = civil
        f.locale = Locale(identifier: "en_US")
        return f
    }()
}

/// A day as VoiceOver says it in the picker and the ordo: "Sunday, March 15, today".
func spokenDay(_ day: CivilDate, today: CivilDate) -> String {
    let f = DateFormatter()
    f.calendar = civil
    f.locale = Locale(identifier: "en_US")
    f.dateFormat = "EEEE, MMMM d"
    return f.string(from: day.date) + (day == today ? ", today" : "")
}

/**
 * The app's pages, as the web's routes: home for a day, an hour of a day, a month of the ordo
 * (brought to `day` when one is asked for, as the web's #d-date), a year's frontispiece, and
 * the reminders.
 */
enum Page: Hashable {
    case home(CivilDate)
    case hour(CivilDate, String)
    case ordo(year: Int, month: Int, day: Int)
    case year(Int)
    case reminders

    /// Pages of one kind replace each other, as following a link does; another kind goes on top.
    var kind: Int {
        switch self {
        case .home: return 0
        case .hour: return 1
        case .ordo: return 2
        case .year: return 3
        case .reminders: return 4
        }
    }

    /// Where `next`, of the same kind, stands after this page in time: later, earlier, or neither.
    func step(to next: Page) -> Motion {
        let way: Int
        switch (self, next) {
        case let (.hour(a, h), .hour(b, k)):
            let hours = hourNames()
            way = a == b ? order(hours.firstIndex(of: h) ?? 0, hours.firstIndex(of: k) ?? 0) : a.compare(b)
        case let (.home(a), .home(b)): way = a.compare(b)
        case let (.ordo(y, m, _), .ordo(z, n, _)): way = order(y * 12 + m, z * 12 + n)
        case let (.year(a), .year(b)): way = order(a, b)
        default: way = 0
        }
        return way < 0 ? .next : way > 0 ? .previous : .fade
    }

    /// A page as saved state writes it: "hour 2026-03-15 vespers".
    var encoded: String {
        switch self {
        case let .home(d): return "home \(d.iso)"
        case let .hour(d, h): return "hour \(d.iso) \(h)"
        case let .ordo(y, m, d): return "ordo \(y) \(m) \(d)"
        case let .year(y): return "year \(y)"
        case .reminders: return "reminders"
        }
    }

    static func decode(_ s: String) -> Page? {
        let f = s.split(separator: " ").map(String.init)
        switch f.first {
        case "home": return f.count > 1 ? CivilDate.parse(f[1]).map(Page.home) : nil
        case "hour":
            guard f.count > 2, let d = CivilDate.parse(f[1]), hourNames().contains(f[2]) else { return nil }
            return .hour(d, f[2])
        case "ordo":
            guard f.count > 2, let y = Int(f[1]), let m = Int(f[2]), (1...12).contains(m) else { return nil }
            return .ordo(year: y, month: m, day: f.count > 3 ? Int(f[3]) ?? 0 : 0)
        case "year": return f.count > 1 ? Int(f[1]).map(Page.year) : nil
        case "reminders": return .reminders
        default: return nil
        }
    }
}

/// -1, 0 or 1 as `a` comes before, with or after `b`.
private func order<T: Comparable>(_ a: T, _ b: T) -> Int { a < b ? -1 : a > b ? 1 : 0 }

extension CivilDate {
    /// -1, 0 or 1 as this day falls before, on or after `other`.
    func compare(_ other: CivilDate) -> Int {
        self == other ? 0 : (year, month, day) < (other.year, other.month, other.day) ? -1 : 1
    }
}

/**
 * How a page replaced in place moves: to the next or previous of its kind (an hour, a day, a
 * month), or with no order between them, a fade. Going deeper and back are the navigation
 * stack's own slide.
 */
enum Motion { case next, previous, fade }

/**
 * One place on the way back. While a page of the same kind replaces its page it stays the same
 * visit to the navigation stack, so the screen changes in place rather than pushing.
 */
struct Entry: Hashable {
    let id: Int
    var page: Page

    static func == (a: Entry, b: Entry) -> Bool { a.id == b.id }
    func hash(into h: inout Hasher) { h.combine(id) }
}

/**
 * What is shown and the reader's remembered choices. The way back is a navigation stack over
 * home, so the edge swipe and the pages' own links both walk it.
 */
final class AppModel: ObservableObject {
    static let shared = AppModel()

    @Published var root: CivilDate
    @Published var path: [Entry] = []
    /// How the page replaced in place last moves, and the page it moved to.
    @Published private(set) var step: (to: Page, motion: Motion)?
    @Published var menuOpen = false
    /// The wide header's Settings panel.
    @Published var settingsOpen = false
    @Published private(set) var form: String
    @Published private(set) var theme: ThemeChoice
    @Published private(set) var textSize: TextSize
    /// Today, read again whenever the app comes forward; a screenshot run can fix it.
    @Published private(set) var today: CivilDate
    /// The hour of the clock, which chooses home's invitation.
    @Published private(set) var clockHour = civil.component(.hour, from: Date())

    private let defaults = UserDefaults.standard
    private let pinnedToday: CivilDate?
    private var nextEntry = 1

    let hours: [String] = hourNames()

    init() {
        pinnedToday = UserDefaults.standard.string(forKey: "today").flatMap(CivilDate.parse)
        let now = pinnedToday ?? .of(Date())
        today = now
        root = now
        form = UserDefaults.standard.string(forKey: "form").flatMap { f in prayerForms.contains { $0.value == f } ? f : nil } ?? "private"
        theme = UserDefaults.standard.string(forKey: "theme").flatMap(ThemeChoice.init(rawValue:)) ?? .system
        textSize = UserDefaults.standard.string(forKey: "text-size").flatMap(TextSize.init(rawValue:)) ?? .standard
    }

    /// The page shown.
    var page: Page { path.last?.page ?? .home(root) }

    /// The pages over home, on the way back.
    var pages: [Page] { path.map(\.page) }

    /// The page a visit on the way back shows now.
    func page(of entry: Int) -> Page? { path.first { $0.id == entry }?.page }

    /// How a page coming in place moves: as it was opened, or after any other change, a fade.
    func motion(to page: Page) -> Motion { step.flatMap { $0.to == page ? $0.motion : nil } ?? .fade }

    private func visits(_ pages: [Page]) -> [Entry] {
        pages.map { page in
            defer { nextEntry += 1 }
            return Entry(id: nextEntry, page: page)
        }
    }

    /// Opens `next` over the current page; a page of the same kind replaces it, as a link would.
    func open(_ next: Page) {
        menuOpen = false
        settingsOpen = false
        if next == page { return }
        if next.kind == page.kind {
            // Replaced in place, as a link is followed: the same visit, so no push; the page
            // itself moves to the next or previous of its kind once composed.
            step = (next, page.step(to: next))
            if path.isEmpty, case let .home(d) = next { root = d } else { path[path.count - 1].page = next }
        } else {
            path += visits([next])
        }
    }

    /// Home for today, clearing the way back, as the brand link does.
    func goHome() {
        step = nil
        menuOpen = false
        settingsOpen = false
        root = today
        path = []
    }

    func chooseForm(_ value: String) {
        step = nil
        form = value
        defaults.set(value, forKey: "form")
    }

    func chooseTheme(_ value: ThemeChoice) {
        theme = value
        defaults.set(value.rawValue, forKey: "theme")
    }

    func chooseTextSize(_ value: TextSize) {
        textSize = value
        defaults.set(value.rawValue, forKey: "text-size")
    }

    /// Called when the app comes forward, and every minute while it is in front: the hour or the
    /// date may have turned since home was composed.
    func refreshToday() {
        let hour = civil.component(.hour, from: Date())
        if hour != clockHour {
            step = nil
            clockHour = hour
        }
        let now = pinnedToday ?? .of(Date())
        guard now != today else { return }
        step = nil
        // Home that was showing today moves on with it.
        if root == today && path.isEmpty { root = now }
        today = now
    }

    /// The way back as saved state writes it, one page a line, for a return after iOS has closed the app.
    var saved: String {
        ([Page.home(root)] + pages).map(\.encoded).joined(separator: "\n")
    }

    func restore(_ saved: String) {
        let pages = saved.split(separator: "\n").compactMap { Page.decode(String($0)) }
        guard case let .home(d)? = pages.first else { return }
        root = d
        path = visits(Array(pages.dropFirst()))
    }

    /**
     * What the launch asked for, for the simulator screenshots: `-page hour -hour lauds
     * -date 2026-03-15` (or home, ordo, year, reminders), with `-today` fixing today.
     */
    func openFromLaunch() -> Bool {
        let d = UserDefaults.standard
        guard let name = d.string(forKey: "page") else { return false }
        let date = d.string(forKey: "date").flatMap(CivilDate.parse) ?? today
        root = date
        switch name {
        case "hour": path = visits([.hour(date, d.string(forKey: "hour") ?? currentOffice(clockHour: 9).hour)])
        case "ordo": path = visits([.ordo(year: Int(date.year), month: Int(date.month), day: 0)])
        case "year": path = visits([.year(Int(date.year))])
        case "reminders": path = visits([.reminders])
        default: path = []
        }
        // `-settings YES` opens the wide header's Settings, for its screenshot.
        settingsOpen = d.bool(forKey: "settings")
        return true
    }
}
