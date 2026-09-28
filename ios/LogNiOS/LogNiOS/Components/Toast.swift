import SwiftUI
import UIKit

/// Aviso curto que desce da Dynamic Island, fica alguns segundos e some. Responde a um
/// toque; estado que persiste (core iniciando, erro do Core) continua na `StatusLine`.
///
/// Spec: `LogN Toast.dc.html` do design system. Um toast por vez, sem pilha nem fila:
/// o mais novo descreve o estado atual. Com o mesmo `id`, o aviso só pulsa e reinicia o
/// tempo; com outro, substitui o que está na tela.
enum ToastVariant {
    case attention, error, success

    var symbol: String {
        switch self {
        case .attention: return "exclamationmark.circle.fill"
        case .error: return "xmark.circle.fill"
        case .success: return "checkmark.circle.fill"
        }
    }

    var tint: Color {
        switch self {
        case .attention: return LognDark.warn
        case .error: return LognDark.wrong
        case .success: return LognDark.correct
        }
    }

    /// Erro fica mais porque costuma pedir releitura; sucesso é o mais curto.
    var duration: TimeInterval {
        switch self {
        case .attention: return 3.5
        case .error: return 5
        case .success: return 2.5
        }
    }

    /// Cor e ícone não chegam ao leitor de tela: a variante vai falada na frente.
    var spokenWord: String {
        switch self {
        case .attention: return Str.Toast.attention
        case .error: return Str.Toast.error
        case .success: return Str.Toast.success
        }
    }

    var haptic: UINotificationFeedbackGenerator.FeedbackType {
        switch self {
        case .attention: return .warning
        case .error: return .error
        case .success: return .success
        }
    }
}

struct ToastMessage: Equatable {
    let id: String
    let variant: ToastVariant
    let text: String
    /// Troca o ícone da variante quando o caso pede outro (`clock.fill`, `wifi.slash`).
    var symbol: String? = nil
}

@MainActor
final class ToastCenter: ObservableObject {
    /// Um só para o app: quem avisa não precisa receber o centro pelo ambiente, e a tela
    /// que chama pode estar num `sheet` que não herdou o `environmentObject`.
    static let shared = ToastCenter()

    @Published private(set) var current: ToastMessage?
    /// Muda a cada repetição do mesmo aviso; a pílula pulsa quando vê a mudança.
    @Published private(set) var pulse = 0

    private var timer: Task<Void, Never>?
    private var swap: Task<Void, Never>?

    func show(_ variant: ToastVariant, _ text: String, id: String, symbol: String? = nil) {
        let message = ToastMessage(id: id, variant: variant, text: text, symbol: symbol)
        if current?.id == id {
            // Tocar três vezes não faz descer três avisos nem falar três vezes.
            pulse += 1
            arm(duration(of: message))
            return
        }
        swap?.cancel()
        if current != nil {
            // O atual sobe mais rápido que no fim do tempo, e só então o novo desce.
            timer?.cancel()
            withAnimation(.easeIn(duration: 0.2)) { current = nil }
            swap = Task { [weak self] in
                try? await Task.sleep(nanoseconds: 220_000_000)
                guard !Task.isCancelled else { return }
                self?.enter(message)
            }
        } else {
            enter(message)
        }
    }

    func dismiss() {
        timer?.cancel()
        swap?.cancel()
        withAnimation(Self.leaveAnimation) { current = nil }
    }

    /// O dedo na pílula segura o relógio.
    func hold() { timer?.cancel() }

    /// Soltou sem dispensar: sobra pouco tempo, que a pessoa já leu.
    func release() { arm(1.5) }

    private func enter(_ message: ToastMessage) {
        withAnimation(Self.enterAnimation) { current = message }
        UINotificationFeedbackGenerator().notificationOccurred(message.variant.haptic)
        announce(message)
        arm(duration(of: message))
    }

    private func arm(_ seconds: TimeInterval) {
        timer?.cancel()
        timer = Task { [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000))
            guard !Task.isCancelled else { return }
            self?.dismiss()
        }
    }

    private func duration(of message: ToastMessage) -> TimeInterval {
        // Uns 32 caracteres cabem numa linha em 393 pt; a segunda linha ganha 1 s.
        var seconds = message.variant.duration + (message.text.count > 32 ? 1 : 0)
        if UIAccessibility.isVoiceOverRunning { seconds *= 2 }
        return seconds
    }

    private func announce(_ message: ToastMessage) {
        let text = "\(message.variant.spokenWord). \(message.text)"
        // Prioridade alta não corta nem é cortada por outra fala; só existe do iOS 17.
        if #available(iOS 17.0, *) {
            let phrase = NSAttributedString(
                string: text,
                attributes: [.accessibilitySpeechAnnouncementPriority: UIAccessibilityPriority.high]
            )
            UIAccessibility.post(notification: .announcement, argument: phrase)
        } else {
            UIAccessibility.post(notification: .announcement, argument: text)
        }
    }

    // Sem Reduzir Movimento: mola que passa um pouco do ponto, como nos avisos do sistema.
    private static var enterAnimation: Animation {
        UIAccessibility.isReduceMotionEnabled
            ? .easeInOut(duration: 0.2)
            : .spring(response: 0.42, dampingFraction: 0.68)
    }

    private static var leaveAnimation: Animation {
        UIAccessibility.isReduceMotionEnabled ? .easeInOut(duration: 0.2) : .easeIn(duration: 0.26)
    }
}

/// A pílula por cima de tudo, no topo. Fora dela os toques passam para a tela.
struct ToastHost: View {
    @ObservedObject var center = ToastCenter.shared
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    @State private var dragY: CGFloat = 0
    @State private var pulsing = false

    var body: some View {
        VStack {
            if let message = center.current {
                pill(message)
                    .offset(y: dragY)
                    .scaleEffect(pulsing ? 1.04 : 1)
                    .gesture(dismissDrag)
                    .transition(transition)
                    // Some antes de alguém chegar nela; o texto já saiu como anúncio.
                    .accessibilityHidden(true)
                    .id(message.id)
            }
        }
        .padding(.horizontal, Space.screenMargin)
        .padding(.top, Space.xs)
        .onChange(of: center.pulse) { _ in bump() }
        .onChange(of: center.current) { _ in dragY = 0 }
    }

    private func pill(_ message: ToastMessage) -> some View {
        HStack(spacing: Space.sm) {
            Image(systemName: message.symbol ?? message.variant.symbol)
                .font(.system(size: 20))
                .foregroundColor(message.variant.tint)
            Text(message.text)
                .font(.plexSansMedium(15, relativeTo: .callout))
                .foregroundColor(LognDark.textPrimary)
                // Sem reticências: texto que não cabe em duas linhas se reescreve.
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(.leading, Space.md)
        .padding(.trailing, Space.lg)
        .padding(.vertical, Space.md)
        .frame(minHeight: 48)
        .background(LognDark.surfaceRaised)
        // O raio foge da régua `Radius` de propósito: é a forma dos avisos do sistema
        // que faz a pílula ser reconhecida. Com raio 8 ela pareceria um card.
        .clipShape(RoundedRectangle(cornerRadius: 24, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 24, style: .continuous)
                .stroke(LognDark.lineStrong, lineWidth: 1)
        )
        .shadow(color: .black.opacity(0.55), radius: 15, y: 10)
    }

    private var transition: AnyTransition {
        reduceMotion
            ? .opacity
            : .move(edge: .top).combined(with: .scale(scale: 0.9, anchor: .top)).combined(with: .opacity)
    }

    /// Para cima segue o dedo; para baixo, elástico a 20%. Soltar a mais de 24 pt acima,
    /// ou jogando para cima, dispensa.
    private var dismissDrag: some Gesture {
        DragGesture(minimumDistance: 0)
            .onChanged { value in
                center.hold()
                let dy = value.translation.height
                dragY = dy > 0 ? dy * 0.2 : dy
            }
            .onEnded { value in
                if dragY < -24 || value.predictedEndTranslation.height < -60 {
                    center.dismiss()
                } else {
                    withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) { dragY = 0 }
                    center.release()
                }
            }
    }

    private func bump() {
        guard !reduceMotion else { return }
        withAnimation(.easeOut(duration: 0.16)) { pulsing = true }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.17) {
            withAnimation(.easeOut(duration: 0.16)) { pulsing = false }
        }
    }
}
