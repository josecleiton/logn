import SwiftUI
import LogN
import App

/// Ranking de celular — tela 6 do DS.
///
/// Abas Global / Sede, linhas densas, e a linha do jogador grudada na base com o mesmo
/// tratamento accent do telão: fundo accent @14%, bordas accent em cima e embaixo, rank
/// e sublinha em `accentInk`.
struct StandingsView: View {
    @EnvironmentObject var core: CoreWrapper
    @State private var tab: Tab = .global

    enum Tab { case global, home }

    private var rows: [LogN.StandingRow] {
        tab == .global ? core.viewModel.standingsGlobal : core.viewModel.standingsHome
    }

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                header

                ScrollView {
                    LazyVStack(spacing: 0) {
                        ForEach(rows, id: \.handle) { row in
                            StandingRowView(row: row)
                        }
                        scoreboardLink
                    }
                }

                // A linha do usuário não rola com a lista: ela fica sempre à vista.
                StandingRowView(row: core.viewModel.userStanding)
            }
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("Standings")
                .font(.plexSansSemiBold(22, relativeTo: .title2))
                .tracking(-0.02 * 22)
                .foregroundColor(LognDark.textPrimary)
                .padding(.bottom, 14)

            HStack(spacing: 20) {
                tabButton("Global", .global)
                tabButton(core.viewModel.userStanding.university, .home)
                Spacer(minLength: 0)
            }
            .overlay(alignment: .bottom) {
                Rectangle().frame(height: 1).foregroundColor(LognDark.line)
            }
        }
        .padding(.horizontal, 20)
        .padding(.top, 18)
    }

    /// Aba ativa: borda inferior 2dp `accent`, 14sp/600.
    private func tabButton(_ title: String, _ value: Tab) -> some View {
        let isActive = tab == value
        return Button { tab = value } label: {
            Text(title)
                .font(isActive ? .plexSansSemiBold(14) : .plexSans(14))
                .foregroundColor(isActive ? LognDark.textPrimary : LognDark.textMuted)
                .padding(.vertical, 10)
                .padding(.horizontal, 2)
                .overlay(alignment: .bottom) {
                    if isActive {
                        Rectangle()
                            .frame(height: 2)
                            .foregroundColor(LognDark.accent)
                            .offset(y: 1)
                    }
                }
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(isActive ? [.isSelected] : [])
    }

    private var scoreboardLink: some View {
        NavigationLink(destination: ScoreboardView().environmentObject(core)) {
            Text("VER O TELÃO COMPLETO")
                .lognLabel()
                .foregroundColor(LognDark.accentInk)
                .frame(maxWidth: .infinity)
                .padding(.vertical, Space.md)
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
        }
        .padding(.horizontal, 20)
        .padding(.vertical, Space.lg)
    }
}

/// Linha 44dp: rank mono 13sp largura 26 · handle 15sp/500 + universidade em label ·
/// `solved · penalty` mono 14sp tabular à direita.
struct StandingRowView: View {
    let row: LogN.StandingRow

    var body: some View {
        HStack(spacing: 14) {
            Text("\(row.rank)")
                .font(.plexMono(13))
                .monospacedDigit()
                .foregroundColor(row.isUser ? LognDark.accentInk : LognDark.textMuted)
                .frame(width: 26, alignment: .leading)

            VStack(alignment: .leading, spacing: 2) {
                Text(row.handle)
                    .font(row.isUser ? .plexSansSemiBold(15) : .plexSansMedium(15))
                    .foregroundColor(LognDark.textPrimary)
                    .lineLimit(1)

                Text(subtitle)
                    .font(.plexMono(11))
                    .foregroundColor(row.isUser ? LognDark.accentInk : LognDark.textMuted)
                    .lineLimit(1)
            }

            Spacer(minLength: 0)

            Text("\(row.solved) · \(row.penalty)")
                .font(row.isUser ? .plexMonoSemiBold(14) : .plexMono(14))
                .monospacedDigit()
                .foregroundColor(LognDark.textPrimary)
        }
        .padding(.horizontal, 20)
        .padding(.vertical, row.isUser ? 16 : 14)
        .background(row.isUser ? LognDark.accentTint : Color.clear)
        .overlay(alignment: .top) {
            if row.isUser {
                Rectangle().frame(height: 1).foregroundColor(LognDark.accent)
            }
        }
        .overlay(alignment: .bottom) {
            Rectangle()
                .frame(height: 1)
                .foregroundColor(row.isUser ? LognDark.accent : LognDark.rowLine)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(
            "\(row.isUser ? "sua posição, " : "")\(row.rank)º, \(row.handle), \(row.university), "
            + "\(row.solved) aceitos, \(row.penalty) de penalidade"
        )
    }

    private var subtitle: String {
        row.note.isEmpty ? row.university : "\(row.university) · \(row.note)"
    }
}
