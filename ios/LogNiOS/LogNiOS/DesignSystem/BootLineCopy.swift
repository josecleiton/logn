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
        }
    }

    var tone: Color {
        switch verdict {
        case .running: return LognDark.textMuted
        case .ok:      return LognDark.correct
        case .warn:    return LognDark.warn
        case .fail:    return LognDark.wrong
        }
    }
}
