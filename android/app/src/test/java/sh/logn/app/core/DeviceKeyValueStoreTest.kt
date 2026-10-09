package sh.logn.app.core

import android.app.Application
import android.content.Context
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import sh.logn.core.LogN.KeyValueOperation
import sh.logn.core.LogN.KeyValueResponse
import sh.logn.core.LogN.KeyValueResult
import sh.logn.core.LogN.Value
import java.io.File

// Robolectric não tem Android Keystore: a credencial cai na memória, e é isso que o
// teste prova, além do roteamento. A cifra de verdade é do teste no aparelho. SDK 35: o
// maior que o Robolectric 4.15 conhece. `Application` pura: a do app carrega o `.so`, que
// não existe na JVM.
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], application = Application::class)
class DeviceKeyValueStoreTest {
    private val context: Context = ApplicationProvider.getApplicationContext()

    private fun bytes(text: String) = text.toByteArray().map { it.toUByte() }

    private suspend fun DeviceKeyValueStore.get(key: String): Value =
        ((perform(KeyValueOperation.Get(key)) as KeyValueResult.Ok).response as KeyValueResponse.Get).value

    @Test
    fun ordinary_keys_round_trip_through_plain_preferences() =
        runTest {
            val store = DeviceKeyValueStore(context)
            store.perform(KeyValueOperation.Set("offline_queue", bytes("[1,2]")))
            assertEquals(Value.Bytes(bytes("[1,2]")), store.get("offline_queue"))
            val raw = context.getSharedPreferences(DeviceKeyValueStore.PLAIN_FILE, Context.MODE_PRIVATE)
            assertEquals("WzEsMl0=", raw.getString("offline_queue", null))
        }

    /** Apagar o retrato é a conta saindo do aparelho: o cache HTTP vai junto, e só ele. */
    @Test
    fun deleting_the_snapshot_wipes_the_account_and_nothing_else_does() =
        runTest {
            var wiped = 0
            val store = DeviceKeyValueStore(context) { wiped++ }
            store.perform(KeyValueOperation.Delete("offline_queue"))
            store.perform(KeyValueOperation.Delete("refresh_token"))
            assertEquals(0, wiped)
            store.perform(KeyValueOperation.Delete("offline_snapshot"))
            assertEquals(1, wiped)
        }

    @Test
    fun credentials_never_reach_plain_preferences() =
        runTest {
            val store = DeviceKeyValueStore(context)
            store.perform(KeyValueOperation.Set("refresh_token", bytes("rt")))
            store.perform(KeyValueOperation.Set("track_key:t1", bytes("k")))
            assertEquals(Value.Bytes(bytes("rt")), store.get("refresh_token"))
            val raw = context.getSharedPreferences(DeviceKeyValueStore.PLAIN_FILE, Context.MODE_PRIVATE)
            assertFalse(raw.contains("refresh_token"))
            assertFalse(raw.contains("track_key:t1"))
        }

    @Test
    fun track_packages_are_files_named_by_hash() =
        runTest {
            val store = DeviceKeyValueStore(context)
            store.perform(KeyValueOperation.Set("track_package:../../escape", bytes("pkg")))
            assertEquals(Value.Bytes(bytes("pkg")), store.get("track_package:../../escape"))
            val files = File(context.filesDir, "TrackPackages").listFiles().orEmpty()
            assertEquals(1, files.size)
            assertTrue(files.single().name.matches(Regex("[0-9a-f]{64}")))
        }

    @Test
    fun delete_returns_the_previous_value_and_exists_turns_false() =
        runTest {
            val store = DeviceKeyValueStore(context)
            store.perform(KeyValueOperation.Set("email", bytes("a@example.com")))
            val deleted = (store.perform(KeyValueOperation.Delete("email")) as KeyValueResult.Ok).response
            assertEquals(KeyValueResponse.Delete(Value.Bytes(bytes("a@example.com"))), deleted)
            val exists = (store.perform(KeyValueOperation.Exists("email")) as KeyValueResult.Ok).response
            assertEquals(KeyValueResponse.Exists(false), exists)
        }

    @Test
    fun a_missing_key_is_none() =
        runTest {
            val store = DeviceKeyValueStore(context)
            assertEquals(Value.None, store.get("never_written"))
            assertNull(DeviceKeyValueStore.decode("not base64 !!"))
        }
}
