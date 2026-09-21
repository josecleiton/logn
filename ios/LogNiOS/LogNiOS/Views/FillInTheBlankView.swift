import SwiftUI
import App

struct FillInTheBlankView: View {
    let title: String
    let codeLines: [String]
    let balloonColor: Color
    
    @State private var answerString: String = ""
    
    var onSubmit: (String) -> Void
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(alignment: .leading, spacing: Space.xl) {
                // Header com o balão do problema ICPC
                HStack(spacing: Space.md) {
                    Circle()
                        .fill(balloonColor)
                        .frame(width: 24, height: 24)
                        .shadow(color: balloonColor.opacity(0.4), radius: 8, x: 0, y: 0)
                    
                    Text(title)
                        .font(LognFont.headlineMedium)
                        .foregroundColor(LognDark.textPrimary)
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.top, Space.lg)
                
                Text("Preencha a lacuna com a sintaxe correta:")
                    .font(LognFont.bodyMedium)
                    .foregroundColor(LognDark.textSecondary)
                    .padding(.horizontal, Space.screenMargin)
                
                // Code Editor Mock
                ScrollView {
                    VStack(alignment: .leading, spacing: 0) {
                        ForEach(Array(codeLines.enumerated()), id: \.offset) { index, line in
                            HStack(alignment: .top, spacing: Space.md) {
                                Text("\(index + 1)")
                                    .font(LognFont.label)
                                    .foregroundColor(LognDark.lineDim)
                                    .frame(width: 24, alignment: .trailing)
                                
                                if line.contains("_____") {
                                    let parts = line.components(separatedBy: "_____")
                                    HStack(spacing: 0) {
                                        Text(parts[0])
                                            .font(LognFont.code)
                                            .foregroundColor(LognDark.textPrimary)
                                        
                                        TextField("...", text: $answerString)
                                            .font(LognFont.code)
                                            .foregroundColor(LognDark.onAccent)
                                            .padding(.horizontal, Space.xs)
                                            .background(LognDark.accent.opacity(0.2))
                                            .cornerRadius(Radius.xs)
                                            .overlay(
                                                RoundedRectangle(cornerRadius: Radius.xs)
                                                    .stroke(LognDark.accent, lineWidth: 1)
                                            )
                                            .frame(width: 80)
                                        
                                        if parts.count > 1 {
                                            Text(parts[1])
                                                .font(LognFont.code)
                                                .foregroundColor(LognDark.textPrimary)
                                        }
                                    }
                                } else {
                                    Text(line)
                                        .font(LognFont.code)
                                        .foregroundColor(LognDark.textPrimary)
                                        .frame(maxWidth: .infinity, alignment: .leading)
                                }
                            }
                            .padding(.vertical, Space.xs)
                            .padding(.horizontal, Space.sm)
                        }
                    }
                    .padding(.vertical, Space.md)
                }
                .background(LognDark.surfaceRaised)
                .cornerRadius(Radius.md)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.md)
                        .stroke(LognDark.line, lineWidth: 1)
                )
                .padding(.horizontal, Space.screenMargin)
                
                Spacer()
                
                // Botão de Submit
                Button(action: {
                    onSubmit(answerString)
                }) {
                    Text("Submeter Resposta")
                        .font(LognFont.titleMedium)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, Space.md)
                        .background(answerString.isEmpty ? LognDark.buttonDisabled : balloonColor)
                        .foregroundColor(answerString.isEmpty ? LognDark.textMuted : LognDark.surface)
                        .cornerRadius(Radius.sm)
                }
                .disabled(answerString.isEmpty)
                .padding(.horizontal, Space.screenMargin)
                .padding(.bottom, Space.xl)
            }
        }
        .colorScheme(.dark)
    }
}
