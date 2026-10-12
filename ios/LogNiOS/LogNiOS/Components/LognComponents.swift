import SwiftUI
import LogNCoreFFI

// MARK: - LognButton

/// Botão do LogN. Altura 52dp, largura total, raio 4dp. Um primário por tela —
/// sempre o que avança a partida.
struct LognButton: View {
    let title: String
    let variant: Variant
    let action: () -> Void
    var isDisabled: Bool = false
    var isLoading: Bool = false

    enum Variant { case primary, secondary, ghost }

    var body: some View {
        Button(action: action) {
            Group {
                if isLoading {
                    ProgressView()
                        .progressViewStyle(CircularProgressViewStyle(tint: foregroundColor))
                } else {
                    Text(title)
                        .font(.custom(fontName, size: 15))
                }
            }
            .frame(maxWidth: .infinity)
            .frame(height: 52)
            .foregroundColor(foregroundColor)
            .background(backgroundColor)
            .cornerRadius(Radius.sm)
            .overlay(
                    variant == .secondary
                        ? RoundedRectangle(cornerRadius: Radius.sm)
                            .stroke(LognColors.lineStrong, lineWidth: 1)
                        : nil
                )
        }
        .buttonStyle(PressSinkStyle())
        .disabled(isDisabled || isLoading)
    }

    /// Primário é SemiBold; secundário e ghost são Medium.
    private var fontName: String {
        variant == .primary ? Plex.sansSemiBold : Plex.sansMedium
    }

    private var foregroundColor: Color {
        if isDisabled { return LognColors.textDim }
        switch variant {
        case .primary:   return LognColors.onAccent
        case .secondary: return LognColors.textPrimary
        case .ghost:     return LognColors.textSecondary
        }
    }

    private var backgroundColor: Color {
        if isDisabled { return LognColors.buttonDisabled }
        switch variant {
        case .primary:             return LognColors.accent
        case .secondary, .ghost:   return .clear
        }
    }
}

/// Pressed = translateY(1dp). Sem sombra, sem escala — hierarquia vem de linha e luminância.
struct PressSinkStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .offset(y: configuration.isPressed ? 1 : 0)
    }
}

// MARK: - PasswordField

extension Image {
    /// A caixa de marcar (`checkmark.square.fill` ou `square`). Marcada, o quadrado fica na
    /// base do acento e o visto, que é o estado, sai escuro no claro (`onAccent`, 7.5:1), como
    /// o texto do botão primário; o canvas vazado ali daria 2.4:1. No escuro o visto continua
    /// vazado, como sempre foi. Vazia, `textDim`.
    @ViewBuilder
    func lognCheckbox(checked: Bool, _ scheme: ColorScheme) -> some View {
        if checked && scheme == .light {
            symbolRenderingMode(.palette).foregroundStyle(LognColors.onAccent, LognColors.accent)
        } else {
            foregroundColor(checked ? LognColors.accent : LognColors.textDim)
        }
    }
}

/// O exemplo de um campo que só tem título. No claro, em `textFaint`: o cinza do sistema
/// fica em 1.7:1 sobre o branco. No escuro, `nil`, e o campo mostra o do sistema, como
/// sempre mostrou (ADR 0031).
func lightPrompt(_ text: String, _ scheme: ColorScheme) -> Text? {
    scheme == .light ? Text(text).foregroundColor(LognColors.textFaint) : nil
}

/// O `SecureField` com o olho na borda direita, que mostra e esconde a senha. Quem usa
/// aplica o mesmo visual de antes (padding, fundo, borda, `textContentType`), que vale
/// para o par de campos por dentro. Some da tela, a senha volta a ficar escondida.
struct PasswordField: View {
    let placeholder: String
    @Binding var text: String
    var prompt: Text? = nil

    @State private var revealed = false
    @FocusState private var focused: Bool
    @Environment(\.colorScheme) private var colorScheme

    /// Sem exemplo próprio, o do tema (`lightPrompt`).
    private var shownPrompt: Text? { prompt ?? lightPrompt(placeholder, colorScheme) }

    init(_ placeholder: String, text: Binding<String>, prompt: Text? = nil) {
        self.placeholder = placeholder
        self._text = text
        self.prompt = prompt
    }

    var body: some View {
        HStack(spacing: 0) {
            field
            Button(action: toggle) {
                Image(systemName: revealed ? "eye.slash" : "eye")
                    .font(.system(size: 15))
                    .foregroundColor(LognColors.textDim)
                    .frame(width: Space.minTouch, height: Space.minTouch)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            // O alvo de 44 come parte do padding do campo, e o olho fica perto da borda.
            .padding(.trailing, -Space.md)
            .accessibilityLabel(revealed ? Str.Field.hide_password : Str.Field.show_password)
        }
    }

    // Os dois campos trocam de lugar; o foco vai junto, e o teclado não fecha.
    @ViewBuilder
    private var field: some View {
        if revealed {
            TextField(placeholder, text: $text, prompt: shownPrompt)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .focused($focused)
        } else {
            SecureField(placeholder, text: $text, prompt: shownPrompt)
                .focused($focused)
        }
    }

    private func toggle() {
        let hadFocus = focused
        revealed.toggle()
        if hadFocus {
            DispatchQueue.main.async { focused = true }
        }
    }
}

// MARK: - LifeBar

/// 3 corações. Cheio = `wrong`; vazio = `heartOff`.
///
/// O contador mono ao lado é o fallback exigido pelo DS — a barra nunca comunica só por
/// cor. Em partida o header usa a forma compacta (`showsCounter: false`), que é a do
/// documento de gameplay, e carrega a mesma informação no rótulo de acessibilidade.
struct LifeBar: View {
    let lives: Int
    var maxLives: Int = 3
    var showsCounter: Bool = true
    var heartSize: CGFloat = 20

    var body: some View {
        HStack(spacing: 4) {
            ForEach(0..<maxLives, id: \.self) { i in
                Image(systemName: i < lives ? "heart.fill" : "heart")
                    .font(.system(size: heartSize))
                    .foregroundColor(i < lives ? LognColors.wrong : LognColors.heartOff)
            }
            if showsCounter {
                Text("\(lives) / \(maxLives)")
                    .lognLabel()
                    .monospacedDigit()
                    .foregroundColor(LognColors.textSecondary)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(Str.Profile.lives_accessibility(lives, maxLives))
    }
}

// MARK: - ContestClock

/// O formato do relógio da partida: `mm:ss`, e `hh:mm:ss` a partir de uma hora. A tela do
/// relógio de contest que morava aqui nunca entrou no app, e saiu com o texto fixo dela.
enum ContestClock {
    static func format(_ seconds: Int) -> String {
        if seconds >= 3600 {
            return String(format: "%02d:%02d:%02d", seconds / 3600, (seconds % 3600) / 60, seconds % 60)
        }
        return String(format: "%02d:%02d", seconds / 60, seconds % 60)
    }
}

// MARK: - VerdictChip

/// Chip de veredito do juiz. Altura 26dp, min-width 46dp, raio 2dp.
/// A sigla é a do juiz, sem tradução.
struct VerdictChip: View {
    let verdict: Verdict

    enum Verdict: String {
        case ac  = "AC"
        case wa  = "WA"
        case tle = "TLE"
        case mle = "MLE"
        case re  = "RE"
        case ce  = "CE"
        case pe  = "PE"
        case judging = "…"

        init(code: String) {
            self = Verdict(rawValue: code.uppercased()) ?? .judging
        }

        /// O texto em português fica ao lado, nunca no lugar da sigla.
        var meaning: String {
            switch self {
            case .ac:  return "Accepted"
            case .wa:  return "Wrong Answer"
            case .tle: return "Time Limit Exceeded"
            case .mle: return "Memory Limit"
            case .re:  return "Runtime Error"
            case .ce:  return "Compile Error"
            case .pe:  return "Presentation Error"
            case .judging: return "Judging"
            }
        }

        /// Cor de **linha**: borda, barra lateral, ícone preenchido.
        var tone: Color {
            switch self {
            case .ac:                   return LognColors.correct
            case .wa, .tle, .mle, .re:  return LognColors.wrong
            case .ce, .pe:              return LognColors.warn
            case .judging:              return LognColors.textMuted
            }
        }

        /// Cor de **tinta**: sigla, número, qualquer glifo. No dark coincide com `tone`;
        /// no light escurece, porque a cor de linha não alcança 4.5:1 sobre o próprio tint.
        var ink: Color {
            switch self {
            case .ac:                   return LognColors.correctInk
            case .wa, .tle, .mle, .re:  return LognColors.wrongInk
            case .ce, .pe:              return LognColors.warnInk
            case .judging:              return LognColors.textMuted
            }
        }

        var tint: Color {
            switch self {
            case .ac:                   return LognColors.tintOk
            case .wa, .tle, .mle, .re:  return LognColors.tintErr
            case .ce, .pe:              return LognColors.tintWarn
            case .judging:              return .clear
            }
        }
    }

    var body: some View {
        Text(verdict.rawValue)
            .font(.plexMonoSemiBold(12))
            .tracking(0.06 * 12)
            .foregroundColor(verdict.ink)
            .padding(.horizontal, 8)
            .frame(minWidth: 46, minHeight: 26)
            .background(verdict.tint)
            .cornerRadius(Radius.xs)
            .overlay(
                RoundedRectangle(cornerRadius: Radius.xs)
                    .stroke(verdict == .judging ? LognColors.lineDim : verdict.tone, lineWidth: 1)
            )
            .accessibilityLabel(Str.Profile.verdict_accessibility(verdict.rawValue, verdict.meaning))
    }
}

// MARK: - MatchHeader

/// Header fixo de partida — exploração `3b · Contest`.
///
/// O placar fica sempre à vista: problema atual, relógio, vidas e a fileira A—M inteira.
/// Fundo `canvas`, divisor inferior `line`, sem card e sem raio: ele é a moldura da tela,
/// não um elemento sobre ela.
struct MatchHeader: View {
    let letter: Character
    let remainingSeconds: Int
    let isFrozen: Bool
    let lives: Int
    let maxLives: Int
    /// (letra, aceito) para as 13 posições do contest.
    let balloonStates: [(Character, Bool)]
    /// Quando falso, o balão do problema atual aparece murcho — usado na tela de erro.
    var currentIsAlive: Bool = true
    var showsBalloonRow: Bool = true
    /// Saída da partida. Sem isto não havia nenhuma: quem abrisse um desafio sem saber a
    /// resposta ficava preso até perder as três vidas.
    var onLeave: (() -> Void)? = nil

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 10) {
                if let onLeave {
                    // Área de toque de 44dp com glifo de 18, recuada para o glifo alinhar
                    // com a margem do conteúdo. Cinza e não accent: sair não é a ação que
                    // o app quer incentivar, e o laranja é a cor do que se quer que a
                    // pessoa faça.
                    Button(action: onLeave) {
                        Image(systemName: "xmark")
                            .font(.system(size: 15, weight: .medium))
                            .foregroundColor(LognColors.textMuted)
                            .frame(width: 44, height: 44)
                    }
                    .buttonStyle(.plain)
                    .padding(.leading, -12)
                    .padding(.trailing, -10)
                    .accessibilityLabel(Str.Leave.accessibility)
                }

                BalloonShape(
                    style: currentIsAlive
                        ? BalloonColor.filled(letter)
                        : .outline(LognColors.lineStrong, 4),
                    width: 14,
                    showString: true
                )

                Text(Str.Match.problem_letter(String(letter)))
                    .font(.plexMono(12))
                    .tracking(0.12 * 12)
                    .foregroundColor(LognColors.textMuted)

                Spacer(minLength: 0)

                Text(ContestClock.format(remainingSeconds))
                    .font(.plexMonoMedium(15))
                    .monospacedDigit()
                    .foregroundColor(clockColor)
                    .accessibilityLabel(Str.Match.time_remaining(remainingSeconds))

                LifeBar(lives: lives, maxLives: maxLives, showsCounter: false, heartSize: 14)
                    .padding(.leading, 12)
            }

            if showsBalloonRow {
                balloonRow
            }
        }
        .padding(.horizontal, 18)
        .padding(.top, 16)
        .padding(.bottom, 14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LognColors.canvas)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognColors.line)
        }
    }

    /// A fileira de problemas do contest, cada um sempre na mesma cor.
    ///
    /// Vai até a letra do último problema **desta** partida, não até M. A fileira
    /// iterava a paleta inteira: uma sessão de cinco problemas mostrava oito balões
    /// vazios que nunca iam encher, e a leitura de tela anunciava "problema F, em
    /// aberto" para um problema que não existe.
    ///
    /// A letra abaixo do balão cai para 8sp aqui, sob o mínimo de 11sp do DS: em fileira
    /// densa ela é reforço visual, e o fallback acessível de verdade é o rótulo de
    /// acessibilidade de cada balão.
    private var letters: [Character] { balloonStates.map(\.0) }

    private var balloonRow: some View {
        HStack(spacing: 7) {
            ForEach(letters, id: \.self) { l in
                VStack(spacing: 2) {
                    BalloonShape(style: balloonStyle(for: l), width: 13)
                    // A paleta de balões é certificada para 3:1 como forma, não 4.5:1
                    // como texto — a letra fica em `textMuted`, a cor vive no balão.
                    Text(String(l))
                        .font(.plexMono(8))
                        .foregroundColor(LognColors.textMuted)
                }
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(balloonDescription(for: l))
            }
            Spacer(minLength: 0)
        }
    }

    private func balloonStyle(for l: Character) -> BalloonShape.Style {
        if isAccepted(l) { return BalloonColor.filled(l) }
        if l == letter   { return .outline(BalloonColor.forLetter(l), 5) }
        return .outline(LognColors.lineStrong, 5)
    }

    /// O balão nunca comunica só por cor — a letra e o estado vão no rótulo.
    private func balloonDescription(for l: Character) -> String {
        let status: String
        if isAccepted(l) {
            status = Str.Verdict.accepted
        } else if l == letter {
            status = Str.Verdict.solving
        } else {
            status = Str.Verdict.open
        }
        return Str.Match.problem_status(String(l), status)
    }

    private func isAccepted(_ l: Character) -> Bool {
        balloonStates.contains { $0.0 == l && $0.1 }
    }

    private var isSolved: Bool { isAccepted(letter) }

    private var clockColor: Color {
        if remainingSeconds <= 15 { return LognColors.wrongInk }
        if isFrozen || remainingSeconds <= 45 { return LognColors.warnInk }
        return LognColors.textSecondary
    }
}

// MARK: - Shake

/// Shake de erro: ±5dp horizontal, 4 oscilações, 240ms ease-out.
/// Com `reduceMotion` ligado o chamador troca isto por um flash de borda.
struct ShakeEffect: GeometryEffect {
    var amount: CGFloat = 5
    var shakesPerUnit = 4
    var animatableData: CGFloat

    func effectValue(size: CGSize) -> ProjectionTransform {
        ProjectionTransform(
            CGAffineTransform(
                translationX: amount * sin(animatableData * .pi * CGFloat(shakesPerUnit)),
                y: 0
            )
        )
    }
}

extension View {
    func shake(animatableData: CGFloat) -> some View {
        modifier(ShakeEffect(animatableData: animatableData))
    }
}
