import SwiftUI
import LogN

struct SkillTreeView: View {
    let nodes: [SkillNode]
    
    // Configurações do Grid Visual
    let rowHeight: CGFloat = 110
    let colWidth: CGFloat = 100 // Distância do centro
    
    var body: some View {
        GeometryReader { geometry in
            ScrollView {
                ZStack {
                    // Desenhar conexões primeiro (Arestas)
                    Canvas { context, size in
                        let centerX = size.width / 2
                        
                        for node in nodes {
                            let endPoint = CGPoint(
                                x: centerX + CGFloat(node.column) * colWidth,
                                y: CGFloat(node.row) * rowHeight + (rowHeight / 2)
                            )
                            
                            // Desenhar linhas para cada pré-requisito
                            for prereqId in node.prerequisites {
                                if let parent = nodes.first(where: { $0.id == prereqId }) {
                                    let startPoint = CGPoint(
                                        x: centerX + CGFloat(parent.column) * colWidth,
                                        y: CGFloat(parent.row) * rowHeight + (rowHeight / 2)
                                    )
                                    
                                    var path = Path()
                                    path.move(to: startPoint)
                                    path.addLine(to: endPoint)
                                    
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
                                y: CGFloat(node.row) * rowHeight + (rowHeight / 2)
                            )
                    }
                }
                // Altura total baseada na maior linha
                .frame(height: CGFloat((nodes.map { Int($0.row) }.max() ?? 0) + 1) * rowHeight + 100)
            }
        }
    }
}

struct SkillNodeView: View {
    let node: SkillNode
    @State private var isPulsing = false
    
    var body: some View {
        NavigationLink(destination: NodeChallengesView(node: node)) {
            VStack(spacing: 6) {
                // Balão
                Circle()
                    .fill(fillColor)
                    .frame(width: 44, height: 44)
                    .overlay(
                        Circle().stroke(strokeColor, lineWidth: 2)
                    )
                    .overlay(
                        Text("\(node.requiredXp)")
                            .font(LognFont.label)
                            .foregroundColor(node.status == .locked ? LognDark.textDim : LognDark.surface)
                    )
                    .scaleEffect(node.status == .active ? (isPulsing ? 1.08 : 1.0) : 1.0)
                    .animation(node.status == .active ? Animation.easeInOut(duration: 1.0).repeatForever(autoreverses: true) : .default, value: isPulsing)
                    .onAppear {
                        if node.status == .active {
                            isPulsing = true
                        }
                    }
                
                // Texto do Nó
                Text(node.name)
                    .font(LognFont.label)
                    .foregroundColor(node.status == .locked ? LognDark.textDim : LognDark.textPrimary)
                    .lineLimit(1)
                    .frame(width: 80)
            }
        }
        .disabled(node.status == .locked)
    }
    
    var fillColor: Color {
        switch node.status {
        case .locked: return LognDark.surface
        case .active: return LognDark.accent
        case .completed: return LognDark.info
        default: return LognDark.surface
        }
    }
    
    var strokeColor: Color {
        switch node.status {
        case .locked: return LognDark.lineDim
        case .active: return LognDark.accent
        case .completed: return LognDark.info
        default: return LognDark.lineDim
        }
    }
}
