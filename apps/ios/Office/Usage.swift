import Foundation

/**
 * The daily usage counts, reported as the web reports them (DEPLOYMENT.md, "Usage metrics"): the
 * page shown, the appearance and prayer form it is read in, and at Prime the reader's After Prime
 * choice on a day with a Martyrology reading, once a day. The beacon's words come from
 * the Rust core, so the server reads the app's as it reads the web's, and only today's pages
 * (a day either side) count.
 *
 * The server knows a reader only by an identifier it hashes with the day. The app makes a fresh
 * random one each reporting day (America/New_York, as the server reckons days), so nothing it
 * sends ties one day to the next. The app also keeps the day it was first opened, and says
 * only whether today is that day (new) or not (returning); home offers the About page for that
 * day's first week. It keeps too the last two days a beacon of its own was counted, and says only
 * whether today is the first in its week and in its month, so the server can add up a week's
 * readers. Beside those it says, in buckets and yes-or-no words the core writes (`UsageHabits`):
 * how long ago it was first counted, on the week's first day how many days it was counted the
 * week before (it keeps a tally of the week's days), an hour opened at its own time by the phone's
 * clock, an hour prayed (home's mark), an hour opened that could not report since a beacon last
 * arrived, and whether reminders are on. Best effort: nothing is queued while
 * offline, retried, or shown to the reader. Only Release builds report (`OfficeCountsUsage`,
 * set in project.yml); Debug builds, and so the tests and the simulator screenshots, never do.
 * Nor does a device Apple installs each uploaded TestFlight build on fresh, to check it: each
 * would count as a new reader, about one for every build. Nothing on those devices says so, but
 * each is done with the build soon after installing it, so an installation reports only once it
 * is `settling` old, as Android's does. TestFlight builds and the copy App Review runs carry a
 * sandbox receipt, and a simulator none of the store's: neither reports (`storeInstall`), so each
 * submission's review is not a new reader, and the store's own installs are the readers counted.
 *
 * Each beacon also carries the build's number on a header of its own (`usageBuildHeader`), which
 * every reader of the build shares, so the report can tell how many readers have a correction.
 *
 * Beacons go to the production server until a reply names the site's new address
 * (`usageEndpointHeader`); the app keeps that and posts there from then on.
 */
final class Usage {
    static let shared = Usage(
        enabled: Bundle.main.object(forInfoDictionaryKey: "OfficeCountsUsage") as? String == "YES" && Usage.storeInstall,
        installed: Usage.installedAt()
    )

    /// Whether this copy came from the App Store. TestFlight's, and the one App Review runs, have
    /// a receipt named for the sandbox; a simulator's build is nobody's install.
    static var storeInstall: Bool {
        #if targetEnvironment(simulator)
        return false
        #else
        return Bundle.main.appStoreReceiptURL?.lastPathComponent != "sandboxReceipt"
        #endif
    }

    /**
     * How old an installation must be before it reports, as Android's `Usage.SETTLING`. A
     * reader's first hours on a new install go uncounted, and every page after them counts as
     * usual (as New, on the first such day). An update keeps the install date, so testers who
     * update keep counting.
     */
    static let settling: TimeInterval = 2 * 3600

    /// When this app was installed on the device: its Documents folder's creation, which an
    /// update keeps. Nil, counting at once, if the system does not say.
    static func installedAt() -> Date? {
        guard let documents = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first else { return nil }
        return (try? FileManager.default.attributesOfItem(atPath: documents.path))?[.creationDate] as? Date
    }

    /// Posts `body` to `endpoint`, then reports whether it counted and the endpoint the reply named.
    typealias Post = (_ endpoint: String, _ id: String, _ body: String, _ done: @escaping (Bool, String?) -> Void) -> Void

    private let enabled: Bool
    private let defaults: UserDefaults
    private let now: () -> Date
    /// When this installation was installed; nil counts at once.
    private let installed: Date?
    private let post: Post
    /// Beacons already counted today by this process, as "day body".
    private var sent = Set<String>()
    private let lock = NSLock()

    init(
        enabled: Bool = Bundle.main.object(forInfoDictionaryKey: "OfficeCountsUsage") as? String == "YES",
        defaults: UserDefaults = .standard,
        now: @escaping () -> Date = Date.init,
        installed: Date? = nil,
        post: @escaping Post = Usage.send
    ) {
        self.enabled = enabled
        self.defaults = defaults
        self.now = now
        self.installed = installed
        self.post = post
    }

    /// `afterPrime` is Prime's `HourView.afterPrime`: the reader's After Prime choice, if it had a reading;
    /// `chant` an hour's `HourView.chant`: whether its scores were shown, if it had any; `prayed`
    /// whether the hour shown is marked prayed; `reminders` whether the app's reminders are on.
    func record(
        _ event: UsageEvent, dark: Bool, form: String, afterPrime: String? = nil, chant: Bool? = nil,
        prayed: Bool = false, reminders: Bool? = nil
    ) {
        guard enabled else { return }
        let date = now()
        if let installed, date.timeIntervalSince(installed) < Usage.settling { return }
        let day = Usage.reportingDay(date)
        let firstDay = Usage.firstDay(day, defaults)
        let first = firstDay == day
        let periods = usagePeriods(today: day, last: defaults.string(forKey: "usage-counted"), previous: defaults.string(forKey: "usage-counted-before"))
        let habits = UsageHabits(
            reportingDay: day, firstDay: firstDay, tally: defaults.string(forKey: "usage-week-days"),
            clockHour: Int32(Calendar.current.component(.hour, from: date)), prayed: prayed,
            offline: defaults.bool(forKey: "usage-offline"), reminders: reminders
        )
        guard let body = usageBeacon(
            event: event, today: .of(date), dark: dark, form: form, client: .ios, afterPrime: afterPrime, first: first, chant: chant,
            week: periods?.week, month: periods?.month, habits: habits
        ) else { return }
        // Saying a failed hour is not a new page: it neither sends again nor counts apart.
        let key = day + " " + body.replacingOccurrences(of: " offline:yes", with: "")
        lock.lock()
        let fresh = sent.insert(key).inserted
        lock.unlock()
        guard fresh else { return }
        let endpoint = defaults.string(forKey: "usage-endpoint") ?? usageEndpoint()
        let hour: Bool
        if case .hour = event { hour = true } else { hour = false }
        post(endpoint, identifier(for: day), body) { ok, advertised in
            if let next = advertised.flatMap({ usageAdvertisedEndpoint(value: $0) }) {
                self.defaults.set(next, forKey: "usage-endpoint")
            }
            // A failure is forgotten, so the next visit tries again. An hour whose beacon did not
            // arrive is said on the next that does.
            guard !ok else { return self.counted(day, body) }
            if hour { self.defaults.set(true, forKey: "usage-offline") }
            self.lock.lock()
            self.sent.remove(key)
            self.lock.unlock()
        }
    }

    /// Keeps `day` as the latest reporting day a beacon was counted, and the one before it: what
    /// `usagePeriods` reads to say whether a day opens the week and the month. Kept only once a
    /// beacon has arrived, so a day that never reported opens nothing.
    private func counted(_ day: String, _ body: String) {
        lock.lock()
        defer { lock.unlock() }
        if body.contains(" offline:yes") { defaults.removeObject(forKey: "usage-offline") }
        let last = defaults.string(forKey: "usage-counted")
        guard last != day else { return }
        defaults.set(last, forKey: "usage-counted-before")
        defaults.set(day, forKey: "usage-counted")
        defaults.set(usageTally(kept: defaults.string(forKey: "usage-week-days"), day: day), forKey: "usage-week-days")
    }

    /// The reporting day the app was first counted. An installation that reported before this
    /// was kept (it holds a day's identifier) is from an earlier day.
    private static func firstDay(_ day: String, _ defaults: UserDefaults) -> String {
        if let first = defaults.string(forKey: "usage-first") { return first }
        let first = defaults.string(forKey: "usage-day") == nil ? day : "before"
        defaults.set(first, forKey: "usage-first")
        return first
    }

    /// Reporting days since the app was first opened, as `aboutNewcomer` reads them: -1 for an
    /// installation from before first days were kept. The day is kept apart from the beacon's
    /// first day, which waits for the install to settle: writing that one here would count a
    /// reader who prays in their first hours as returning the next day. It stays on the phone.
    static func daysSinceFirst(defaults: UserDefaults = .standard, now: Date = Date()) -> Int32 {
        let today = reportingDay(now)
        let opened = defaults.string(forKey: "about-first") ?? defaults.string(forKey: "usage-first") ?? {
            let first = defaults.string(forKey: "usage-day") == nil ? today : "before"
            defaults.set(first, forKey: "about-first")
            return first
        }()
        guard let first = CivilDate.parse(opened), let day = CivilDate.parse(today) else { return -1 }
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

    /// The build's number (`CFBundleVersion`: master's commit count, as Android's version code is).
    private static let build: String = Bundle.main.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? ""

    /**
     * Posts one beacon as the web's page does: the header that marks it as a beacon, and the
     * day's identifier as the cookie the server would otherwise have set. Beside them, the build's
     * number, which the server counts apart from the beacon's words, so the report can say how
     * soon a correction reaches readers.
     */
    static func send(endpoint: String, id: String, body: String, done: @escaping (Bool, String?) -> Void) {
        guard let url = URL(string: endpoint) else { return done(false, nil) }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("1", forHTTPHeaderField: "X-Office-Usage")
        request.setValue("text/plain", forHTTPHeaderField: "Content-Type")
        request.setValue("office-usage=\(id)", forHTTPHeaderField: "Cookie")
        request.setValue(agent, forHTTPHeaderField: "User-Agent")
        if !build.isEmpty { request.setValue(build, forHTTPHeaderField: usageBuildHeader()) }
        request.httpBody = Data(body.utf8)
        session.dataTask(with: request) { _, response, error in
            let http = response as? HTTPURLResponse
            let status = http?.statusCode ?? 0
            done(error == nil && (200..<300).contains(status), http?.value(forHTTPHeaderField: usageEndpointHeader()))
        }.resume()
    }
}

extension Page {
    /// The page as the usage count names it: the year's frontispiece is part of the ordo. About,
    /// the search and the feast pages are not counted, as on the web.
    var usageEvent: UsageEvent? {
        switch self {
        case let .home(d): return .home(date: d)
        case let .hour(d, h): return .hour(date: d, hour: h)
        case let .ordo(y, _, _): return .ordo(year: Int32(y))
        case let .year(y): return .ordo(year: Int32(y))
        case .reminders: return .remindersPage
        case .about, .find, .feast: return nil
        }
    }
}
