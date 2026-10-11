import SwiftUI
import LogNCoreFFI
import LogN

/// A cópia e o tom de cada linha do log da abertura.
///
/// O Core manda a verificação, o veredito e o detalhe como enum, e o número quando há.
/// A frase é do catálogo, como a do `StatusKey`: o Core não sabe em que língua o app está.
extension LogN.BootLine {
    var checkLabel: String {
        switch check {
        case .session: return Str.Boot.check_session
        case .sync:    return Str.Boot.check_sync
        case .terms:   return Str.Boot.check_terms
        }
    }

    var detailLabel: String {
        let n = Int(count)
        switch detail {
        case .checking:        return Str.Boot.checking
        case .tokenRenewed:    return Str.Boot.token_renewed
        case .localTokenValid: return Str.Boot.local_token_valid
        case .sessionEnded:    return Str.Boot.session_ended
        case .sending:         return Str.Boot.sending(n)
        case .nothingToSend:   return Str.Boot.nothing_to_send
        case .sent:            return Str.Boot.sent(n)
        case .noNetwork:       return Str.Boot.no_network(n)
        case .rejected:        return Str.Boot.rejected
        case .stillSending:    return Str.Boot.still_sending
        case .termsChecking:   return Str.Boot.terms_checking
        case .termsCurrent:    return Str.Boot.terms_current
        case .termsChanged:    return Str.Boot.terms_changed
        case .termsNotice:     return Str.Boot.terms_notice
        case .termsDeferred:   return Str.Boot.terms_deferred
        }
    }

    /// O sinal na frente da linha, como no log de um juiz. É SF Symbol, não caractere:
    /// a Plex Mono não tem o ✓, e o sistema desenhava no lugar um sinal parecido com √.
    var symbol: String {
        switch verdict {
        case .running: return "ellipsis"
        case .ok:      return "checkmark"
        case .warn:    return "exclamationmark"
        case .fail:    return "xmark"
        case .skipped: return "minus"
        }
    }

    var tone: Color {
        // Termos que mudaram não são erro nem acerto: o design marca com a cor de
        // informação, e a tela de aceite vem em seguida. É o sinal da linha, um glifo:
        // tinta, nunca a base.
        if detail == .termsChanged { return LognColors.infoInk }
        switch verdict {
        case .running: return LognColors.textMuted
        case .ok:      return LognColors.correctInk
        case .warn:    return LognColors.warnInk
        case .fail:    return LognColors.wrongInk
        case .skipped: return LognColors.textMuted
        }
    }
}
