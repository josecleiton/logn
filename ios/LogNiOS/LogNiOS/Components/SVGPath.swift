import SwiftUI
import LogNCoreFFI

/// Converte a string `d` de um path SVG do Design System em um `Path` do SwiftUI.
///
/// O DS entrega as formas como paths SVG (balão, cordinha, ícones de assunto) e manda
/// desenhar "a partir do mesmo path, não reconstruir com formas primitivas". Este parser
/// existe para que os comandos possam ser colados literalmente do design.
///
/// Suporta os comandos usados pelo DS: `M m L l H h V v C c Z z`.
enum SVGPath {

    /// Constrói o path já escalado de `viewBox` para `size`, preservando a proporção
    /// e centralizando o resultado — equivalente a `preserveAspectRatio="xMidYMid meet"`.
    static func path(_ d: String, viewBox: CGSize, in size: CGSize) -> Path {
        let scale = min(size.width / viewBox.width, size.height / viewBox.height)
        let dx = (size.width - viewBox.width * scale) / 2
        let dy = (size.height - viewBox.height * scale) / 2
        return path(d).applying(
            CGAffineTransform(translationX: dx, y: dy).scaledBy(x: scale, y: scale)
        )
    }

    /// Constrói o path em coordenadas cruas da viewBox.
    static func path(_ d: String) -> Path {
        var path = Path()
        var cursor = CGPoint.zero
        var subpathStart = CGPoint.zero
        var tokens = Tokenizer(d)
        var command: Character = "M"

        while let next = tokens.peekCommandOrNumber() {
            if let letter = next.command {
                command = letter
                tokens.consumeCommand()
                if command == "Z" || command == "z" {
                    path.closeSubpath()
                    cursor = subpathStart
                    continue
                }
            }

            let relative = command.isLowercase
            switch Character(command.uppercased()) {
            case "M":
                guard let p = tokens.point(relativeTo: relative ? cursor : .zero) else { return path }
                path.move(to: p)
                cursor = p
                subpathStart = p
                // Um `M` com coordenadas extras vira `L`, como manda a spec SVG.
                command = relative ? "l" : "L"

            case "L":
                guard let p = tokens.point(relativeTo: relative ? cursor : .zero) else { return path }
                path.addLine(to: p)
                cursor = p

            case "H":
                guard let x = tokens.number() else { return path }
                let p = CGPoint(x: relative ? cursor.x + x : x, y: cursor.y)
                path.addLine(to: p)
                cursor = p

            case "V":
                guard let y = tokens.number() else { return path }
                let p = CGPoint(x: cursor.x, y: relative ? cursor.y + y : y)
                path.addLine(to: p)
                cursor = p

            case "C":
                let origin = relative ? cursor : .zero
                guard let c1 = tokens.point(relativeTo: origin),
                      let c2 = tokens.point(relativeTo: origin),
                      let end = tokens.point(relativeTo: origin) else { return path }
                path.addCurve(to: end, control1: c1, control2: c2)
                cursor = end

            default:
                // Comando não usado pelo DS — aborta em vez de desenhar algo errado.
                return path
            }
        }

        return path
    }

    // MARK: - Tokenizer

    private struct Tokenizer {
        private let chars: [Character]
        private var i = 0

        init(_ s: String) { chars = Array(s) }

        struct Next { let command: Character? }

        mutating func peekCommandOrNumber() -> Next? {
            skipSeparators()
            guard i < chars.count else { return nil }
            let c = chars[i]
            if c.isLetter { return Next(command: c) }
            return Next(command: nil)
        }

        mutating func consumeCommand() { i += 1 }

        mutating func point(relativeTo origin: CGPoint) -> CGPoint? {
            guard let x = number(), let y = number() else { return nil }
            return CGPoint(x: origin.x + x, y: origin.y + y)
        }

        mutating func number() -> CGFloat? {
            skipSeparators()
            guard i < chars.count else { return nil }
            let start = i
            if chars[i] == "-" || chars[i] == "+" { i += 1 }
            while i < chars.count, chars[i].isNumber || chars[i] == "." { i += 1 }
            // Notação científica não aparece no DS, mas custa pouco aceitar.
            if i < chars.count, chars[i] == "e" || chars[i] == "E" {
                i += 1
                if i < chars.count, chars[i] == "-" || chars[i] == "+" { i += 1 }
                while i < chars.count, chars[i].isNumber { i += 1 }
            }
            guard i > start, let value = Double(String(chars[start..<i])) else { return nil }
            return CGFloat(value)
        }

        private mutating func skipSeparators() {
            while i < chars.count, chars[i] == " " || chars[i] == "," || chars[i] == "\n" || chars[i] == "\t" {
                i += 1
            }
        }
    }
}
