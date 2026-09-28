import SwiftUI
import LogNCoreFFI
import LogN

// Peças comuns às telas de trilha (LogN Trilhas, LogN Validade Offline).

extension TrackView {
    /// A cor do balão da trilha, que chega do servidor como `#RRGGBB`.
    var tint: Color {
        let hex = color.hasPrefix("#") ? String(color.dropFirst()) : color
        return hex.count == 6 ? Color(hex: hex) : LognDark.accent
    }

    /// "PT EN ES", na ordem do produto, só as línguas publicadas.
    var languagesLabel: String {
        var codes: [String] = []
        for (locale, code) in [("pt-BR", "PT"), ("en", "EN"), ("es", "ES")] where languages.contains(locale) {
            codes.append(code)
        }
        return codes.joined(separator: " ")
    }

    /// O próximo nó a jogar, a partir de 1, para "Continuar · Nó N".
    var activeNodeIndex: Int? {
        for (i, row) in nodes.enumerated() where row.active {
            return i + 1
        }
        return nil
    }

    /// Nenhuma licença que valha agora, mas a trilha é da conta.
    var isExpired: Bool { owned && offline == .expired }
}

/// Balão de uma trilha, na cor dela.
struct TrackBalloon: View {
    let track: TrackView
    var width: CGFloat = 30
    var dimmed = false

    var body: some View {
        BalloonShape(style: .filled(track.tint), width: width, showString: true, showHighlight: width >= 24)
            .frame(width: width, height: width * 150 / 96)
            .opacity(dimmed ? 0.45 : 1)
            .accessibilityHidden(true)
    }
}

/// O selo de estado de uma trilha no catálogo: preço, comprada, dias sem rede,
/// revogada. Um lugar só decide, para o card e o detalhe não divergirem.
struct TrackChipStyle {
    let text: String
    let ink: Color
    let line: Color

    static func of(_ track: TrackView, price: String?) -> TrackChipStyle {
        if track.revoked {
            return TrackChipStyle(text: Str.Catalog.no_access, ink: LognDark.textSecondary, line: LognDark.lineStrong)
        }
        if track.owned {
            switch track.offline {
            case .soon:
                return TrackChipStyle(text: Str.Catalog.days(Int(track.offlineDaysLeft)), ink: LognDark.warnInk, line: LognDark.warn)
            case .today:
                return TrackChipStyle(text: Str.Catalog.today, ink: LognDark.warnInk, line: LognDark.warn)
            case .expired:
                return TrackChipStyle(text: Str.Catalog.connect, ink: LognDark.wrongInk, line: LognDark.wrong)
            default:
                break
            }
            if track.discontinued {
                return TrackChipStyle(text: Str.Catalog.discontinued, ink: LognDark.textSecondary, line: LognDark.lineStrong)
            }
            return TrackChipStyle(text: Str.Catalog.owned, ink: LognDark.correctInk, line: LognDark.correct)
        }
        return TrackChipStyle(text: price ?? Str.Catalog.see, ink: LognDark.textPrimary, line: LognDark.lineStrong)
    }
}

struct TrackChip: View {
    let style: TrackChipStyle

    var body: some View {
        Text(style.text)
            .font(.plexMonoMedium(10.5))
            .foregroundColor(style.ink)
            .padding(.horizontal, 6)
            .padding(.vertical, 2)
            .overlay(RoundedRectangle(cornerRadius: Radius.xs).stroke(style.line, lineWidth: 1))
    }
}

/// Os 30 dias da licença offline, um traço por dia: os usados em cinza, os três
/// últimos em âmbar, os que faltam vazios (V3).
struct OfflineDaysBar: View {
    let usedDays: Int

    private var cells: [Color] {
        var out: [Color] = []
        for i in 0..<30 {
            if i < usedDays {
                out.append(i >= 26 ? LognDark.warn : LognDark.lineDim)
            } else {
                out.append(LognDark.line)
            }
        }
        return out
    }

    var body: some View {
        HStack(spacing: 2) {
            ForEach(Array(cells.enumerated()), id: \.offset) { _, color in
                RoundedRectangle(cornerRadius: 1).fill(color).frame(height: 10)
            }
        }
        .accessibilityHidden(true)
    }
}

enum TrackDates {
    /// "26 OUT": o dia até quando a licença vale, na língua do app.
    static func short(_ unix: Int64) -> String {
        let f = DateFormatter()
        f.locale = Locale.current
        f.setLocalizedDateFormatFromTemplate("d MMM")
        return f.string(from: Date(timeIntervalSince1970: TimeInterval(unix))).uppercased()
    }

    /// "terça": o último dia em que conectar ainda salva a licença.
    static func weekday(_ unix: Int64) -> String {
        let f = DateFormatter()
        f.locale = Locale.current
        f.setLocalizedDateFormatFromTemplate("EEEE")
        return f.string(from: Date(timeIntervalSince1970: TimeInterval(unix - 1)))
    }
}
