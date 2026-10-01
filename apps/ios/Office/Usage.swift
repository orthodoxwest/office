import Foundation

/**
 * The daily usage counts, reported as the web reports them (README, "Usage metrics"): the page
 * shown, the appearance and prayer form it is read in, once a day. The beacon's words come from
 * the Rust core, so the server reads the app's as it reads the web's, and only today's pages
 * (a day either side) count.
 *
 * The server knows a reader only by an identifier it hashes with the day. The app makes a fresh
 * random one each reporting day (America/New_York, as the server reckons days), so nothing it
 * sends ties one day to the next. Best effort: nothing is queued while offline, retried, or
 * shown to the reader. Only Release builds report (`OfficeCountsUsage`, set in project.yml);
 * Debug builds, and so the tests and the simulator screenshots, never do.
 */
final class Usage {
    static let shared = Usage()

    typealias Post = (_ id: String, _ body: String, _ done: @escaping (Bool) -> Void) -> Void

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

    func record(_ event: UsageEvent, dark: Bool, form: String) {
        guard enabled else { return }
        let date = now()
        guard let body = usageBeacon(event: event, today: .of(date), dark: dark, form: form, client: .ios) else { return }
        let day = Usage.reportingDay(date)
        let key = day + " " + body
        lock.lock()
        let fresh = sent.insert(key).inserted
        lock.unlock()
        guard fresh else { return }
        post(identifier(for: day), body) { ok in
            // A failure is forgotten, so the next visit tries again.
            guard !ok else { return }
            self.lock.lock()
            self.sent.remove(key)
            self.lock.unlock()
        }
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
    static func send(id: String, body: String, done: @escaping (Bool) -> Void) {
        guard let url = URL(string: usageEndpoint()) else { return done(false) }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("1", forHTTPHeaderField: "X-Office-Usage")
        request.setValue("text/plain", forHTTPHeaderField: "Content-Type")
        request.setValue("office-usage=\(id)", forHTTPHeaderField: "Cookie")
        request.setValue(agent, forHTTPHeaderField: "User-Agent")
        request.httpBody = Data(body.utf8)
        session.dataTask(with: request) { _, response, error in
            let status = (response as? HTTPURLResponse)?.statusCode ?? 0
            done(error == nil && (200..<300).contains(status))
        }.resume()
    }
}

extension Page {
    /// The page as the usage count names it: the year's frontispiece is part of the ordo.
    var usageEvent: UsageEvent {
        switch self {
        case let .home(d): return .home(date: d)
        case let .hour(d, h): return .hour(date: d, hour: h)
        case let .ordo(y, _, _): return .ordo(year: Int32(y))
        case let .year(y): return .ordo(year: Int32(y))
        case .reminders: return .remindersPage
        }
    }
}
