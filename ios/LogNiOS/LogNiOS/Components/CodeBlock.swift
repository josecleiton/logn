import SwiftUI

/// Bloco de código com numeração de linhas, syntax highlighting básico,
/// e suporte a seleção de linha (para SPOT_THE_BUG).
/// DS: Fundo `canvas`, borda `line`, raio 2dp. Plex Mono 13-15sp, entrelinha 24dp.
/// Linha realçada: fundo tint + barra lateral 2dp.
struct CodeBlock: View {
    let lines: [String]
    let selectedLine: Int? // 0-indexed
    let onSelectLine: ((Int) -> Void)?
    let highlightColor: Color // accent para seleção, correct/wrong para feedback

    init(
        lines: [String],
        selectedLine: Int? = nil,
        highlightColor: Color = LognDark.accent,
        onSelectLine: ((Int) -> Void)? = nil
    ) {
        self.lines = lines
        self.selectedLine = selectedLine
        self.highlightColor = highlightColor
        self.onSelectLine = onSelectLine
    }

    var body: some View {
        VStack(spacing: 0) {
            ForEach(Array(lines.enumerated()), id: \.offset) { index, line in
                HStack(spacing: 0) {
                    

                    // Número de linha
                    Text("\(index + 1)")
                        .font(.custom("IBMPlexMono-Regular", size: 13))
                        .foregroundColor(index == selectedLine ? highlightColor : LognDark.textMuted)
                        .frame(width: 32, alignment: .trailing)
                        .padding(.trailing, 16)

                    // Código com syntax highlighting
                    SyntaxHighlightedText(code: line)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .padding(.vertical, 8)
                .frame(minHeight: 34) 
                .background(index == selectedLine ? highlightColor.opacity(0.14) : .clear)
                .contentShape(Rectangle())
                .onTapGesture {
                    onSelectLine?(index)
                }

                
            }
        }
        .padding(.vertical, 8)
        .background(LognDark.canvas)
        .cornerRadius(2)
        .overlay(
            RoundedRectangle(cornerRadius: 2)
                .stroke(LognDark.line, lineWidth: 1)
        )
    }
}

/// Syntax highlighting básico para C/C++/pseudocode.
/// DS: keywords em `synKeyword`, funções em `synFunction`, texto base em `textSecondary`.
struct SyntaxHighlightedText: View {
    let code: String

    // Palavras-chave comuns
    private let keywords: Set<String> = [
        "int", "void", "return", "if", "else", "for", "while", "do",
        "break", "continue", "bool", "true", "false", "string",
        "char", "long", "double", "float", "auto", "const",
        "struct", "class", "public", "private", "static",
        "vector", "map", "set", "pair", "queue", "stack",
        "using", "namespace", "std", "include", "define"
    ]

    var body: some View {
        HStack(spacing: 0) {
            ForEach(Array(tokenize(code).enumerated()), id: \.offset) { _, token in
                switch token.kind {
                case .keyword:
                    Text(token.text).foregroundColor(LognDark.synKeyword)
                case .function:
                    Text(token.text).foregroundColor(LognDark.synFunction)
                case .number:
                    Text(token.text).foregroundColor(LognDark.warn)
                case .placeholder:
                    // DS: dashed box for placeholder
                    Text("____")
                        .foregroundColor(.clear)
                        .padding(.horizontal, 4)
                        .padding(.vertical, 2)
                        .overlay(
                            RoundedRectangle(cornerRadius: 2)
                                .stroke(LognDark.accent, style: StrokeStyle(lineWidth: 1, dash: [3]))
                        )
                default:
                    // Fix spaces rendering correctly in HStack by preserving them
                    Text(token.text).foregroundColor(LognDark.textSecondary)
                }
            }
        }
        .font(.custom("IBMPlexMono-Regular", size: 13))
    }

    private enum TokenKind {
        case keyword, function, number, placeholder, plain
    }

    private struct Token {
        let text: String
        let kind: TokenKind
    }

    private func tokenize(_ code: String) -> [Token] {
        var tokens: [Token] = []
        var current = ""
        let chars = Array(code)
        var i = 0

        while i < chars.count {
            let c = chars[i]

            if c == "_" && i + 4 < chars.count && String(chars[i..<i+5]) == "_____" {
                if !current.isEmpty { tokens.append(Token(text: current, kind: .plain)); current = "" }
                tokens.append(Token(text: "_____", kind: .placeholder))
                i += 5
                continue
            }

            if c.isLetter || c == "_" || c == "#" {
                if !current.isEmpty && !current.last!.isLetter && current.last != "_" {
                    tokens.append(Token(text: current, kind: .plain))
                    current = ""
                }
                current.append(c)
            } else if c.isNumber {
                if !current.isEmpty && !current.last!.isNumber && !current.last!.isLetter {
                    tokens.append(Token(text: current, kind: .plain))
                    current = ""
                }
                current.append(c)
            } else {
                if !current.isEmpty {
                    // Classify the accumulated word
                    if keywords.contains(current) {
                        tokens.append(Token(text: current, kind: .keyword))
                    } else if c == "(" {
                        tokens.append(Token(text: current, kind: .function))
                    } else if current.allSatisfy({ $0.isNumber }) {
                        tokens.append(Token(text: current, kind: .number))
                    } else {
                        tokens.append(Token(text: current, kind: .plain))
                    }
                    current = ""
                }
                tokens.append(Token(text: String(c), kind: .plain))
            }
            i += 1
        }

        if !current.isEmpty {
            if keywords.contains(current) {
                tokens.append(Token(text: current, kind: .keyword))
            } else if current.allSatisfy({ $0.isNumber }) {
                tokens.append(Token(text: current, kind: .number))
            } else {
                tokens.append(Token(text: current, kind: .plain))
            }
        }

        return tokens
    }
}
