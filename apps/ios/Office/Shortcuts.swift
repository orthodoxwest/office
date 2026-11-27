import UIKit

/**
 * Long-pressing the app's icon offers the hours most kept at home, and the ordo, as Android's
 * launcher shortcuts (Shortcuts.kt). The items are project.yml's `UIApplicationShortcutItems`;
 * they carry no date, so each opens the day it is chosen.
 */
enum Shortcuts {
    static let ordo = "ordo"
    static let hours = ["lauds", "vespers", "compline"]

    /**
     * The page a shortcut opens: the ordo at this month, or an hour today, except Compline in the
     * small hours, which belongs to the day before, as home's "Pray now" reckons it.
     */
    static func page(_ type: String, today: CivilDate, clockHour: Int) -> Page? {
        if type == ordo { return .ordo(year: Int(today.year), month: Int(today.month), day: Int(today.day)) }
        guard hours.contains(type) else { return nil }
        let current = currentOffice(clockHour: Int32(clockHour))
        return .hour(current.hour == type ? today.adding(days: Int(current.dayOffset)) : today, type)
    }

    /// Opens the shortcut's page over whatever is shown; false for one this build does not know.
    @discardableResult
    static func open(_ item: UIApplicationShortcutItem) -> Bool {
        let model = AppModel.shared
        model.refreshToday()
        guard let page = page(item.type, today: model.today, clockHour: civil.component(.hour, from: Date())) else { return false }
        model.openedExternally = true
        model.open(page)
        return true
    }
}

/// The scene's delegate, for the shortcut chosen: at launch, or while the app is in the background.
final class SceneDelegate: NSObject, UIWindowSceneDelegate {
    func scene(_ scene: UIScene, willConnectTo session: UISceneSession, options connectionOptions: UIScene.ConnectionOptions) {
        if let item = connectionOptions.shortcutItem { DispatchQueue.main.async { Shortcuts.open(item) } }
    }

    func windowScene(_ windowScene: UIWindowScene, performActionFor shortcutItem: UIApplicationShortcutItem, completionHandler: @escaping (Bool) -> Void) {
        completionHandler(Shortcuts.open(shortcutItem))
    }
}
