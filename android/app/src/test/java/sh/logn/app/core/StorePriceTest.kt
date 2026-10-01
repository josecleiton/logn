package sh.logn.app.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class StorePriceTest {
    private val base = offer(micros = 40_000_000, formatted = "R$ 40,00", token = "base")

    private fun offer(
        micros: Long,
        formatted: String,
        token: String,
        option: String = "buy",
        currency: String = "BRL",
        percent: Int? = null,
        plain: Boolean = true,
    ) = StoreOffer(option, micros, formatted, currency, percent, token, plain)

    @Test
    fun noBaseMeansNoPrice() {
        assertNull(quote(null, listOf(base)))
    }

    @Test
    fun withoutDiscountTheFullPriceGoesWithoutToken() {
        val q = quote(base, listOf(base))!!
        assertEquals(StorePrice("R$ 40,00"), q.price)
        assertFalse(q.price.discounted)
        assertNull(q.offerToken)
    }

    @Test
    fun theEligibleDiscountWinsWithItsToken() {
        val first = offer(28_000_000, "R$ 28,00", "first", percent = 30)
        val q = quote(base, listOf(base, first))!!
        assertEquals(StorePrice("R$ 28,00", "R$ 40,00", 30), q.price)
        assertTrue(q.price.discounted)
        assertEquals("first", q.offerToken)
    }

    @Test
    fun theCheapestOfSeveralDiscountsWins() {
        val small = offer(36_000_000, "R$ 36,00", "small", percent = 10)
        val big = offer(20_000_000, "R$ 20,00", "big", percent = 50)
        assertEquals("big", quote(base, listOf(small, base, big))!!.offerToken)
    }

    @Test
    fun aFixedAmountDiscountGetsItsPercentComputed() {
        val fixed = offer(30_000_000, "R$ 30,00", "fixed")
        assertEquals(25, quote(base, listOf(fixed))!!.price.percent)
    }

    @Test
    fun aTinyDiscountNeverShowsZeroPercent() {
        val cent = offer(39_990_000, "R$ 39,99", "cent")
        assertEquals(1, quote(base, listOf(cent))!!.price.percent)
    }

    @Test
    fun rentalPreorderOtherOptionAndOtherCurrencyAreIgnored() {
        val rental = offer(5_000_000, "R$ 5,00", "rent", plain = false)
        val other = offer(5_000_000, "R$ 5,00", "other", option = "rent")
        val dollars = offer(5_000_000, "US$ 5.00", "usd", currency = "USD")
        val q = quote(base, listOf(rental, other, dollars))!!
        assertNull(q.offerToken)
        assertEquals(StorePrice("R$ 40,00"), q.price)
    }

    @Test
    fun anOfferNotBelowTheFullPriceIsNotADiscount() {
        val same = offer(40_000_000, "R$ 40,00", "same", percent = 0)
        assertNull(quote(base, listOf(same))!!.offerToken)
    }
}
