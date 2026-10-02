import SwiftUI
import UIKit
import UserNotifications

/**
 * The Divine Office on the shared Rust core, set as the web sets it.
 *
 * Launch arguments open a page at a date, for the simulator screenshots:
 * `-page hour -hour lauds -date 2026-03-15`, or `-page home`, `ordo`, `year`, `reminders`;
 * `-today 2026-03-15` fixes today, and `-theme apse` or `-text-size large` choose the look.
 */
@main
struct OfficeApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @StateObject private var model = AppModel.shared

    var body: some Scene {
        WindowGroup {
            RootView().environmentObject(model)
        }
    }
}

/// Reminders: how a tapped one opens its hour, and how "Remind me in 10 min" puts one off.
final class AppDelegate: NSObject, UIApplicationDelegate, UNUserNotificationCenterDelegate {
    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        let center = UNUserNotificationCenter.current()
        center.delegate = self
        ReminderScheduler.register(center)
        ReminderScheduler.registerRefresh()
        return true
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse, withCompletionHandler completionHandler: @escaping () -> Void) {
        let info = response.notification.request.content.userInfo
        if response.actionIdentifier == ReminderScheduler.snoozeAction {
            ReminderScheduler.snooze(response.notification)
        } else if response.actionIdentifier == UNNotificationDefaultActionIdentifier,
                  let hour = info["hour"] as? String,
                  let date = (info["date"] as? String).flatMap(CivilDate.parse) {
            DispatchQueue.main.async { AppModel.shared.open(.hour(date, hour)) }
        }
        completionHandler()
    }

    // A reminder that comes while the app is open still rings and shows.
    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification, withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        completionHandler([.banner, .list, .sound])
    }
}

/// The app on its plaster wall: whichever page is open, under the shared header and menu.
struct RootView: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.colorScheme) private var system
    @Environment(\.dynamicTypeSize) private var dynamicType
    @Environment(\.scenePhase) private var phase
    /// The way back, kept for a return after iOS has closed the app in the background.
    @SceneStorage("way-back") private var wayBack = ""
    @State private var started = false
    /// The clock, read every minute while the app is in front: a timer to the next hour stops
    /// counting while the phone sleeps, so home could still highlight Sext at Vespers.
    private let minute = Timer.publish(every: 60, on: .main, in: .common).autoconnect()

    var body: some View {
        let palette = model.theme.dark(system) ? Palette.apse : Palette.nave
        let visit = Visit(page: model.page, form: model.form, dark: palette.dark, today: model.today, active: phase == .active)
        GeometryReader { geo in
            NavigationStack(path: $model.path) {
                HomePage(date: model.root)
                    .navigationDestination(for: Entry.self) { entry in EntryPage(entry: entry.id) }
            }
            .overlay(alignment: .topTrailing) {
                if model.menuOpen && geo.size.width < wideFrom {
                    ZStack(alignment: .topTrailing) {
                        // A tap anywhere else closes the menu.
                        Color.black.opacity(0.001).ignoresSafeArea().onTapGesture { model.menuOpen = false }
                            .accessibilityHidden(true)
                        MenuPanel()
                            .padding(.top, 52)
                            .padding(.horizontal, gutter)
                    }
                    .transition(.opacity.combined(with: .offset(y: -8)))
                }
                if model.settingsOpen && geo.size.width >= wideFrom {
                    ZStack(alignment: .topTrailing) {
                        Color.black.opacity(0.001).ignoresSafeArea().onTapGesture { model.settingsOpen = false }
                            .accessibilityHidden(true)
                        // Under the header's end: the nav shell is held to 68rem and centred.
                        MenuPanel(prefsOnly: true)
                            .padding(.top, 52)
                            .padding(.trailing, max(0, (geo.size.width - 1088) / 2) + gutter)
                    }
                    .transition(.opacity.combined(with: .offset(y: -8)))
                }
            }
            .animation(.easeOut(duration: 0.15), value: model.menuOpen)
            .animation(.easeOut(duration: 0.15), value: model.settingsOpen)
            .environment(\.wide, geo.size.width >= wideFrom)
        }
        .environment(\.palette, palette)
        .environment(\.ornament, Ornament.of(palette, season: ""))
        .environment(\.metrics, Metrics(textSize: model.textSize, dynamicType: dynamicType))
        .background(palette.bg.ignoresSafeArea())
        .tint(palette.accent)
        .preferredColorScheme(model.theme.scheme)
        .onAppear {
            guard !started else { return }
            started = true
            if !model.openFromLaunch() && !wayBack.isEmpty { model.restore(wayBack) }
        }
        .onChange(of: model.saved) { _, saved in wayBack = saved }
        // The page shown counts in the day's usage whenever it, how it is read, or the day
        // changes while the app is in front; each once a day (Usage.swift).
        .onChange(of: visit, initial: true) { _, v in
            guard v.active else { return }
            Usage.shared.record(v.page.usageEvent, dark: v.dark, form: v.form)
        }
        .onReceive(minute) { _ in
            if phase == .active { model.refreshToday() }
        }
        .onChange(of: phase) { _, now in
            guard now == .active else { return }
            model.refreshToday()
            // Every visit keeps the reminders running ahead.
            ReminderScheduler.sync()
            ReminderScheduler.scheduleRefresh()
        }
    }
}

/**
 * A visit's page, read from the way back as it is now: a page of the same kind replacing it
 * changes it here, in place.
 */
private struct EntryPage: View {
    let entry: Int
    @EnvironmentObject private var model: AppModel

    var body: some View {
        switch model.page(of: entry) {
        case let .home(d)?: HomePage(date: d)
        case let .hour(d, h)?: HourPage(date: d, hour: h)
        case let .ordo(y, m, d)?: OrdoPage(year: y, month: m, day: d)
        case let .year(y)?: YearPage(year: y)
        case .reminders?: RemindersPage()
        // Gone from the way back, while the stack lets it go.
        case nil: Color.clear
        }
    }
}

/// What the day's usage count reads: the page, how it is read, the day, and whether the app is in front.
private struct Visit: Equatable {
    let page: Page
    let form: String
    let dark: Bool
    let today: CivilDate
    let active: Bool
}

/// The pages hide the navigation bar, as the web has none; the edge swipe back stays.
extension UINavigationController: @retroactive UIGestureRecognizerDelegate {
    override open func viewDidLoad() {
        super.viewDidLoad()
        interactivePopGestureRecognizer?.delegate = self
    }

    public func gestureRecognizerShouldBegin(_ gestureRecognizer: UIGestureRecognizer) -> Bool {
        viewControllers.count > 1
    }
}
