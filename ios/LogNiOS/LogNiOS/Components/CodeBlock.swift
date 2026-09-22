import SwiftUI

/// Bloco de código com numeração de linha, realce de sintaxe e seleção de linha.
///
/// Exploração `3b · Contest`: fundo `surface`, borda `line`, raio 4dp, Plex Mono 12.5sp com
/// entrelinha 33dp, divisor `rowLine` entre linhas. Linha selecionada ganha fundo `accentTint`
/// e barra lateral 2dp em `accent`.
///
/// Sem scroll horizontal — linhas longas quebram preservando a indentação.
struct CodeBlock: View {
    let lines: [String]
    /// Índice 0-based da linha selecionada.
    let selectedLine: Int?
    let onSelectLine: ((Int) -> Void)?
    /// `accent` durante a escolha; `correct`/`wrong` quando o juiz já respondeu.
    /// É cor de **linha** — a tinta do número sai de `highlightInk`.
    let highlightColor: Color
    let highlightInk: Color
    let fontSize: CGFloat
    let lineHeight: CGFloat
    let cornerRadius: CGFloat
    /// Em SPOT_THE_BUG o bloco ocupa a altura que sobra (o `flex:1` do documento):
    /// a moldura vai até o CTA em vez de parar na última linha.
    var fillsHeight: Bool = false

    /// Um bloco tocável precisa de divisor e de linha alta o bastante para ser alvo;
    /// um bloco só de leitura é mais denso e não se fatia.
    private var isSelectable: Bool { onSelectLine != nil }

    init(
        lines: [String],
        selectedLine: Int? = nil,
        highlightColor: Color = LognDark.accent,
        highlightInk: Color = LognDark.accentInk,
        fontSize: CGFloat? = nil,
        lineHeight: CGFloat? = nil,
        cornerRadius: CGFloat? = nil,
        fillsHeight: Bool = false,
        onSelectLine: ((Int) -> Void)? = nil
    ) {
        self.lines = lines
        self.selectedLine = selectedLine
        self.highlightColor = highlightColor
        self.highlightInk = highlightInk
        self.onSelectLine = onSelectLine
        self.fillsHeight = fillsHeight

        let selectable = onSelectLine != nil
        // SPOT_THE_BUG (3b): 12.5 / 33dp, raio 4. Bloco inerte (DS): 13 / 26dp, raio 2.
        self.fontSize = fontSize ?? (selectable ? 12.5 : 13)
        self.lineHeight = lineHeight ?? (selectable ? 33 : 26)
        self.cornerRadius = cornerRadius ?? (selectable ? Radius.sm : Radius.xs)
    }

    var body: some View {
        VStack(spacing: 0) {
            ForEach(Array(lines.enumerated()), id: \.offset) { index, line in
                CodeLineRow(
                    number: index + 1,
                    code: line,
                    isSelected: index == selectedLine,
                    showsDivider: isSelectable && index < lines.count - 1,
                    highlightColor: highlightColor,
                    highlightInk: highlightInk,
                    fontSize: fontSize,
                    lineHeight: lineHeight,
                    onTap: onSelectLine.map { handler in { handler(index) } }
                )
            }
            if fillsHeight {
                Spacer(minLength: 0)
            }
        }
        .frame(maxHeight: fillsHeight ? .infinity : nil, alignment: .top)
        .padding(.vertical, isSelectable ? 0 : 8)
        .background(LognDark.surface)
        .cornerRadius(cornerRadius)
        .overlay(
            RoundedRectangle(cornerRadius: cornerRadius)
                .stroke(LognDark.line, lineWidth: 1)
        )
    }
}

/// Uma linha do bloco. Alvo de toque = a linha inteira; as linhas são contíguas, então
/// não há área morta entre elas. A escolha só vira resposta no CTA, que nomeia a linha.
private struct CodeLineRow: View {
    let number: Int
    let code: String
    let isSelected: Bool
    let showsDivider: Bool
    /// Cor de linha: barra lateral e fundo.
    let highlightColor: Color
    /// Cor de tinta: o número da linha realçada, que é glifo.
    let highlightInk: Color
    let fontSize: CGFloat
    let lineHeight: CGFloat
    let onTap: (() -> Void)?

    /// A lacuna do FILL_IN_THE_BLANK é uma caixa tracejada, não um texto sublinhado.
    private var blankRange: Range<String.Index>? { code.range(of: "_____") }

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Text("\(number)")
                .font(.plexMono(fontSize))
                .monospacedDigit()
                .foregroundColor(isSelected ? highlightInk : LognDark.textMuted)
                .frame(width: 12, alignment: .trailing)

            if let blankRange {
                HStack(spacing: 0) {
                    Text(SyntaxHighlighter.highlight(String(code[code.startIndex..<blankRange.lowerBound])))
                    blankBox
                    Text(SyntaxHighlighter.highlight(String(code[blankRange.upperBound...])))
                }
                .font(.plexMono(fontSize))
                .frame(maxWidth: .infinity, alignment: .leading)
            } else {
                Text(SyntaxHighlighter.highlight(code, isEmphasised: isSelected))
                    .font(.plexMono(fontSize))
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(.horizontal, 12)
        .frame(minHeight: lineHeight, alignment: .center)
        .background(background)
        .overlay(alignment: .leading) { sideBar }
        .overlay(alignment: .bottom) { divider }
        .contentShape(Rectangle())
        .onTapGesture { onTap?() }
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(isSelected ? [.isSelected] : [])
    }

    /// Lacuna inline: min-width 74dp, altura 30dp, tracejado `accent`, raio 2dp.
    private var blankBox: some View {
        RoundedRectangle(cornerRadius: Radius.xs)
            .strokeBorder(LognDark.accent, style: StrokeStyle(lineWidth: 1, dash: [3]))
            .frame(minWidth: 74, maxWidth: 74, minHeight: 30)
            .padding(.horizontal, 2)
            .accessibilityLabel("lacuna a preencher")
    }

    private var background: Color {
        isSelected ? highlightColor.opacity(0.14) : Color.clear
    }

    /// Barra lateral 2dp — o estado nunca depende só do fundo.
    @ViewBuilder
    private var sideBar: some View {
        if isSelected {
            Rectangle().frame(width: 2).foregroundColor(highlightColor)
        }
    }

    @ViewBuilder
    private var divider: some View {
        if showsDivider {
            Rectangle().frame(height: 1).foregroundColor(LognDark.rowLine)
        }
    }
}

// MARK: - Realce de sintaxe

/// Realce básico para o pseudocódigo do LogN (sabor C/C++/Kotlin).
///
/// DS: keywords em `synKeyword`, chamadas em `synFunction`, números em `warn`,
/// texto base em `textSecondary`, lacuna em `accent` tracejado.
enum SyntaxHighlighter {

    private static let keywords: Set<String> = [
        // C / C++
        "int", "void", "return", "if", "else", "for", "while", "do",
        "break", "continue", "bool", "true", "false", "string",
        "char", "long", "double", "float", "auto", "const",
        "struct", "class", "public", "private", "static",
        "vector", "map", "set", "pair", "queue", "stack",
        "using", "namespace", "std", "include", "define",
        // Kotlin — o dialeto usado nas telas do documento de gameplay
        "fun", "val", "var", "in", "is", "when", "null",
    ]

    static func highlight(_ code: String, isEmphasised: Bool = false) -> AttributedString {
        var out = AttributedString()
        let baseColor = isEmphasised ? LognDark.textPrimary : LognDark.textSecondary

        for token in tokenize(code) {
            var piece = AttributedString(token.text)
            switch token.kind {
            case .keyword:     piece.foregroundColor = LognDark.synKeyword
            case .function:    piece.foregroundColor = LognDark.synFunction
            case .number:      piece.foregroundColor = LognDark.warn
            case .placeholder: piece.foregroundColor = LognDark.accent
            case .plain:       piece.foregroundColor = baseColor
            }
            out.append(piece)
        }
        return out
    }

    private enum Kind { case keyword, function, number, placeholder, plain }
    private struct Token { let text: String; let kind: Kind }

    private static func tokenize(_ code: String) -> [Token] {
        var tokens: [Token] = []
        let chars = Array(code)
        var i = 0

        func isWordChar(_ c: Character) -> Bool { c.isLetter || c.isNumber || c == "_" || c == "#" }

        while i < chars.count {
            // Lacuna do FILL_IN_THE_BLANK.
            if chars[i] == "_", chars[i...].prefix(5).count == 5, String(chars[i..<i+5]) == "_____" {
                tokens.append(Token(text: "_____", kind: .placeholder))
                i += 5
                continue
            }

            if isWordChar(chars[i]) {
                let start = i
                while i < chars.count, isWordChar(chars[i]) { i += 1 }
                let word = String(chars[start..<i])

                let kind: Kind
                if keywords.contains(word) {
                    kind = .keyword
                } else if word.allSatisfy({ $0.isNumber }) {
                    kind = .number
                } else if i < chars.count, chars[i] == "(" {
                    kind = .function
                } else {
                    kind = .plain
                }
                tokens.append(Token(text: word, kind: kind))
                continue
            }

            // Corre a sequência de não-palavra de uma vez, para não fatiar espaços.
            let start = i
            while i < chars.count, !isWordChar(chars[i]), chars[i] != "_" { i += 1 }
            if i == start { i += 1 }
            tokens.append(Token(text: String(chars[start..<i]), kind: .plain))
        }

        return tokens
    }
}
