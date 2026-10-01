package sh.logn.app.ui.copy

import android.content.Context
import androidx.compose.ui.graphics.Color
import sh.logn.app.ui.components.Glyph
import sh.logn.app.ui.theme.LognDark
import sh.logn.core.LogN.BootCheck
import sh.logn.core.LogN.BootDetail
import sh.logn.core.LogN.BootLine
import sh.logn.core.LogN.BootVerdict
import sh.logn.coreshell.i18n.Str

// A cópia e o tom de cada linha do log da abertura, espelho de BootLineCopy.swift. O Core
// manda a verificação, o veredito e o detalhe como enum, e o número quando há; a frase é
// do catálogo. `when` exaustivo e sem `else`: variante nova no Core quebra este build.

fun BootLine.checkLabel(context: Context): String =
    when (check) {
        BootCheck.SESSION -> Str.Boot.check_session(context)
        BootCheck.SYNC -> Str.Boot.check_sync(context)
        BootCheck.TERMS -> Str.Boot.check_terms(context)
    }

fun BootLine.detailLabel(context: Context): String {
    val n = count.toInt()
    return when (detail) {
        BootDetail.CHECKING -> Str.Boot.checking(context)
        BootDetail.TOKENRENEWED -> Str.Boot.token_renewed(context)
        BootDetail.LOCALTOKENVALID -> Str.Boot.local_token_valid(context)
        BootDetail.SESSIONENDED -> Str.Boot.session_ended(context)
        BootDetail.SENDING -> Str.Boot.sending(context, n)
        BootDetail.NOTHINGTOSEND -> Str.Boot.nothing_to_send(context)
        BootDetail.SENT -> Str.Boot.sent(context, n)
        BootDetail.NONETWORK -> Str.Boot.no_network(context, n)
        BootDetail.REJECTED -> Str.Boot.rejected(context)
        BootDetail.STILLSENDING -> Str.Boot.still_sending(context)
        BootDetail.TERMSCHECKING -> Str.Boot.terms_checking(context)
        BootDetail.TERMSCURRENT -> Str.Boot.terms_current(context)
        BootDetail.TERMSCHANGED -> Str.Boot.terms_changed(context)
        BootDetail.TERMSNOTICE -> Str.Boot.terms_notice(context)
        BootDetail.TERMSDEFERRED -> Str.Boot.terms_deferred(context)
    }
}

/** O sinal na frente da linha, como no log de um juiz. */
val BootLine.glyph: Glyph
    get() =
        when (verdict) {
            BootVerdict.RUNNING -> Glyph.Ellipsis
            BootVerdict.OK -> Glyph.Check
            BootVerdict.WARN -> Glyph.Exclamation
            BootVerdict.FAIL -> Glyph.Cross
            BootVerdict.SKIPPED -> Glyph.Minus
        }

val BootLine.tone: Color
    get() {
        // Termos que mudaram não são erro nem acerto: a cor de informação, e a tela de
        // aceite vem em seguida.
        if (detail == BootDetail.TERMSCHANGED) return LognDark.info
        return when (verdict) {
            BootVerdict.RUNNING -> LognDark.textMuted
            BootVerdict.OK -> LognDark.correct
            BootVerdict.WARN -> LognDark.warn
            BootVerdict.FAIL -> LognDark.wrong
            BootVerdict.SKIPPED -> LognDark.textMuted
        }
    }
