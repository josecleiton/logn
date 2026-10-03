import SwiftUI
import LogNCoreFFI
import LogN
import App

/// O nome do placar: o apelido, ou "jogador #N" do catálogo.
func leaderboardName(anonNumber: Int32, nickname: String?) -> String {
    nickname ?? Str.Leaderboard.anon_name(String(anonNumber))
}

/// O texto de 14 do placar tem altura de linha 21 no canvas. `lineSpacing` soma à
/// entrelinha natural da Plex Sans 14 (perto de 18), não a substitui: são 3.
let bodyLineSpacing: CGFloat = 3

/// XP com o separador de milhar da língua do app ("4.850").
func leaderboardXp(_ xp: Int32) -> String {
    let format = NumberFormatter()
    format.numberStyle = .decimal
    format.locale = Locale(identifier: AppLocale.current)
    return format.string(from: NSNumber(value: xp)) ?? "\(xp)"
}

/// O placar geral de XP (docs/specs/logn_placar_spec.md), no canvas "LogN — Placar geral
/// de XP": a Global, sem abas, com a linha do jogador fixa embaixo quando ela sai de vista.
struct LeaderboardScreen: View {
    @EnvironmentObject var core: CoreWrapper
    let onGoToTrail: () -> Void

    /// A linha do jogador está na tela. iOS 16 não tem `onScrollVisibilityChange`.
    @State private var meVisible = false

    private var board: LeaderboardView { core.viewModel.leaderboard }

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            VStack(spacing: 0) {
                header
                content
            }
        }
        .onAppear { core.dispatch(event: .leaderboardOpened) }
    }

    @ViewBuilder
    private var content: some View {
        switch board.state {
        case .signedOut:
            guestState
        case .loading:
            skeleton
        case .unavailable:
            unavailableState
        case .closed:
            if board.offline { offlineBanner }
            closedState
            pinnedRow
        case .open:
            if board.offline { offlineBanner }
            openList
            if board.meStatus == .zeroXp {
                zeroXpFooter
            } else if !board.meInRows || !meVisible {
                pinnedRow
            }
        }
    }

    // MARK: Cabeçalho

    private var header: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(Str.Leaderboard.title)
                .font(.plexSansSemiBold(22, relativeTo: .title2))
                .tracking(-0.02 * 22)
                .foregroundColor(LognDark.textPrimary)
                .accessibilityAddTraits(.isHeader)
            Text(Str.Leaderboard.subtitle)
                .font(.plexMono(11))
                .tracking(0.1 * 11)
                .foregroundColor(LognDark.textMuted)
                .padding(.bottom, 12)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, Space.screenMargin)
        .padding(.top, 18)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    private var offlineBanner: some View {
        HStack(spacing: Space.sm) {
            Image(systemName: "wifi.slash")
                .font(.system(size: 12, weight: .semibold))
            Text(Str.Leaderboard.offline(ageText))
                .font(.plexSans(12.5))
            Spacer(minLength: 0)
        }
        .foregroundColor(LognDark.infoInk)
        .padding(.horizontal, Space.screenMargin)
        .padding(.vertical, 10)
        .background(LognDark.tintInfo)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.info)
        }
    }

    private var ageText: String {
        let n = Int(board.ageValue)
        switch board.ageUnit {
        case .justNow: return Str.Leaderboard.age_now
        case .minutes: return Str.Leaderboard.age_minutes(n)
        case .hours:   return Str.Leaderboard.age_hours(n)
        case .days:    return Str.Leaderboard.age_days(n)
        }
    }

    // MARK: Lista

    private var openList: some View {
        ScrollView {
            LazyVStack(spacing: 0) {
                ForEach(board.rows, id: \.rank) { row in
                    rowView(row)
                }
            }
        }
        .refreshable { core.dispatch(event: .leaderboardRefresh) }
    }

    @ViewBuilder
    private func rowView(_ row: LeaderboardRow) -> some View {
        if row.isMe {
            // Na lista, a linha do jogador já tem o tratamento da fixa.
            LeaderboardMeRow(board: board)
                .onAppear { meVisible = true }
                .onDisappear { meVisible = false }
        } else {
            LeaderboardRowView(row: row)
        }
    }

    private var pinnedRow: some View {
        LeaderboardMeRow(board: board)
    }

    // MARK: Fechado

    private var players: Int {
        max(0, Int(board.threshold) - Int(board.missing))
    }

    private var closedBody: String {
        if board.meStatus == .waiting {
            return Str.Leaderboard.closed_body + " " + Str.Leaderboard.closed_counted
        }
        return Str.Leaderboard.closed_body
    }

    private var closedState: some View {
        VStack(alignment: .leading, spacing: 22) {
            Spacer(minLength: 0)
            Text(Str.Leaderboard.closed_eyebrow)
                .font(.plexMono(11))
                .tracking(0.1 * 11)
                .foregroundColor(LognDark.textMuted)
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text(String(players))
                    .font(.plexMonoSemiBold(48))
                    .foregroundColor(LognDark.textPrimary)
                Text(Str.Leaderboard.closed_of(Int(board.threshold)))
                    .font(.plexMono(20))
                    .foregroundColor(LognDark.textMuted)
            }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(Str.Leaderboard.closed_accessibility(players, Int(board.threshold)))
            closedBar
            VStack(alignment: .leading, spacing: 8) {
                Text(Str.Leaderboard.closed_missing(Int(board.missing)))
                    .font(.plexSansSemiBold(18))
                    .foregroundColor(LognDark.textPrimary)
                Text(closedBody)
                    .font(.plexSans(14))
                    .lineSpacing(bodyLineSpacing)
                    .foregroundColor(LognDark.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: 0)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 32)
    }

    private var closedBar: some View {
        HStack(spacing: 4) {
            ForEach(0..<Int(board.threshold), id: \.self) { i in
                RoundedRectangle(cornerRadius: 1)
                    .fill(segmentColor(i))
                    .frame(height: 6)
            }
        }
    }

    private func segmentColor(_ index: Int) -> Color {
        index < players ? LognDark.accent : LognDark.line
    }

    // MARK: Sem XP

    private var zeroXpFooter: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 10) {
                Text(leaderboardName(anonNumber: board.me.anonNumber, nickname: board.me.nickname))
                    .font(nameFont(board.me.nickname, bold: true))
                    .foregroundColor(LognDark.textPrimary)
                Text(Str.Leaderboard.you + " · " + Str.Leaderboard.xp("0"))
                    .font(.plexMono(11))
                    .tracking(0.1 * 11)
                    .foregroundColor(LognDark.accentInk)
            }
            Text(Str.Leaderboard.zero_xp)
                .font(.plexSans(14))
                .lineSpacing(bodyLineSpacing)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
            Button {
                core.dispatch(event: .selectTrack(trackId: ""))
                onGoToTrail()
            } label: {
                Text(Str.Leaderboard.zero_xp_action)
                    .font(.plexSansSemiBold(14))
                    .foregroundColor(LognDark.accentInk)
                    .frame(maxWidth: .infinity, minHeight: 44)
                    .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.accent, lineWidth: 1))
            }
            .buttonStyle(.plain)
        }
        .padding(.horizontal, Space.screenMargin)
        .padding(.top, 16)
        .padding(.bottom, 20)
        .background(LognDark.surface)
        .overlay(alignment: .top) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    // MARK: Visitante, sem conexão, carregando

    private var guestState: some View {
        VStack(alignment: .leading, spacing: 20) {
            Spacer(minLength: 0)
            Circle()
                .fill(LognDark.surface)
                .frame(width: 52, height: 52)
                .overlay(Circle().strokeBorder(LognDark.lineDim, style: StrokeStyle(lineWidth: 1, dash: [3])))
                .overlay(
                    Image(systemName: "person")
                        .font(.system(size: 20, weight: .regular))
                        .foregroundColor(LognDark.textMuted)
                )
            stateText(Str.Leaderboard.guest_title, Str.Leaderboard.guest_body)
            VStack(spacing: 10) {
                Button {
                    core.wantsRegistration = true
                    core.dispatch(event: .logout)
                } label: {
                    Text(Str.Leaderboard.guest_create)
                        .font(.plexSansSemiBold(15))
                        .foregroundColor(LognDark.onAccent)
                        .frame(maxWidth: .infinity, minHeight: 48)
                        .background(LognDark.accent)
                        .cornerRadius(Radius.sm)
                }
                .buttonStyle(.plain)
                outlineButton(Str.Leaderboard.guest_sign_in, height: 48, size: 15) {
                    core.dispatch(event: .logout)
                }
            }
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 32)
    }

    private var unavailableState: some View {
        VStack(alignment: .leading, spacing: 20) {
            Spacer(minLength: 0)
            Image(systemName: "wifi.slash")
                .font(.system(size: 24, weight: .regular))
                .foregroundColor(LognDark.textMuted)
            stateText(Str.Leaderboard.unavailable_title, Str.Leaderboard.unavailable_body)
            outlineButton(Str.Dashboard.try_again, height: 44, size: 14) {
                core.dispatch(event: .leaderboardRefresh)
            }
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 32)
    }

    private var skeleton: some View {
        VStack(spacing: 0) {
            ForEach(Self.skeletonWidths, id: \.self) { width in
                HStack(spacing: 14) {
                    RoundedRectangle(cornerRadius: 2).fill(LognDark.rowLine).frame(width: 18, height: 10)
                    RoundedRectangle(cornerRadius: 2).fill(LognDark.surface).frame(width: width, height: 12)
                    Spacer(minLength: 0)
                    RoundedRectangle(cornerRadius: 2).fill(LognDark.surface).frame(width: 58, height: 12)
                }
                .padding(.horizontal, Space.screenMargin)
                .frame(minHeight: 44)
                .padding(.vertical, 1)
                .overlay(alignment: .bottom) {
                    Rectangle().frame(height: 1).foregroundColor(LognDark.rowLine)
                }
            }
            Spacer(minLength: 0)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(Str.Leaderboard.loading_accessibility)
    }

    private static let skeletonWidths: [CGFloat] = [110, 132, 84, 120, 96, 140, 88, 118, 104, 126, 92, 114]

    private func stateText(_ title: String, _ body: String) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.plexSansSemiBold(20))
                .foregroundColor(LognDark.textPrimary)
                .fixedSize(horizontal: false, vertical: true)
            Text(body)
                .font(.plexSans(14))
                .lineSpacing(bodyLineSpacing)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private func outlineButton(_ title: String, height: CGFloat, size: CGFloat, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(.plexSansMedium(size))
                .foregroundColor(LognDark.textPrimary)
                .frame(maxWidth: .infinity, minHeight: height)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
        }
        .buttonStyle(.plain)
    }
}

/// A fonte do nome: o apelido em Sans, o anônimo em mono, para não parecer erro.
func nameFont(_ nickname: String?, bold: Bool) -> Font {
    if nickname != nil {
        return bold ? .plexSansSemiBold(15) : .plexSansMedium(15)
    }
    return bold ? .plexMonoSemiBold(14) : .plexMono(14)
}

/// Uma linha: posição, nome e XP.
struct LeaderboardRowView: View {
    let row: LeaderboardRow

    private var name: String { leaderboardName(anonNumber: row.anonNumber, nickname: row.nickname) }
    private var xp: String { leaderboardXp(row.xp) }

    var body: some View {
        HStack(spacing: 14) {
            Text("\(Int(row.rank))")
                .font(.plexMono(13))
                .monospacedDigit()
                .foregroundColor(LognDark.textMuted)
                .frame(width: 26, alignment: .leading)
            Text(name)
                .font(nameFont(row.nickname, bold: false))
                .foregroundColor(row.nickname != nil ? LognDark.textPrimary : LognDark.textSecondary)
                .lineLimit(1)
            Spacer(minLength: 0)
            Text(Str.Leaderboard.xp(xp))
                .font(.plexMono(14))
                .monospacedDigit()
                .foregroundColor(LognDark.textPrimary)
        }
        .padding(.horizontal, Space.screenMargin)
        .padding(.vertical, 13)
        .frame(minHeight: 44)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.rowLine)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(Str.Leaderboard.row_accessibility(Int(row.rank), name, xp))
    }
}

/// A linha do jogador: fundo em acento, fio de acento em cima e embaixo, e "VOCÊ".
struct LeaderboardMeRow: View {
    let board: LeaderboardView

    private var me: LeaderboardRow { board.me }
    private var ranked: Bool { board.meStatus == .ranked && me.rank > 0 }
    private var name: String { leaderboardName(anonNumber: me.anonNumber, nickname: me.nickname) }
    private var xp: String { leaderboardXp(me.xp) }

    private var label: String {
        if ranked {
            return Str.Leaderboard.row_accessibility_me(Int(me.rank), name, xp)
        }
        return Str.Leaderboard.me_unranked_accessibility(name, xp)
    }

    var body: some View {
        HStack(spacing: 14) {
            Text(ranked ? "\(Int(me.rank))" : Str.Leaderboard.rank_none)
                .font(.plexMono(13))
                .monospacedDigit()
                .foregroundColor(LognDark.accentInk)
                .frame(width: 26, alignment: .leading)
            VStack(alignment: .leading, spacing: 2) {
                Text(name)
                    .font(nameFont(me.nickname, bold: true))
                    .foregroundColor(LognDark.textPrimary)
                    .lineLimit(1)
                Text(board.meStatus == .hidden ? Str.Leaderboard.hidden_label : Str.Leaderboard.you)
                    .font(.plexMono(11))
                    .tracking(0.1 * 11)
                    .foregroundColor(LognDark.accentInk)
            }
            Spacer(minLength: 0)
            Text(Str.Leaderboard.xp(xp))
                .font(.plexMonoSemiBold(14))
                .monospacedDigit()
                .foregroundColor(LognDark.textPrimary)
        }
        .padding(.horizontal, Space.screenMargin)
        .padding(.vertical, 16)
        .background(LognDark.accentTint)
        .overlay(alignment: .top) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.accent)
        }
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.accent)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(label)
    }
}
