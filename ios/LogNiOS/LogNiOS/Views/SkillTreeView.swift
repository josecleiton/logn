import SwiftUI
import LogN

struct SkillTreeView: View {
    let nodes: [SkillNode]
    
    // Configurações do Grid Visual
    let rowHeight: CGFloat = 110
    let colWidth: CGFloat = 100 // Distância do centro
    
    @EnvironmentObject var core: CoreWrapper
    @State private var selectedNode: SkillNode? = nil
    @State private var matchNodeId: String = ""
    @State private var navigateToMatch: Bool = false
    
    var body: some View {
        GeometryReader { geometry in
            ZStack {
                // Hidden navigation link outside ScrollView
                NavigationLink(destination: MatchView(nodeId: matchNodeId).environmentObject(core), isActive: $navigateToMatch) {
                    EmptyView()
                }
                
                VStack(spacing: 0) {
                    // Top Header
                    HStack {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("LogN App")
                                .font(.system(size: 20, weight: .bold))
                                .foregroundColor(LognDark.textPrimary)
                            Text("7 balões no ar · 1200 XP")
                                .font(.system(size: 14))
                                .foregroundColor(LognDark.textSecondary)
                        }
                        Spacer()
                        Circle()
                            .fill(LognDark.surface)
                            .frame(width: 40, height: 40)
                            .overlay(
                                Image(systemName: "person.crop.circle")
                                    .font(.system(size: 24))
                                    .foregroundColor(LognDark.textSecondary)
                            )
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 20)
                    .padding(.bottom, 10)
                    
                    ScrollView {
                ZStack {
                    // Desenhar conexões primeiro (Arestas)
                    Canvas { context, size in
                        let centerX = size.width / 2
                        
                        for node in nodes {
                            let endPoint = CGPoint(
                                x: centerX + CGFloat(node.column) * colWidth,
                                y: CGFloat(node.row) * rowHeight + (rowHeight / 2) + 30
                            )
                            
                            // Desenhar linhas para cada pré-requisito
                            for prereqId in node.prerequisites {
                                if let parent = nodes.first(where: { $0.id == prereqId }) {
                                    let startPoint = CGPoint(
                                        x: centerX + CGFloat(parent.column) * colWidth,
                                        y: CGFloat(parent.row) * rowHeight + (rowHeight / 2) + 30
                                    )
                                    
                                    var path = Path()
                                    path.move(to: startPoint)
                                    let cp1 = CGPoint(x: startPoint.x, y: startPoint.y + (rowHeight / 2))
                                    let cp2 = CGPoint(x: endPoint.x, y: endPoint.y - (rowHeight / 2))
                                    path.addCurve(to: endPoint, control1: cp1, control2: cp2)
                                    
                                    // Cor da aresta baseada no status
                                    // Aresta Percorrida: ambos completed
                                    // Aresta Ativa: parent completed, child active
                                    // Fechada: parent not completed
                                    let isParentCompleted = parent.status == .completed
                                    let isChildCompleted = node.status == .completed
                                    let isChildActive = node.status == .active
                                    
                                    if isChildCompleted {
                                        context.stroke(path, with: .color(LognDark.info), lineWidth: 3) // Percorrida
                                    } else if isParentCompleted && isChildActive {
                                        context.stroke(path, with: .color(LognDark.accent), lineWidth: 3) // Ativa
                                    } else {
                                        // Tracejada
                                        context.stroke(path, with: .color(LognDark.lineDim), style: StrokeStyle(lineWidth: 3, dash: [4, 6]))
                                    }
                                }
                            }
                        }
                    }
                    
                    // Desenhar os nós interativos
                    ForEach(nodes, id: \.id) { node in
                        SkillNodeView(node: node)
                            .position(
                                x: geometry.size.width / 2 + CGFloat(node.column) * colWidth,
                                y: CGFloat(node.row) * rowHeight + (rowHeight / 2) + 30
                            )
                            .onTapGesture {
                                if node.status != .locked {
                                    selectedNode = node
                                }
                            }
                    }
                    
                }
                // Altura total baseada na maior linha
                .frame(height: CGFloat((nodes.map { Int($0.row) }.max() ?? 0) + 1) * rowHeight + 130)
                }
                    
                    // Footer Legend
                    HStack {
                        Spacer()
                        Text("CORDINHA TRACEJADA = ARESTA FECHADA")
                            .font(.custom("IBMPlexMono-Regular", size: 10))
                            .tracking(0.14 * 10)
                            .foregroundColor(LognDark.textDim)
                        Spacer()
                    }
                    .padding(.vertical, 12)
                    .background(LognDark.canvas)
                }
                .sheet(item: $selectedNode) { node in
                    NodeSheetView(node: node, onStartMatch: {
                        matchNodeId = node.id
                        navigateToMatch = true
                    })
                }
            }
        }
    }
}

struct SkillNodeView: View {
    let node: SkillNode
    
    var body: some View {
        VStack(spacing: 6) {
            // Balão
            BalloonShape(
                color: node.status == .completed ? LognDark.info : LognDark.textDim,
                state: balloonState,
                bodySize: balloonSize,
                showString: false,
                showHighlight: true
            )
            .overlay(
                Text("\(node.requiredXp)")
                    .font(LognFont.label)
                    .foregroundColor(node.status == .locked ? LognDark.textDim : (node.status == .active ? LognDark.textPrimary : LognDark.surface))
                    .offset(y: -4) // Center vertically inside the balloon body
            )
                .scaleEffect(node.status == .active ? 1.08 : 1.0, anchor: .bottom)
            
            // Texto do Nó -> NodeTag
            VStack(spacing: 4) {
                Text(node.name)
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundColor(node.status == .locked ? LognDark.textDim : LognDark.textPrimary)
                    .lineLimit(1)
                
                if node.status == .active {
                    Text("INFLANDO · 0/5")
                        .font(.custom("IBMPlexMono-Regular", size: 10))
                        .foregroundColor(LognDark.accent)
                        .padding(.horizontal, 8)
                        .padding(.vertical, 2)
                        .overlay(
                            RoundedRectangle(cornerRadius: 8)
                                .stroke(LognDark.accent, lineWidth: 1)
                        )
                }
            }
            .frame(width: 120)
        }
        .opacity(node.status == .locked ? 0.6 : 1.0)
        .overlay(
            Group {
                if node.prerequisites.count >= 2 {
                    Circle()
                        .fill(LognDark.canvas)
                        .frame(width: 22, height: 22)
                        .overlay(
                            Circle()
                                .stroke(LognDark.line, lineWidth: 1)
                        )
                        .overlay(
                            Text("\(node.prerequisites.count)")
                                .font(.system(size: 10, weight: .bold))
                                .foregroundColor(LognDark.textSecondary)
                        )
                        .offset(x: 28, y: -20)
                }
            }
        )
    }
    
        
    var balloonState: BalloonShape.BalloonState {
        switch node.status {
        case .locked: return .locked
        case .active: return .active
        case .completed: return .filled
        }
    }
    
    var balloonSize: CGFloat {
        switch node.status {
        case .locked: return 54
        case .active: return 70
        case .completed: return 56
        }
    }

    var fillColor: Color {
        switch node.status {
        case .locked: return LognDark.surface
        case .active: return LognDark.accent
        case .completed: return LognDark.info
        }
    }
    
    var strokeColor: Color {
        switch node.status {
        case .locked: return LognDark.lineDim
        case .active: return LognDark.accent
        case .completed: return LognDark.info
        }
    }
}

extension LogN.SkillNode: @retroactive Identifiable {
    // A propriedade `id` já existe na struct gerada.
}
