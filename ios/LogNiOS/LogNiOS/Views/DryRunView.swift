import SwiftUI
import LogN

/// Dry run — exploração `2b · Watch`.
///
/// O código não tem bug: o desafio é o trace. Um painel de watch mostra as variáveis no
/// estado inicial, como num debugger pausado na primeira linha — não entrega a resposta,
/// ensina *o que* acompanhar. O console da saída prevista usa o **acento**: verde continua
/// sendo exclusivo do juiz.
struct DryRunPanel: View {
    let codeLines: [String]
    let watchVariables: [LogN.WatchVariable]
    let watchNote: String
    let predictedOutput: String
    let onOutputChange: (String) -> Void

    @FocusState private var isTyping: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            CodeBlock(lines: codeLines, fontSize: 12, lineHeight: 25)

            if !watchVariables.isEmpty {
                watchPanel
            }

            outputPanel
        }
    }

    // MARK: Painel de watch

    private var watchPanel: some View {
        VStack(spacing: 0) {
            HStack(spacing: 8) {
                Text("WATCH")
                    .font(.plexMono(10))
                    .tracking(0.14 * 10)
                    .foregroundColor(LognDark.textMuted)

                Spacer(minLength: 0)

                if !watchNote.isEmpty {
                    Text(watchNote)
                        .font(.plexMono(10))
                        .foregroundColor(LognDark.textDim)
                }
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 9)
            .overlay(alignment: .bottom) {
                Rectangle().frame(height: 1).foregroundColor(LognDark.line)
            }

            HStack(spacing: 0) {
                ForEach(Array(watchVariables.enumerated()), id: \.offset) { index, variable in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(variable.name)
                            .font(.plexMono(11))
                            .foregroundColor(LognDark.textMuted)
                        Text(variable.value)
                            .font(.plexMono(15))
                            .monospacedDigit()
                            .foregroundColor(LognDark.textPrimary)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 10)
                    .overlay(alignment: .trailing) {
                        if index < watchVariables.count - 1 {
                            Rectangle().frame(width: 1).foregroundColor(LognDark.line)
                        }
                    }
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("\(variable.name) vale \(variable.value)")
                }
            }
        }
        .background(LognDark.surfaceRaised)
        .cornerRadius(Radius.xs)
        .overlay(RoundedRectangle(cornerRadius: Radius.xs).stroke(LognDark.line, lineWidth: 1))
    }

    // MARK: Saída prevista

    private var outputPanel: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Text("SAÍDA PREVISTA")
                    .font(.plexMono(10))
                    .tracking(0.14 * 10)
                    .foregroundColor(LognDark.textMuted)

                Spacer(minLength: 0)

                // A sanitização é visível: o jogador sabe que espaço não conta.
                if !predictedOutput.isEmpty {
                    Text("espaços ignorados")
                        .font(.plexMono(10))
                        .foregroundColor(LognDark.info)
                }
            }

            HStack(spacing: 8) {
                Text(">")
                    .font(.plexMono(17))
                    .foregroundColor(LognDark.accent)

                TextField(
                    "",
                    text: Binding(get: { predictedOutput }, set: onOutputChange),
                    prompt: Text("toque para digitar")
                        .font(.plexMono(17))
                        .foregroundColor(LognDark.textDim)
                )
                .font(.plexMono(predictedOutput.isEmpty ? 17 : 19))
                .foregroundColor(LognDark.textPrimary)
                .tint(LognDark.accent)
                .focused($isTyping)
                .keyboardType(.numbersAndPunctuation)
                .autocorrectionDisabled()
                .textInputAutocapitalization(.never)
                .submitLabel(.done)
            }
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 13)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LognDark.canvas)
        .cornerRadius(Radius.xs)
        .overlay(
            RoundedRectangle(cornerRadius: Radius.xs)
                .stroke(isTyping || !predictedOutput.isEmpty ? LognDark.accent : LognDark.line, lineWidth: 1)
        )
        .contentShape(Rectangle())
        .onTapGesture { isTyping = true }
        .accessibilityLabel("saída prevista")
    }
}
