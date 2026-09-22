import SwiftUI
import LogN

/// Trilha como DAG — exploração `4b · Grafo de balões`, escolhida e promovida ao DS.
///
/// Os vértices são balões da marca com o ícone do assunto dentro; as arestas são as
/// cordinhas que amarram cada balão ao pré-requisito. A etiqueta pendura no nó e a
/// cordinha segue adiante, saindo de baixo dela.
struct SkillTreeView: View {
    let nodes: [SkillNode]

    @EnvironmentObject var core: CoreWrapper
    @State private var selectedNode: SkillNode? = nil
    @State private var matchNodeId: String = ""
    @State private var navigateToMatch: Bool = false

    var body: some View {
        GeometryReader { geo in
            let layout = SkillTreeLayout(nodes: nodes, canvasWidth: geo.size.width)

            ZStack {
                NavigationLink(
                    destination: MatchView(nodeId: matchNodeId).environmentObject(core),
                    isActive: $navigateToMatch
                ) { EmptyView() }
                .hidden()

                VStack(spacing: 0) {
                    ScrollView {
                        ZStack(alignment: .topLeading) {
                            SkillTreeEdges(layout: layout)

                            ForEach(nodes, id: \.id) { node in
                                if let placement = layout.placement(for: node.id) {
                                    SkillNodeView(node: node, placement: placement)
                                        .position(
                                            x: placement.centerX,
                                            y: placement.balloonTop + placement.totalHeight / 2
                                        )
                                        .onTapGesture {
                                            selectedNode = node
                                        }
                                }
                            }
                        }
                        .frame(height: layout.contentHeight)
                    }

                    // Legenda ancorada — explica a gramática do grafo sem tirar espaço do mapa.
                    Text("A ETIQUETA PENDURA NO NÓ · A CORDINHA SEGUE ADIANTE")
                        .font(.plexMono(10))
                        .tracking(0.14 * 10)
                        .foregroundColor(LognDark.textMuted)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal, Space.screenMargin)
                        .padding(.vertical, Space.md)
                }
            }
            .sheet(item: $selectedNode) { node in
                NodeSheetView(
                    node: node,
                    incoming: nodes.filter { node.prerequisites.contains($0.id) },
                    unlocks: nodes.filter { $0.prerequisites.contains(node.id) },
                    onStartMatch: {
                        matchNodeId = node.id
                        navigateToMatch = true
                    }
                )
                .presentationDetents([.height(NodeSheetView.preferredHeight)])
                .presentationDragIndicator(.hidden)
                .modifier(SheetCorners())
            }
        }
    }
}

/// Raio de 8dp só nos cantos superiores do bottom sheet — o único raio de 8 do DS.
struct SheetCorners: ViewModifier {
    func body(content: Content) -> some View {
        if #available(iOS 17.0, *) {
            content.presentationCornerRadius(Radius.md)
        } else {
            content
        }
    }
}

// MARK: - Layout

/// Posiciona o grafo a partir da topologia que o Core entrega (`row` / `column`).
///
/// A régua vem da exploração 4b: passo de 88pt entre colunas, 116pt entre linhas, e cada
/// linha centrada verticalmente — balões de tamanhos diferentes (ativo é maior) se alinham
/// pelo centro, não pelo topo.
struct SkillTreeLayout {
    struct Placement {
        let centerX: CGFloat
        let balloonTop: CGFloat
        let balloonWidth: CGFloat
        let balloonHeight: CGFloat
        let labelHeight: CGFloat

        var totalHeight: CGFloat { balloonHeight + labelHeight }
        /// De onde a cordinha parte: a base da etiqueta.
        var stringOrigin: CGPoint { CGPoint(x: centerX, y: balloonTop + totalHeight) }
        /// Onde a cordinha chega: o topo do balão.
        var stringTarget: CGPoint { CGPoint(x: centerX, y: balloonTop) }
    }

    static let columnPitch: CGFloat = 88
    static let topInset: CGFloat = 16
    static let bottomInset: CGFloat = 40
    /// Quanto de cordinha fica visível entre a etiqueta de um nó e o balão seguinte.
    /// Medido no documento: os vãos são 30, 30, 24 e 23 — 28 é o meio-termo.
    static let stringGap: CGFloat = 28

    let nodes: [SkillNode]
    let canvasWidth: CGFloat
    private let placements: [String: Placement]
    private let totalHeight: CGFloat

    init(nodes: [SkillNode], canvasWidth: CGFloat) {
        self.nodes = nodes
        self.canvasWidth = canvasWidth

        // As linhas empilham sequencialmente: cada uma começa depois da etiqueta mais
        // baixa da anterior. Um passo fixo não serve porque o nó ativo é maior e tem
        // etiqueta de duas linhas — ele comeria a folga da cordinha.
        var result: [String: Placement] = [:]
        var rowTop = Self.topInset
        var bottom = rowTop

        let rows = Dictionary(grouping: nodes, by: { $0.row }).sorted { $0.key < $1.key }

        for (_, rowNodes) in rows {
            let maxBalloonHeight = rowNodes
                .map { Self.balloonWidth(for: $0.status) * 95 / 96 }
                .max() ?? 0
            // Balões de tamanhos diferentes na mesma linha se alinham pelo centro.
            let rowCenter = rowTop + maxBalloonHeight / 2
            var rowBottom = rowTop

            for node in rowNodes {
                let width = Self.balloonWidth(for: node.status)
                let balloonHeight = width * 95 / 96
                let labelHeight = Self.labelHeight(for: node.status)
                let balloonTop = rowCenter - balloonHeight / 2

                result[node.id] = Placement(
                    centerX: canvasWidth / 2 + CGFloat(node.column) * Self.columnPitch,
                    balloonTop: balloonTop,
                    balloonWidth: width,
                    balloonHeight: balloonHeight,
                    labelHeight: labelHeight
                )
                rowBottom = max(rowBottom, balloonTop + balloonHeight + labelHeight)
            }

            bottom = rowBottom
            rowTop = rowBottom + Self.stringGap
        }

        self.placements = result
        self.totalHeight = bottom + Self.bottomInset
    }

    func placement(for id: String) -> Placement? { placements[id] }

    /// Larguras de viewBox do documento: conquistado 64, ativo 80, bloqueado 56.
    static func balloonWidth(for status: NodeStatus) -> CGFloat {
        switch status {
        case .completed: return 64
        case .active:    return 80
        case .locked:    return 56
        }
    }

    /// A etiqueta do nó ativo tem duas linhas (título + `INFLANDO · n/m`).
    static func labelHeight(for status: NodeStatus) -> CGFloat {
        status == .active ? 38 : 23
    }

    var contentHeight: CGFloat { totalHeight }

    /// Arestas do DAG, já resolvidas em pai → filho.
    var edges: [(parent: SkillNode, child: SkillNode)] {
        nodes.flatMap { child in
            child.prerequisites.compactMap { parentId in
                nodes.first(where: { $0.id == parentId }).map { ($0, child) }
            }
        }
    }
}

// MARK: - Arestas

/// As cordinhas. Três estados, como manda a legenda do documento:
/// percorrida (sólida, na cor do balão que a sustenta), ativa (accent) e
/// pré-requisito pendente (tracejada).
struct SkillTreeEdges: View {
    let layout: SkillTreeLayout

    var body: some View {
        Canvas { context, _ in
            for edge in layout.edges {
                guard let from = layout.placement(for: edge.parent.id),
                      let to = layout.placement(for: edge.child.id) else { continue }

                let start = from.stringOrigin
                let end = to.stringTarget
                let slack = (end.y - start.y) * 0.45

                var path = Path()
                path.move(to: start)
                path.addCurve(
                    to: end,
                    control1: CGPoint(x: start.x, y: start.y + slack),
                    control2: CGPoint(x: end.x, y: end.y - slack)
                )

                let parentDone = edge.parent.status == .completed

                if !parentDone {
                    // Pré-requisito pendente — o caminho ainda está fechado.
                    context.stroke(
                        path,
                        with: .color(LognDark.lineDim),
                        style: StrokeStyle(lineWidth: 2.5, lineCap: .round, dash: [4, 6])
                    )
                } else if edge.child.status == .active {
                    // Aresta ativa — é por aqui que o jogador segue agora.
                    context.stroke(
                        path,
                        with: .color(LognDark.accent),
                        style: StrokeStyle(lineWidth: 3, lineCap: .round)
                    )
                } else {
                    // Percorrida: um balão conquistado sustenta os seguintes, então a
                    // cordinha leva a cor do assunto do pai.
                    context.stroke(
                        path,
                        with: .color(LognTopic.of(nodeName: edge.parent.name).color.opacity(0.8)),
                        style: StrokeStyle(lineWidth: 2.5, lineCap: .round)
                    )
                }
            }
        }
    }
}

// MARK: - Nó

struct SkillNodeView: View {
    let node: SkillNode
    let placement: SkillTreeLayout.Placement

    private var topic: LognTopic { LognTopic.of(nodeName: node.name) }

    var body: some View {
        VStack(spacing: 0) {
            balloon
            label
                .frame(height: placement.labelHeight)
        }
        .frame(width: placement.balloonWidth, height: placement.totalHeight)
        .contentShape(Rectangle())
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(accessibilityDescription)
        .accessibilityAddTraits(node.status == .locked ? [] : .isButton)
    }

    // MARK: Balão com o ícone do assunto dentro

    private var balloon: some View {
        ZStack {
            BalloonShape(style: balloonStyle, width: placement.balloonWidth)

            TopicIcon(
                topic: topic,
                color: iconColor,
                size: placement.balloonWidth * iconSizeRatio,
                lineWidth: iconLineWidth
            )
            .position(
                x: placement.balloonWidth / 2,
                y: placement.balloonWidth * iconCenterRatio
            )
        }
        .frame(width: placement.balloonWidth, height: placement.balloonHeight)
        .overlay(alignment: .topTrailing) {
            // Grau de entrada 2 = dois pré-requisitos. O número é a informação;
            // a posição só evita que ele cubra o ícone.
            if node.prerequisites.count >= 2 {
                Text("\(node.prerequisites.count)")
                    .font(.plexMono(9.5))
                    .foregroundColor(LognDark.textSecondary)
                    .frame(width: 19, height: 19)
                    .background(Circle().fill(LognDark.canvas))
                    .overlay(Circle().stroke(LognDark.lineDim, lineWidth: 1))
                    .offset(x: 5, y: 2)
            }
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
        case .completed: return LognDark.onAccent   // tinta escura sobre o corpo cheio
        case .active:    return LognDark.accent
        case .locked:    return LognDark.textMuted
        }
    }

    private var iconSizeRatio: CGFloat {
        node.status == .locked ? 0.34 : 0.36
    }

    /// Centro óptico do ícone dentro do corpo. A silhueta murcha do nó bloqueado
    /// se apoia mais abaixo na viewBox, então o ícone acompanha.
    private var iconCenterRatio: CGFloat {
        node.status == .locked ? 0.50 : 0.42
    }

    private var iconLineWidth: CGFloat {
        switch node.status {
        case .completed: return 2.4
        case .active:    return 2.2
        case .locked:    return 2.0
        }
    }

    // MARK: Etiqueta pendurada

    @ViewBuilder
    private var label: some View {
        if node.status == .active {
            VStack(spacing: 3) {
                Text(node.name)
                    .font(.plexSansSemiBold(13.5))
                    .foregroundColor(LognDark.textPrimary)
                Text("INFLANDO · \(node.requiredXp) XP")
                    .font(.plexMono(10))
                    .tracking(0.08 * 10)
                    .foregroundColor(LognDark.accent)
            }
            .fixedSize()
            .padding(.horizontal, 11)
            .padding(.vertical, 5)
            .background(LognDark.accentTint)
            .overlay(RoundedRectangle(cornerRadius: 3).stroke(LognDark.accent, lineWidth: 1))
            .cornerRadius(3)
        } else {
            Text(node.name)
                .font(.plexSansSemiBold(12.5))
                .foregroundColor(node.status == .locked ? LognDark.textSecondary : LognDark.textPrimary)
                .fixedSize()
                .padding(.horizontal, 10)
                .padding(.vertical, 4)
                .background(LognDark.canvas)
                .overlay(RoundedRectangle(cornerRadius: 3).stroke(LognDark.line, lineWidth: 1))
                .cornerRadius(3)
        }
    }

    // MARK: Acessibilidade

    /// O balão nunca comunica só por cor — o assunto e o estado vão no rótulo.
    private var accessibilityDescription: String {
        let state: String
        switch node.status {
        case .completed: return "\(node.name), conquistado"
        case .active:    state = "em curso"
        case .locked:
            let missing = node.prerequisites.count
            state = missing >= 2 ? "bloqueado, \(missing) pré-requisitos" : "bloqueado"
        }
        return "\(node.name), \(state)"
    }
}

extension LogN.SkillNode: @retroactive Identifiable {
    // A propriedade `id` já existe na struct gerada.
}
