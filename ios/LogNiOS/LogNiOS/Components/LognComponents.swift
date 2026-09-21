import SwiftUI

// MARK: - LognButton

/// Botão padronizado do LogN Design System.
/// Altura 52dp, largura total, raio 4dp. Um primário por tela.
struct LognButton: View {
    let title: String
    let variant: Variant
    let action: () -> Void
    var isDisabled: Bool = false

    enum Variant {
        case primary
        case secondary
        case ghost
    }

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(.custom("IBMPlexSans-SemiBold", size: 15))
                .frame(maxWidth: .infinity)
                .frame(height: 52)
                .foregroundColor(foregroundColor)
                .background(backgroundColor)
                .cornerRadius(Radius.sm)
                .overlay(
                    variant == .secondary
                        ? RoundedRectangle(cornerRadius: Radius.sm)
                            .stroke(LognDark.lineStrong, lineWidth: 1)
                        : nil
                )
        }
        .disabled(isDisabled)
    }

    private var foregroundColor: Color {
        if isDisabled { return LognDark.textDim }
        switch variant {
        case .primary: return LognDark.onAccent
        case .secondary: return LognDark.textPrimary
        case .ghost: return LognDark.textSecondary
        }
    }

    private var backgroundColor: Color {
        if isDisabled { return LognDark.buttonDisabled }
        switch variant {
        case .primary: return LognDark.accent
        case .secondary: return .clear
        case .ghost: return .clear
        }
    }
}

// MARK: - LifeBar

/// 3 corações 20sp. Cheio = `wrong`; vazio = `heartOff`. Contador mono ao lado.
struct LifeBar: View {
    let lives: Int // 0-3
    let maxLives: Int = 3

    var body: some View {
        HStack(spacing: 4) {
            ForEach(0..<maxLives, id: \.self) { i in
                Image(systemName: i < lives ? "heart.fill" : "heart")
                    .font(.system(size: 20))
                    .foregroundColor(i < lives ? LognDark.wrong : LognDark.heartOff)
            }
            Text("\(lives) / \(maxLives)")
                .font(LognFont.label)
                .foregroundColor(LognDark.textSecondary)
        }
    }
}

// MARK: - ContestClock

/// Relógio do contest. Plex Mono, tabular-nums.
/// 3 estados: normal, congelado (última hora), crítico (< 15s).
struct ContestClock: View {
    let remainingSeconds: Int
    let isFrozen: Bool

    var body: some View {
        if isCritical {
            HStack(alignment: .lastTextBaseline, spacing: 10) {
                Text(formattedTime)
                    .font(.custom("IBMPlexMono-Medium", size: 28))
                    .fontWeight(.semibold)
                    .monospacedDigit()
                    .foregroundColor(LognDark.wrong)
                
                Text("RELÓGIO DA QUESTÃO · CRÍTICO")
                    .font(.custom("IBMPlexMono-Regular", size: 11))
                    .tracking(1.1)
                    .textCase(.uppercase)
                    .foregroundColor(LognDark.wrong)
            }
        } else {
            VStack(alignment: .leading, spacing: 6) {
                Text(isFrozen ? "PLACAR CONGELADO" : "TEMPO RESTANTE")
                    .font(.custom("IBMPlexMono-Regular", size: 10.5))
                    .tracking(1.6)
                    .textCase(.uppercase)
                    .foregroundColor(isFrozen ? LognDark.warn : LognDark.textMuted)

                Text(formattedTime)
                    .font(.custom("IBMPlexMono-Medium", size: 38))
                    .fontWeight(.medium)
                    .tracking(-0.02 * 38)
                    .monospacedDigit()
                    .foregroundColor(isFrozen ? LognDark.warn : LognDark.textPrimary)
            }
            .padding(.horizontal, 18)
            .padding(.vertical, 16)
            .background(isFrozen ? LognDark.tintWarn : LognDark.canvas)
            .cornerRadius(4)
            .overlay(
                RoundedRectangle(cornerRadius: 4)
                    .stroke(isFrozen ? LognDark.warn : LognDark.line, lineWidth: 1)
            )
        }
    }

    private var isCritical: Bool { remainingSeconds <= 15 && !isFrozen }

    private var formattedTime: String {
        if remainingSeconds > 3600 {
            let h = remainingSeconds / 3600
            let m = (remainingSeconds % 3600) / 60
            let s = remainingSeconds % 60
            return String(format: "%02d:%02d:%02d", h, m, s)
        } else {
            let m = remainingSeconds / 60
            let s = remainingSeconds % 60
            return String(format: "%02d:%02d", m, s)
        }
    }
}

// MARK: - VerdictChip

/// Chip de veredito do juiz. Altura 26dp, min-width 46dp, raio 2dp.
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
    }

    var body: some View {
        Text(verdict.rawValue)
            .font(.custom("IBMPlexMono-SemiBold", size: 12))
            .tracking(0.72)
            .foregroundColor(inkColor)
            .padding(.horizontal, 8)
            .frame(minWidth: 46, minHeight: 26)
            .background(tintColor)
            .overlay(
                RoundedRectangle(cornerRadius: 2)
                    .stroke(borderColor, lineWidth: 1)
            )
            .cornerRadius(2)
    }

    private var inkColor: Color {
        switch verdict {
        case .ac: return LognDark.correct
        case .wa, .tle, .mle, .re: return LognDark.wrong
        case .ce, .pe: return LognDark.warn
        case .judging: return LognDark.textMuted
        }
    }

    private var borderColor: Color {
        switch verdict {
        case .ac: return LognDark.correct
        case .wa, .tle, .mle, .re: return LognDark.wrong
        case .ce, .pe: return LognDark.warn
        case .judging: return LognDark.lineDim
        }
    }

    private var tintColor: Color {
        switch verdict {
        case .ac: return LognDark.tintOk
        case .wa, .tle, .mle, .re: return LognDark.tintErr
        case .ce, .pe: return LognDark.tintWarn
        case .judging: return .clear
        }
    }
}

// MARK: - MatchHeader

/// Header fixo de partida (84dp). Rótulo + ContestClock + LifeBar + fileira de balões.
struct MatchHeader: View {
    let sessionLabel: String
    let remainingSeconds: Int
    let isFrozen: Bool
    let lives: Int
    let balloonStates: [(Character, Bool)] // (letter, isAccepted)

    private var formattedTime: String {
        let m = remainingSeconds / 60
        let s = remainingSeconds % 60
        return String(format: "%02d:%02d", m, s)
    }

    var body: some View {
        VStack(spacing: 8) {
            HStack {
                Text(sessionLabel)
                    .font(LognFont.label)
                    .foregroundColor(LognDark.textMuted)

                Spacer()

                // Small inline timer for MatchHeader
                Text(formattedTime)
                    .font(.custom("IBMPlexMono-Medium", size: 13))
                    .foregroundColor(remainingSeconds <= 15 ? LognDark.wrong : (isFrozen ? LognDark.warn : LognDark.textPrimary))
                    .monospacedDigit()

                Spacer()

                LifeBar(lives: lives)
            }

            // Fileira de balões A—M
            HStack(spacing: 7) {
                let letters: [Character] = ["A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M"]
                ForEach(letters, id: \.self) { letter in
                    let isAccepted = balloonStates.contains(where: { $0.0 == letter && $0.1 == true })
                    VStack(spacing: 2) {
                        BalloonShape(
                            color: BalloonColor.forLetter(letter),
                            state: isAccepted ? .filled : .outline,
                            bodySize: 13,
                            showString: true,
                            showHighlight: false
                        )
                        .opacity(isAccepted ? 1.0 : 0.55)
                        
                        Text(String(letter))
                            .font(.custom("IBMPlexMono-SemiBold", size: 8))
                            .foregroundColor(isAccepted ? BalloonColor.forLetter(letter) : LognDark.textMuted)
                    }
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 14)
        .frame(height: 84)
        .background(LognDark.canvas)
        .cornerRadius(4)
        .overlay(
            RoundedRectangle(cornerRadius: 4)
                .stroke(LognDark.line, lineWidth: 1)
        )
        .padding(.horizontal, Space.screenMargin)
        .padding(.top, 10)
    }
}
import SwiftUI

/// Um modificador que aplica um efeito de shake (tremor lateral) quando o trigger for incrementado.
struct ShakeEffect: GeometryEffect {
    var amount: CGFloat = 5
    var shakesPerUnit = 4
    var animatableData: CGFloat
    
    func effectValue(size: CGSize) -> ProjectionTransform {
        ProjectionTransform(CGAffineTransform(translationX:
            amount * sin(animatableData * .pi * CGFloat(shakesPerUnit)),
            y: 0))
    }
}

extension View {
    func shake(animatableData: CGFloat) -> some View {
        self.modifier(ShakeEffect(animatableData: animatableData))
    }
}
