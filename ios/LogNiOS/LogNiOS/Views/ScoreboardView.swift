import SwiftUI
import LogN
import App

/// Telão do ginásio — tela 1 do DS, e a única que não é phone-first.
///
/// O grid do documento (`52 | 190 | 56 | 68 | 13 × 44`) é mais largo que o aparelho, então
/// as colunas de identidade ficam **congeladas à esquerda** enquanto A—M rolam. É um desvio
/// consciente: sem isso você vê uma célula verde e não sabe de quem é.
struct ScoreboardView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) private var dismiss

    private enum Col {
        static let rank: CGFloat = 52
        static let team: CGFloat = 150
        static let solved: CGFloat = 56
        static let penalty: CGFloat = 68
        static let cell: CGFloat = 44
        static var frozen: CGFloat { rank + team }
    }

    private var rows: [LogN.ScoreboardRow] { core.viewModel.scoreboard }
    private var isFrozen: Bool { core.viewModel.matchView.isFrozen }

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                contestBar

                ScrollView(.vertical) {
                    HStack(spacing: 0) {
                        // Identidade: fixa.
                        VStack(spacing: 0) {
                            identityHeader
                            ForEach(rows, id: \.rank) { IdentityCell(row: $0) }
                        }
                        .frame(width: Col.frozen)
                        .overlay(alignment: .trailing) {
                            Rectangle().frame(width: 1).foregroundColor(LognDark.line)
                        }

                        // Números e as 13 células: rolam na horizontal.
                        ScrollView(.horizontal, showsIndicators: true) {
                            VStack(spacing: 0) {
                                problemHeader
                                ForEach(rows, id: \.rank) { ScoreRowCells(row: $0) }
                            }
                        }
                    }
                }

                legend
            }
        }
        .navigationBarHidden(true)
    }

    // MARK: Barra do contest

    private var contestBar: some View {
        HStack(spacing: 18) {
            Button { dismiss() } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundColor(LognDark.textMuted)
            }
            .accessibilityLabel("Fechar")

            Text(core.viewModel.contestName)
                .font(.plexMonoSemiBold(13))
                .tracking(0.12 * 13)
                .foregroundColor(LognDark.textPrimary)
                .lineLimit(1)

            if isFrozen {
                Text("CONGELADO")
                    .font(.plexMono(11))
                    .tracking(0.1 * 11)
                    .foregroundColor(LognDark.warnInk)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 5)
                    .background(LognDark.tintWarn)
                    .cornerRadius(Radius.xs)
                    .overlay(RoundedRectangle(cornerRadius: Radius.xs).stroke(LognDark.warn, lineWidth: 1))
            }

            Spacer(minLength: 0)

            // Fora de partida não há relógio de contest: melhor não mostrar nada do
            // que mostrar 00:00, que o jogador lê como "acabou".
            if core.viewModel.matchView.contestSeconds > 0 {
                Text(ContestClock.format(Int(core.viewModel.matchView.contestSeconds)))
                    .font(.plexMonoMedium(24))
                    .monospacedDigit()
                    .tracking(-0.01 * 24)
                    .foregroundColor(LognDark.textPrimary)
            }
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 16)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    // MARK: Cabeçalhos

    private var identityHeader: some View {
        HStack(spacing: 0) {
            Text("#")
                .frame(width: Col.rank, alignment: .leading)
                .padding(.leading, 20)
            Text("EQUIPE")
                .frame(width: Col.team - 20, alignment: .leading)
        }
        .font(.plexMono(10))
        .tracking(0.12 * 10)
        .foregroundColor(LognDark.textMuted)
        .frame(height: 40)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LognDark.surfaceRaised)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    private var problemHeader: some View {
        HStack(spacing: 0) {
            Text("SLV").frame(width: Col.solved)
            Text("PEN").frame(width: Col.penalty)

            ForEach(BalloonColor.all, id: \.self) { letter in
                VStack(spacing: 3) {
                    // Coluna do placar: balão de 15, sem cordinha.
                    BalloonShape(style: .filled(BalloonColor.forLetter(letter)), width: 15)
                    Text(String(letter))
                        .font(.plexMonoSemiBold(11))
                        .foregroundColor(LognDark.textPrimary)
                }
                .frame(width: Col.cell)
            }
        }
        .font(.plexMono(10))
        .tracking(0.12 * 10)
        .foregroundColor(LognDark.textMuted)
        .frame(height: 40)
        .background(LognDark.surfaceRaised)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    // MARK: Legenda

    /// "Legenda das quatro cores no rodapé, sempre visível" — então ela quebra em duas
    /// linhas no celular em vez de rolar, que esconderia metade.
    private var legend: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 20) {
                legendItem(.accepted, "aceito · tentativas / minuto")
                Spacer(minLength: 0)
                legendItem(.failed, "tentado, sem AC")
            }
            HStack(spacing: 20) {
                legendItem(.frozen, "submetido após o congelamento")
                Spacer(minLength: 0)
                legendItem(.untried, "não tentado")
            }
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 14)
        .background(LognDark.surface)
        .overlay(alignment: .top) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    private func legendItem(_ state: LogN.ScoreCellState, _ label: String) -> some View {
        HStack(spacing: 8) {
            RoundedRectangle(cornerRadius: Radius.xs)
                .fill(ScoreCellStyle.background(state))
                .frame(width: 22, height: 14)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.xs)
                        .stroke(ScoreCellStyle.border(state), lineWidth: 1)
                )
            Text(label)
                .font(.plexMono(11))
                .foregroundColor(LognDark.textMuted)
                .fixedSize()
        }
    }
}

// MARK: - Cores das células

/// Os quatro estados. Borda e fundo usam o token de **linha**; o texto usa a **tinta**.
enum ScoreCellStyle {
    static func border(_ state: LogN.ScoreCellState) -> Color {
        switch state {
        case .accepted: return LognDark.correct
        case .failed:   return LognDark.wrong
        case .frozen:   return LognDark.info
        case .untried:  return LognDark.line
        }
    }

    static func background(_ state: LogN.ScoreCellState) -> Color {
        switch state {
        case .accepted: return LognDark.tintOk
        case .failed:   return LognDark.tintErr
        case .frozen:   return LognDark.tintInfo
        case .untried:  return .clear
        }
    }

    static func ink(_ state: LogN.ScoreCellState) -> Color {
        switch state {
        case .accepted: return LognDark.correctInk
        case .failed:   return LognDark.wrongInk
        case .frozen:   return LognDark.infoInk
        case .untried:  return LognDark.textMuted
        }
    }

    static func describe(_ state: LogN.ScoreCellState) -> String {
        switch state {
        case .accepted: return "aceito"
        case .failed:   return "tentado, sem AC"
        case .frozen:   return "submetido após o congelamento"
        case .untried:  return "não tentado"
        }
    }
}

// MARK: - Linhas

private struct IdentityCell: View {
    let row: LogN.ScoreboardRow

    var body: some View {
        HStack(spacing: 0) {
            Text("\(row.rank)")
                .font(.plexMonoSemiBold(14))
                .monospacedDigit()
                .foregroundColor(row.isUser ? LognDark.accentInk : LognDark.textPrimary)
                .frame(width: 52, alignment: .leading)
                .padding(.leading, 20)

            VStack(alignment: .leading, spacing: 2) {
                Text(row.team)
                    .font(.plexSansSemiBold(14))
                    .foregroundColor(row.isUser ? LognDark.accentInk : LognDark.textPrimary)
                    .lineLimit(1)
                Text(row.university)
                    .font(.plexMono(10))
                    .tracking(0.08 * 10)
                    .foregroundColor(LognDark.textMuted)
                    .lineLimit(1)
            }
            .frame(width: 130, alignment: .leading)
            .padding(.trailing, 12)
        }
        .frame(height: 52)
        .background(row.isUser ? LognDark.accentTint : LognDark.surface)
        .overlay(alignment: .bottom) {
            Rectangle()
                .frame(height: 1)
                .foregroundColor(row.isUser ? LognDark.accent : LognDark.rowLine)
        }
    }
}

private struct ScoreRowCells: View {
    let row: LogN.ScoreboardRow

    var body: some View {
        HStack(spacing: 0) {
            Text("\(row.solved)")
                .font(.plexMonoSemiBold(15))
                .monospacedDigit()
                .foregroundColor(LognDark.textPrimary)
                .frame(width: 56)

            Text("\(row.penalty)")
                .font(.plexMono(13))
                .monospacedDigit()
                .foregroundColor(LognDark.textSecondary)
                .frame(width: 68)

            ForEach(Array(row.cells.enumerated()), id: \.offset) { index, cell in
                ScoreCellView(cell: cell, letter: BalloonColor.all[min(index, 12)])
                    .frame(width: 44)
            }
        }
        .frame(height: 52)
        .background(row.isUser ? LognDark.accentTint : LognDark.surface)
        .overlay(alignment: .bottom) {
            Rectangle()
                .frame(height: 1)
                .foregroundColor(row.isUser ? LognDark.accent : LognDark.rowLine)
        }
    }
}

/// Célula 38dp de altura, margem lateral 2dp, raio 2dp.
/// Topo = símbolo 12sp/600; base = minuto 9sp a 75%.
private struct ScoreCellView: View {
    let cell: LogN.ScoreCell
    let letter: Character

    var body: some View {
        VStack(spacing: 1) {
            if !cell.top.isEmpty {
                Text(cell.top)
                    .font(.plexMonoSemiBold(12))
                    .monospacedDigit()
                    .foregroundColor(ScoreCellStyle.ink(cell.state))
            }
            if !cell.bottom.isEmpty {
                Text(cell.bottom)
                    .font(.plexMono(9))
                    .monospacedDigit()
                    .foregroundColor(ScoreCellStyle.ink(cell.state).opacity(0.75))
            }
        }
        .frame(maxWidth: .infinity)
        .frame(height: 38)
        .background(ScoreCellStyle.background(cell.state))
        .cornerRadius(Radius.xs)
        .overlay(
            RoundedRectangle(cornerRadius: Radius.xs)
                .stroke(ScoreCellStyle.border(cell.state), lineWidth: 1)
        )
        .padding(.horizontal, 2)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("problema \(letter), \(ScoreCellStyle.describe(cell.state))")
    }
}
