package sh.logn.app.ui.copy

import android.content.Context
import sh.logn.core.LogN.StatusKey
import sh.logn.coreshell.i18n.Str

/**
 * A cópia de cada estado que o Core reporta. Espelho de StatusKeyCopy.swift: o Core
 * manda a chave, nunca a frase, porque não sabe em que língua o app está.
 *
 * `null` quando não há nada a dizer: a tela não mostra linha de status nenhuma.
 */
fun StatusKey.copy(context: Context): String? =
    when (this) {
        StatusKey.SILENT -> null
        StatusKey.SIGNINGIN -> Str.Status.signing_in(context)
        StatusKey.WRONGCREDENTIALS -> Str.Status.wrong_credentials(context)
        StatusKey.SIGNINFAILED -> Str.Status.sign_in_failed(context)
        StatusKey.NOCONNECTION -> Str.Status.no_connection(context)
        StatusKey.SERVERUNREADABLE -> Str.Status.server_unreadable(context)
        StatusKey.SESSIONEXPIRED -> Str.Status.session_expired(context)
        StatusKey.SIGNINGOUT -> Str.Status.signing_out(context)
        StatusKey.RESUMINGSESSION -> Str.Status.resuming_session(context)
        StatusKey.SENDINGCODE -> Str.Status.sending_code(context)
        StatusKey.CODESENTFAILED -> Str.Status.code_sent_failed(context)
        StatusKey.CHECKINGCODE -> Str.Status.checking_code(context)
        StatusKey.CODEINVALID -> Str.Status.code_invalid(context)
        StatusKey.CREATINGACCOUNT -> Str.Status.creating_account(context)
        StatusKey.ACCOUNTFAILED -> Str.Status.account_failed(context)
        StatusKey.RESETTINGPASSWORD -> Str.Status.resetting_password(context)
        StatusKey.RESETFAILED -> Str.Status.reset_failed(context)
        StatusKey.SYNCING -> Str.Status.syncing(context)
        StatusKey.SYNCDIVERGED -> Str.Status.sync_diverged(context)
        StatusKey.SYNCFAILED -> Str.Status.sync_failed(context)
        StatusKey.SYNCOFFLINE -> Str.Status.sync_offline(context)
        StatusKey.SIGNINTOSYNC -> Str.Status.sign_in_to_sync(context)
        StatusKey.TREEUNAVAILABLE -> Str.Dashboard.tree_unavailable(context)
        StatusKey.RATELIMITED -> Str.Status.rate_limited(context)
        StatusKey.INVALIDEMAIL -> Str.Status.invalid_email(context)
        StatusKey.PASSWORDTOOSHORT -> Str.Status.password_too_short(context)
        StatusKey.PASSWORDTOOLONG -> Str.Status.password_too_long(context)
        StatusKey.EMAILTAKEN -> Str.Status.email_taken(context)
        StatusKey.PURCHASECONFIRMING -> Str.Status.purchase_confirming(context)
        StatusKey.PURCHASECONFIRMED -> Str.Status.purchase_confirmed(context)
        StatusKey.PURCHASEFAILED -> Str.Status.purchase_failed(context)
        StatusKey.PURCHASEOWNEDBYOTHERACCOUNT -> Str.Status.purchase_owned_by_other_account(context)
        StatusKey.PURCHASEACCOUNTMISMATCH -> Str.Status.purchase_account_mismatch(context)
        StatusKey.PURCHASEREVOKED -> Str.Status.purchase_revoked(context)
        StatusKey.PURCHASENEEDSACCOUNT -> Str.Status.purchase_needs_account(context)
        StatusKey.TRACKREVOKED -> Str.Status.track_revoked(context)
        StatusKey.TRACKDOWNLOADFAILED -> Str.Status.track_download_failed(context)
        StatusKey.SOCIALSIGNINFAILED -> Str.Status.social_sign_in_failed(context)
        StatusKey.SOCIALEMAILUNVERIFIED -> Str.Status.social_email_unverified(context)
        StatusKey.SOCIALPROVIDERDISABLED -> Str.Status.social_provider_disabled(context)
        StatusKey.PROVIDERREAUTHREQUIRED -> Str.Status.provider_reauth_required(context)
    }

/** Estado em andamento: a tela mostra em tom neutro, não no vermelho de erro. */
val StatusKey.isInProgress: Boolean
    get() =
        when (this) {
            StatusKey.SIGNINGIN, StatusKey.RESUMINGSESSION, StatusKey.SENDINGCODE, StatusKey.CHECKINGCODE,
            StatusKey.CREATINGACCOUNT, StatusKey.RESETTINGPASSWORD, StatusKey.SIGNINGOUT, StatusKey.SYNCING,
            StatusKey.PURCHASECONFIRMING,
            -> true
            else -> false
        }
