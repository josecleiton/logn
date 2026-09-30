package sh.logn.coreshell

import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.currentTime
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import sh.logn.core.LogN.Duration
import sh.logn.core.LogN.Instant
import sh.logn.core.LogN.TimeRequest
import sh.logn.core.LogN.TimerId
import sh.logn.core.TimeResponse

@OptIn(ExperimentalCoroutinesApi::class)
class TimerRegistryTest {
    private val resolved = mutableListOf<Pair<UInt, TimeResponse>>()

    private fun TestScope.registry() = TimerRegistry(this, WallClock { currentTime }) { id, response -> resolved += id to response }

    @Test
    fun now_answers_at_once_with_the_clock() =
        runTest {
            advanceTimeBy(1_500)
            registry().handle(1u, TimeRequest.Now)
            val (id, response) = resolved.single()
            assertEquals(1u, id)
            assertEquals(TimeResponse.Now(sh.logn.core.Instant(1u, 500_000_000u)), response)
        }

    @Test
    fun notify_after_waits_for_the_duration() =
        runTest {
            registry().handle(2u, TimeRequest.NotifyAfter(TimerId(7u), Duration(2_000_000_000u)))
            advanceTimeBy(1_999)
            assertTrue(resolved.isEmpty())
            advanceTimeBy(2)
            assertEquals(2u to TimeResponse.DurationElapsed(sh.logn.core.TimerId(7u)), resolved.single())
        }

    @Test
    fun notify_at_in_the_past_fires_now() =
        runTest {
            advanceTimeBy(10_000)
            registry().handle(3u, TimeRequest.NotifyAt(TimerId(8u), Instant(1u, 0u)))
            runCurrent()
            assertEquals(3u to TimeResponse.InstantArrived(sh.logn.core.TimerId(8u)), resolved.single())
        }

    @Test
    fun clear_cancels_the_timer_and_answers() =
        runTest {
            val timers = registry()
            timers.handle(4u, TimeRequest.NotifyAfter(TimerId(9u), Duration(1_000_000_000u)))
            timers.handle(5u, TimeRequest.Clear(TimerId(9u)))
            advanceTimeBy(5_000)
            assertEquals(5u to TimeResponse.Cleared(sh.logn.core.TimerId(9u)), resolved.single())
        }
}
