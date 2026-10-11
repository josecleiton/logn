import SwiftUI
import LogNCoreFFI
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
                Text(Str.Dry_run.watch)
                    .font(.plexMono(10))
                    .tracking(0.14 * 10)
                    .foregroundColor(LognColors.textMuted)

                Spacer(minLength: 0)

                if !watchNote.isEmpty {
                    Text(watchNote)
                        .font(.plexMono(10))
                        .foregroundColor(LognColors.textMuted)
                }
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 9)
            .overlay(alignment: .bottom) {
                Rectangle().frame(height: 1).foregroundColor(LognColors.line)
            }

            HStack(spacing: 0) {
                ForEach(Array(watchVariables.enumerated()), id: \.offset) { index, variable in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(variable.name)
                            .font(.plexMono(11))
                            .foregroundColor(LognColors.textMuted)
                        Text(variable.value)
                            .font(.plexMono(15))
                            .monospacedDigit()
                            .foregroundColor(LognColors.textPrimary)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 10)
                    .overlay(alignment: .trailing) {
                        if index < watchVariables.count - 1 {
                            Rectangle().frame(width: 1).foregroundColor(LognColors.line)
                        }
                    }
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(Str.Dry_run.watch_accessibility(variable.name, variable.value))
                }
            }
        }
        .background(LognColors.surfaceRaised)
        .cornerRadius(Radius.xs)
        .overlay(RoundedRectangle(cornerRadius: Radius.xs).stroke(LognColors.line, lineWidth: 1))
    }

    // MARK: Saída prevista

    private var outputPanel: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Text(Str.Dry_run.expected_output)
                    .font(.plexMono(10))
                    .tracking(0.14 * 10)
                    .foregroundColor(LognColors.textMuted)

                Spacer(minLength: 0)

                // A sanitização é visível: o jogador sabe que espaço não conta.
                if !predictedOutput.isEmpty {
                    Text(Str.Dry_run.ignored_spaces)
                        .font(.plexMono(10))
                        .foregroundColor(LognColors.infoInk)
                }
            }

            HStack(spacing: 8) {
                Text(">")
                    .font(.plexMono(17))
                    .foregroundColor(LognColors.accentInk)

                TextField(
                    "",
                    text: Binding(get: { predictedOutput }, set: onOutputChange),
                    prompt: Text(Str.Dry_run.tap_to_type)
                        .font(.plexMono(17))
                        .foregroundColor(LognColors.textFaint)
                )
                .font(.plexMono(predictedOutput.isEmpty ? 17 : 19))
                .foregroundColor(LognColors.textPrimary)
                .tint(LognColors.accentInk)
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
        .background(LognColors.canvas)
        .cornerRadius(Radius.xs)
        // O foco e a resposta escrita só aparecem nesta borda: em tinta, que no claro passa
        // de 3:1.
        .overlay(
            RoundedRectangle(cornerRadius: Radius.xs)
                .stroke(isTyping || !predictedOutput.isEmpty ? LognColors.accentInk : LognColors.line, lineWidth: 1)
        )
        .contentShape(Rectangle())
        .onTapGesture { isTyping = true }
        .accessibilityLabel(Str.Dry_run.output_accessibility)
    }
}
