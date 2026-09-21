import SwiftUI

// Gerado a partir do LogN Design System v2. Não invente cores fora deste arquivo.

extension Color {
    init(hex: String) {
        var v: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&v)
        self.init(.sRGB,
                  red:   Double((v >> 16) & 0xFF) / 255,
                  green: Double((v >> 8)  & 0xFF) / 255,
                  blue:  Double(v & 0xFF) / 255,
                  opacity: 1)
    }
}

enum LognDark {
    static let canvas          = Color(hex: "0B0C0D")
    static let surface         = Color(hex: "111316")
    static let surfaceRaised   = Color(hex: "171A1E")
    static let line            = Color(hex: "24282D")
    static let lineStrong      = Color(hex: "343A41")
    static let textPrimary     = Color(hex: "EDEEEF")
    static let textSecondary   = Color(hex: "99A0A7")
    static let textMuted       = Color(hex: "7E858D")
    static let textDim         = Color(hex: "5F656C")
    static let lineDim         = Color(hex: "4C535B")
    static let heartOff        = Color(hex: "2E3238")
    static let rowLine         = Color(hex: "16191C")
    static let buttonDisabled  = Color(hex: "1B1D20")
    static let accent          = Color(hex: "FF7A45")
    static let onAccent        = Color(hex: "160B05")
    static let correct         = Color(hex: "3DD68C")
    static let wrong           = Color(hex: "FF5C5C")
    static let warn            = Color(hex: "F5C451")
    static let info            = Color(hex: "5AA9FF")
    static let tintOk          = Color(hex: "0F2018")
    static let tintErr         = Color(hex: "231113")
    static let tintWarn        = Color(hex: "221C0C")
    static let tintInfo        = Color(hex: "0D1B2B")
    static let synKeyword      = Color(hex: "C792EA")
    static let synFunction     = Color(hex: "82AAFF")
}

enum LognLight {
    static let canvas          = Color(hex: "F6F6F4")
    static let surface         = Color(hex: "FFFFFF")
    static let surfaceRaised   = Color(hex: "F0F1EE")
    static let line            = Color(hex: "E2E3DF")
    static let lineStrong      = Color(hex: "C6C8C2")
    static let textPrimary     = Color(hex: "14161A")
    static let textSecondary   = Color(hex: "555B62")
    static let textMuted       = Color(hex: "6B7178")
    static let textDim         = Color(hex: "7C838A")
    static let lineDim         = Color(hex: "A9AFB5")
    static let heartOff        = Color(hex: "D6D8D3")
    static let rowLine         = Color(hex: "ECEDE9")
    static let buttonDisabled  = Color(hex: "E6E7E3")
    static let accent          = Color(hex: "FF7A45")
    static let onAccent        = Color(hex: "160B05")
    static let correct         = Color(hex: "0E8F52")
    static let wrong           = Color(hex: "C93636")
    static let warn            = Color(hex: "8A5B00")
    static let info            = Color(hex: "1660C4")
    static let tintOk          = Color(hex: "E6F5EC")
    static let tintErr         = Color(hex: "FBEAEA")
    static let tintWarn        = Color(hex: "FAF1DC")
    static let tintInfo        = Color(hex: "E6EFFB")
    static let synKeyword      = Color(hex: "7A28C4")
    static let synFunction     = Color(hex: "0A4FA8")
}

/// Balões A—M. Identidade categórica do problema, NUNCA estado.
enum Balloon {
    static let dark: [Color] = [
        Color(hex: "E4572E"), // A · vermelho
        Color(hex: "F5C451"), // B · amarelo
        Color(hex: "3DB2FF"), // C · azul
        Color(hex: "6BCB77"), // D · verde
        Color(hex: "C77DFF"), // E · violeta
        Color(hex: "FF6FB5"), // F · rosa
        Color(hex: "4ECDC4"), // G · turquesa
        Color(hex: "F4A261"), // H · âmbar
        Color(hex: "9BC53D"), // I · lima
        Color(hex: "D64550"), // J · carmim
        Color(hex: "7C8BFF"), // K · índigo
        Color(hex: "D8DEE4"), // L · prata
        Color(hex: "00B894"), // M · esmeralda
    ]
    static let light: [Color] = [
        Color(hex: "C43F19"), // A · vermelho
        Color(hex: "A67A00"), // B · amarelo
        Color(hex: "0B6FBF"), // C · azul
        Color(hex: "2E8B45"), // D · verde
        Color(hex: "8A3FD1"), // E · violeta
        Color(hex: "C2367E"), // F · rosa
        Color(hex: "18867E"), // G · turquesa
        Color(hex: "B26320"), // H · âmbar
        Color(hex: "5F8410"), // I · lima
        Color(hex: "A3202B"), // J · carmim
        Color(hex: "4352C9"), // K · índigo
        Color(hex: "5B646D"), // L · prata
        Color(hex: "007A61"), // M · esmeralda
    ]
    static func of(_ letter: Character, isLight: Bool = false) -> Color {
        let i = Int(letter.uppercased().first!.asciiValue! - 65)
        return (isLight ? light : dark)[i]
    }
}

enum LognFont {
    static let displayLarge   = Font.custom("IBMPlexSans-SemiBold", size: 40, relativeTo: .largeTitle)
    static let headlineMedium = Font.custom("IBMPlexSans-SemiBold", size: 24, relativeTo: .title2)
    static let titleMedium    = Font.custom("IBMPlexSans-SemiBold", size: 19, relativeTo: .headline)
    static let bodyLarge      = Font.custom("IBMPlexSans-Regular",  size: 17, relativeTo: .body)
    static let bodyMedium     = Font.custom("IBMPlexSans-Regular",  size: 15, relativeTo: .callout)
    static let code           = Font.custom("IBMPlexMono-Regular",  size: 15, relativeTo: .body)
    static let label          = Font.custom("IBMPlexMono-Medium",   size: 11, relativeTo: .caption)
}

enum Radius { static let xs: CGFloat = 2; static let sm: CGFloat = 4; static let md: CGFloat = 8 }

enum Space {
    static let xs: CGFloat = 4;  static let sm: CGFloat = 8;   static let md: CGFloat = 12
    static let lg: CGFloat = 16; static let xl: CGFloat = 24;  static let xxl: CGFloat = 32
    static let xxxl: CGFloat = 48
    static let screenMargin: CGFloat = 20
    static let listGap: CGFloat = 8
    static let minTouch: CGFloat = 44      // piso HIG
    static let matchTouch: CGFloat = 56    // alvos em partida
}
