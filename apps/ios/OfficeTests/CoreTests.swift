import SwiftUI
import UserNotifications
import XCTest
@testable import Office

/// The Rust core, linked into the app for iPhone: its embedded corpus loads and composes.
final class CoreTests: XCTestCase {
    func testComposesAnHourFromTheEmbeddedCorpus() throws {
        let vespers = try Office.core.get().compose(hour: "vespers", year: 2026, month: 3, day: 15, form: "private")
        XCTAssertEqual(vespers.title, "Vespers")
        XCTAssertEqual(vespers.color, "violet")
        XCTAssertFalse(vespers.sections.isEmpty)
        XCTAssertTrue(vespers.sections.flatMap(\.blocks).contains { $0.kind == .versicle })
    }

    func testTheCurrentHourFollowsTheWebSchedule() {
        XCTAssertEqual(currentOffice(clockHour: 1).hour, "compline")
        XCTAssertEqual(currentOffice(clockHour: 1).dayOffset, -1)
        XCTAssertEqual(currentOffice(clockHour: 18).hour, "vespers")
    }

    func testTheGapsFollowTheWebsRhythm() throws {
        let blocks = try Office.core.get().compose(hour: "lauds", year: 2026, month: 3, day: 15, form: "private").sections.flatMap(\.blocks)
        let heading = try XCTUnwrap(blocks.indices.dropFirst().first { blocks[$0].kind == .heading })
        XCTAssertEqual(gapBefore(blocks[heading - 1], blocks[heading]), 38)
    }
}

/// What VoiceOver says for the office: every word, and none of the marks meant for the eye.
final class SpokenTests: XCTestCase {
    private lazy var blocks: [BlockView] = {
        let core = try! Office.core.get()
        return [(3, 15), (4, 5), (12, 25)].flatMap { month, day in
            hourNames().flatMap { hour in (try! core.compose(hour: hour, year: 2026, month: Int32(month), day: Int32(day), form: "priest")).sections.flatMap(\.blocks) }
        }
    }()

    func testTheMarksForTheEyeAreSilent() {
        for b in blocks {
            let said = spoken(b)
            for mark in ["*", "†", "℣", "℟", "✠", "\u{E000}"] { XCTAssertFalse(said.contains(mark), "\(mark) in: \(said)") }
            XCTAssertTrue(!said.trimmingCharacters(in: .whitespaces).isEmpty || b.kind == .gap, "empty: \(b.kind)")
        }
    }

    func testSigilsAreNamedAndTheCrossIsSaid() {
        let said = blocks.map(spoken)
        XCTAssertTrue(said.contains { $0.hasPrefix("Versicle. ") })
        XCTAssertTrue(said.contains { $0.hasPrefix("Response. ") })
        XCTAssertTrue(said.contains { $0.hasPrefix("Antiphon. ") })
        let cross = said.filter { $0.contains("sign of the cross") }
        XCTAssertFalse(cross.isEmpty)
        for s in cross { XCTAssertTrue(!s.contains(", ,") && !s.contains(",.") && !s.hasSuffix(","), s) }
    }

    func testAPostureCueIsAnAside() throws {
        let verse = try XCTUnwrap(blocks.first { b in b.kind == .verse && b.runs.contains { $0.style == .posture } })
        let cue = try XCTUnwrap(verse.runs.first { $0.style == .posture }).text.trimmingCharacters(in: .whitespaces)
        XCTAssertTrue(spoken(verse).contains("(\(cue))"), spoken(verse))
    }
}

/// The typesetter keeps the web's line boxes and seats the initial as the web does.
final class ProseTests: XCTestCase {
    private func spec(_ text: String, initial: Bool) -> ProseSpec {
        let font = Scale.body.uiFont()
        var s = ProseSpec(text: NSAttributedString(string: text, attributes: [.font: font]), font: font, line: 32)
        if initial {
            s.initial = Initial(letter: "O", deep: garamond(61), raised: garamond(42), gap: 3.66, alwaysRaised: false, color: .brown)
        }
        return s
    }

    func testEveryLineBoxIsTheStylesLineHeight() {
        let short = ProseLayout(spec: spec("Lord, have mercy.", initial: false), width: 320)
        XCTAssertEqual(short.height, 32)
        let long = ProseLayout(spec: spec(String(repeating: "Glory be to the Father, and to the Son, and to the Holy Ghost. ", count: 4), initial: false), width: 320)
        XCTAssertEqual(long.height.truncatingRemainder(dividingBy: 32), 0)
        XCTAssertGreaterThan(long.height, 32 * 4)
    }

    func testALongOpeningRunsBesideADeepInitial() {
        let text = String(repeating: "ord, open thou our lips, and our mouth shall shew forth thy praise. ", count: 3)
        let plain = ProseLayout(spec: spec(text, initial: false), width: 320)
        let opened = ProseLayout(spec: spec(text, initial: true), width: 320)
        // Two lines stand beside the capital, so the text takes at least as many lines, and no raised space above.
        XCTAssertGreaterThanOrEqual(opened.height, plain.height)
        XCTAssertEqual(opened.height.truncatingRemainder(dividingBy: 32), 0)
    }

    func testAShortOpeningRaisesItsInitial() {
        let opened = ProseLayout(spec: spec("men.", initial: true), width: 320)
        // The raised capital stands taller than the one line beside it.
        XCTAssertGreaterThan(opened.height, 32)
    }

    func testTheSmallCapsTakeTheRestOfTheFirstWord() throws {
        let core = try Office.core.get()
        let blocks = try core.compose(hour: "lauds", year: 2026, month: 3, day: 15, form: "private").sections.flatMap(\.blocks)
        let opening = try XCTUnwrap(blocks.first { $0.dropCap && $0.runs.first?.style == .plain })
        let text = runs(opening, Scale.body, color: .black, .nave, Metrics())
        let split = try XCTUnwrap(splitInitial(opening, text))
        XCTAssertEqual(split.letter.count, 1)
        XCTAssertTrue(split.letter.first?.isLetter ?? false)
        XCTAssertEqual(split.letter + split.rest.string, String(text.string.drop { $0.isWhitespace }))
    }

    func testEachHymnHasOneCentredColumn() throws {
        let core = try Office.core.get()
        let hour = try core.compose(hour: "vespers", year: 2026, month: 3, day: 15, form: "private")
        let columns = hymnColumns(hour.sections, .nave, Ornament.of(.nave, season: ""), Metrics())
        let stanzas = hour.sections.enumerated().flatMap { si, s in s.blocks.enumerated().filter { $0.element.kind == .stanza }.map { BlockAt(section: si, block: $0.offset) } }
        XCTAssertFalse(stanzas.isEmpty)
        for at in stanzas {
            let column = try XCTUnwrap(columns[at])
            XCTAssertGreaterThan(column, 150)
            XCTAssertLessThanOrEqual(column, 448)
        }
    }
}

/// The web's ornaments, drawn from their SVG paths.
final class SVGTests: XCTestCase {
    func testArcsAndRelativeCommands() {
        // The morning engraving's sun: a half circle of radius 5 standing on y = 13.
        let sun = svg("M7 13a5 5 0 0 1 10 0").boundingRect
        XCTAssertEqual(sun.minX, 7, accuracy: 0.01)
        XCTAssertEqual(sun.maxX, 17, accuracy: 0.01)
        XCTAssertEqual(sun.minY, 8, accuracy: 0.05)
        XCTAssertEqual(sun.maxY, 13, accuracy: 0.01)
        // The evening star, relative from its first point, closed.
        let star = svg("m12 1 1.8 4.7L19 7.5l-5.2 1.8L12 14l-1.8-4.7L5 7.5l5.2-1.8Z").boundingRect
        XCTAssertEqual(star, CGRect(x: 5, y: 1, width: 14, height: 13))
        // The day's full circle, two arcs.
        let disc = svg("M8.5 8a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0 -7 0").boundingRect
        XCTAssertEqual(disc.width, 7, accuracy: 0.05)
        XCTAssertEqual(disc.height, 7, accuracy: 0.05)
    }

    func testThePaintedMarksStandInTheirBoxes() {
        // The consecration cross's ring, its arcs' flags set off by commas: radius 19.3 about the box's centre.
        let ring = Mark.consecration.drawing.shapes[0].path.boundingRect
        XCTAssertEqual(ring.minX, 0.7, accuracy: 0.05)
        XCTAssertEqual(ring.maxX, 39.3, accuracy: 0.05)
        XCTAssertEqual(ring.minY, 0.7, accuracy: 0.05)
        XCTAssertEqual(ring.maxY, 39.3, accuracy: 0.05)
        // The moon: its outer disc's left, top and foot, cut on the right by the inner arc.
        let moon = Mark.moon.drawing.shapes[0].path.boundingRect
        XCTAssertEqual(moon.minX, 2.4, accuracy: 0.05)
        XCTAssertEqual(moon.minY, 2.4, accuracy: 0.05)
        XCTAssertEqual(moon.maxX, 20.18, accuracy: 0.05)
        XCTAssertEqual(moon.maxY, 21.6, accuracy: 0.05)
        for mark in [Mark.consecration, .cross, .sun, .moon] {
            let drawing = mark.drawing
            for shape in drawing.shapes {
                let bounds = shape.path.boundingRect
                XCTAssertFalse(bounds.isEmpty, "\(mark)")
                XCTAssertTrue(CGRect(x: 0, y: 0, width: drawing.box, height: drawing.box).contains(bounds), "\(mark)")
            }
        }
    }
}

/// The way back, and where the app opens again.
final class PlaceTests: XCTestCase {
    private let day = CivilDate(year: 2026, month: 3, day: 15)

    func testPagesRoundTripThroughSavedState() {
        let pages: [Page] = [.home(day), .hour(day, "vespers"), .ordo(year: 2026, month: 3, day: 15), .year(2027), .reminders]
        for page in pages { XCTAssertEqual(Page.decode(page.encoded), page) }
        XCTAssertNil(Page.decode("hour 2026-03-15 matins"))
        XCTAssertNil(Page.decode("ordo 2026 13 1"))
    }

    func testAPageOfTheSameKindReplacesTheOneShown() {
        let model = AppModel()
        model.path = []
        model.open(.hour(day, "lauds"))
        model.open(.hour(day, "prime"))
        XCTAssertEqual(model.path, [.hour(day, "prime")])
        model.open(.ordo(year: 2026, month: 3, day: 0))
        XCTAssertEqual(model.path.count, 2)
        model.open(.home(day.adding(days: 1)))
        XCTAssertEqual(model.path.count, 3)
        let saved = model.saved
        let again = AppModel()
        again.restore(saved)
        XCTAssertEqual(again.path, model.path)
        XCTAssertEqual(again.root, model.root)
        model.goHome()
        XCTAssertTrue(model.path.isEmpty)
    }

    func testCivilDatesStepAcrossMonths() {
        XCTAssertEqual(CivilDate(year: 2026, month: 2, day: 28).adding(days: 1), CivilDate(year: 2026, month: 3, day: 1))
        XCTAssertEqual(CivilDate(year: 2026, month: 3, day: 15).weekday, 1)
        XCTAssertEqual(CivilDate.parse("2026-03-15"), day)
    }
}

/// The reminders iOS is asked to keep.
final class ReminderTests: XCTestCase {
    private func settings(_ chosen: [String: (Int, Int)], days: Set<Int> = Set(1...7), lead: Int = 10) -> ReminderSettings {
        let hours = reminderDefaults().map { d -> HourReminder in
            let t = chosen[d.hour]
            return HourReminder(hour: d.hour, name: d.name, chosen: t != nil, hourOfDay: t?.0 ?? Int(d.hourOfDay), minute: t?.1 ?? Int(d.minute))
        }
        return ReminderSettings(on: true, hours: hours, days: days, lead: lead)
    }

    private let calendar: Calendar = {
        var c = Calendar(identifier: .gregorian)
        c.timeZone = TimeZone(identifier: "America/New_York")!
        return c
    }()

    func testEachReminderRingsItsLeadBeforeTheOffice() throws {
        let now = try XCTUnwrap(calendar.date(from: DateComponents(year: 2026, month: 3, day: 15, hour: 6, minute: 0)))
        let plan = try ReminderScheduler.plan(settings(["lauds": (7, 0), "vespers": (17, 30)]), now: now, core: Office.core.get(), calendar: calendar)
        let first = try XCTUnwrap(plan.first)
        XCTAssertEqual(first.hour, "lauds")
        XCTAssertEqual(first.at.hour, 6)
        XCTAssertEqual(first.at.minute, 50)
        XCTAssertEqual(first.title, "Lauds")
        XCTAssertFalse(first.feast.isEmpty)
        XCTAssertEqual(plan[1].hour, "vespers")
        XCTAssertEqual(plan[1].at.hour, 17)
        XCTAssertEqual(plan[1].at.minute, 20)
        XCTAssertEqual(Set(plan.map(\.id)).count, plan.count)
        let content = ReminderScheduler.content(first)
        XCTAssertEqual(content.categoryIdentifier, ReminderScheduler.category)
        XCTAssertEqual(content.userInfo["hour"] as? String, "lauds")
        XCTAssertEqual(content.userInfo["date"] as? String, "2026-03-15")
        XCTAssertNotNil(Bundle.main.url(forResource: "bell", withExtension: "caf"))
    }

    func testTheScheduleStopsAtIOSsLimit() throws {
        let now = try XCTUnwrap(calendar.date(from: DateComponents(year: 2026, month: 3, day: 15, hour: 0, minute: 0)))
        let all = Dictionary(uniqueKeysWithValues: hourNames().enumerated().map { ($0.element, (6 + 2 * $0.offset, 0)) })
        let plan = try ReminderScheduler.plan(settings(all), now: now, core: Office.core.get(), calendar: calendar)
        XCTAssertEqual(plan.count, ReminderScheduler.capacity)
        XCTAssertEqual(plan.map(\.fireDate), plan.map(\.fireDate).sorted())
    }

    func testOnlyTheChosenDaysAndNothingWhenOff() throws {
        let now = try XCTUnwrap(calendar.date(from: DateComponents(year: 2026, month: 3, day: 15, hour: 0, minute: 0)))
        let sundays = try ReminderScheduler.plan(settings(["vespers": (18, 0)], days: [1]), now: now, core: Office.core.get(), calendar: calendar)
        XCTAssertEqual(sundays.count, 4)
        XCTAssertTrue(sundays.allSatisfy { $0.date.weekday == 1 })
        var off = settings(["vespers": (18, 0)])
        off.on = false
        XCTAssertTrue(try ReminderScheduler.plan(off, now: now, core: Office.core.get(), calendar: calendar).isEmpty)
    }
}

/// The daily usage beacon: what counts, once a day, under an identifier that lasts the day.
final class UsageTests: XCTestCase {
    private let suite = "usage-tests"
    private var posted: [(id: String, body: String)] = []
    private var online = true
    // 15 March 2026, mid-morning in New York.
    private var now = Date(timeIntervalSince1970: 1_773_586_800)
    private let today = CivilDate(year: 2026, month: 3, day: 15)

    override func setUp() {
        UserDefaults().removePersistentDomain(forName: suite)
        posted = []
        online = true
    }

    private func usage() -> Usage {
        Usage(enabled: true, defaults: UserDefaults(suiteName: suite)!, now: { [unowned self] in self.now }) { [unowned self] id, body, done in
            if self.online { self.posted.append((id, body)) }
            done(self.online)
        }
    }

    func testEachPageCountsOnceADay() {
        let usage = usage()
        usage.record(.home(date: today), dark: false, form: "private")
        usage.record(.home(date: today), dark: false, form: "private")
        usage.record(.hour(date: today, hour: "vespers"), dark: true, form: "priest")
        usage.record(.remindersOn, dark: true, form: "priest")
        XCTAssertEqual(posted.map { $0.body }, [
            "site appearance:nave screen:mobile client:ios",
            "vespers appearance:apse screen:mobile prayer-form:priest client:ios",
            "reminders appearance:apse screen:mobile client:ios",
        ])
        XCTAssertEqual(Set(posted.map { $0.id }).count, 1)
        XCTAssertNotNil(posted[0].id.range(of: "^[0-9a-f]{32}$", options: .regularExpression))
    }

    func testTheArchiveIsNotCounted() {
        let usage = usage()
        usage.record(.hour(date: CivilDate(year: 2019, month: 3, day: 4), hour: "lauds"), dark: false, form: "private")
        usage.record(.ordo(year: 2031), dark: false, form: "private")
        usage.record(Page.year(2026).usageEvent, dark: false, form: "private")
        XCTAssertEqual(posted.map { $0.body }, ["ordo appearance:nave screen:mobile client:ios"])
    }

    func testTheIdentifierLastsOneReportingDay() {
        usage().record(.home(date: today), dark: false, form: "private")
        // Half past midnight in New York: a new identifier, and home counts again.
        now = Date(timeIntervalSince1970: 1_773_635_400)
        XCTAssertEqual(Usage.reportingDay(now), "2026-03-16")
        let later = usage()
        later.record(.home(date: today), dark: false, form: "private")
        later.record(.hour(date: today, hour: "compline"), dark: false, form: "private")
        XCTAssertEqual(posted.count, 3)
        XCTAssertNotEqual(posted[0].id, posted[1].id)
        XCTAssertEqual(posted[1].id, posted[2].id)
    }

    func testAFailedBeaconIsTriedAgainOnTheNextVisit() {
        let usage = usage()
        online = false
        usage.record(.home(date: today), dark: false, form: "private")
        online = true
        usage.record(.home(date: today), dark: false, form: "private")
        XCTAssertEqual(posted.count, 1)
    }

    func testDebugBuildsNeverReport() {
        XCTAssertNotEqual(Bundle.main.object(forInfoDictionaryKey: "OfficeCountsUsage") as? String, "YES")
    }
}
