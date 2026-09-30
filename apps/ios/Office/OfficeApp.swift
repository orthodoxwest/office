import SwiftUI

/**
 * The Divine Office on the shared Rust core. This first screen composes one hour, to prove the
 * pipeline end to end; the design pass follows the Android app's.
 *
 * Launch arguments choose what is shown, for the simulator screenshots:
 * `-hour lauds -date 2026-03-15 -form private`.
 */
@main
struct OfficeApp: App {
    var body: some Scene {
        WindowGroup { RootView() }
    }
}

/// The engine, loaded once per process.
enum Office {
    static let core: Result<OfficeCore, Error> = Result { try OfficeCore() }
}

/// What the launch asked for, else the hour being prayed now.
struct Request {
    let hour: String
    let year: Int32
    let month: Int32
    let day: Int32
    let form: String

    static func fromLaunch(now: Date = Date()) -> Request {
        let defaults = UserDefaults.standard
        let calendar = Calendar.current
        let current = currentOffice(clockHour: Int32(calendar.component(.hour, from: now)))
        var date = calendar.date(byAdding: .day, value: Int(current.dayOffset), to: now) ?? now
        if let asked = defaults.string(forKey: "date"), let parsed = Request.day.date(from: asked) {
            date = parsed
        }
        let parts = calendar.dateComponents([.year, .month, .day], from: date)
        return Request(
            hour: defaults.string(forKey: "hour") ?? current.hour,
            year: Int32(parts.year ?? 2026),
            month: Int32(parts.month ?? 1),
            day: Int32(parts.day ?? 1),
            form: defaults.string(forKey: "form") ?? "private"
        )
    }

    private static let day: DateFormatter = {
        let f = DateFormatter()
        f.calendar = Calendar(identifier: .gregorian)
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "yyyy-MM-dd"
        return f
    }()
}

struct RootView: View {
    @State private var hour: HourView?
    @State private var failure: String?
    private let p = Palette.nave

    var body: some View {
        ZStack {
            p.bg.ignoresSafeArea()
            if let hour {
                HourScreen(view: hour, p: p)
            } else if let failure {
                Text(failure).style(Scale.small).foregroundStyle(p.rubric).padding()
            } else {
                Text("Preparing the office…").style(Scale.small).foregroundStyle(p.muted)
            }
        }
        .task { load() }
    }

    /// Parsing the corpus takes a moment, so it runs off the main thread.
    private func load() {
        let request = Request.fromLaunch()
        DispatchQueue.global(qos: .userInitiated).async {
            let result = Result { () throws -> HourView in
                try Office.core.get().compose(hour: request.hour, year: request.year, month: request.month, day: request.day, form: request.form)
            }
            DispatchQueue.main.async {
                switch result {
                case let .success(view): hour = view
                case let .failure(error): failure = "\(error)"
                }
            }
        }
    }
}
