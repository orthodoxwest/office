import SwiftUI
import UIKit

/// The engine, loaded once per process: the app, its reminders and their background refresh share it.
enum Office {
    static let core: Result<OfficeCore, Error> = Result { try OfficeCore() }
}

/// The prayer forms: value, the control's label, and the chooser's phrase, as the web's.
let prayerForms: [(value: String, label: String, phrase: String)] =
    prayerFormChoices().map { (value: $0.value, label: $0.label, phrase: $0.phrase) }

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
 * (brought to `day` when one is asked for, as the web's #d-date), a year's frontispiece, the
 * reminders, the search for a feast, and a feast's page.
 */
enum Page: Hashable {
    case home(CivilDate)
    case hour(CivilDate, String)
    case ordo(year: Int, month: Int, day: Int)
    case year(Int)
    case reminders
    case find(String)
    case feast(String)

    /// Pages of one kind replace each other, as following a link does; another kind goes on top.
    var kind: Int {
        switch self {
        case .home: return 0
        case .hour: return 1
        case .ordo: return 2
        case .year: return 3
        case .reminders: return 4
        case .find: return 5
        case .feast: return 6
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
        case let .find(q): return "find \(q)"
        case let .feast(id): return "feast \(id)"
        }
    }

    static func decode(_ s: String) -> Page? {
        // A query keeps its own spaces.
        if s.hasPrefix("find ") { return .find(String(s.dropFirst(5))) }
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
        case "feast": return f.count > 1 ? .feast(f[1]) : nil
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
    /// Whether Prime reads the next day's Martyrology; off by default.
    @Published private(set) var martyrology: Bool
    /// Whether a scored hymn or Marian antiphon shows its score in place of its words; off by default.
    @Published private(set) var chant: Bool
    /// Whether any text has a score, so the menu offers the Chant setting (as the web's `chant` global).
    let offersChant: Bool = (try? Office.core.get().offersChant()) ?? false
    /// What each hour composed on the way back shows, for the usage beacon: whether its Martyrology
    /// was shown and whether its scores were (nil: it had none to show).
    @Published private(set) var hourShows: [Page: HourShows] = [:]
    /// The ornament season of each home or hour composed on the way back, which retints the
    /// gilding of the chrome over it (the menu's painted line), as the page's own.
    @Published private(set) var seasons: [Page: String] = [:]
    /// Today, read again whenever the app comes forward; a screenshot run can fix it.
    @Published private(set) var today: CivilDate
    /// The hour of the clock, which chooses home's invitation.
    @Published private(set) var clockHour = civil.component(.hour, from: Date())
    /// The hours prayed, for home's marks, as the web's `office-prayed`: "2026-10-07/lauds", kept
    /// on this device a week.
    @Published private(set) var prayed: Set<String>

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
        martyrology = UserDefaults.standard.bool(forKey: "martyrology")
        chant = UserDefaults.standard.bool(forKey: "chant")
        prayed = Set(UserDefaults.standard.stringArray(forKey: "prayed") ?? [])
    }

    /// The hours prayed on `date`.
    func hoursPrayed(on date: CivilDate) -> Set<String> {
        Set(prayed.filter { $0.hasPrefix(date.iso + "/") }.map { String($0.dropFirst(date.iso.count + 1)) })
    }

    /// Marks `hour` of `date` prayed, letting marks older than a week go.
    func markPrayed(_ date: CivilDate, _ hour: String) {
        let oldest = CivilDate.of(Date()).adding(days: -7)
        var kept = prayed.filter { CivilDate.parse(String($0.prefix(10))).map { $0.compare(oldest) >= 0 } ?? false }
        kept.insert("\(date.iso)/\(hour)")
        prayed = kept
        defaults.set(Array(kept).sorted(), forKey: "prayed")
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

    /// The search's words changed: the search page takes them in place, with no move between pages.
    func search(_ query: String) {
        guard case let .find(q)? = path.last?.page, q != query else { return }
        step = nil
        path[path.count - 1].page = .find(query)
    }

    /**
     * The way from a feast page to the search: back to the search the reader came from, with its
     * words and results, or else a new one.
     */
    func findAgain() {
        if path.count > 1, case .find = path[path.count - 2].page {
            menuOpen = false
            settingsOpen = false
            path.removeLast()
        } else {
            open(.find(""))
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

    func chooseMartyrology(_ value: Bool) {
        step = nil
        martyrology = value
        defaults.set(value, forKey: "martyrology")
    }

    func chooseChant(_ value: Bool) {
        step = nil
        chant = value
        defaults.set(value, forKey: "chant")
    }

    /// Notes what an hour page shows, once composed: at Prime, whether its Martyrology was shown,
    /// and whether its scores were.
    func composed(_ view: HourView, page: Page) {
        let kept = Set(pages)
        var now = hourShows.filter { kept.contains($0.key) }
        now[page] = HourShows(martyrology: view.martyrology, chant: view.chant)
        if now != hourShows { hourShows = now }
    }

    /// The ornament season of the page shown: its own once composed, else none.
    var season: String { seasons[page] ?? "" }

    /// Notes a composed page's ornament season, letting go of pages no longer on the way back.
    func composed(season: String, page: Page) {
        let kept = Set([Page.home(root)] + pages)
        var now = seasons.filter { kept.contains($0.key) }
        now[page] = season
        if now != seasons { seasons = now }
    }

    /// Whether `page` showed its Martyrology: nil unless it is a Prime composed, with a reading.
    func martyrologyShown(on page: Page) -> Bool? { hourShows[page]?.martyrology }

    /// Whether `page` showed its scores: nil unless it is an hour composed, with a score.
    func chantShown(on page: Page) -> Bool? { hourShows[page]?.chant }

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
     * -date 2026-03-15` (or home, ordo, year, reminders; find with `-query`, feast with
     * `-feast`), with `-today` fixing today.
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
        case "find": path = visits([.find(d.string(forKey: "query") ?? "")])
        case "feast": path = visits([.feast(d.string(forKey: "feast") ?? "st-benedict")])
        default: path = []
        }
        // `-settings YES` opens the wide header's Settings, for its screenshot.
        settingsOpen = d.bool(forKey: "settings")
        return true
    }
}

/// What a composed hour shows that the usage beacon reports (nil: nothing of the kind to show).
struct HourShows: Equatable {
    let martyrology: Bool?
    let chant: Bool?
}
