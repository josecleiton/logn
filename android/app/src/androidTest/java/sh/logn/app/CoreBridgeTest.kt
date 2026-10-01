package sh.logn.app

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import sh.logn.core.LogN.Event

/**
 * A ponte com o `.so` de verdade, no aparelho: o `ViewModel` decodifica, a abertura roda,
 * e o `bincode` dos tipos gerados bate com o do Core. É o teste que pega `generated/`
 * fora de sincronia antes do jogador.
 */
@RunWith(AndroidJUnit4::class)
class CoreBridgeTest {
    private val app get() = InstrumentationRegistry.getInstrumentation().targetContext.applicationContext as LognApplication

    @Test
    fun the_core_boots_and_renders_on_the_device() {
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            val core = app.core
            // A abertura já foi despachada pelo Application; um evento a mais não pode
            // derrubar a ponte, e o ViewModel tem de chegar com o que o Core pôs nele.
            core.update(Event.Tick(System.currentTimeMillis() / 1000))
            val view = core.view.value
            assertEquals(13, view.minAge.toInt())
        }
    }

    @Test
    fun guest_mode_reaches_the_app() {
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            val core = app.core
            core.update(Event.ContinueAsGuest)
            assertTrue(core.view.value.isGuest)
        }
    }
}
