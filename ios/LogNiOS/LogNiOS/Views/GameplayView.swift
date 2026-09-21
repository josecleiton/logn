import SwiftUI
import App

struct GameplayView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) var dismiss
    
    let challenge: Challenge
    
    @State private var selectedLine: Int? = nil
    @State private var inlineEditMode: Bool = false
    @State private var inlineText: String = ""
    
    @State private var showFeedback = false
    @State private var isCorrect = false
    @State private var explanation = ""
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(alignment: .leading, spacing: Space.lg) {
                // Header
                HStack {
                    Button(action: { dismiss() }) {
                        Image(systemName: "xmark")
                            .font(.system(size: 20, weight: .bold))
                            .foregroundColor(LognDark.textSecondary)
                    }
                    Spacer()
                    Text(challenge.payload.content.title)
                        .font(LognFont.titleMedium)
                        .foregroundColor(LognDark.textPrimary)
                    Spacer()
                    // Placeholder for XP or Timer
                    Image(systemName: "bolt.fill")
                        .foregroundColor(LognDark.warn)
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.top, Space.md)
                
                // Description
                Text(challenge.payload.content.description)
                    .font(LognFont.bodyMedium)
                    .foregroundColor(LognDark.textSecondary)
                    .padding(.horizontal, Space.screenMargin)
                
                // Code Snippet (Horizontal Scroll)
                ScrollView(.horizontal, showsIndicators: false) {
                    VStack(alignment: .leading, spacing: 0) {
                        ForEach(Array(challenge.payload.content.codeLines.enumerated()), id: \.offset) { index, line in
                            let isSelected = selectedLine == index
                            
                            HStack(alignment: .top, spacing: Space.md) {
                                // Line number
                                Text("\\(index + 1)")
                                    .font(LognFont.code)
                                    .foregroundColor(isSelected ? LognDark.accent : LognDark.textDim)
                                    .frame(width: 24, alignment: .trailing)
                                
                                // Line content
                                if isSelected && inlineEditMode {
                                    TextField("", text: $inlineText)
                                        .font(LognFont.code)
                                        .foregroundColor(LognDark.textPrimary)
                                        .autocapitalization(.none)
                                        .disableAutocorrection(true)
                                        .padding(.vertical, 4)
                                        .padding(.horizontal, Space.xs)
                                        .background(LognDark.surfaceRaised)
                                        .cornerRadius(Radius.sm)
                                        // Focus handling omitted for brevity, but should be auto-focused
                                } else {
                                    SyntaxTextView(text: line)
                                        .font(LognFont.code)
                                        .foregroundColor(isSelected ? LognDark.onAccent : LognDark.textPrimary)
                                        .padding(.vertical, 4)
                                        .padding(.horizontal, Space.xs)
                                        .background(isSelected ? LognDark.accent.opacity(0.8) : Color.clear)
                                        .cornerRadius(Radius.xs)
                                }
                            }
                            .padding(.vertical, 2)
                            .padding(.horizontal, Space.screenMargin)
                            .background(isSelected ? LognDark.accent.opacity(0.1) : Color.clear)
                            .contentShape(Rectangle())
                            .onTapGesture {
                                selectLine(index: index, lineText: line)
                            }
                        }
                    }
                    .padding(.vertical, Space.md)
                    .background(LognDark.surface)
                    .cornerRadius(Radius.md)
                    .padding(.horizontal, Space.screenMargin)
                }
                
                Spacer()
                
                // Submit Button
                Button(action: submitAnswer) {
                    Text("Confirmar Correção")
                        .font(LognFont.titleMedium)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, Space.md)
                        .background(canSubmit ? LognDark.accent : LognDark.buttonDisabled)
                        .foregroundColor(canSubmit ? LognDark.onAccent : LognDark.textDim)
                        .cornerRadius(Radius.sm)
                }
                .disabled(!canSubmit)
                .padding(.horizontal, Space.screenMargin)
                .padding(.bottom, Space.xl)
            }
            
            // Feedback Bottom Sheet overlay
            if showFeedback {
                FeedbackSheet(
                    isCorrect: isCorrect,
                    explanation: explanation,
                    onContinue: {
                        showFeedback = false
                        if isCorrect {
                            // Enviar evento de acerto para o Core e sair
                            core.dispatch(event: .challengeAnswered(challengeId: challenge.id, nodeId: challenge.nodeId, isCorrect: true))
                            dismiss()
                        } else {
                            // Enviar evento de erro para o Core e sair (Punição 0 XP)
                            core.dispatch(event: .challengeAnswered(challengeId: challenge.id, nodeId: challenge.nodeId, isCorrect: false))
                            dismiss()
                        }
                    }
                )
            }
        }
    }
    
    private var canSubmit: Bool {
        if selectedLine == nil { return false }
        if challenge.payload.validation.validationType == "EXACT_MATCH" && inlineText.isEmpty { return false }
        return true
    }
    
    private func selectLine(index: Int, lineText: String) {
        selectedLine = index
        if challenge.payload.validation.validationType == "EXACT_MATCH" {
            inlineEditMode = true
            inlineText = lineText // preenche com a string atual
        }
    }
    
    private func submitAnswer() {
        guard let lineIdx = selectedLine else { return }
        
        // Validação local imediata conforme a spec
        let validation = challenge.payload.validation
        
        if validation.validationType == "LINE_MATCH" {
            if let expected = validation.correctLine, Int(expected) == lineIdx {
                isCorrect = true
                explanation = "Excelente! Você identificou a linha problemática que causa o bug."
            } else {
                isCorrect = false
                explanation = "Incorreto. O erro de lógica não está nessa linha. Volte aos estudos do nó e tente novamente."
            }
        } else {
            // EXACT_MATCH
            if let expectedStr = validation.expectedString, inlineText.trimmingCharacters(in: .whitespaces) == expectedStr {
                isCorrect = true
                explanation = "Excelente! A correção de sintaxe/lógica foi perfeita."
            } else {
                isCorrect = false
                explanation = "Incorreto. A correção não gera o comportamento esperado. Esperado: \\(validation.expectedString ?? "")"
            }
        }
        
        withAnimation(.spring()) {
            showFeedback = true
        }
    }
}

// Simple Syntax Highlighter View for MVP
struct SyntaxTextView: View {
    let text: String
    
    var body: some View {
        let parts = text.split(separator: " ", omittingEmptySubsequences: false)
        
        // We use reduce to combine Text views.
        // Swift 5.7+ can build these seamlessly
        if parts.isEmpty {
            return Text(text)
        }
        
        var combinedText = Text("")
        for (i, part) in parts.enumerated() {
            let str = String(part)
            if isKeyword(str) {
                combinedText = combinedText + Text(str).foregroundColor(LognDark.synKeyword)
            } else if isFunction(str) {
                combinedText = combinedText + Text(str).foregroundColor(LognDark.synFunction)
            } else {
                combinedText = combinedText + Text(str)
            }
            
            if i < parts.count - 1 {
                combinedText = combinedText + Text(" ")
            }
        }
        return combinedText
    }
    
    func isKeyword(_ w: String) -> Bool {
        let keywords = ["func", "var", "let", "if", "else", "return", "for", "while", "class", "struct", "import"]
        return keywords.contains(w)
    }
    
    func isFunction(_ w: String) -> Bool {
        return w.contains("(") || w == "print" || w == "max" || w == "min"
    }
}

struct FeedbackSheet: View {
    let isCorrect: Bool
    let explanation: String
    let onContinue: () -> Void
    
    var body: some View {
        VStack {
            Spacer()
            
            VStack(alignment: .leading, spacing: Space.lg) {
                HStack {
                    Image(systemName: isCorrect ? "checkmark.circle.fill" : "xmark.octagon.fill")
                        .font(.system(size: 32))
                        .foregroundColor(isCorrect ? LognDark.correct : LognDark.wrong)
                    
                    Text(isCorrect ? "Acertou!" : "Incorreto")
                        .font(LognFont.headlineMedium)
                        .foregroundColor(isCorrect ? LognDark.correct : LognDark.wrong)
                    
                    Spacer()
                }
                
                Text(explanation)
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textPrimary)
                
                Button(action: onContinue) {
                    Text(isCorrect ? "Continuar (+50 XP)" : "Voltar")
                        .font(LognFont.titleMedium)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, Space.md)
                        .background(isCorrect ? LognDark.correct : LognDark.wrong)
                        .foregroundColor(LognDark.surface)
                        .cornerRadius(Radius.sm)
                }
                .padding(.top, Space.md)
            }
            .padding(.all, Space.xl)
            .background(LognDark.surfaceRaised)
            .cornerRadius(Radius.md, corners: [.topLeft, .topRight])
            .shadow(color: .black.opacity(0.5), radius: 20, y: -10)
        }
        .ignoresSafeArea(edges: .bottom)
        .transition(.move(edge: .bottom))
        .zIndex(1)
    }
}

extension View {
    func cornerRadius(_ radius: CGFloat, corners: UIRectCorner) -> some View {
        clipShape( RoundedCorner(radius: radius, corners: corners) )
    }
}

struct RoundedCorner: Shape {
    var radius: CGFloat = .infinity
    var corners: UIRectCorner = .allCorners
    
    func path(in rect: CGRect) -> Path {
        let path = UIBezierPath(roundedRect: rect, byRoundingCorners: corners, cornerRadii: CGSize(width: radius, height: radius))
        return Path(path.cgPath)
    }
}
