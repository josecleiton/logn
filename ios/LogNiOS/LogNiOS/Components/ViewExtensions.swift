import SwiftUI
import LogNCoreFFI

extension View {
    func cornerRadius(_ radius: CGFloat, corners: UIRectCorner) -> some View {
        clipShape(RoundedCorner(radius: radius, corners: corners))
    }

    /// Tela de formulário com `Spacer`: sem teclado ocupa a altura toda, com teclado rola.
    /// Sem isto, o teclado encolhe a área, o conteúdo não cabe e o `VStack` inteiro é
    /// recentralizado num salto, de novo a cada campo, porque o teclado do e-mail e o da
    /// senha têm alturas diferentes. É o `verticalScroll` com `heightIn(min)` do Android.
    func scrollsAboveKeyboard() -> some View {
        GeometryReader { geo in
            ScrollView {
                frame(minHeight: geo.size.height)
            }
            .scrollDismissesKeyboard(.interactively)
        }
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
