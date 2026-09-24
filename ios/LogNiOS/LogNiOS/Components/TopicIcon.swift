import SwiftUI
import LogNCoreFFI

/// Ícone de assunto da trilha. ViewBox 24×24, traço 2, cap e join redondos.
///
/// O traço escala junto com o ícone para manter a proporção do desenho — o DS especifica
/// traço 2 sobre 24 unidades de viewBox, não 2pt absolutos.
struct TopicIcon: View {
    let topic: LognTopic
    let color: Color
    let size: CGFloat
    /// Traço em unidades da viewBox 24×24. O DS usa 2; dentro do balão o desenho
    /// aparece engrossado (2.2 no nó ativo, 2.4 no nó conquistado).
    var lineWidth: CGFloat = 2

    private static let viewBox = CGSize(width: 24, height: 24)

    var body: some View {
        Canvas { context, canvasSize in
            let scale = min(canvasSize.width / Self.viewBox.width,
                            canvasSize.height / Self.viewBox.height)
            let style = StrokeStyle(lineWidth: lineWidth * scale, lineCap: .round, lineJoin: .round)

            for element in topic.icon {
                switch element {
                case .stroke(let d):
                    let path = SVGPath.path(d, viewBox: Self.viewBox, in: canvasSize)
                    context.stroke(path, with: .color(color), style: style)

                case .circle(let x, let y, let r):
                    let center = CGPoint(x: x * scale, y: y * scale)
                    let rect = CGRect(x: center.x - r * scale, y: center.y - r * scale,
                                      width: r * 2 * scale, height: r * 2 * scale)
                    context.stroke(Path(ellipseIn: rect), with: .color(color), style: style)
                }
            }
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}
