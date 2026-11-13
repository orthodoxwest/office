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
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @Environment(\.scenePhase) private var phase
    @Environment(\.openURL) private var openURL
    @Environment(\.accessibilityReduceMotion) private var still
    @State private var settings = ReminderStore.load()
    @State private var permission = ReminderPermission.unknown
    @State private var editing: HourReminder?
    /// The reminder that will really come next, from the scheduler's own plan; nil while none will.
    @State private var next: String?
    /// When `next` goes stale, and a count of its readings, so each reading sets the timer afresh.
    @State private var stale: (at: Date, read: Int)?
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
                        toggled(follow)
                        var s = settings
                        s.followSun = follow
                        // Back to the reader's own times, the place is forgotten.
                        if !follow { s.place = nil }
                        // The sun's paragraph unfolds, as a disclosure does, rather than jumping in.
                        withAnimation(still ? nil : unfolding) { change(s) }
                        if follow { location.refresh() }
                    } allowLocation: {
                        if let url = URL(string: UIApplication.openSettingsURLString) { openURL(url) }
                    }
                    .padding(.top, m.px(28))
                    Band(name: "Hours") {
                        // The hours in home's three periods, each beside its painted sun or moon, as
                        // home and About set them; the periods parted by the table's rule.
                        let periods = reminderPeriods()
                        // Where the name and its time would crowd (large text), the period keeps its sun
                        // or moon and gives up its word, as the web's does.
                        let compact = m.type > 1.3
                        ForEach(Array(periods.enumerated()), id: \.offset) { i, period in
                            if i > 0 { Hairline(color: tableRule) }
                            HStack(spacing: 0) {
                                VStack(spacing: m.px(2.56)) {
                                    PeriodIcon(period: Period.named(period.label), color: o.flat)
                                    if !compact {
                                        Text(period.label).type(TextStyle(size: 11.52, line: 13.8, tracking: 11.52 * 0.08, smallCaps: true))
                                            .foregroundStyle(p.muted).lineLimit(1).fixedSize()
                                    }
                                }
                                .frame(width: compact ? 32 : m.px(70.4))
                                .frame(maxHeight: .infinity)
                                .accessibilityHidden(true)
                                Rectangle().fill(tableRule).frame(width: 1).accessibilityHidden(true)
                                VStack(alignment: .leading, spacing: 0) {
                                    ForEach(Array(period.hours.enumerated()), id: \.offset) { j, slug in
                                        if let h = settings.hours.first(where: { $0.hour == slug }) {
                                            if j > 0 { Hairline(color: p.border) }
                                            hourRow(h, sun: sun?[h.hour])
                                        }
                                    }
                                }
                                .padding(.leading, m.px(9.6))
                            }
                            .fixedSize(horizontal: false, vertical: true)
                            .accessibilityElement(children: .contain)
                            .accessibilityLabel(period.label)
                        }
                    }
                    .padding(.top, m.px(28))
                    Band(name: "Days") {
                        // One ruled week, as the date picker sets its week: a chosen day's name in ink
                        // over the gold rule of a chosen thing, an unchosen day's muted with none, Sunday
                        // in rubric; the hand-set box beneath is still the control's sign.
                        HStack(spacing: 0) {
                            ForEach(Array(reminderWeek.enumerated()), id: \.offset) { i, d in
                                let on = settings.days.contains(d)
                                let sunday = d == 1
                                Button {
                                    toggled(!on)
                                    var s = settings
                                    if on { s.days.remove(d) } else { s.days.insert(d) }
                                    change(s)
                                } label: {
                                    VStack(spacing: m.px(6)) {
                                        Text(DateFormatter.english.shortWeekdaySymbols[d - 1]).type(text.sized(15.2, line: 20))
                                            .foregroundStyle(sunday ? (on ? p.rubric : p.rubric.opacity(0.6)) : (on ? p.text : p.muted))
                                            .padding(.bottom, m.px(3))
                                            .frame(maxWidth: .infinity)
                                            .overlay(alignment: .bottom) { Rectangle().fill(p.goldLine).frame(height: 1).opacity(on ? 1 : 0) }
                                        Tick(checked: on)
                                    }
                                    .padding(.top, m.px(8)).padding(.bottom, m.px(8))
                                    .frame(maxWidth: .infinity, minHeight: 44)
                                    .overlay(alignment: .leading) {
                                        if i > 0 { Rectangle().fill(p.border.opacity(0.58)).frame(width: 1) }
                                    }
                                    .animation(tickMotion(on, still: still), value: on)
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
                                Caret(open: false, color: p.goldLine, share: 1)
                            }
                            // A worded line with the app's caret, not a boxed field.
                            .padding(.leading, m.px(2)).padding(.trailing, m.px(4))
                            .frame(maxWidth: .infinity, minHeight: 46)
                            .contentShape(Rectangle())
                        }
                        .accessibilityLabel("Remind me")
                        .accessibilityValue(reminderLeads.first { $0.minutes == settings.lead }?.label ?? "")
                    }
                    .padding(.top, m.px(28))
                    ruleLines(sun: sun).padding(.top, m.px(18))
                    // The web's "Your calendar link": a titulus over the tablet, no rule between.
                    Text("On this phone").type(TextStyle(size: 13.12, line: 21, tracking: 1.57, smallCaps: true)).foregroundStyle(p.titulus)
                        .frame(maxWidth: .infinity).padding(.top, m.px(32)).accessibilityAddTraits(.isHeader)
                    phoneSwitch
                }
                .measured(m)
                .padding(.top, m.px(24))
                Footer()
            }
        }
        .onAppear(perform: readPermission)
        // A place read on coming back to the app is saved apart from the page; read it again.
        .onChange(of: location.revision) { _, _ in
            settings = ReminderStore.load()
            readNext()
        }
        .onChange(of: phase) { _, now in if now == .active { readPermission() } }
        // Read again while the page stays open as that reminder comes or the day turns (#808);
        // the wait ends with the page.
        .task(id: stale?.read) {
            guard let at = stale?.at else { return }
            try? await Task.sleep(for: .seconds(max(1, at.timeIntervalSinceNow)))
            if !Task.isCancelled { readNext() }
        }
        .sheet(item: Binding(get: { editing.map(Editing.init) }, set: { editing = $0?.hour })) { e in
            TimeSheet(hour: e.hour) { h in
                editing = nil
                change(settings.with(h))
            }
            .presentationDetents([.height(340)])
            .environment(\.palette, p)
            .environment(\.ornament, o)
            .environment(\.metrics, m)
        }
    }

    /// Home's table rule, between the hours' periods (the web's `--frontispiece-rule`).
    private var tableRule: Color {
        p.dark ? Color(rgb: 208, 176, 106, 0.24) : Color(rgb: 107, 58, 31, 0.22)
    }

    /// The rule, written out (`presentation::reminders`): what the choices ask for, in one
    /// sentence, and under it the reminder that will really come next. Two lines are kept, so a
    /// change of a day or a time moves nothing below; the sentence cross-fades as it changes.
    private func ruleLines(sun: [String: (hour: Int, minute: Int)]?) -> some View {
        let chosen = settings.hours.filter(\.chosen).map { h in
            RuleHourChoice(hour: h.hour, time: clock(h.hourOfDay, h.minute), bySun: sun != nil && h.hour != "compline")
        }
        let phrases = reminderRule(hours: chosen, weekdays: settings.weekdays, lead: Int32(settings.lead))
        let said = phrases.map(\.text).joined()
        let empty = phrases.first?.key == "empty"
        let style = Scale.body.sized(18, line: 27)
        return VStack(spacing: m.px(4)) {
            Text(said).type(style).foregroundStyle(empty ? p.muted : p.text)
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity)
                .frame(minHeight: 2 * 27 * m.type, alignment: .top)
                .contentTransition(.opacity)
                .animation(still ? nil : .easeInOut(duration: 0.18), value: said)
            if let next, settings.on, !empty {
                Text(next).type(Scale.body.sized(16, line: 23)).foregroundStyle(p.muted)
                    .multilineTextAlignment(.center)
                    .frame(maxWidth: .infinity)
                    .contentTransition(.opacity)
                    .transition(.opacity)
            }
        }
        .animation(still ? nil : .easeInOut(duration: 0.18), value: next)
    }

    /// Reads the next reminder from the scheduler's own plan, off the main thread: "Next: Vespers,
    /// today at 5:50 PM." None while reminders are off or notifications are refused (the note says so).
    private func readNext() {
        let s = settings
        guard s.on, permission != .denied else {
            next = nil
            stale = nil
            return
        }
        DispatchQueue.global(qos: .userInitiated).async {
            let now = Date()
            let first = (try? Office.core.get()).flatMap { try? ReminderScheduler.plan(s, now: now, core: $0).first }
            let line = first.map { r -> String in
                let day = { (d: Date) -> CivilDate in
                    let c = Calendar.current.dateComponents([.year, .month, .day], from: d)
                    return CivilDate(year: Int32(c.year ?? 2026), month: Int32(c.month ?? 1), day: Int32(c.day ?? 1))
                }
                return reminderNext(hour: r.hour, today: day(now), on: day(r.fireDate), time: r.fireDate.formatted(date: .omitted, time: .shortened))
            }
            let until = ReminderScheduler.nextLineStale(now: now, next: first?.fireDate)
            DispatchQueue.main.async {
                next = line
                stale = (until, (stale?.read ?? 0) + 1)
            }
        }
    }

    /// The tablet stays where it is: its words change in place to "Reminders are on", and the way
    /// to turn them off unfolds beneath it as a worded link, not a boxed button.
    @ViewBuilder private var phoneSwitch: some View {
        TabletButton(label: settings.on ? "Reminders are on" : "Turn on reminders") {
            if !settings.on { turn(on: true) }
        }
        .contentTransition(.opacity)
        .accessibilityRemoveTraits(settings.on ? .isButton : [])
        .padding(.top, m.px(12))
        if settings.on {
            VStack(spacing: 0) {
                Button { turn(on: false) } label: {
                    Text("Turn off reminders").type(Scale.body.sized(16, line: 22)).foregroundStyle(p.accent)
                        .goldUnderline(true, p.goldLine)
                        .frame(minHeight: 44)
                }
                .buttonStyle(Quiet())
                .frame(maxWidth: .infinity)
                .padding(.top, m.px(6))
                if permission == .denied {
                    Note(text: "Notifications are off for the Daily Office, so reminders cannot appear.", action: "Allow notifications") {
                        if let url = URL(string: UIApplication.openNotificationSettingsURLString) { openURL(url) }
                    }
                }
            }
            .transition(.unfold)
        }
    }

    private func hourRow(_ h: HourReminder, sun: (hour: Int, minute: Int)?) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 0) {
                // The box and the hour's name are one checkbox; its time is a control of its own.
                Button {
                    toggled(!h.chosen)
                    var changed = h
                    changed.chosen.toggle()
                    change(settings.with(changed))
                } label: {
                    HStack(spacing: m.px(12)) {
                        Tick(checked: h.chosen)
                        // An hour not chosen stands back in the muted ink (the web's `.reminder-hour-name`).
                        Text(h.name).type(Scale.body).foregroundStyle(h.chosen ? p.text : p.muted)
                            .animation(tickMotion(h.chosen, still: still), value: h.chosen)
                        Spacer(minLength: 0)
                    }
                    .frame(minHeight: 46)
                }
                .buttonStyle(Quiet())
                .accessibilityAddTraits(h.chosen ? .isSelected : [])
                if let sun, h.hour != "compline" {
                    let time = SunTime(name: h.name, hour: h.hour, label: clock(sun.hour, sun.minute), chosen: h.chosen)
                    if h.chosen {
                        time
                    } else {
                        // A tap on an unchosen hour's faint time chooses it; the box beside it is the
                        // way in for a screen reader.
                        Button { choose(h) } label: { time }
                            .buttonStyle(Quiet())
                            .accessibilityHidden(true)
                    }
                } else {
                    let label = clock(h.hourOfDay, h.minute)
                    // The accent's figures on the chosen control's gold rule, with the caret that says
                    // the time can be changed (the web's time field). The figures, the rule and the caret
                    // answer the tick together; an unchosen hour's faint time chooses the hour when tapped.
                    Button { if h.chosen { editing = h } else { choose(h) } } label: {
                        HStack(spacing: 0) {
                            Text(label).type(TextStyle(size: 20, line: 32, lining: true)).foregroundStyle(h.chosen ? p.accent : p.muted.opacity(0.55))
                            Caret(open: false, color: p.goldLine, share: 1).opacity(h.chosen ? 1 : 0)
                        }
                        .padding(.horizontal, m.px(2)).padding(.vertical, m.px(3))
                        .overlay(alignment: .bottom) {
                            Rectangle().fill(p.goldLine).frame(height: 1).scaleEffect(x: h.chosen ? 1 : 0.001, anchor: .leading)
                        }
                        .frame(minHeight: 44)
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(Quiet())
                    .animation(tickMotion(h.chosen, still: still), value: h.chosen)
                    .accessibilityLabel("\(h.name) at \(label)")
                    .accessibilityHint("Changes the time")
                    // Unchosen, the checkbox beside it is the way in for a screen reader.
                    .accessibilityHidden(!h.chosen)
                }
            }
            // Compline keeps the reader's time while the hours follow the sun, but waits for the evening.
            if let sun, h.chosen && h.hour == "compline" {
                let moved = (sun.hour, sun.minute) != (h.hourOfDay, h.minute)
                Text(moved ? "Tonight at \(clock(sun.hour, sun.minute)), an hour after sunset." : "Your time, moved later when the sun sets late.").type(Scale.body.sized(14, line: 20)).foregroundStyle(p.muted)
                    .padding(.leading, m.px(32)).padding(.bottom, m.px(8))
            }
        }
    }

    /// Chooses an hour, from its faint time as from its box.
    private func choose(_ h: HourReminder) {
        toggled(true)
        var changed = h
        changed.chosen = true
        change(settings.with(changed))
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
        readNext()
    }

    /// Turning reminders on asks for notifications first; on a refusal they are still kept, and the page says how to let them through.
    private func turn(on: Bool) {
        // Turning reminders on counts as the web counts a generated feed link.
        if on && !settings.on { Usage.shared.record(.remindersOn, dark: p.dark, form: AppModel.shared.form) }
        var s = settings
        s.on = on
        // What turning on adds below the tablet unfolds, as a disclosure does.
        withAnimation(still ? nil : unfolding) { change(s) }
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
            DispatchQueue.main.async {
                permission = now
                readNext()
            }
        }
    }
}

/// The hour whose time is being changed, as a sheet's item.
private struct Editing: Identifiable {
    let hour: HourReminder
    var id: String { hour.hour }
}

/// The time of one hour's reminder, set on the platform's wheels in the church's frame: the
/// panel's painted line across the top, the hour's name as a titulus beside its period's sun or
/// moon, "Cancel" muted and "Set" in the accent on the gold rule, and by night the menu's Apse
/// plaster under it. The wheels are the system's own.
private struct TimeSheet: View {
    let hour: HourReminder
    let done: (HourReminder) -> Void
    @Environment(\.dismiss) private var dismiss
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @State private var time: Date

    init(hour: HourReminder, done: @escaping (HourReminder) -> Void) {
        self.hour = hour
        self.done = done
        _time = State(initialValue: Calendar.current.date(from: DateComponents(year: 2026, month: 1, day: 1, hour: hour.hourOfDay, minute: hour.minute)) ?? Date())
    }

    var body: some View {
        let words = Scale.body.sized(18, line: 24)
        let period = reminderPeriods().first { $0.hours.contains(hour.hour) }.map { Period.named($0.label) } ?? .day
        VStack(spacing: 0) {
            HStack(spacing: 0) {
                Button { dismiss() } label: {
                    Text("Cancel").type(words).foregroundStyle(p.muted).frame(minHeight: 44)
                }
                .buttonStyle(Quiet())
                Spacer(minLength: 8)
                HStack(spacing: m.px(6)) {
                    PeriodIcon(period: period, color: o.flat)
                    Text(hour.name).type(TextStyle(size: 13.12, line: 21, tracking: 1.57, smallCaps: true)).foregroundStyle(p.titulus)
                }
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(hour.name)
                .accessibilityAddTraits(.isHeader)
                Spacer(minLength: 8)
                Button {
                    var h = hour
                    let parts = Calendar.current.dateComponents([.hour, .minute], from: time)
                    h.hourOfDay = parts.hour ?? h.hourOfDay
                    h.minute = parts.minute ?? h.minute
                    done(h)
                } label: {
                    Text("Set").type(words).foregroundStyle(p.accent).goldUnderline(true, p.goldLine).frame(minHeight: 44)
                }
                .buttonStyle(Quiet())
            }
            .padding(.horizontal, 20)
            .padding(.top, 12)
            DatePicker(hour.name, selection: $time, displayedComponents: .hourAndMinute)
                .datePickerStyle(.wheel)
                .labelsHidden()
                .tint(p.accent)
        }
        .frame(maxHeight: .infinity, alignment: .top)
        .background {
            // By night the sheet takes the menu's Apse plaster, not the flat surface.
            if p.dark, let plaster = UIImage(named: p.panel) {
                Image(uiImage: plaster).resizable().scaledToFill()
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
                    .clipped()
                    .ignoresSafeArea()
            } else {
                p.surface.ignoresSafeArea()
            }
        }
        .overlay(alignment: .top) { Rectangle().fill(o.line).frame(height: 2) }
    }
}

/// A group of the page's controls under its name, ruled as home's hours are and as the web's
/// Reminders fieldsets: the name in the titulus's small caps over a rule of the lining, the rows
/// on the page itself, parted by hairlines, and a closing rule. A screen reader takes the name as
/// the group's heading.
private struct Band<Content: View>: View {
    let name: String
    @ViewBuilder let content: Content
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let rule = p.lining.opacity(0.6)
        VStack(alignment: .leading, spacing: 0) {
            Text(name).type(TextStyle(size: 13.12, line: 21, tracking: 1.57, smallCaps: true)).foregroundStyle(p.titulus)
                .padding(.bottom, m.px(5))
                .frame(maxWidth: .infinity, alignment: .leading)
                .overlay(alignment: .bottom) { Rectangle().fill(rule).frame(height: 1) }
                .accessibilityAddTraits(.isHeader)
                .accessibilitySortPriority(1)
            VStack(alignment: .leading, spacing: 0) { content }
                .frame(maxWidth: .infinity, alignment: .leading)
                .overlay(alignment: .bottom) { Rectangle().fill(rule).frame(height: 1) }
        }
        .accessibilityElement(children: .contain)
    }
}

/// What marks each hour's time by the sun, under the time.
private let sunMarks = [
    "lauds": "sunrise", "prime": "first hour", "terce": "third hour",
    "sext": "solar noon", "none": "ninth hour", "vespers": "sunset",
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
        // Past the limit the reader's own times are in force, whatever was chosen; the choice is
        // kept so the sun takes over again when the app is next opened nearer the equator.
        let bySun = settings.followSun && !unavailable
        Band(name: "Times") {
            choice("At times I choose", selected: !bySun, enabled: true, sun: false)
            Hairline(color: p.border)
            choice("By the sun where I am", selected: bySun, enabled: !unavailable, sun: true)
            if unavailable {
                Remark(text: sunUnavailableNote())
            } else if settings.followSun {
                Text("Lauds at sunrise, Vespers at sunset, and the little hours at the first, third, sixth and ninth hours of daylight. Times for where you were when you last opened the app.")
                    .type(Scale.body.sized(16, line: 23)).foregroundStyle(p.muted)
                    .padding(.top, m.px(2)).padding(.bottom, m.px(10))
                    .transition(.unfold)
            }
            if settings.followSun && locationDenied {
                Note(text: "Allow location to time reminders by the sun.", action: "Open settings", perform: allowLocation)
                    .padding(.bottom, m.px(6))
                    .transition(.unfold)
            }
        }
    }

    private func choice(_ label: String, selected: Bool, enabled: Bool, sun: Bool) -> some View {
        Button {
            if !selected { choose(sun) }
        } label: {
            HStack(spacing: m.px(12)) {
                RadioMark(selected: selected)
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
private struct RadioMark: View {
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
 * around it is the control. The oak square grows from the box's centre as the name takes its ink,
 * in the web's 140ms (120ms back), and is simply there under Reduce Motion.
 */
private struct Tick: View {
    let checked: Bool
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.accessibilityReduceMotion) private var still

    var body: some View {
        let side = m.px(15.2)
        ZStack {
            Rectangle().fill(p.bg).frame(width: side, height: side)
            Rectangle().fill(p.accent).frame(width: side - 8, height: side - 8).scaleEffect(checked ? 1 : 0.001)
            RoundedRectangle(cornerRadius: 1).strokeBorder(checked ? p.accent : p.muted, lineWidth: 1).frame(width: side, height: side)
        }
        .frame(width: m.px(20), height: m.px(20))
        .animation(tickMotion(checked, still: still), value: checked)
        .accessibilityHidden(true)
    }
}

/// How a tick answers, on every front as the web's: 140ms out to the chosen state, 120ms back.
private func tickMotion(_ on: Bool, still: Bool) -> Animation? {
    still ? nil : (on ? .easeOut(duration: 0.14) : .easeIn(duration: 0.12))
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
