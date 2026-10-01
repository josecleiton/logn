package sh.logn.app.core

import android.app.Activity
import com.google.android.play.core.ktx.launchReview
import com.google.android.play.core.ktx.requestReview
import com.google.android.play.core.review.ReviewManagerFactory
import kotlin.coroutines.cancellation.CancellationException

/**
 * `Effect.StoreReview`: a avaliação dentro do app, o par do `requestReview` do iOS.
 *
 * Quem decide se o cartão aparece é o Play, pela cota dele; o app não sabe se apareceu
 * nem o que o jogador respondeu, e não tem o que fazer com uma falha. Por isso ela é
 * engolida: avaliação é cortesia, não pode derrubar a partida que acabou de terminar.
 */
suspend fun requestPlayReview(activity: Activity) {
    try {
        val manager = ReviewManagerFactory.create(activity)
        manager.launchReview(activity, manager.requestReview())
    } catch (e: CancellationException) {
        throw e
    } catch (_: Exception) {
        // Sem Play Store, sem rede ou fora da cota: segue o jogo.
    }
}
