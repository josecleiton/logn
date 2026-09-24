import SwiftUI
import LogNCoreFFI

/// Primitiva do Balão LogN, desenhada a partir dos paths SVG do Design System.
///
/// **Dimensionamento:** `width` é a largura da *viewBox*, não do corpo — é assim que o
/// design especifica cada uso (64, 80, 56, 14, 13…). A altura sai da proporção da viewBox:
/// `0 0 96 95` sem cordinha, `0 0 96 150` com. O corpo ocupa x 16…80, ou seja 2/3 da largura.
struct BalloonShape: View {

    enum Style: Equatable {
        /// Problema aceito / nó conquistado — corpo preenchido na cor.
        case filled(Color)
        /// Em aberto — contorno na cor, com a espessura dada (unidades de viewBox).
        case outline(Color, CGFloat)
        /// Nó ativo da trilha — contorno accent, traço 5, brilho em accent.
        case active
        /// Bloqueado — silhueta murcha, contorno tracejado.
        case deflated
    }

    let style: Style
    /// Largura da viewBox. A altura é derivada.
    let width: CGFloat
    var showString: Bool = false
    var showHighlight: Bool = true

    // MARK: - Paths do Design System

    private enum D {
        static let body    = "M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z"
        static let knot    = "M41 76 L55 76 L48 91 Z"
        static let shine   = "M31 21 C27 27 25 33 25 40"
        static let string  = "M48 90 C38 98 58 106 48 114 C38 122 58 130 48 138"
        // Silhueta murcha do nó bloqueado — corpo menor, nó mais curto.
        static let bodyDeflated = "M48 14 C62 14 74 26 74 42 C74 56 62 68 52 74 L48 80 L44 74 C34 68 22 56 22 42 C22 26 34 14 48 14 Z"
        static let knotDeflated = "M42 74 L54 74 L48 88 Z"
    }

    private static let viewBoxWidth: CGFloat = 96
    private static let viewBoxHeightShort: CGFloat = 95
    private static let viewBoxHeightTall: CGFloat = 150

    var height: CGFloat {
        width * (showString ? Self.viewBoxHeightTall : Self.viewBoxHeightShort) / Self.viewBoxWidth
    }

    /// Largura do corpo do balão — o que o README do DS chama de "tamanho de corpo".
    var bodyWidth: CGFloat { width * 64 / Self.viewBoxWidth }

    var body: some View {
        Canvas { context, size in
            let s = size.width / Self.viewBoxWidth

            let isDeflated = style == .deflated
            let bodyPath = scaled(isDeflated ? D.bodyDeflated : D.body, s)
            let knotPath = scaled(isDeflated ? D.knotDeflated : D.knot, s)

            switch style {
            case .filled(let color):
                context.fill(bodyPath, with: .color(color))
                drawShine(context, s, color: .white.opacity(0.72))
                context.fill(knotPath, with: .color(color))

            case .outline(let color, let lineWidth):
                context.stroke(bodyPath, with: .color(color), lineWidth: lineWidth * s)
                context.fill(knotPath, with: .color(color))

            case .active:
                context.stroke(bodyPath, with: .color(LognDark.accent), lineWidth: 5 * s)
                drawShine(context, s, color: LognDark.accent.opacity(0.4))
                context.fill(knotPath, with: .color(LognDark.accent))

            case .deflated:
                context.stroke(
                    bodyPath,
                    with: .color(LognDark.lineDim),
                    style: StrokeStyle(lineWidth: 4 * s, dash: [5 * s, 4 * s])
                )
                context.fill(knotPath, with: .color(LognDark.lineDim))
            }

            if showString {
                context.stroke(
                    scaled(D.string, s),
                    with: .color(stringColor),
                    style: StrokeStyle(lineWidth: 5 * s, lineCap: .round)
                )
            }
        }
        .frame(width: width, height: height)
        .accessibilityHidden(true)
    }

    // MARK: -

    private func scaled(_ d: String, _ s: CGFloat) -> Path {
        SVGPath.path(d).applying(CGAffineTransform(scaleX: s, y: s))
    }

    /// O brilho especular só aparece em balão preenchido e some abaixo de 12dp de corpo.
    private func drawShine(_ context: GraphicsContext, _ s: CGFloat, color: Color) {
        guard showHighlight, bodyWidth >= 12 else { return }
        context.stroke(
            scaled(D.shine, s),
            with: .color(color),
            style: StrokeStyle(lineWidth: 5.5 * s, lineCap: .round)
        )
    }

    private var stringColor: Color {
        switch style {
        case .filled(let color):     return color
        case .outline(let color, _): return color
        case .active:                return LognDark.accent
        case .deflated:              return LognDark.lineDim
        }
    }
}

// MARK: - Símbolo da marca

/// O símbolo do LogN. Mesmo corpo, brilho e nó do balão de UI — só a cauda difere:
/// aqui ela é a **curva logarítmica**, não a cordinha ondulada.
///
/// ViewBox `0 0 96 122`. Abaixo de 24px o brilho especular sai.
struct BrandSymbol: View {
    var color: Color = LognDark.accent
    /// Altura do símbolo. A largura sai da proporção da viewBox.
    let height: CGFloat

    private static let viewBox = CGSize(width: 96, height: 122)

    private enum D {
        static let body  = "M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z"
        static let shine = "M31 21 C27 27 25 33 25 40"
        static let knot  = "M41 76 L55 76 L48 91 Z"
        /// Cauda: sobe rápido, depois estabiliza.
        static let tail  = "M48 90 C49 104 56 111 68 113 C78 115 84 115 90 116"
    }

    var width: CGFloat { height * Self.viewBox.width / Self.viewBox.height }

    var body: some View {
        Canvas { context, size in
            let s = size.height / Self.viewBox.height
            func p(_ d: String) -> Path {
                SVGPath.path(d).applying(CGAffineTransform(scaleX: s, y: s))
            }

            context.fill(p(D.body), with: .color(color))
            if width >= 24 {
                context.stroke(
                    p(D.shine),
                    with: .color(.white.opacity(0.72)),
                    style: StrokeStyle(lineWidth: 5.5 * s, lineCap: .round)
                )
            }
            context.fill(p(D.knot), with: .color(color))
            context.stroke(
                p(D.tail),
                with: .color(color),
                style: StrokeStyle(lineWidth: 5 * s, lineCap: .round)
            )
        }
        .frame(width: width, height: height)
        .accessibilityHidden(true)
    }
}

/// Lockup horizontal — uso diário: header, loja, e-mail.
/// Símbolo + `LogN` em Plex Sans SemiBold, tracking −0.035em, gap 14dp.
struct BrandLockup: View {
    var fontSize: CGFloat = 44
    var color: Color = LognDark.accent
    var textColor: Color = LognDark.textPrimary

    /// Proporção medida no lockup do design system: símbolo de 58px ao lado de texto
    /// de 46px. (O README fala em "1,27× a caixa-alta", mas o HTML — que é o que
    /// renderiza — usa 1,26× o corpo da fonte.)
    private var symbolHeight: CGFloat { fontSize * 58 / 46 }

    var body: some View {
        HStack(alignment: .center, spacing: 14) {
            BrandSymbol(color: color, height: symbolHeight)
            Text("LogN")
                .font(.plexSansSemiBold(fontSize))
                .tracking(-0.035 * fontSize)
                .foregroundColor(textColor)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("LogN")
    }
}

/// Fileira decorativa A—M das telas de entrada. Cinco balões no ar, o resto em aberto.
struct BalloonMarquee: View {
    var filledCount: Int = 5

    var body: some View {
        HStack(spacing: 7) {
            ForEach(Array(BalloonColor.all.enumerated()), id: \.element) { index, letter in
                BalloonShape(
                    style: index < filledCount
                        ? .filled(BalloonColor.forLetter(letter))
                        : .outline(LognDark.lineStrong, 5),
                    width: 13
                )
            }
        }
        .opacity(0.5)
        .accessibilityHidden(true)
    }
}

// MARK: - Balloon Colors (A—M)

enum BalloonColor {
    /// Cor da letra do problema. Endereço, nunca estado — acesse sempre por aqui.
    static func forLetter(_ letter: Character) -> Color {
        guard let ascii = letter.uppercased().first?.asciiValue,
              (65...77).contains(ascii) else { return LognDark.textMuted }
        return Balloon.of(letter)
    }

    static let all: [Character] = Array("ABCDEFGHIJKLM")
}
