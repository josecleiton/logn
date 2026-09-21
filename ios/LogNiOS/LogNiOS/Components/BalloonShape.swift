import SwiftUI

/// Primitiva do Balão LogN — desenhada a partir dos paths SVG do Design System.
/// ViewBox de referência: 0 0 96 150 (com cordinha) ou 0 0 96 95 (sem).
/// O corpo ocupa x 16…80 — 66.7% da largura da viewBox.
struct BalloonShape: View {
    let color: Color
    let state: BalloonState
    let bodySize: CGFloat // Largura do corpo do balão
    let showString: Bool
    let showHighlight: Bool

    enum BalloonState {
        case filled   // Conquistado / AC
        case active   // Nó ativo — contorno accent, traço 5
        case outline  // Em aberto — contorno `lineStrong`, opacidade 0.55
        case locked   // Bloqueado — contorno tracejado `lineDim`, silhueta murcha
    }

    init(
        color: Color = LognDark.accent,
        state: BalloonState = .filled,
        bodySize: CGFloat = 44,
        showString: Bool = true,
        showHighlight: Bool = true
    ) {
        self.color = color
        self.state = state
        self.bodySize = bodySize
        self.showString = showString
        // DS: brilho só aparece em balão preenchido e sai abaixo de 12dp de corpo
        self.showHighlight = showHighlight && state == .filled && bodySize >= 12
    }

    // viewBox: 0 0 96 150. Corpo em 16..80 = 64 unidades de largura.
    // Scale factor to map viewBox to our bodySize
    private var scale: CGFloat { bodySize / 64.0 }

    var body: some View {
        Canvas { context, size in
            let s = scale
            // Offset to center: corpo vai de x=16 a x=80, centro = 48
            let ox = size.width / 2 - 48 * s
            let oy: CGFloat = 0

            // --- Corpo ---
            var bodyPath = Path()
            bodyPath.move(to: p(48, 4, s, ox, oy))
            bodyPath.addCurve(to: p(80, 41, s, ox, oy), control1: p(66, 4, s, ox, oy), control2: p(80, 20, s, ox, oy))
            bodyPath.addCurve(to: p(53, 79, s, ox, oy), control1: p(80, 60, s, ox, oy), control2: p(66, 74, s, ox, oy))
            bodyPath.addLine(to: p(48, 83, s, ox, oy))
            bodyPath.addLine(to: p(43, 79, s, ox, oy))
            bodyPath.addCurve(to: p(16, 41, s, ox, oy), control1: p(30, 74, s, ox, oy), control2: p(16, 60, s, ox, oy))
            bodyPath.addCurve(to: p(48, 4, s, ox, oy), control1: p(16, 20, s, ox, oy), control2: p(30, 4, s, ox, oy))
            bodyPath.closeSubpath()

            switch state {
            case .filled:
                context.fill(bodyPath, with: .color(color))
            case .active:
                context.fill(bodyPath, with: .color(LognDark.accent))
            case .outline:
                // DS: br: t.line2 (we map line2 to lineStrong or line), op: 0.55 is applied on the container
                context.stroke(bodyPath, with: .color(LognDark.lineStrong), lineWidth: 4 * s)
            case .locked:
                context.stroke(bodyPath, with: .color(LognDark.lineDim), style: StrokeStyle(lineWidth: 3 * s, dash: [5 * s, 4 * s]))
            }

            // --- Brilho (Specular Highlight) ---
            if showHighlight {
                var highlightPath = Path()
                highlightPath.move(to: p(31, 21, s, ox, oy))
                highlightPath.addCurve(to: p(25, 40, s, ox, oy), control1: p(27, 27, s, ox, oy), control2: p(25, 33, s, ox, oy))
                context.stroke(
                    highlightPath,
                    with: .color(.white.opacity(0.72)),
                    style: StrokeStyle(lineWidth: 5.5 * s, lineCap: .round)
                )
            }

            // --- Nó ---
            var knotPath = Path()
            knotPath.move(to: p(41, 76, s, ox, oy))
            knotPath.addLine(to: p(55, 76, s, ox, oy))
            knotPath.addLine(to: p(48, 91, s, ox, oy))
            knotPath.closeSubpath()

            switch state {
            case .filled:
                context.fill(knotPath, with: .color(color))
            case .active:
                context.fill(knotPath, with: .color(LognDark.accent))
            case .outline:
                context.stroke(knotPath, with: .color(LognDark.lineStrong), lineWidth: 2 * s)
            case .locked:
                context.stroke(knotPath, with: .color(LognDark.lineDim), style: StrokeStyle(lineWidth: 2 * s, dash: [5 * s, 4 * s]))
            }

            // --- Cordinha (UI) ---
            if showString {
                var stringPath = Path()
                stringPath.move(to: p(48, 90, s, ox, oy))
                stringPath.addCurve(to: p(48, 114, s, ox, oy), control1: p(38, 98, s, ox, oy), control2: p(58, 106, s, ox, oy))
                stringPath.addCurve(to: p(48, 138, s, ox, oy), control1: p(38, 122, s, ox, oy), control2: p(58, 130, s, ox, oy))

                let stringColor: Color = {
                    switch state {
                    case .filled: return color
                    case .active: return LognDark.accent
                    case .outline: return LognDark.lineStrong
                    case .locked: return LognDark.lineDim
                    }
                }()

                if state == .locked {
                    context.stroke(stringPath, with: .color(stringColor), style: StrokeStyle(lineWidth: 4 * s, lineCap: .round, dash: [5 * s, 4 * s]))
                } else {
                    context.stroke(stringPath, with: .color(stringColor), style: StrokeStyle(lineWidth: 4 * s, lineCap: .round))
                }
            }
        }
        .frame(width: bodySize * 1.5, height: showString ? bodySize * 2.34 : bodySize * 1.45)
    }

    private func p(_ x: CGFloat, _ y: CGFloat, _ s: CGFloat, _ ox: CGFloat, _ oy: CGFloat) -> CGPoint {
        CGPoint(x: x * s + ox, y: y * s + oy)
    }
}

// MARK: - Balloon Colors (A—M)

enum BalloonColor {
    static func forLetter(_ letter: Character) -> Color {
        switch letter {
        case "A": return Color(hex: "E4572E")
        case "B": return Color(hex: "F5C451")
        case "C": return Color(hex: "3DB2FF")
        case "D": return Color(hex: "6BCB77")
        case "E": return Color(hex: "C77DFF")
        case "F": return Color(hex: "FF6FB5")
        case "G": return Color(hex: "4ECDC4")
        case "H": return Color(hex: "F4A261")
        case "I": return Color(hex: "9BC53D")
        case "J": return Color(hex: "D64550")
        case "K": return Color(hex: "7C8BFF")
        case "L": return Color(hex: "D8DEE4")
        case "M": return Color(hex: "00B894")
        default: return LognDark.textMuted
        }
    }
}
