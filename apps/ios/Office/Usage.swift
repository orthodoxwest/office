import Foundation

/**
 * The daily usage counts, reported as the web reports them (README, "Usage metrics"): the page
 * shown, the appearance and prayer form it is read in, and at Prime whether the Martyrology was
 * shown, once a day. The beacon's words come from
 * the Rust core, so the server reads the app's as it reads the web's, and only today's pages
 * (a day either side) count.
 *
 * The server knows a reader only by an identifier it hashes with the day. The app makes a fresh
 * random one each reporting day (America/New_York, as the server reckons days), so nothing it
 * sends ties one day to the next. The app also keeps the day it was first opened, and says
 * only whether today is that day (new) or not (returning); home offers the About page for that
 * day's first week. Best effort: nothing is queued while
 * offline, retried, or shown to the reader. Only Release builds report (`OfficeCountsUsage`,
 * set in project.yml); Debug builds, and so the tests and the simulator screenshots, never do.
 *
 * Beacons go to the production server until a reply names the site's new address
 * (`usageEndpointHeader`); the app keeps that and posts there from then on.
 */
final class Usage {
    static let shared = Usage()

    /// Posts `body` to `endpoint`, then reports whether it counted and the endpoint the reply named.
    typealias Post = (_ endpoint: String, _ id: String, _ body: String, _ done: @escaping (Bool, String?) -> Void) -> Void

    private let enabled: Bool
    private let defaults: UserDefaults
    private let now: () -> Date
    private let post: Post
    /// Beacons already counted today by this process, as "day body".
    private var sent = Set<String>()
    private let lock = NSLock()

    init(
        enabled: Bool = Bundle.main.object(forInfoDictionaryKey: "OfficeCountsUsage") as? String == "YES",
        defaults: UserDefaults = .standard,
        now: @escaping () -> Date = Date.init,
        post: @escaping Post = Usage.send
    ) {
        self.enabled = enabled
        self.defaults = defaults
        self.now = now
        self.post = post
    }

    /// `martyrology` is Prime's `HourView.martyrology`: whether its reading was shown, if it had one.
    func record(_ event: UsageEvent, dark: Bool, form: String, martyrology: Bool? = nil) {
        guard enabled else { return }
        let date = now()
        let day = Usage.reportingDay(date)
        let first = Usage.firstDay(day, defaults) == day
        guard let body = usageBeacon(event: event, today: .of(date), dark: dark, form: form, client: .ios, martyrology: martyrology, first: first) else { return }
        let key = day + " " + body
        lock.lock()
        let fresh = sent.insert(key).inserted
        lock.unlock()
        guard fresh else { return }
        let endpoint = defaults.string(forKey: "usage-endpoint") ?? usageEndpoint()
        post(endpoint, identifier(for: day), body) { ok, advertised in
            if let next = advertised.flatMap({ usageAdvertisedEndpoint(value: $0) }) {
                self.defaults.set(next, forKey: "usage-endpoint")
            }
            // A failure is forgotten, so the next visit tries again.
            guard !ok else { return }
            self.lock.lock()
            self.sent.remove(key)
            self.lock.unlock()
        }
    }

    /// The reporting day the app was first opened, kept in every build (it is never sent), as
    /// the web's `office-first-counted` is kept whether or not a beacon goes. An installation
    /// that reported before this was kept (it holds a day's identifier) is from an earlier day.
    private static func firstDay(_ day: String, _ defaults: UserDefaults) -> String {
        if let first = defaults.string(forKey: "usage-first") { return first }
        let first = defaults.string(forKey: "usage-day") == nil ? day : "before"
        defaults.set(first, forKey: "usage-first")
        return first
    }

    /// Reporting days since the app was first opened, as `aboutNewcomer` reads them: -1 for an
    /// installation from before first days were kept.
    static func daysSinceFirst(defaults: UserDefaults = .standard, now: Date = Date()) -> Int32 {
        let today = reportingDay(now)
        guard let first = CivilDate.parse(firstDay(today, defaults)), let day = CivilDate.parse(today) else { return -1 }
        return Int32(civil.dateComponents([.day], from: first.date, to: day.date).day ?? -1)
    }

    /// A page of the site the beacons go to: the production site, or the new address a reply
    /// named. "/privacy" → "https://orthodoxwestbreviary.com/privacy".
    static func siteURL(_ path: String, defaults: UserDefaults = .standard) -> URL? {
        let endpoint = defaults.string(forKey: "usage-endpoint") ?? usageEndpoint()
        let api = "/api/usage"
        guard endpoint.hasSuffix(api) else { return nil }
        return URL(string: String(endpoint.dropLast(api.count)) + path)
    }

    /// The reporting day's identifier: kept all day, replaced the next.
    private func identifier(for day: String) -> String {
        if defaults.string(forKey: "usage-day") == day, let id = defaults.string(forKey: "usage-id") { return id }
        let id = UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased()
        defaults.set(day, forKey: "usage-day")
        defaults.set(id, forKey: "usage-id")
        return id
    }

    private static let reporting: Calendar = {
        var c = Calendar(identifier: .gregorian)
        c.timeZone = TimeZone(identifier: "America/New_York") ?? .current
        return c
    }()

    /// The server's reporting day of an instant: "2026-03-15".
    static func reportingDay(_ date: Date) -> String {
        let p = reporting.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", p.year ?? 0, p.month ?? 0, p.day ?? 0)
    }

    /// No cookie store and no cache: the day's identifier is the only cookie sent.
    private static let session: URLSession = {
        let c = URLSessionConfiguration.ephemeral
        c.timeoutIntervalForRequest = 4
        c.httpShouldSetCookies = false
        c.httpCookieAcceptPolicy = .never
        c.urlCache = nil
        return URLSession(configuration: c)
    }()

    private static let agent: String = {
        let version = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "0"
        let os = ProcessInfo.processInfo.operatingSystemVersion
        return "DivineOffice/\(version) (iOS \(os.majorVersion).\(os.minorVersion))"
    }()

    /**
     * Posts one beacon as the web's page does: the header that marks it as a beacon, and the
     * day's identifier as the cookie the server would otherwise have set.
     */
    static func send(endpoint: String, id: String, body: String, done: @escaping (Bool, String?) -> Void) {
        guard let url = URL(string: endpoint) else { return done(false, nil) }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("1", forHTTPHeaderField: "X-Office-Usage")
        request.setValue("text/plain", forHTTPHeaderField: "Content-Type")
        request.setValue("office-usage=\(id)", forHTTPHeaderField: "Cookie")
        request.setValue(agent, forHTTPHeaderField: "User-Agent")
        request.httpBody = Data(body.utf8)
        session.dataTask(with: request) { _, response, error in
            let http = response as? HTTPURLResponse
            let status = http?.statusCode ?? 0
            done(error == nil && (200..<300).contains(status), http?.value(forHTTPHeaderField: usageEndpointHeader()))
        }.resume()
    }
}

extension Page {
    /// The page as the usage count names it: the year's frontispiece is part of the ordo. About
    /// is not counted, as the web counts only home, the hours, the ordo and the reminders.
    var usageEvent: UsageEvent? {
        switch self {
        case let .home(d): return .home(date: d)
        case let .hour(d, h): return .hour(date: d, hour: h)
        case let .ordo(y, _, _): return .ordo(year: Int32(y))
        case let .year(y): return .ordo(year: Int32(y))
        case .reminders: return .remindersPage
        case .about: return nil
        }
    }
}
