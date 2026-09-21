import SwiftUI
import LogN
import App

struct MatchView: View {
    @EnvironmentObject var core: CoreWrapper
    let nodeId: String
    @Environment(\.dismiss) var dismiss
    
    // We observe matchView from the ViewModel
    var mv: LogN.MatchViewModel {
        core.viewModel.matchView
    }
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            if !mv.isActive {
                // Fim de partida (Match Over)
                MatchReportView()
            } else {
                VStack(spacing: 0) {
                    // Header de Partida Fixo
                    MatchHeader(
                        sessionLabel: "ARENA • PROBLEM \(mv.currentLetter)",
                        remainingSeconds: Int(mv.questionSeconds),
                        isFrozen: mv.isFrozen,
                        lives: Int(mv.lives),
                        balloonStates: mv.balloonStates.map { ($0.letter.first ?? "?", $0.isAccepted) }
                    )
                    
                    ScrollView {
                        VStack(alignment: .leading, spacing: Space.lg) {
                            // Título da questão
                            Text(mv.currentTitle)
                                .font(LognFont.headlineMedium)
                                .foregroundColor(LognDark.textPrimary)
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
                                
                                // Placeholder for Drag & Drop Dropzone implementation
                                Text("DIGITE A RESPOSTA (BETA):")
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.textMuted)
                                
                                TextField("Ex: n", text: Binding(
                                    get: { mv.answerString },
                                    set: { core.dispatch(event: .matchSetAnswer(answer: $0)) }
                                ))
                                .font(.custom("IBMPlexMono-Regular", size: 15))
                                .padding()
                                .background(LognDark.surface)
                                .cornerRadius(Radius.sm)
                                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                                .foregroundColor(LognDark.textPrimary)
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
    @Environment(\.dismiss) var dismiss
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
                        dismiss()
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
        HStack(alignment: .top, spacing: Space.md) {
            BalloonShape(color: LognDark.wrong, state: .filled, bodySize: 24, showString: false, showHighlight: false)
                .overlay(
                    Text(letter)
                        .font(.custom("IBMPlexMono-Medium", size: 10))
                        .foregroundColor(LognDark.surface)
                )
            
            VStack(alignment: .leading, spacing: 4) {
                Text(title)
                    .font(LognFont.titleMedium)
                    .foregroundColor(LognDark.textPrimary)
                
                Text(desc)
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
            }
            Spacer()
        }
        .padding(Space.md)
        .background(LognDark.surface)
        .cornerRadius(Radius.md)
        .overlay(RoundedRectangle(cornerRadius: Radius.md).stroke(LognDark.line, lineWidth: 1))
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
                    .font(LognFont.label)
                    .foregroundColor(LognDark.wrong)
                
                Text(title)
                    .font(LognFont.headlineMedium)
                    .foregroundColor(LognDark.textPrimary)
                
                Text(explanation)
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
                
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
