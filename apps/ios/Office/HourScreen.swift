import SwiftUI

/// One block in the page's order, with the space the web sets above it.
private struct Item: Identifiable {
    let id: String
    let block: BlockView
    let gap: CGFloat
}

/// An hour of the office: its colour band, framed title, and sections, on the web's measure.
struct HourScreen: View {
    let view: HourView
    let p: Palette
    /// The collapsible sections the reader has opened (the preparation, closed by default).
    @State private var open: Set<Int> = []

    var body: some View {
        ScrollView {
            VStack(spacing: 0) {
                Rectangle().fill(dayColor(view.color)).frame(height: 3)
                title.padding(.top, 17.6)
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(Array(view.sections.enumerated()), id: \.offset) { i, section in
                        if section.collapsible {
                            toggle(i, section)
                        }
                        if !section.collapsible || open.contains(i) {
                            ForEach(items(i, section)) { item in
                                BlockRow(block: item.block, p: p).padding(.top, item.gap)
                            }
                        }
                    }
                }
                .frame(maxWidth: 608)
                .padding(.horizontal, 16)
                .padding(.bottom, 48)
            }
        }
        .background(p.bg)
    }

    private var title: some View {
        VStack(spacing: 2) {
            Text(view.title.uppercased()).style(Scale.hourTitle).foregroundStyle(p.text)
            Rectangle().fill(p.goldLine).frame(width: 240, height: 1)
            Text([view.dateLabel, view.feast, view.seasonLabel].filter { !$0.isEmpty }.joined(separator: "\u{00a0}· "))
                .style(Scale.meta)
                .foregroundStyle(p.muted)
                .multilineTextAlignment(.center)
                .padding(.top, 4)
        }
        .padding(.horizontal, 16)
        .padding(.bottom, 12)
    }

    private func toggle(_ i: Int, _ section: SectionView) -> some View {
        Button {
            if open.contains(i) { open.remove(i) } else { open.insert(i) }
        } label: {
            HStack(spacing: 4) {
                Text(section.label).style(Scale.heading).foregroundStyle(p.text)
                Text(open.contains(i) ? "▴" : "▾").font(.system(size: 9)).foregroundStyle(p.goldLine)
            }
            .frame(maxWidth: .infinity, minHeight: 44)
        }
        .buttonStyle(.plain)
        .padding(.top, 5.6)
    }

    /// The section's blocks, each with its gap after the one before it.
    private func items(_ i: Int, _ section: SectionView) -> [Item] {
        var prev: BlockView?
        return section.blocks.enumerated().map { j, block in
            let gap = prev == nil ? (block.kind == .heading ? 12 : 0) : gapBefore(prev, block)
            prev = block
            return Item(id: "\(i)-\(j)", block: block, gap: gap)
        }
    }
}
