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
