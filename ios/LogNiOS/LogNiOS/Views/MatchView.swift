import SwiftUI
import LogN
import App
import UniformTypeIdentifiers

struct MatchView: View {
    @EnvironmentObject var core: CoreWrapper
    let nodeId: String
    @Environment(\.dismiss) var dismiss
    
    @State private var shakeTrigger: CGFloat = 0
    
    // We observe matchView from the ViewModel
    var mv: LogN.MatchViewModel {
        core.viewModel.matchView
    }
    
    private var formattedTime: String {
        let m = mv.questionSeconds / 60
        let s = mv.questionSeconds % 60
        return String(format: "%02d:%02d", m, s)
    }
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            if !mv.isActive {
                // Fim de partida (Match Over)
                MatchReportView(onDismiss: { dismiss() })
            } else {
                VStack(spacing: 0) {
                    // Header de Partida Fixo (DS Simples)
                    HStack {
                        HStack(spacing: 8) {
                            BalloonShape(
                                color: BalloonColor.forLetter(mv.currentLetter.first ?? "A"),
                                state: .filled,
                                bodySize: 18,
                                showString: true,
                                showHighlight: true
                            )
                            .offset(y: 4) // Ajuste para a cordinha não empurrar demais
                            
                            Text("PROBLEM \(mv.currentLetter)")
                                .font(LognFont.label)
                                .foregroundColor(LognDark.textSecondary)
                        }
                        
                        Spacer()
                        
                        if mv.currentTemplateType == "SPOT_THE_BUG" {
                            Text(formattedTime)
                                .font(.custom("IBMPlexMono-Medium", size: 14))
                                .foregroundColor(LognDark.warn)
                                .monospacedDigit()
                        } else {
                            LifeBar(lives: Int(mv.lives))
                        }
                    }
                    .padding(.horizontal, 16)
                    .padding(.vertical, 14)
                    .background(LognDark.canvas)
                    .overlay(
                        Rectangle()
                            .frame(height: 1)
                            .foregroundColor(LognDark.line),
                        alignment: .bottom
                    )
                    
                    ScrollView {
                        VStack(alignment: .leading, spacing: Space.lg) {
                            // Título da questão
                            Text(mv.currentTitle)
                                .font(.custom("IBMPlexSans-SemiBold", size: 18))
                                .foregroundColor(.white)
                                .padding(.top, Space.lg)
                            
                            // Enunciado
                            Text(mv.currentDescription)
                                .font(LognFont.bodyLarge)
                                .foregroundColor(LognDark.textSecondary)
                                .fixedSize(horizontal: false, vertical: true)
                            
                            // Corpo dependendo do template type
                            if mv.currentTemplateType == "SPOT_THE_BUG" {
                                CodeBlock(
                                    lines: mv.currentCodeLines,
                                    selectedLine: mv.selectedLine >= 0 ? Int(mv.selectedLine) : nil,
                                    highlightColor: highlightColor,
                                    onSelectLine: { line in
                                        if !isEvaluating {
                                            core.dispatch(event: .matchSelectLine(line: Int32(line)))
                                        }
                                    }
                                )
                            } else if mv.currentTemplateType == "FILL_IN_THE_BLANK" {
                                CodeBlock(
                                    lines: mv.currentCodeLines,
                                    selectedLine: nil,
                                    highlightColor: LognDark.accent,
                                    onSelectLine: nil
                                )
                                
                                Text("ARRASTE A RESPOSTA CORRETA:")
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.textMuted)
                                
                                DropZone(
                                    title: "Código faltando",
                                    value: mv.answerString,
                                    onDrop: { val in core.dispatch(event: .matchSetAnswer(answer: val)) },
                                    onRemove: { core.dispatch(event: .matchSetAnswer(answer: "")) }
                                )
                                .padding(.vertical, Space.md)
                                
                                // Banco de opções
                                LazyVGrid(columns: [GridItem(.adaptive(minimum: 100))], spacing: Space.sm) {
                                    ForEach(mv.currentOptions.filter { $0 != mv.answerString }, id: \.self) { opt in
                                        DraggableChip(text: opt)
                                    }
                                }
                                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                                .foregroundColor(LognDark.textPrimary)
                            } else if mv.currentTemplateType == "COMPLEXITY_MATCH" {
                                Text("COMPLEXIDADE DE TEMPO E ESPAÇO")
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.textMuted)
                                
                                HStack(spacing: Space.md) {
                                    DropZone(
                                        title: "Tempo",
                                        value: mv.dropTime,
                                        onDrop: { val in core.dispatch(event: .matchSetDropTime(value: val)) },
                                        onRemove: { core.dispatch(event: .matchSetDropTime(value: "")) }
                                    )
                                    DropZone(
                                        title: "Espaço",
                                        value: mv.dropSpace,
                                        onDrop: { val in core.dispatch(event: .matchSetDropSpace(value: val)) },
                                        onRemove: { core.dispatch(event: .matchSetDropSpace(value: "")) }
                                    )
                                }
                                .padding(.vertical, Space.md)
                                
                                // Banco de opções
                                LazyVGrid(columns: [GridItem(.adaptive(minimum: 80))], spacing: Space.sm) {
                                    ForEach(mv.currentOptions.filter { $0 != mv.dropTime && $0 != mv.dropSpace }, id: \.self) { opt in
                                        DraggableChip(text: opt)
                                    }
                                }
                                .padding(.top, Space.md)
                                
                            } else if mv.currentTemplateType == "TAG_THE_PATTERN" {
                                Text("SELECIONE \(mv.maxSelections)")
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.textMuted)
                                
                                // Tag Grid
                                LazyVGrid(columns: [GridItem(.adaptive(minimum: 100))], spacing: 8) {
                                    ForEach(mv.currentOptions, id: \.self) { opt in
                                        let isSelected = mv.selectedTags.contains(opt)
                                        Button(action: {
                                            core.dispatch(event: .matchToggleTag(tag: opt))
                                        }) {
                                            Text(opt)
                                                .font(.custom("IBMPlexMono-Medium", size: 12))
                                                .padding(.horizontal, 12)
                                                .padding(.vertical, 8)
                                                .frame(maxWidth: .infinity)
                                                .background(isSelected ? LognDark.accent.opacity(0.14) : LognDark.canvas)
                                                .foregroundColor(isSelected ? LognDark.accent : LognDark.textSecondary)
                                                .overlay(
                                                    RoundedRectangle(cornerRadius: 2)
                                                        .stroke(isSelected ? LognDark.accent : LognDark.lineStrong, lineWidth: 1)
                                                )
                                        }
                                    }
                                }
                            }
                            
                            Spacer().frame(height: 100)
                        }
                        .padding(.horizontal, Space.screenMargin)
                    }
                    
                    // CTA Ancorado
                    VStack {
                        LognButton(
                            title: buttonTitle,
                            variant: .primary,
                            action: submitAction,
                            isDisabled: !canSubmit
                        )
                    }
                    .padding(.horizontal, Space.screenMargin)
                    .padding(.bottom, Space.lg)
                    .padding(.top, Space.md)
                    .background(LognDark.canvas)
                }
                
                // Shake / Flash overlay logic goes here
                
                // Trap Sheet
                if mv.hasTrap {
                    TrapSheet(
                        category: mv.trapCategory,
                        title: mv.trapTitle,
                        explanation: mv.trapExplanation,
                        onDismiss: {
                            core.dispatch(event: .matchDismissTrap)
                        }
                    )
                }
            }
        }
        .navigationBarHidden(true)
        .onAppear {
            core.dispatch(event: .startMatch(nodeId: nodeId))
            startTimer()
        }
    }
    
    // Timer para o relógio da partida
    @State private var timer: Timer?
    private func startTimer() {
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: 1.0, repeats: true) { _ in
            if mv.isActive && !mv.hasTrap {
                core.dispatch(event: .matchTimerTick)
            }
        }
    }
    
    @State private var isEvaluating = false
    @State private var shakeOffset: CGFloat = 0
    
    private var canSubmit: Bool {
        if isEvaluating { return false }
        switch mv.currentTemplateType {
        case "SPOT_THE_BUG": return mv.selectedLine >= 0
        case "FILL_IN_THE_BLANK": return !mv.answerString.isEmpty
        case "TAG_THE_PATTERN": return mv.selectedTags.count == Int(mv.maxSelections)
        case "COMPLEXITY_MATCH": return !mv.dropTime.isEmpty && !mv.dropSpace.isEmpty
        default: return false
        }
    }
    
    private var buttonTitle: String {
        switch mv.currentTemplateType {
        case "SPOT_THE_BUG": return "Confirmar linha \(mv.selectedLine + 1)"
        case "FILL_IN_THE_BLANK": return "Confirmar resposta"
        case "TAG_THE_PATTERN": return "Confirmar \(mv.selectedTags.count) tags"
        default: return "Confirmar"
        }
    }
    
    private var highlightColor: Color {
        // Se avaliar vermelho, fica vermelho
        if isEvaluating && mv.lastVerdict == "WA" { return LognDark.wrong }
        if isEvaluating && mv.lastVerdict == "AC" { return LognDark.correct }
        return LognDark.accent
    }
    
    private func submitAction() {
        // Dispara haptic
        // SwiftUI Haptic feedback
        let generator = UIImpactFeedbackGenerator(style: .medium)
        generator.impactOccurred()
        
        // Pede pro core avaliar
        let now = Int64(Date().timeIntervalSince1970)
        core.dispatch(event: .matchSubmit(timestamp: now))
    }
}

// Relatório Pós-Jogo (Mock temporário para o fim da partida)
struct MatchReportView: View {
    let onDismiss: () -> Void
    @EnvironmentObject var core: CoreWrapper
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: 0) {
                // Cabecalho
                VStack(spacing: Space.md) {
                    Text("CONTEST ENCERRADO")
                        .font(LognFont.label)
                        .foregroundColor(LognDark.textMuted)
                    
                    Text("\(core.viewModel.matchView.solvedCount) / \(core.viewModel.matchView.totalProblems) aceitos")
                        .font(.custom("IBMPlexMono-Medium", size: 48))
                        .foregroundColor(LognDark.textPrimary)
                    
                    // Fila de Balões Conquistados
                    ScrollView(.horizontal, showsIndicators: false) {
                        HStack(spacing: Space.sm) {
                            ForEach(0..<Int(core.viewModel.matchView.totalProblems), id: \.self) { idx in
                                let letter = String(Character(UnicodeScalar(65 + idx)!))
                                // Gambiarra provisória, precisariamos do dicionário real de verdicts no viewmodel
                                let isSolved = idx < Int(core.viewModel.matchView.solvedCount)
                                
                                BalloonShape(
                                    color: isSolved ? LognDark.correct : LognDark.surface,
                                    state: isSolved ? .filled : .locked,
                                    bodySize: 32,
                                    showString: false,
                                    showHighlight: isSolved
                                )
                                .overlay(
                                    Text(letter)
                                        .font(.custom("IBMPlexMono-Medium", size: 12))
                                        .foregroundColor(isSolved ? LognDark.surface : LognDark.textDim)
                                )
                            }
                        }
                        .padding(.horizontal, Space.screenMargin)
                    }
                    .padding(.vertical, Space.md)
                }
                .padding(.top, Space.xl)
                .padding(.bottom, Space.lg)
                .background(LognDark.surfaceRaised)
                
                // Content
                ScrollView {
                    VStack(alignment: .leading, spacing: Space.lg) {
                        Text("REVISÃO DE ERROS")
                            .font(LognFont.label)
                            .foregroundColor(LognDark.textMuted)
                            .padding(.horizontal, Space.screenMargin)
                            .padding(.top, Space.lg)
                        
                        // Fake Error Card for now
                        if core.viewModel.matchView.lives < core.viewModel.matchView.maxLives {
                            ErrorReviewCard(
                                letter: "A",
                                title: "Complexidade Subótima",
                                desc: "Você usou O(N^2) mas era possível usar O(N log N)."
                            )
                        } else {
                            Text("Nenhum erro cometido! Perfect clear.")
                                .font(LognFont.bodyLarge)
                                .foregroundColor(LognDark.correct)
                                .padding(.horizontal, Space.screenMargin)
                        }
                    }
                }
                
                // Footer
                VStack {
                    LognButton(title: "Voltar para a Skill Tree", variant: .primary) {
                        onDismiss()
                    }
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.bottom, Space.xl)
                .padding(.top, Space.md)
                .background(LognDark.canvas)
            }
        }
    }
}

struct ErrorReviewCard: View {
    let letter: String
    let title: String
    let desc: String
    
    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(alignment: .top) {
                Text(title)
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundColor(LognDark.textPrimary)
                
                Spacer()
                
                Text("WA")
                    .font(.custom("IBMPlexMono-Medium", size: 11))
                    .foregroundColor(LognDark.wrong)
            }
            
            Text("sua resposta: incorreta")
                .font(.custom("IBMPlexMono-Medium", size: 12))
                .foregroundColor(LognDark.textSecondary)
                .padding(.top, 6)
            
            Text(desc)
                .font(.system(size: 13))
                .lineSpacing(4) // approximate line-height 1.5
                .foregroundColor(Color(hex: "99A0A7"))
                .padding(.top, 8)
        }
        .padding(14)
        .background(LognDark.surface)
        .cornerRadius(4)
        .overlay(RoundedRectangle(cornerRadius: 4).stroke(LognDark.line, lineWidth: 1))
        .padding(.horizontal, Space.screenMargin)
    }
}

// Trap Sheet
struct TrapSheet: View {
    let category: String
    let title: String
    let explanation: String
    let onDismiss: () -> Void
    
    var body: some View {
        VStack {
            Spacer()
            
            VStack(alignment: .leading, spacing: Space.md) {
                // Handle
                Center {
                    Capsule()
                        .fill(LognDark.lineStrong)
                        .frame(width: 36, height: 3)
                        .padding(.top, 8)
                }
                
                Text(category)
                    .font(.custom("IBMPlexMono-Medium", size: 11))
                    .foregroundColor(LognDark.wrong)
                
                Text(title)
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundColor(LognDark.textPrimary)
                    .lineSpacing(5) // approx 1.3 line-height
                
                Text(explanation)
                    .font(.system(size: 14))
                    .foregroundColor(Color(hex: "99A0A7"))
                    .lineSpacing(7) // approx 1.5 line-height
                
                HStack(spacing: 12) {
                    LognButton(title: "Ler explicação", variant: .secondary) {
                        // Expandir
                    }
                    LognButton(title: "Pular", variant: .primary, action: onDismiss)
                }
                .padding(.top, Space.md)
                .padding(.bottom, 24)
            }
            .padding(.horizontal, Space.xl)
            .background(LognDark.surfaceRaised)
            .cornerRadius(8, corners: [.topLeft, .topRight])
            .shadow(color: .black.opacity(0.65), radius: 48, y: -16)
        }
        .ignoresSafeArea(edges: .bottom)
        .transition(.move(edge: .bottom))
        .zIndex(2)
        .background(Color.black.opacity(0.45).ignoresSafeArea().onTapGesture { onDismiss() })
    }
}

struct Center<Content: View>: View {
    let content: () -> Content
    var body: some View {
        HStack { Spacer(); content(); Spacer() }
    }
}

// MARK: - Drag & Drop Components

struct DraggableChip: View {
    let text: String
    
    var body: some View {
        Text(text)
            .font(.custom("IBMPlexMono-Medium", size: 14))
            .foregroundColor(LognDark.textPrimary)
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
            .background(LognDark.surfaceRaised)
            .cornerRadius(Radius.sm)
            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
            .onDrag {
                NSItemProvider(object: text as NSString)
            }
    }
}

struct DropZone: View {
    let title: String
    let value: String
    let onDrop: (String) -> Void
    let onRemove: () -> Void
    
    @State private var isTargeted = false
    
    var body: some View {
        VStack(spacing: 8) {
            Text(title)
                .font(LognFont.label)
                .foregroundColor(LognDark.textSecondary)
            
            ZStack {
                RoundedRectangle(cornerRadius: Radius.sm)
                    .stroke(isTargeted ? LognDark.accent : LognDark.line, style: StrokeStyle(lineWidth: 1, dash: [4]))
                    .background(isTargeted ? LognDark.accent.opacity(0.1) : LognDark.canvas)
                
                if !value.isEmpty {
                    Text(value)
                        .font(.custom("IBMPlexMono-Medium", size: 14))
                        .foregroundColor(LognDark.onAccent)
                        .padding(.horizontal, 16)
                        .padding(.vertical, 12)
                        .background(LognDark.accent)
                        .cornerRadius(Radius.sm)
                        .onTapGesture {
                            onRemove()
                        }
                } else {
                    Text("Soltar aqui")
                        .font(.custom("IBMPlexMono-Regular", size: 12))
                        .foregroundColor(LognDark.textDim)
                }
            }
            .frame(height: 56)
            .onDrop(of: [.plainText], isTargeted: $isTargeted) { providers in
                if let provider = providers.first {
                    provider.loadItem(forTypeIdentifier: "public.plain-text", options: nil) { (item, error) in
                        if let data = item as? Data, let text = String(data: data, encoding: .utf8) {
                            DispatchQueue.main.async {
                                onDrop(text)
                            }
                        } else if let url = item as? URL {
                            // Some versions return URL instead of Data for plain text drag
                            DispatchQueue.main.async {
                                onDrop(url.lastPathComponent)
                            }
                        } else if let text = item as? String {
                            DispatchQueue.main.async {
                                onDrop(text)
                            }
                        }
                    }
                    return true
                }
                return false
            }
        }
    }
}
