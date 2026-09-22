import SwiftUI
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
    let onStartMatch: () -> Void

    @Environment(\.dismiss) var dismiss

    /// Somatório do conteúdo: handle 18+3+18 · cabeçalho 60 · 20 · vizinhança 72 ·
    /// 20 · faixa 62 · 18 · CTA 52 · 22. O sheet é compacto por design — nada de
    /// esticar para preencher tela.
    static let preferredHeight: CGFloat = 365

    private var topic: LognTopic { LognTopic.of(nodeName: node.name) }

    var body: some View {
        ZStack(alignment: .top) {
            LognDark.surfaceRaised.ignoresSafeArea()

            VStack(alignment: .leading, spacing: 0) {
                handle
                header
                neighbourhood.padding(.top, 20)
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
                    .font(.plexSansSemiBold(19))
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
        case .locked:    return .deflated
        }
    }

    private var iconColor: Color {
        switch node.status {
        case .completed: return LognDark.onAccent
        case .active:    return LognDark.accent
        case .locked:    return LognDark.textMuted
        }
    }

    private var stateLabel: String {
        switch node.status {
        case .completed: return "CONQUISTADO"
        case .active:    return "INFLANDO · \(node.requiredXp) XP"
        case .locked:    return "BLOQUEADO"
        }
    }

    private var stateColor: Color {
        switch node.status {
        case .completed: return topic.color
        case .active:    return LognDark.accent
        case .locked:    return LognDark.textMuted
        }
    }

    // MARK: Vizinhança — vem de / destrava

    private var neighbourhood: some View {
        HStack(alignment: .top, spacing: 20) {
            column(title: "VEM DE", nodes: incoming, isUpstream: true)
            column(title: "DESTRAVA", nodes: unlocks, isUpstream: false)
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
                Text(isUpstream ? "ponto de partida" : "fim da trilha")
                    .font(.plexSans(13.5))
                    .foregroundColor(LognDark.textMuted)
                    .padding(.top, 10)
            } else {
                ForEach(Array(nodes.prefix(2).enumerated()), id: \.element.id) { index, neighbour in
                    HStack(spacing: 9) {
                        TopicIcon(
                            topic: LognTopic.of(nodeName: neighbour.name),
                            color: isUpstream ? LognTopic.of(nodeName: neighbour.name).color : LognDark.textMuted,
                            size: 17
                        )
                        Text(neighbour.name)
                            .font(.plexSans(13.5))
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

    // MARK: Faixa de números

    private var statStrip: some View {
        HStack(spacing: 1) {
            statCell("PRÉ-REQ", "\(node.prerequisites.count)")
            statCell("XP", "\(node.requiredXp)")
            statCell("DESTRAVA", "\(unlocks.count)")
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

    @ViewBuilder
    private var cta: some View {
        if node.status == .locked {
            LognButton(title: "Bloqueado", variant: .primary, action: {}, isDisabled: true)
        } else {
            LognButton(
                title: node.status == .completed ? "Jogar de novo" : "Começar partida",
                variant: .primary
            ) {
                dismiss()
                // Deixa o sheet fechar antes de empurrar a navegação.
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
                    onStartMatch()
                }
            }
        }
    }
}
