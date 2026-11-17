import SwiftUI

/**
 * For the simulator screenshots (apps/ios/screenshots.sh): once the page a launch's `-page` asked
 * for has its content on screen (composed, laid out, and scrolled to an `-anchor` asked for), a
 * marker in the app's temporary directory says so, and the script shoots only after it. Before
 * then the page can be the bare wall, which is as still as a finished page. A launch without
 * `-page` writes nothing.
 */
enum ScreenshotReady {
    static let marker = FileManager.default.temporaryDirectory.appendingPathComponent("screenshot-ready")

    /// Marks `page` drawn, if it is the page the launch asked for.
    static func mark(_ page: Page, in model: AppModel) {
        guard model.launchPage == page else { return }
        // A turn of the main queue after the content appeared: laid out, and drawn on the next frame.
        DispatchQueue.main.async {
            try? Data("\(page.encoded)\n".utf8).write(to: marker, options: .atomic)
        }
    }
}

extension View {
    /// Marks the screenshot ready when this, the content of `page`, appears and `ready` holds.
    func screenshotReady(_ page: Page, ready: Bool = true) -> some View {
        modifier(ScreenshotReadyMark(page: page, ready: ready))
    }
}

private struct ScreenshotReadyMark: ViewModifier {
    let page: Page
    let ready: Bool
    @EnvironmentObject private var model: AppModel

    func body(content: Content) -> some View {
        // Again when the launch's page is known, should the content come first.
        content.onChange(of: ready && model.launchPage == page, initial: true) { _, now in
            if now { ScreenshotReady.mark(page, in: model) }
        }
    }
}

/**
 * For the simulator screenshots too: what a page measured of its own layout, kept beside its shot
 * (apps/ios/screenshots.sh saves it as `<shot>.geometry.txt`), so a shot that differs from its
 * baseline says which measure moved (#891). Each value is kept with the first it took and how
 * often it changed. A launch without `-page` notes nothing.
 */
enum ScreenshotGeometry {
    static let file = FileManager.default.temporaryDirectory.appendingPathComponent("screenshot-geometry.txt")
    private static let on = ProcessInfo.processInfo.arguments.contains("-page")
    private static var notes: [String: (first: String, last: String, changes: Int)] = [:]

    /// Notes `value` under `key`, and writes the page's notes out again.
    static func note(_ key: String, _ value: String) {
        guard on else { return }
        DispatchQueue.main.async {
            if let old = notes[key] {
                guard old.last != value else { return }
                notes[key] = (first: old.first, last: value, changes: old.changes + 1)
            } else {
                notes[key] = (first: value, last: value, changes: 0)
            }
            let text = notes.keys.sorted().map { key in
                let n = notes[key]!
                return n.changes == 0 ? "\(key): \(n.last)" : "\(key): \(n.last) (first \(n.first), changed \(n.changes) times)"
            }.joined(separator: "\n")
            try? Data((text + "\n").utf8).write(to: file, options: .atomic)
        }
    }
}
