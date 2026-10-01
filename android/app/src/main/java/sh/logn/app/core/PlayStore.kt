package sh.logn.app.core

import android.app.Activity
import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.content.edit
import com.android.billingclient.api.BillingClient
import com.android.billingclient.api.BillingClientStateListener
import com.android.billingclient.api.BillingFlowParams
import com.android.billingclient.api.BillingResult
import com.android.billingclient.api.PendingPurchasesParams
import com.android.billingclient.api.ProductDetails
import com.android.billingclient.api.Purchase
import com.android.billingclient.api.PurchasesUpdatedListener
import com.android.billingclient.api.QueryProductDetailsParams
import com.android.billingclient.api.QueryPurchasesParams
import com.android.billingclient.api.queryProductDetails
import com.android.billingclient.api.queryPurchasesAsync
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import sh.logn.core.LogN.Event
import sh.logn.coreshell.Core
import sh.logn.coreshell.i18n.Str
import java.security.MessageDigest
import kotlin.coroutines.resume

/** A loja do Android, como o Core e o servidor a nomeiam. */
private const val PROVIDER = "google_play"

/**
 * A ponte com o Google Play: compra, escuta e restauração. Espelho de StoreKitManager.swift.
 *
 * Quem decide que comprou é o servidor. A compra aprovada pela loja vai ao Core
 * (`SubmitPurchase` com o token), o Core manda ao servidor, e o servidor reconhece a compra
 * no Play depois de gravar a licença (ADR 0022). O aparelho não reconhece nada: um app que
 * reconhecesse antes do servidor saber perdia a compra numa queda de rede.
 *
 * Na abertura, o que o Play diz que foi pago e ainda não reconhecido volta ao servidor: é o
 * `Transaction.unfinished` do iOS.
 */
class PlayStore(
    context: Context,
    private val scope: CoroutineScope,
) {
    /** O que a loja respondeu e o Core não sabe. A cópia sai do catálogo. */
    enum class Problem { Cancelled, Pending, StoreUnavailable, ProductMissing, NeedsAccount, OwnedElsewhere }

    var isPurchasing by mutableStateOf(false)
        private set
    var problem by mutableStateOf<Problem?>(null)
        private set

    /** O preço de cada produto na moeda da loja do jogador. Nunca um valor fixo no app. */
    val prices = mutableStateMapOf<String, String>()

    /** O visitante pediu uma compra: a folha "a compra fica na sua conta" abre com ela. */
    var guestPrompt by mutableStateOf<String?>(null)

    private var core: Core? = null
    private var pendingAfterSignUp: String? = null
    private val queued = mutableListOf<Submission>()
    private val details = mutableMapOf<String, ProductDetails>()

    /** O produto da última compra pedida, para o `ITEM_ALREADY_OWNED` saber qual reenviar. */
    private var requested: String? = null

    /**
     * As compras que o Core já fechou para cada conta, como SHA-256 de conta e token. No
     * iOS fechar tira a transação da fila da loja; no Play não há o que fechar no aparelho,
     * e sem esta lista a compra recusada voltava ao servidor a cada abertura. O token é a
     * prova da compra: só o hash fica guardado.
     */
    private val settledPrefs = context.getSharedPreferences(SETTLED_FILE, Context.MODE_PRIVATE)

    /** A compra e a conta que a fez: na troca de conta, o que era de outra não sai. */
    private data class Submission(
        val token: String,
        val productId: String,
        val restore: Boolean,
        val account: String,
    )

    private val listener =
        PurchasesUpdatedListener { result, purchases ->
            when (result.responseCode) {
                BillingClient.BillingResponseCode.OK -> purchases.orEmpty().forEach { handle(it, restore = false) }
                BillingClient.BillingResponseCode.USER_CANCELED -> problem = Problem.Cancelled
                // Já é da conta Google: reenvia a dela, reconhecida ou não, e a trilha abre.
                BillingClient.BillingResponseCode.ITEM_ALREADY_OWNED -> requested?.let { scope.launch { resubmitProduct(it) } }
                else -> problem = Problem.StoreUnavailable
            }
            isPurchasing = false
        }

    private val billing: BillingClient =
        BillingClient
            .newBuilder(context.applicationContext)
            .setListener(listener)
            .enablePendingPurchases(PendingPurchasesParams.newBuilder().enableOneTimeProducts().build())
            .enableAutoServiceReconnection()
            .build()

    fun attach(core: Core) {
        if (this.core != null) return
        this.core = core
        scope.launch {
            // O que o servidor confirmou sai da lista do Core. No Play não há transação a
            // finalizar no aparelho: o servidor já reconheceu.
            core.view.map { it.purchasesToFinish }.distinctUntilChanged().collect { ids ->
                val account = core.view.value.accountUserId
                for (id in ids) {
                    markSettled(account, id)
                    core.update(Event.PurchaseFinished(id))
                }
            }
        }
        scope.launch {
            core.view.map { it.hasAccessToken && !it.isGuest }.distinctUntilChanged().collect { ready ->
                if (!ready) return@collect
                flush()
                resubmitOwned()
                // O visitante acabou de ganhar conta: a compra que ele pediu continua, na
                // próxima tela que tiver uma Activity (`continuePending`).
            }
        }
    }

    /** A compra que o visitante pediu, depois do cadastro: roda assim que houver Activity. */
    fun continuePending(activity: Activity) {
        val product = pendingAfterSignUp ?: return
        val view = core?.view?.value ?: return
        if (!view.hasAccessToken || view.isGuest) return
        pendingAfterSignUp = null
        scope.launch { purchase(activity, product) }
    }

    fun loadPrices(productIds: List<String>) {
        val missing = productIds.filter { it.isNotEmpty() && it !in prices }
        if (missing.isEmpty()) return
        scope.launch {
            if (!connect()) return@launch
            for (d in queryDetails(missing)) {
                details[d.productId] = d
                d.oneTimePurchaseOfferDetails?.formattedPrice?.let { prices[d.productId] = it }
            }
        }
    }

    /** Comprar de qualquer tela: o visitante vê primeiro a folha da conta. */
    fun buy(
        activity: Activity,
        productId: String,
    ) {
        val view = core?.view?.value ?: return
        if (view.isGuest || !view.hasSession) {
            guestPrompt = productId
            return
        }
        scope.launch { purchase(activity, productId) }
    }

    /** "Criar conta e continuar": guarda a compra e leva ao cadastro (ou ao login). */
    fun continueAfterSignUp(
        productId: String,
        register: Boolean,
        onRegister: (Boolean) -> Unit,
    ) {
        pendingAfterSignUp = productId
        guestPrompt = null
        onRegister(register)
        core?.update(Event.Logout)
    }

    private suspend fun purchase(
        activity: Activity,
        productId: String,
    ) {
        problem = null
        val core = core ?: return
        val view = core.view.value
        // A compra nasce com o id da conta: o servidor recusa a de outra conta.
        if (view.isGuest || view.accountUserId.isEmpty()) {
            problem = Problem.NeedsAccount
            return
        }
        // Só a compra pedida agora abre o passo a passo; a reentregue segue em silêncio.
        core.update(Event.PurchaseIntent(productId))
        requested = productId
        isPurchasing = true
        if (!connect()) {
            problem = Problem.StoreUnavailable
            isPurchasing = false
            return
        }
        val product = details[productId] ?: queryDetails(listOf(productId)).firstOrNull()
        if (product == null) {
            problem = Problem.ProductMissing
            isPurchasing = false
            return
        }
        val params =
            BillingFlowParams
                .newBuilder()
                .setProductDetailsParamsList(
                    listOf(BillingFlowParams.ProductDetailsParams.newBuilder().setProductDetails(product).build()),
                ).setObfuscatedAccountId(view.accountUserId)
                .build()
        val result = billing.launchBillingFlow(activity, params)
        // O resultado de verdade chega no `listener`; aqui só a falha de abrir a folha.
        if (result.responseCode != BillingClient.BillingResponseCode.OK) {
            problem = if (result.responseCode == BillingClient.BillingResponseCode.USER_CANCELED) Problem.Cancelled else Problem.StoreUnavailable
            isPurchasing = false
        }
    }

    /** "Restaurar compras": cada compra da conta Google vai ao servidor, contadas. */
    fun restore() {
        problem = null
        val core = core ?: return
        val view = core.view.value
        if (view.isGuest || !view.hasAccessToken) {
            problem = Problem.NeedsAccount
            return
        }
        scope.launch {
            if (!connect()) {
                problem = Problem.StoreUnavailable
                return@launch
            }
            // A restauração manda o que foi pago, desta conta ou de nenhuma (compra antiga,
            // sem conta marcada): o servidor diz de quem é, e conta as de outra.
            val owned = queryOwned().filter { it.purchaseState == Purchase.PurchaseState.PURCHASED }
            val account = view.accountUserId
            core.update(Event.RestoreStarted(owned.size.toUInt()))
            for (p in owned) for (id in p.products) submit(Submission(p.purchaseToken, id, restore = true, account = account))
        }
    }

    /** A conta LogN que fez a compra, marcada no `obfuscatedAccountId`. Vazia se não marcada. */
    private fun Purchase.buyer(): String = accountIdentifiers?.obfuscatedAccountId.orEmpty()

    private fun handle(
        purchase: Purchase,
        restore: Boolean,
    ) {
        val account = core?.view?.value?.accountUserId.orEmpty()
        when (purchase.purchaseState) {
            Purchase.PurchaseState.PURCHASED -> {
                // Compra de outra conta LogN no mesmo Google: o servidor recusaria, e ela
                // voltaria a cada abertura. Não sai daqui.
                if (purchase.buyer().isNotEmpty() && purchase.buyer() != account) return
                for (id in purchase.products) submit(Submission(purchase.purchaseToken, id, restore, account))
            }
            // Boleto ou dinheiro: a compra existe, o pagamento não. O Play entrega de novo
            // quando cair, e aí ela vai ao servidor.
            Purchase.PurchaseState.PENDING -> problem = Problem.Pending
            else -> Unit
        }
    }

    /** Pago e nunca reconhecido: a rede caiu depois da cobrança, ou o app fechou. */
    private suspend fun resubmitOwned() {
        if (!connect()) return
        val account = core?.view?.value?.accountUserId.orEmpty()
        for (p in queryOwned()) {
            if (p.purchaseState != Purchase.PurchaseState.PURCHASED || p.isAcknowledged) continue
            if (isSettled(account, p.purchaseToken)) continue
            handle(p, restore = false)
        }
    }

    /** O produto que o Play diz já ser da conta Google: reenvia a compra dele. */
    private suspend fun resubmitProduct(productId: String) {
        if (!connect()) return
        val account = core?.view?.value?.accountUserId.orEmpty()
        val owned =
            queryOwned().firstOrNull {
                productId in it.products && it.purchaseState == Purchase.PurchaseState.PURCHASED
            } ?: return
        if (owned.buyer().isNotEmpty() && owned.buyer() != account) {
            problem = Problem.OwnedElsewhere
            return
        }
        submit(Submission(owned.purchaseToken, productId, restore = false, account = account))
    }

    private fun submit(item: Submission) {
        queued += item
        flush()
    }

    private fun flush() {
        val core = core ?: return
        val view = core.view.value
        if (!view.hasAccessToken || view.isGuest) return
        val pending = queued.toList()
        queued.clear()
        for (item in pending) {
            // A compra que esperava sessão pertence a quem comprou; outra conta não a manda.
            if (item.account != view.accountUserId) continue
            // O token é a prova e o id da transação (ADR 0022).
            core.update(Event.SubmitPurchase("", item.token, item.productId, item.restore, PROVIDER, item.token))
        }
    }

    private fun settledKey(
        account: String,
        token: String,
    ): String = MessageDigest.getInstance("SHA-256").digest("$account:$token".toByteArray()).joinToString("") { "%02x".format(it) }

    private fun markSettled(
        account: String,
        token: String,
    ) {
        val current = settledPrefs.getStringSet(SETTLED_KEY, emptySet()).orEmpty()
        settledPrefs.edit { putStringSet(SETTLED_KEY, current + settledKey(account, token)) }
    }

    private fun isSettled(
        account: String,
        token: String,
    ): Boolean = settledKey(account, token) in settledPrefs.getStringSet(SETTLED_KEY, emptySet()).orEmpty()

    private suspend fun queryDetails(ids: List<String>): List<ProductDetails> {
        val params =
            QueryProductDetailsParams
                .newBuilder()
                .setProductList(
                    ids.map {
                        QueryProductDetailsParams.Product
                            .newBuilder()
                            .setProductId(it)
                            .setProductType(BillingClient.ProductType.INAPP)
                            .build()
                    },
                ).build()
        return runCatching { billing.queryProductDetails(params).productDetailsList.orEmpty() }.getOrDefault(emptyList())
    }

    private suspend fun queryOwned(): List<Purchase> {
        val params = QueryPurchasesParams.newBuilder().setProductType(BillingClient.ProductType.INAPP).build()
        return runCatching { billing.queryPurchasesAsync(params).purchasesList }.getOrDefault(emptyList())
    }

    private suspend fun connect(): Boolean {
        if (billing.isReady) return true
        return suspendCancellableCoroutine { cont ->
            billing.startConnection(
                object : BillingClientStateListener {
                    override fun onBillingSetupFinished(result: BillingResult) {
                        if (cont.isActive) cont.resume(result.responseCode == BillingClient.BillingResponseCode.OK)
                    }

                    override fun onBillingServiceDisconnected() {
                        if (cont.isActive) cont.resume(false)
                    }
                },
            )
        }
    }

    companion object {
        fun copy(
            context: Context,
            problem: Problem,
        ): String =
            when (problem) {
                Problem.Cancelled -> Str.Paywall.cancelled(context)
                Problem.Pending -> Str.Status.purchase_pending(context)
                Problem.StoreUnavailable -> Str.Status.store_unavailable(context)
                Problem.ProductMissing -> Str.Paywall.product_missing(context)
                Problem.NeedsAccount -> Str.Paywall.needs_account(context)
                Problem.OwnedElsewhere -> Str.Status.purchase_owned_by_other_account(context)
            }

        private const val SETTLED_FILE = "logn_play"
        private const val SETTLED_KEY = "settled"
    }
}
