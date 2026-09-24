import Foundation
import LogNCoreFFI
import LogN

/// A cópia de cada estado que o Core reporta.
///
/// O Core manda `StatusKey`, nunca a frase: ele é a mesma lógica no iOS e no Android e
/// não sabe em que idioma o app está. Quem resolve a chave é o cliente, contra o
/// catálogo de `i18n/locales/`. Antes o Core escrevia a frase pronta e ela vazava para
/// a tela — a de login abria com "Logged out successfully", em inglês, no vermelho de
/// erro, num app em português.
extension LogN.StatusKey {
    /// `nil` quando não há nada a dizer: a tela não mostra linha de status nenhuma.
    var copy: String? {
        switch self {
        case .silent:            return nil
        case .signingIn:         return Str.Status.signing_in
        case .wrongCredentials:  return Str.Status.wrong_credentials
        case .signInFailed:      return Str.Status.sign_in_failed
        case .noConnection:      return Str.Status.no_connection
        case .serverUnreadable:  return Str.Status.server_unreadable
        case .sessionExpired:    return Str.Status.session_expired
        case .signingOut:        return Str.Status.signing_out
        case .resumingSession:   return Str.Status.resuming_session
        case .sendingCode:       return Str.Status.sending_code
        case .codeSentFailed:    return Str.Status.code_sent_failed
        case .checkingCode:      return Str.Status.checking_code
        case .codeInvalid:       return Str.Status.code_invalid
        case .creatingAccount:   return Str.Status.creating_account
        case .accountFailed:     return Str.Status.account_failed
        case .resettingPassword: return Str.Status.resetting_password
        case .resetFailed:       return Str.Status.reset_failed
        case .syncing:           return Str.Status.syncing
        case .syncDiverged:      return Str.Status.sync_diverged
        case .syncFailed:        return Str.Status.sync_failed
        case .syncOffline:       return Str.Status.sync_offline
        case .signInToSync:      return Str.Status.sign_in_to_sync
        case .treeUnavailable:   return Str.Dashboard.tree_unavailable
        case .rateLimited:       return Str.Status.rate_limited
        }
    }

    /// Estado em andamento: a tela mostra em tom neutro, não no vermelho de erro.
    var isInProgress: Bool {
        switch self {
        case .signingIn, .resumingSession, .sendingCode, .checkingCode,
             .creatingAccount, .resettingPassword, .signingOut, .syncing:
            return true
        default:
            return false
        }
    }
}
