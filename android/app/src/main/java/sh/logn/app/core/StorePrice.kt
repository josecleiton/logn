package sh.logn.app.core

/** O preço que o jogador vê: o que ele paga e, com desconto, o cheio e o percentual. */
data class StorePrice(
    val formatted: String,
    val full: String? = null,
    val percent: Int? = null,
) {
    val discounted: Boolean get() = full != null && percent != null
}

/**
 * Uma oferta de compra única como o Play a manda, só com o que a escolha do preço lê.
 * `plain` é a compra simples: nem aluguel nem pré-venda.
 */
data class StoreOffer(
    val purchaseOptionId: String?,
    val priceMicros: Long,
    val formatted: String,
    val currency: String,
    val percent: Int?,
    val token: String?,
    val plain: Boolean,
)

/** O preço a mostrar e o `offerToken` a mandar na compra; sem desconto, nenhum token. */
data class StoreQuote(
    val price: StorePrice,
    val offerToken: String?,
)

/**
 * O preço da conta para um produto. `base` é a oferta da opção backwards compatible, a que
 * o app comprava sem token; `offers` é a lista do Play, que só traz o que esta conta pode
 * usar. O desconto de primeira compra some da lista depois de usado, e aí o preço volta ao
 * cheio sem estado nosso.
 *
 * Vale só desconto da mesma opção de compra, na mesma moeda e abaixo do preço cheio: outra
 * opção pode ser aluguel, e o percentual de uma moeda não se compara com o de outra.
 */
fun quote(
    base: StoreOffer?,
    offers: List<StoreOffer>,
): StoreQuote? {
    if (base == null) return null
    val best =
        offers
            .filter { it.plain && it.purchaseOptionId == base.purchaseOptionId && it.currency == base.currency && it.priceMicros < base.priceMicros }
            .minByOrNull { it.priceMicros }
            ?: return StoreQuote(StorePrice(base.formatted), null)
    // O Play manda o percentual quando o desconto é por percentual; por valor fixo, a conta é
    // nossa. Nunca zero: um centavo de desconto ainda é desconto.
    val percent = best.percent ?: Math.round((base.priceMicros - best.priceMicros) * 100.0 / base.priceMicros).toInt()
    return StoreQuote(StorePrice(best.formatted, base.formatted, percent.coerceAtLeast(1)), best.token)
}
