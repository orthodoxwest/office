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
    @ObservedObject private var location = PhoneLocation.shared

    var body: some View {
        let text = Scale.body
        // Today's times by the sun, while the hours follow it.
        let sun = (try? Office.core.get()).flatMap { settings.sunTimes(on: Date(), core: $0) }
        ScrollView {
            VStack(spacing: 0) {
                SiteHeader()
                VStack(alignment: .leading, spacing: 0) {
                    PlainHeadpiece()
                    Text("Set prayer reminders").type(text.sized(26, line: 32)).foregroundStyle(p.text).padding(.top, m.px(10)).accessibilityAddTraits(.isHeader)
                    Text("Choose the hours you pray and their times. Each reminder names the office and the feast of the day. Your schedule stays on this phone, and reminders come without a connection.")
                        .type(text).foregroundStyle(p.muted).padding(.top, m.px(10))
                    TimesGroup(settings: settings, locationDenied: location.denied) { follow in
                        toggled()
                        var s = settings
                        s.followSun = follow
                        // Back to the reader's own times, the place is forgotten.
                        if !follow { s.place = nil }
                        change(s)
                        if follow { location.refresh() }
                    } allowLocation: {
                        if let url = URL(string: UIApplication.openSettingsURLString) { openURL(url) }
                    }
                    .padding(.top, m.px(28))
                    Band(name: "Hours") {
                        ForEach(Array(settings.hours.enumerated()), id: \.offset) { i, h in
                            if i > 0 { Hairline(color: p.border) }
                            hourRow(h, sun: sun?[h.hour])
                        }
                    }
                    .padding(.top, m.px(28))
                    Band(name: "Days") {
                        FlowRow(spacing: m.px(18)) {
                            ForEach(reminderWeek, id: \.self) { d in
                                let on = settings.days.contains(d)
                                Button {
                                    toggled()
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
                    Band(name: "Remind me") {
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
                                Caret(open: false, color: p.text)
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
        // A place read on coming back to the app is saved apart from the page; read it again.
        .onChange(of: location.revision) { _, _ in settings = ReminderStore.load() }
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
                Note(text: "Notifications are off for the Daily Office, so reminders cannot appear.", action: "Allow notifications") {
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

    private func hourRow(_ h: HourReminder, sun: (hour: Int, minute: Int)?) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 0) {
                // The box and the hour's name are one checkbox; its time is a control of its own.
                Button {
                    toggled()
                    var changed = h
                    changed.chosen.toggle()
                    change(settings.with(changed))
                } label: {
                    HStack(spacing: m.px(12)) {
                        Tick(checked: h.chosen)
                        // An hour not chosen stands back in the muted ink (the web's `.reminder-hour-name`).
                        Text(h.name).type(Scale.body).foregroundStyle(h.chosen ? p.text : p.muted)
                            .animation(.easeInOut(duration: 0.14), value: h.chosen)
                        Spacer(minLength: 0)
                    }
                    .frame(minHeight: 46)
                }
                .buttonStyle(Quiet())
                .accessibilityAddTraits(h.chosen ? .isSelected : [])
                if let sun, h.hour != "compline" {
                    SunTime(name: h.name, hour: h.hour, label: clock(sun.hour, sun.minute), chosen: h.chosen)
                } else {
                    let label = clock(h.hourOfDay, h.minute)
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
            // Compline keeps the reader's time while the hours follow the sun, but waits for the evening.
            if sun != nil && h.chosen && h.hour == "compline" {
                Text("Your time, moved later when the sun sets late.").type(Scale.body.sized(14, line: 20)).foregroundStyle(p.muted)
                    .padding(.leading, m.px(32)).padding(.bottom, m.px(8))
            }
        }
    }

    /// A time as the phone shows it: "7:00 AM", or "07:00" on a 24-hour clock.
    private func clock(_ hour: Int, _ minute: Int) -> String {
        let date = civil.date(from: DateComponents(year: 2026, month: 1, day: 1, hour: hour, minute: minute)) ?? Date()
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
        // Turning reminders on counts as the web counts a generated feed link.
        if on && !settings.on { Usage.shared.record(.remindersOn, dark: p.dark, form: AppModel.shared.form) }
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

/**
 * A group of the page's controls under its name, set as home's "Pray the hours" band: the name
 * in small caps between gilt lozenges, a gold fillet above and below, and the rows on the page
 * itself beneath. A screen reader takes the name as the group's heading.
 */
private struct Band<Content: View>: View {
    let name: String
    @ViewBuilder let content: Content
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Home's band is gilt on its green earth; on the bare page the gold reads where the gilt would fade.
            Lozenged(text: name, style: TextStyle(size: 11, line: 17.6, tracking: 1.76, smallCaps: true), ink: p.gold)
                .padding(.vertical, m.px(7))
                .frame(maxWidth: .infinity)
                .overlay(alignment: .top) { Rectangle().fill(p.goldLine).frame(height: 1) }
                .overlay(alignment: .bottom) { Rectangle().fill(p.goldLine).frame(height: 1) }
                .accessibilityElement(children: .combine)
                .accessibilityAddTraits(.isHeader)
                .accessibilitySortPriority(1)
            VStack(alignment: .leading, spacing: 0) { content }
                .padding(.top, m.px(6))
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .accessibilityElement(children: .contain)
    }
}

/// What marks each hour's time by the sun, under the time.
private let sunMarks = [
    "lauds": "sunrise", "prime": "first hour", "terce": "third hour",
    "sext": "noon by the sun", "none": "ninth hour", "vespers": "sunset",
]

/**
 * Whether the hours keep the reader's times or follow the sun, the reason the sun cannot be
 * followed here, and the way to let the app know where it is. Kept apart from the page around it,
 * to be restyled on its own. Past the latitude limit the sun's row is greyed and cannot be chosen;
 * choosing the reader's own times forgets the place, so the sun can be chosen again elsewhere.
 */
private struct TimesGroup: View {
    let settings: ReminderSettings
    let locationDenied: Bool
    let choose: (Bool) -> Void
    let allowLocation: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let unavailable = settings.place.map { !$0.sunAvailable } ?? false
        Band(name: "Times") {
            choice("At times I choose", selected: !settings.followSun, enabled: true, sun: false)
            Hairline(color: p.border)
            choice("By the sun where I am", selected: settings.followSun, enabled: !unavailable, sun: true)
            if unavailable {
                Remark(text: sunUnavailableNote())
            } else if settings.followSun {
                Text("Lauds at sunrise, Vespers at sunset, and the little hours at the first, third, sixth and ninth hours of daylight. Times for where you were when you last opened the app.")
                    .type(Scale.body.sized(16, line: 23)).foregroundStyle(p.muted)
                    .padding(.top, m.px(2)).padding(.bottom, m.px(10))
            }
            if settings.followSun && locationDenied {
                Note(text: "Allow location to time reminders by the sun.", action: "Open settings", perform: allowLocation)
                    .padding(.bottom, m.px(6))
            }
            Hairline(color: p.border)
        }
    }

    private func choice(_ label: String, selected: Bool, enabled: Bool, sun: Bool) -> some View {
        Button {
            if !selected { choose(sun) }
        } label: {
            HStack(spacing: m.px(12)) {
                Mark(selected: selected)
                Text(label).type(Scale.body).foregroundStyle(p.text)
                Spacer(minLength: 0)
            }
            .frame(minHeight: 46)
        }
        .buttonStyle(Quiet())
        .disabled(!enabled)
        .opacity(enabled ? 1 : 0.5)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}

/// An hour's time as the sun sets it today: plain, with what marks it beneath; faint when not chosen.
private struct SunTime: View {
    let name: String
    let hour: String
    let label: String
    let chosen: Bool
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        if chosen {
            let mark = sunMarks[hour] ?? ""
            VStack(alignment: .trailing, spacing: 0) {
                Text(label).type(TextStyle(size: 20, line: 32, lining: true)).foregroundStyle(p.text)
                Text(mark).type(TextStyle(size: 11, line: 14, tracking: 0.88, smallCaps: true)).foregroundStyle(p.gold)
            }
            .padding(.horizontal, m.px(11))
            .padding(.vertical, m.px(4))
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("\(name) at \(label), \(mark)")
        } else {
            Text(label).type(TextStyle(size: 20, line: 32, lining: true)).foregroundStyle(p.muted.opacity(0.6)).padding(.horizontal, m.px(11))
                .accessibilityHidden(true)
        }
    }
}

/// The radio mark, the round sister of `Tick`: a ruled circle, an oak dot inside a margin of the ground when chosen.
private struct Mark: View {
    let selected: Bool
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let side = m.px(15.2)
        Canvas { ctx, size in
            let rect = CGRect(x: (size.width - side) / 2, y: (size.height - side) / 2, width: side, height: side)
            ctx.fill(Path(ellipseIn: rect), with: .color(p.bg))
            if selected {
                ctx.fill(Path(ellipseIn: rect.insetBy(dx: 1 + 3, dy: 1 + 3)), with: .color(p.accent))
            }
            ctx.stroke(Path(ellipseIn: rect.insetBy(dx: 0.5, dy: 0.5)), with: .color(selected ? p.accent : p.muted), lineWidth: 1)
        }
        .frame(width: m.px(20), height: m.px(20))
        .accessibilityHidden(true)
    }
}

/// A remark set as a `Note` is, with its gold rule, but with nothing to do about it.
private struct Remark: View {
    let text: String
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Text(text).type(Scale.body.sized(16, line: 23)).foregroundStyle(p.muted)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.leading, m.px(12))
            .overlay(alignment: .leading) { Rectangle().fill(p.goldLine).frame(width: 1) }
            .padding(.top, m.px(6))
            .padding(.bottom, m.px(12))
    }
}

/**
 * A checkbox set by hand, as the web's are: a ruled square, 0.95rem, that fills with oak inside a
 * margin of the ground when chosen. The platform's own box belongs to no theme here. The row
 * around it is the control.
 */
private struct Tick: View {
    let checked: Bool
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let side = m.px(15.2)
        Canvas { ctx, size in
            let rect = CGRect(x: (size.width - side) / 2, y: (size.height - side) / 2, width: side, height: side)
            let box = Path(roundedRect: rect.insetBy(dx: 0.5, dy: 0.5), cornerRadius: 1)
            ctx.fill(Path(rect), with: .color(p.bg))
            if checked {
                ctx.fill(Path(rect.insetBy(dx: 1 + 3, dy: 1 + 3)), with: .color(p.accent))
            }
            ctx.stroke(box, with: .color(checked ? p.accent : p.muted), lineWidth: 1)
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
