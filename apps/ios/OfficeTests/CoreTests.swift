import XCTest
@testable import Office

/// The Rust core, linked into the app for iPhone: its embedded corpus loads and composes.
final class CoreTests: XCTestCase {
    func testComposesAnHourFromTheEmbeddedCorpus() throws {
        let core = try OfficeCore()
        let vespers = try core.compose(hour: "vespers", year: 2026, month: 3, day: 15, form: "private")
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
        let core = try OfficeCore()
        let blocks = try core.compose(hour: "lauds", year: 2026, month: 3, day: 15, form: "private").sections.flatMap(\.blocks)
        let heading = try XCTUnwrap(blocks.indices.dropFirst().first { blocks[$0].kind == .heading })
        XCTAssertEqual(gapBefore(blocks[heading - 1], blocks[heading]), 38)
    }
}
