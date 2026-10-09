import SwiftUI

/// The About page's words, as the web's /about sets them (presentation::about): read once, as they never change.
let aboutPage: AboutView = about()

/// Where a link on the About page leads: a page of the app, or the browser.
enum AboutLink: Equatable {
    case ordo
    case reminders
    case web(URL)
}

/**
 * The page an About link's target opens: the site's own paths map to the app's pages, except
 * the privacy policy, which lives only on the site the beacons go to.
 */
func aboutLink(_ target: String, defaults: UserDefaults = .standard) -> AboutLink? {
    switch target {
    case "/calendar": return .ordo
    case "/reminders": return .reminders
    case "/privacy": return Usage.siteURL("/privacy", defaults: defaults).map(AboutLink.web)
    default: return target.hasPrefix("https://") ? URL(string: target).map(AboutLink.web) : nil
    }
}

struct AboutPage: View {
    var body: some View {
        AboutScreen(view: aboutPage)
            .background(PlasterWall())
            .toolbar(.hidden, for: .navigationBar)
    }
}

/**
 * About the Office, set as the web's: the reminders page's headpiece and measure, the verse as
 * red work, tituli over the sections, and the seven hours in home's table of periods.
 */
struct AboutScreen: View {
    let view: AboutView
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.openURL) private var openURL

    var body: some View {
        ScrollView {
            VStack(spacing: 0) {
                SiteHeader()
                VStack(alignment: .leading, spacing: 0) {
                    PlainHeadpiece()
                    Text(view.title).type(Scale.body.sized(28.8, line: 34.56)).foregroundStyle(p.text)
                        .padding(.top, m.px(8.8))
                        .accessibilityAddTraits(.isHeader)
                    ForEach(Array(view.blocks.enumerated()), id: \.offset) { i, b in
                        let next = i + 1 < view.blocks.count ? view.blocks[i + 1] : nil
                        block(b)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            // The web's margins, collapsing between neighbours as CSS's do.
                            .padding(.top, m.px(max(i == 0 ? 8.8 : below(view.blocks[i - 1], next: b), above(b))))
                            .padding(.bottom, next == nil ? m.px(below(b, next: nil)) : 0)
                    }
                }
                .measured(m)
                .padding(.top, m.px(24))
                Footer()
            }
        }
        // A paragraph's links open the app's own pages, or the browser.
        .environment(\.openURL, OpenURLAction { url in
            switch aboutLink(url.absoluteString) {
            case .ordo?:
                let t = model.today
                model.open(.ordo(year: Int(t.year), month: Int(t.month), day: Int(t.day)))
                return .handled
            case .reminders?:
                model.open(.reminders)
                return .handled
            case let .web(site)?:
                openURL(site)
                return .handled
            case nil:
                return .discarded
            }
        })
    }

    /// The space a block keeps above it (CSS margin-top).
    private func above(_ b: AboutBlockView) -> CGFloat {
        switch b.kind {
        case "verse": return 19.2
        case "heading": return 28.8
        case "note": return 8.8
        default: return 0
        }
    }

    /// The space a block keeps below it (CSS margin-bottom): a key's list ends as a paragraph does.
    private func below(_ b: AboutBlockView, next: AboutBlockView?) -> CGFloat {
        switch b.kind {
        case "heading": return 7.2
        case "verse", "hours": return 0
        case "key": return next?.kind == "key" ? 8.8 : 12.8
        default: return 12.8
        }
    }

    @ViewBuilder private func block(_ b: AboutBlockView) -> some View {
        switch b.kind {
        case "verse":
            VStack(spacing: 0) {
                Text(b.text).type(Scale.bodyItalic.sized(20, line: 31))
                Text(b.cite).type(TextStyle(size: 13.12, line: 20.3, tracking: 13.12 * 0.06, smallCaps: true))
            }
            .foregroundStyle(p.rubric)
            .multilineTextAlignment(.center)
            .frame(maxWidth: .infinity)
        case "heading":
            // A titulus, as the web's `--titulus` in tracked small capitals.
            Text(b.text).type(TextStyle(size: 15.2, line: 23.6, tracking: 15.2 * 0.08, smallCaps: true)).foregroundStyle(p.titulus)
                .accessibilityAddTraits(.isHeader)
        case "paragraph":
            Text(linked(b.runs)).type(Scale.body.sized(20, line: 31)).foregroundStyle(p.text)
        case "hours":
            AboutHours(periods: view.periods)
        case "note":
            Text(b.text).type(Scale.body.sized(16.8, line: 26)).foregroundStyle(p.muted)
        case "key":
            (Text(b.mark).foregroundColor(b.red ? p.rubric : p.text) + Text(" " + b.text))
                .type(Scale.body.sized(20, line: 31)).foregroundStyle(p.text)
        default:
            // The intro, and any block a later core adds, as plain prose.
            Text(b.text).type(Scale.body.sized(20, line: 31)).foregroundStyle(p.text)
        }
    }

    /// A paragraph's runs, its links in the accent (the app's tint) over a gold hairline, as the web's `.about a`.
    private func linked(_ runs: [AboutRunView]) -> AttributedString {
        runs.reduce(into: AttributedString()) { out, run in
            var s = AttributedString(run.text)
            if !run.link.isEmpty, let url = URL(string: run.link) {
                s.link = url
                s[AttributeScopes.SwiftUIAttributes.UnderlineStyleAttribute.self] = Text.LineStyle(pattern: .solid, color: p.goldLine)
            }
            out += s
        }
    }
}

/**
 * The seven hours in home's three periods: the period cells in the frontispiece's wash with their
 * painted sun and moon, ruled in its ink and framed in the lining thinned. Each hour's name opens
 * it today; its gloss is in italic, and when it is said muted beneath.
 */
private struct AboutHours: View {
    let periods: [AboutPeriodView]
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        let ink = FrontispieceInk.of(p)
        let label = TextStyle(size: 11.52, line: 13.8, tracking: 11.52 * 0.08, smallCaps: true)
        VStack(spacing: 0) {
            ForEach(Array(periods.enumerated()), id: \.offset) { i, period in
                if i > 0 { Hairline(color: ink.rule) }
                HStack(spacing: 0) {
                    VStack(spacing: m.px(2.56)) {
                        PeriodIcon(period: Period.named(period.label), color: o.flat)
                        Text(period.label).type(label).foregroundStyle(p.muted).lineLimit(1).fixedSize()
                    }
                    .padding(.horizontal, m.px(6.4))
                    .frame(minWidth: m.px(83.2))
                    .frame(maxHeight: .infinity)
                    .background(ink.band)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(period.label)
                    .accessibilityAddTraits(.isHeader)
                    Rectangle().fill(ink.rule).frame(width: 1)
                    VStack(spacing: 0) {
                        ForEach(Array(period.hours.enumerated()), id: \.offset) { j, h in
                            if j > 0 { Hairline(color: ink.rule) }
                            row(h)
                        }
                    }
                }
                .fixedSize(horizontal: false, vertical: true)
            }
        }
        .overlay(Rectangle().strokeBorder(ink.panelRule, lineWidth: 1).allowsHitTesting(false))
    }

    /// The whole row opens its hour: the web links the name alone, but a finger wants the row.
    private func row(_ h: AboutHourView) -> some View {
        Button { model.open(.hour(model.today, h.hour)) } label: {
            HStack(alignment: .firstTextBaseline, spacing: m.px(12)) {
                Text(h.name).type(TextStyle(size: 19.2, line: 24)).foregroundStyle(p.text)
                    .lineLimit(1)
                    .minimumScaleFactor(0.7)
                    .frame(width: m.px(88), alignment: .leading)
                VStack(alignment: .leading, spacing: 0) {
                    Text(h.gloss).type(TextStyle(size: 17.6, line: 22.9, italic: true)).foregroundStyle(p.text)
                    Text(h.time).type(TextStyle(size: 14.4, line: 18.7)).foregroundStyle(p.muted)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(.vertical, m.px(8))
            .padding(.horizontal, m.px(13.6))
        }
        .buttonStyle(Quiet())
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(h.name), \(h.gloss), \(h.time)")
        .accessibilityAddTraits(.isButton)
    }
}

extension Period {
    /// A period by its table's label, as the web's `period_ornament`: the moon for any but morning and day.
    static func named(_ label: String) -> Period {
        switch label.lowercased() {
        case "morning": return .morning
        case "day": return .day
        default: return .evening
        }
    }
}
