import SwiftUI
import UIKit
import UserNotifications

struct RemindersPage: View {
    var body: some View {
        RemindersScreen()
            .background(PlasterWall())
            .toolbar(.hidden, for: .navigationBar)
    }
}

/**
 * The reminders page, set as the web's: the hours with their times, the days, how long before;
 * then, where the web gives a calendar link, the switch that asks the phone itself.
 */
struct RemindersScreen: View {
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.scenePhase) private var phase
    @Environment(\.openURL) private var openURL
    @State private var settings = ReminderStore.load()
    @State private var permission = ReminderPermission.unknown
    @State private var editing: HourReminder?

    var body: some View {
        let text = Scale.body
        ScrollView {
            VStack(spacing: 0) {
                SiteHeader()
                VStack(alignment: .leading, spacing: 0) {
                    PlainHeadpiece()
                    Text("Set prayer reminders").type(text.sized(26, line: 32)).foregroundStyle(p.text).padding(.top, m.px(10)).accessibilityAddTraits(.isHeader)
                    Text("Choose the hours you pray and their times. Each reminder names the office and the feast of the day. Your schedule stays on this phone, and reminders come without a connection.")
                        .type(text).foregroundStyle(p.muted).padding(.top, m.px(10))
                    Fieldset(legend: "Hours") {
                        ForEach(Array(settings.hours.enumerated()), id: \.offset) { i, h in
                            if i > 0 { Hairline(color: p.border) }
                            hourRow(h)
                        }
                    }
                    .padding(.top, m.px(28))
                    Fieldset(legend: "Days") {
                        FlowRow(spacing: m.px(18)) {
                            ForEach(reminderWeek, id: \.self) { d in
                                let on = settings.days.contains(d)
                                Button {
                                    var s = settings
                                    if on { s.days.remove(d) } else { s.days.insert(d) }
                                    change(s)
                                } label: {
                                    HStack(spacing: m.px(8)) {
                                        Tick(checked: on)
                                        Text(DateFormatter.english.shortWeekdaySymbols[d - 1]).type(text).foregroundStyle(p.text)
                                    }
                                    .frame(minHeight: 44)
                                }
                                .buttonStyle(Quiet())
                                .accessibilityLabel(DateFormatter.english.weekdaySymbols[d - 1])
                                .accessibilityAddTraits(on ? .isSelected : [])
                            }
                        }
                    }
                    .padding(.top, m.px(28))
                    Fieldset(legend: "Remind me") {
                        Menu {
                            ForEach(reminderLeads, id: \.minutes) { lead in
                                Button(lead.label) {
                                    var s = settings
                                    s.lead = lead.minutes
                                    change(s)
                                }
                            }
                        } label: {
                            HStack(spacing: 0) {
                                Text(reminderLeads.first { $0.minutes == settings.lead }?.label ?? "").type(TextStyle(size: 20, line: 32, lining: true)).foregroundStyle(p.text)
                                Spacer(minLength: 8)
                                Caret(open: false)
                            }
                            .padding(.horizontal, m.px(12))
                            .padding(.vertical, m.px(8))
                            .frame(width: m.px(220))
                            .background(p.bg)
                            .overlay(Rectangle().stroke(p.border, lineWidth: 1))
                        }
                        .accessibilityLabel("Remind me")
                        .accessibilityValue(reminderLeads.first { $0.minutes == settings.lead }?.label ?? "")
                    }
                    .padding(.top, m.px(28))
                    DoubleRule().padding(.top, m.px(28))
                    Text("On this phone").type(text.sized(22, line: 28)).foregroundStyle(p.text).padding(.top, m.px(16)).accessibilityAddTraits(.isHeader)
                    phoneSwitch
                }
                .measured(m)
                .padding(.top, m.px(24))
                Footer()
            }
        }
        .onAppear(perform: readPermission)
        .onChange(of: phase) { _, now in if now == .active { readPermission() } }
        .sheet(item: Binding(get: { editing.map(Editing.init) }, set: { editing = $0?.hour })) { e in
            TimeSheet(hour: e.hour) { h in
                editing = nil
                change(settings.with(h))
            }
            .presentationDetents([.height(340)])
            .environment(\.palette, p)
            .environment(\.metrics, m)
        }
    }

    @ViewBuilder private var phoneSwitch: some View {
        if settings.on {
            let idle = !settings.hours.contains(where: \.chosen) || settings.days.isEmpty
            Text(idle ? "Reminders are on, but no hour or day is chosen." : "Reminders are on.").type(Scale.body).foregroundStyle(p.muted).padding(.top, m.px(8))
            Button { turn(on: false) } label: {
                Text("Turn off reminders").type(Scale.body.sized(16, line: 22)).foregroundStyle(p.accent)
                    .padding(.horizontal, m.px(16)).frame(minHeight: 44)
                    .overlay(Rectangle().stroke(p.border, lineWidth: 1))
            }
            .buttonStyle(Quiet())
            .padding(.top, m.px(12))
            if permission == .denied {
                Note(text: "Notifications are off for the Divine Office, so reminders cannot appear.", action: "Allow notifications") {
                    if let url = URL(string: UIApplication.openNotificationSettingsURLString) { openURL(url) }
                }
            }
        } else {
            Button { turn(on: true) } label: {
                Text("Turn on reminders").type(Scale.body.sized(17, line: 24)).foregroundStyle(p.bg)
                    .frame(maxWidth: .infinity, minHeight: 46)
                    .background(p.accent)
            }
            .buttonStyle(Quiet())
            .padding(.top, m.px(12))
        }
    }

    private func hourRow(_ h: HourReminder) -> some View {
        HStack(spacing: 0) {
            // The box and the hour's name are one checkbox; its time is a control of its own.
            Button {
                var changed = h
                changed.chosen.toggle()
                change(settings.with(changed))
            } label: {
                HStack(spacing: m.px(12)) {
                    Tick(checked: h.chosen)
                    Text(h.name).type(Scale.body).foregroundStyle(p.text)
                    Spacer(minLength: 0)
                }
                .frame(minHeight: 46)
            }
            .buttonStyle(Quiet())
            .accessibilityAddTraits(h.chosen ? .isSelected : [])
            let label = clock(h)
            if h.chosen {
                Button { editing = h } label: {
                    Text(label).type(TextStyle(size: 20, line: 32, lining: true)).foregroundStyle(p.text)
                        .padding(.horizontal, m.px(10)).padding(.vertical, m.px(5))
                        .background(p.bg)
                        .overlay(Rectangle().stroke(p.border, lineWidth: 1))
                }
                .buttonStyle(Quiet())
                .accessibilityLabel("\(h.name) at \(label)")
                .accessibilityHint("Changes the time")
            } else {
                Text(label).type(TextStyle(size: 20, line: 32, lining: true)).foregroundStyle(p.muted.opacity(0.6)).padding(.horizontal, m.px(11))
                    .accessibilityHidden(true)
            }
        }
    }

    /// A time as the phone shows it: "7:00 AM", or "07:00" on a 24-hour clock.
    private func clock(_ h: HourReminder) -> String {
        let date = civil.date(from: DateComponents(year: 2026, month: 1, day: 1, hour: h.hourOfDay, minute: h.minute)) ?? Date()
        return date.formatted(date: .omitted, time: .shortened)
    }

    /// Saves the page's choices and, when reminders are on, reschedules at once.
    private func change(_ s: ReminderSettings) {
        settings = s
        ReminderStore.save(s)
        ReminderScheduler.sync()
    }

    /// Turning reminders on asks for notifications first; on a refusal they are still kept, and the page says how to let them through.
    private func turn(on: Bool) {
        var s = settings
        s.on = on
        change(s)
        guard on else { return }
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { _, _ in
            DispatchQueue.main.async { readPermission() }
        }
    }

    private func readPermission() {
        UNUserNotificationCenter.current().getNotificationSettings { s in
            let now: ReminderPermission
            switch s.authorizationStatus {
            case .authorized, .provisional, .ephemeral: now = .allowed
            case .denied: now = .denied
            default: now = .unknown
            }
            DispatchQueue.main.async { permission = now }
        }
    }
}

/// The hour whose time is being changed, as a sheet's item.
private struct Editing: Identifiable {
    let hour: HourReminder
    var id: String { hour.hour }
}

/// The time of one hour's reminder, set on the wheels.
private struct TimeSheet: View {
    let hour: HourReminder
    let done: (HourReminder) -> Void
    @Environment(\.dismiss) private var dismiss
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @State private var time: Date

    init(hour: HourReminder, done: @escaping (HourReminder) -> Void) {
        self.hour = hour
        self.done = done
        _time = State(initialValue: Calendar.current.date(from: DateComponents(year: 2026, month: 1, day: 1, hour: hour.hourOfDay, minute: hour.minute)) ?? Date())
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Button("Cancel") { dismiss() }.foregroundStyle(p.muted)
                Spacer()
                Text(hour.name).type(Scale.body).foregroundStyle(p.text)
                Spacer()
                Button("Set") {
                    var h = hour
                    let parts = Calendar.current.dateComponents([.hour, .minute], from: time)
                    h.hourOfDay = parts.hour ?? h.hourOfDay
                    h.minute = parts.minute ?? h.minute
                    done(h)
                }
                .foregroundStyle(p.accent)
            }
            .font(Font(garamond(18 * m.type)))
            .padding(.horizontal, 20)
            .padding(.top, 18)
            DatePicker(hour.name, selection: $time, displayedComponents: .hourAndMinute)
                .datePickerStyle(.wheel)
                .labelsHidden()
                .tint(p.accent)
        }
        .frame(maxHeight: .infinity, alignment: .top)
        .background(p.surface)
    }
}

/// A bordered group with its legend set into the top rule, as a web fieldset.
private struct Fieldset<Content: View>: View {
    let legend: String
    @ViewBuilder let content: Content
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        ZStack(alignment: .topLeading) {
            VStack(alignment: .leading, spacing: 0) { content }
                .padding(.top, m.px(4))
                .padding(.horizontal, m.px(14))
                .padding(.vertical, m.px(10))
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(p.surface)
                .overlay(Rectangle().stroke(p.border, lineWidth: 1))
                .padding(.top, m.px(8))
            Text(legend.uppercased()).type(TextStyle(size: 10.56, line: 16, tracking: 1.056, smallCaps: true)).foregroundStyle(p.accent)
                .padding(.horizontal, m.px(6))
                .background(p.surface)
                .padding(.leading, m.px(12))
                .accessibilityAddTraits(.isHeader)
                .accessibilitySortPriority(1)
        }
        .accessibilityElement(children: .contain)
    }
}

/// A checkbox in the accent colour, as the web's `accent-color` sets them; the row around it is the control.
private struct Tick: View {
    let checked: Bool
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            let box = Path(roundedRect: CGRect(origin: .zero, size: size).insetBy(dx: 1, dy: 1), cornerRadius: 2)
            if checked {
                ctx.fill(box, with: .color(p.accent))
                var tick = Path()
                tick.move(to: CGPoint(x: size.width * 0.24, y: size.height * 0.52))
                tick.addLine(to: CGPoint(x: size.width * 0.42, y: size.height * 0.70))
                tick.addLine(to: CGPoint(x: size.width * 0.78, y: size.height * 0.32))
                ctx.stroke(tick, with: .color(p.bg), lineWidth: 2)
            } else {
                ctx.stroke(box, with: .color(p.muted), lineWidth: 1.2)
            }
        }
        .frame(width: m.px(20), height: m.px(20))
        .accessibilityHidden(true)
    }
}

/// A note about the phone's permissions, with the way to change them.
private struct Note: View {
    let text: String
    let action: String
    let perform: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(text).type(Scale.body.sized(16, line: 23)).foregroundStyle(p.muted)
            Button(action: perform) {
                Text(action.uppercased()).type(Scale.menu).foregroundStyle(p.accent).goldUnderline(true, p.goldLine).padding(.vertical, m.px(10))
            }
            .buttonStyle(Quiet())
        }
        .padding(.leading, m.px(12))
        .overlay(alignment: .leading) { Rectangle().fill(p.goldLine).frame(width: 1) }
        .padding(.top, m.px(16))
    }
}

/// Items set in lines, as many to a line as fit, as a flex row wraps.
struct FlowRow: Layout {
    var spacing: CGFloat

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let rows = arrange(proposal.width ?? .infinity, subviews)
        let width = rows.map { $0.width }.max() ?? 0
        return CGSize(width: proposal.width ?? width, height: rows.reduce(0) { $0 + $1.height })
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var y = bounds.minY
        for row in arrange(bounds.width, subviews) {
            var x = bounds.minX
            for i in row.items {
                let size = subviews[i].sizeThatFits(.unspecified)
                subviews[i].place(at: CGPoint(x: x, y: y + (row.height - size.height) / 2), proposal: .unspecified)
                x += size.width + spacing
            }
            y += row.height
        }
    }

    private func arrange(_ width: CGFloat, _ subviews: Subviews) -> [(items: [Int], width: CGFloat, height: CGFloat)] {
        var rows: [(items: [Int], width: CGFloat, height: CGFloat)] = []
        var current: (items: [Int], width: CGFloat, height: CGFloat) = ([], 0, 0)
        for (i, view) in subviews.enumerated() {
            let size = view.sizeThatFits(.unspecified)
            let next = current.items.isEmpty ? size.width : current.width + spacing + size.width
            if next > width && !current.items.isEmpty {
                rows.append(current)
                current = ([i], size.width, size.height)
            } else {
                current = (current.items + [i], next, max(current.height, size.height))
            }
        }
        if !current.items.isEmpty { rows.append(current) }
        return rows
    }
}
