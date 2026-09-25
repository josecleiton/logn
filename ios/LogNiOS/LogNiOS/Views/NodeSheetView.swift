import SwiftUI
import LogNCoreFFI
import LogN

/// Sheet do nó — exploração `4b · Nó ativo · vizinhança`.
///
/// Em vez de repetir a descrição do assunto, o sheet mostra a **vizinhança no grafo**:
/// de onde o nó vem e o que ele destrava. É a mesma informação que a topologia da tela
/// já carrega, agora legível em texto.
struct NodeSheetView: View {
    let node: SkillNode
    let incoming: [SkillNode]
    let unlocks: [SkillNode]
    /// Quantos problemas este nó tem. Zero é estado real: a árvore é maior do que o
    /// currículo escrito até agora.
    let problemCount: Int
    let onStartMatch: () -> Void

    @Environment(\.dismiss) var dismiss

    /// Somatório do conteúdo: handle 18+3+18 · cabeçalho 60 · 20 · vizinhança 72 ·
    /// 20 · faixa 62 · 18 · CTA 52 · 22. O sheet é compacto por design — nada de
    /// esticar para preencher tela.
    static let preferredHeight: CGFloat = 365

    /// Linha de problemas: 20 de respiro · rótulo 14 · 10 · balão 36 · 4 · legenda 12.
    static let problemsHeight: CGFloat = 96

    /// O nó sem desafio ganha uma linha de explicação embaixo do botão; o destravado com
    /// desafio ganha a linha de problemas.
    static func preferredHeight(problemCount: Int, isLocked: Bool) -> CGFloat {
        if problemCount == 0 { return preferredHeight + 24 }
        return isLocked ? preferredHeight : preferredHeight + problemsHeight
    }

    private var solvedCount: Int { node.problemsSolved.filter { $0 }.count }
    private var openCount: Int { node.problemsSolved.count - solvedCount }
    private var showsProblems: Bool { node.status != .locked && !node.problemsSolved.isEmpty }

    private var topic: LognTopic { LognTopic.of(node: node) }

    var body: some View {
        ZStack(alignment: .top) {
            LognDark.surfaceRaised.ignoresSafeArea()

            VStack(alignment: .leading, spacing: 0) {
                handle
                header
                neighbourhood.padding(.top, 20)
                if showsProblems {
                    problems.padding(.top, 20)
                }
                statStrip.padding(.top, 20)
                cta.padding(.top, 18)
            }
            .padding(.horizontal, 20)
            .padding(.top, 18)
            .padding(.bottom, 22)
        }
        .overlay(alignment: .top) {
            Rectangle()
                .frame(height: 1)
                .foregroundColor(LognDark.lineStrong)
        }
    }

    // MARK: Handle

    private var handle: some View {
        Capsule()
            .fill(LognDark.lineStrong)
            .frame(width: 36, height: 3)
            .frame(maxWidth: .infinity)
            .padding(.bottom, 18)
    }

    // MARK: Cabeçalho — balão do assunto + nome + estado

    private var header: some View {
        HStack(spacing: 14) {
            ZStack {
                BalloonShape(style: balloonStyle, width: 56)
                TopicIcon(topic: topic, color: iconColor, size: 21, lineWidth: 2.2)
                    .position(x: 28, y: 56 * 0.42)
            }
            .frame(width: 56, height: 56 * 95 / 96)

            VStack(alignment: .leading, spacing: 3) {
                Text(node.name)
                    .font(.plexSansSemiBold(19, relativeTo: .title3))
                    .foregroundColor(LognDark.textPrimary)
                Text(stateLabel)
                    .font(.plexMono(11))
                    .tracking(0.1 * 11)
                    .foregroundColor(stateColor)
            }
            Spacer(minLength: 0)
        }
    }

    private var balloonStyle: BalloonShape.Style {
        switch node.status {
        case .completed: return .filled(topic.color)
        case .active:    return .active
        case .locked:    return .locked
        }
    }

    private var iconColor: Color {
        switch node.status {
        case .completed: return LognDark.onAccent
        case .active:    return LognDark.accentInk
        case .locked:    return LognDark.textSecondary
        }
    }

    private var stateLabel: String {
        switch node.status {
        case .completed: return Str.Node.completed
        case .active:    return Str.Node.active(Int(node.requiredXp))
        case .locked:    return Str.Node.locked
        }
    }

    /// Tinta do rótulo de estado. A cor da família vive no balão, não no texto:
    /// a paleta é certificada para 3:1 como forma, não 4.5:1 como glifo.
    private var stateColor: Color {
        switch node.status {
        case .completed: return LognDark.textSecondary
        case .active:    return LognDark.accentInk
        case .locked:    return LognDark.textMuted
        }
    }

    // MARK: Vizinhança — vem de / destrava

    private var neighbourhood: some View {
        HStack(alignment: .top, spacing: 20) {
            column(title: Str.Node.upstream, nodes: incoming, isUpstream: true)
            column(title: Str.Node.downstream, nodes: unlocks, isUpstream: false)
        }
    }

    private func column(title: String, nodes: [SkillNode], isUpstream: Bool) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(title)
                .font(.plexMono(10))
                .tracking(0.14 * 10)
                .foregroundColor(LognDark.textMuted)

            if nodes.isEmpty {
                // Vazio nunca é ilustração: uma frase e pronto.
                // `textDim` é só forma e placeholder — isto é texto lido.
                Text(isUpstream ? Str.Node.start_point : Str.Node.end_point)
                    .font(.plexSans(13.5, relativeTo: .footnote))
                    .foregroundColor(LognDark.textMuted)
                    .padding(.top, 10)
            } else {
                ForEach(Array(nodes.prefix(2).enumerated()), id: \.element.id) { index, neighbour in
                    HStack(spacing: 9) {
                        TopicIcon(
                            topic: LognTopic.of(node: neighbour),
                            color: isUpstream ? LognTopic.of(node: neighbour).color : LognDark.textMuted,
                            size: 17
                        )
                        Text(neighbour.name)
                            .font(.plexSans(13.5, relativeTo: .footnote))
                            .foregroundColor(isUpstream ? LognDark.textPrimary : LognDark.textSecondary)
                            .lineLimit(1)
                    }
                    .padding(.top, index == 0 ? 10 : 7)
                }
            }
            Spacer(minLength: 0)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    // MARK: Problemas — quais já renderam XP

    /// Um balão por problema, na ordem das letras da partida. Cheio é o que já rendeu XP
    /// e não paga de novo; em contorno é o que ainda vale.
    private var problems: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                Text(Str.Solved.problems)
                    .font(.plexMono(10))
                    .tracking(0.14 * 10)
                    .foregroundColor(LognDark.textMuted)
                Spacer(minLength: 0)
                Text(Str.Solved.count(solvedCount, node.problemsSolved.count))
                    .font(.plexMono(11))
                    .foregroundColor(LognDark.textSecondary)
            }

            HStack(spacing: 10) {
                ForEach(Array(node.problemsSolved.enumerated()), id: \.offset) { index, solved in
                    let letter = Character(UnicodeScalar(UInt8(65 + index)))
                    VStack(spacing: 4) {
                        ZStack {
                            BalloonShape(
                                style: solved
                                    ? .filled(BalloonColor.forLetter(letter))
                                    : .outline(BalloonColor.forLetter(letter), 6),
                                width: 36,
                                showHighlight: false
                            )
                            Text(String(letter))
                                .font(.plexMono(12))
                                .fontWeight(.semibold)
                                .foregroundColor(solved ? LognDark.onAccent : LognDark.textSecondary)
                                .position(x: 18, y: 36 * 0.42)
                        }
                        .frame(width: 36, height: 36 * 95 / 96)

                        Text(solved ? Str.Solved.done : Str.Solved.worth(XPPerAccepted))
                            .font(.plexMono(9))
                            .tracking(0.08 * 9)
                            .foregroundColor(solved ? LognDark.textMuted : LognDark.textPrimary)
                    }
                    .frame(width: 44)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(solved
                        ? Str.Solved.problem_done_accessibility(String(letter))
                        : Str.Solved.problem_open_accessibility(String(letter), XPPerAccepted))
                }
            }
        }
    }

    /// O mesmo valor do Core (`XP_PER_ACCEPTED`). Só a legenda usa.
    private let XPPerAccepted = 50

    // MARK: Faixa de números

    private var statStrip: some View {
        HStack(spacing: 1) {
            statCell(Str.Node.prereq, "\(node.prerequisites.count)")
            // Destravado, o número que importa é quanto o nó ainda paga. Bloqueado, é o
            // portão.
            if showsProblems {
                statCell(Str.Solved.in_play, "\(openCount * XPPerAccepted) XP")
            } else {
                statCell("XP", "\(node.requiredXp)")
            }
            statCell(Str.Node.unlocks, "\(unlocks.count)")
        }
        .background(LognDark.line)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
    }

    private func statCell(_ label: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(label)
                .font(.plexMono(10))
                .tracking(0.12 * 10)
                .foregroundColor(LognDark.textMuted)
            Text(value)
                .font(.plexMono(16))
                .monospacedDigit()
                .foregroundColor(LognDark.textPrimary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 14)
        .padding(.vertical, 12)
        .background(LognDark.surface)
    }

    // MARK: CTA

    /// Rejogar só é "de novo" quando não há mais nada para ganhar no nó. Conquistado
    /// com problema em aberto ainda é caminho, não revisão.
    private var ctaTitle: String {
        if solvedCount == 0 { return Str.Solved.start }
        return openCount > 0 ? Str.Solved.resume : Str.Solved.replay
    }

    @ViewBuilder
    private var cta: some View {
        if node.status == .locked {
            LognButton(title: Str.Node.locked, variant: .primary, action: {}, isDisabled: true)
        } else if problemCount == 0 {
            // Nó destravado e vazio existe: a árvore tem sete assuntos e o currículo
            // ainda não cobre todos. O botão prometia partida e não fazia nada — para
            // quem está jogando, um app travado. Dizer que não há é melhor.
            VStack(spacing: 8) {
                LognButton(title: Str.Node.no_problems, variant: .primary, action: {}, isDisabled: true)

                Text(Str.Node.no_problems_desc)
                    .font(.plexMono(11))
                    .foregroundColor(LognDark.textMuted)
                    .frame(maxWidth: .infinity)
            }
        } else {
            LognButton(title: ctaTitle, variant: .primary) {
                dismiss()
                // Deixa o sheet fechar antes de empurrar a navegação.
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
                    onStartMatch()
                }
            }
        }
    }
}
