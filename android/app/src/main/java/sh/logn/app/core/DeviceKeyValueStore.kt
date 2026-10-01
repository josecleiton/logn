package sh.logn.app.core

import android.content.Context
import android.content.SharedPreferences
import android.util.AtomicFile
import android.util.Base64
import androidx.core.content.edit
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import sh.logn.core.LogN.KeyValueError
import sh.logn.core.LogN.KeyValueOperation
import sh.logn.core.LogN.KeyValueResponse
import sh.logn.core.LogN.KeyValueResult
import sh.logn.core.LogN.Value
import sh.logn.coreshell.KeyValuePort
import java.io.File
import java.security.MessageDigest
import kotlin.coroutines.cancellation.CancellationException

/**
 * `Effect.SecureStore` (crux_kv), dividido por chave como em CoreWrapper.swift:
 *
 * - `refresh_token` e `track_key:*` são credencial: SharedPreferences cifradas, com a
 *   chave mestra no Android Keystore deste aparelho (o Keychain de iOS).
 * - `track_package:*` é o pacote cifrado da trilha paga: binário e grande, num arquivo.
 *   O nome é o hash da chave, e não a chave: ela vem do Core, e nada de fora vira caminho.
 * - O resto (fila offline, retrato da trilha, e-mail, preferências) vai em
 *   SharedPreferences comuns: não autentica ninguém.
 *
 * Se o Keystore falhar (acontece depois de restauração ou troca de tela de bloqueio), o
 * arquivo cifrado é apagado e recriado; se ainda assim falhar, as credenciais ficam só
 * na memória. A sessão cai ao reiniciar, mas o app não quebra.
 */
class DeviceKeyValueStore(
    private val context: Context,
) : KeyValuePort {
    private val mutex = Mutex()
    private val plain: SharedPreferences = context.getSharedPreferences(PLAIN_FILE, Context.MODE_PRIVATE)
    private var secure: SharedPreferences? = null
    private var secureBroken = false
    private val memory = mutableMapOf<String, ByteArray>()
    private var prepared = false

    override suspend fun perform(operation: KeyValueOperation): KeyValueResult =
        withContext(Dispatchers.IO) {
            mutex.withLock {
                try {
                    apply(operation)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    // Disco cheio, pacote corrompido: o Core recebe o erro e segue. Só a
                    // classe, porque a mensagem pode trazer a chave.
                    KeyValueResult.Err(KeyValueError.Io(e.javaClass.simpleName))
                }
            }
        }

    private fun apply(operation: KeyValueOperation): KeyValueResult {
        prepare()
        return when (operation) {
            is KeyValueOperation.Get -> ok(KeyValueResponse.Get(value(read(operation.key))))
            is KeyValueOperation.Set -> {
                val previous = read(operation.key)
                write(operation.key, ByteArray(operation.value.size) { operation.value[it].toByte() })
                ok(KeyValueResponse.Set(value(previous)))
            }
            is KeyValueOperation.Delete -> {
                val previous = read(operation.key)
                remove(operation.key)
                ok(KeyValueResponse.Delete(value(previous)))
            }
            is KeyValueOperation.Exists -> ok(KeyValueResponse.Exists(read(operation.key) != null))
            // O Core não lista chaves; iOS responde o mesmo erro.
            is KeyValueOperation.ListKeys -> KeyValueResult.Err(KeyValueError.Io("listKeys unsupported"))
        }
    }

    /**
     * A primeira abertura depois de instalar: apaga credencial que tenha sobrevivido
     * sem o resto (o equivalente a `prepareKeychain` de iOS). Sem isso, quem reinstalou
     * para sair da conta voltava logado, com fila e retrato zerados.
     */
    private fun prepare() {
        if (prepared) return
        prepared = true
        if (!plain.getBoolean(PREPARED_KEY, false)) {
            context.deleteSharedPreferences(SECURE_FILE)
            plain.edit(commit = true) { putBoolean(PREPARED_KEY, true) }
        }
    }

    private fun read(key: String): ByteArray? =
        when (route(key)) {
            Route.Secure -> onSecure({ it.getString(key, null)?.let(::decode) }) { memory[key] }
            Route.File -> packageFile(key).takeIf { it.exists() }?.let { AtomicFile(it).readFully() }
            Route.Plain -> plain.getString(key, null)?.let(::decode)
        }

    private fun write(
        key: String,
        bytes: ByteArray,
    ) {
        when (route(key)) {
            Route.Secure -> onSecure({ it.edit(commit = true) { putString(key, encode(bytes)) } }) { memory[key] = bytes }
            Route.File -> {
                val file = AtomicFile(packageFile(key))
                val out = file.startWrite()
                try {
                    out.write(bytes)
                    file.finishWrite(out)
                } catch (e: java.io.IOException) {
                    file.failWrite(out)
                    throw e
                }
            }
            Route.Plain -> plain.edit(commit = true) { putString(key, encode(bytes)) }
        }
    }

    private fun remove(key: String) {
        when (route(key)) {
            Route.Secure -> {
                memory.remove(key)
                onSecure({ it.edit(commit = true) { remove(key) } }) {}
            }
            Route.File -> packageFile(key).delete()
            Route.Plain -> plain.edit(commit = true) { remove(key) }
        }
    }

    /**
     * Uma operação no arquivo cifrado, ou `fallback` na memória se ele não existe ou falha.
     * A chave mestra invalidada (restauração, troca da tela de bloqueio) abre o arquivo e
     * lança ao decifrar: aí o arquivo é apagado e a memória vale até o app fechar.
     */
    private fun <T> onSecure(
        block: (SharedPreferences) -> T,
        fallback: () -> T,
    ): T {
        val store = secureStore() ?: return fallback()
        return try {
            block(store)
        } catch (e: CancellationException) {
            throw e
        } catch (_: Exception) {
            secureBroken = true
            secure = null
            context.deleteSharedPreferences(SECURE_FILE)
            fallback()
        }
    }

    private fun secureStore(): SharedPreferences? {
        if (secureBroken) return null
        secure?.let { return it }
        secure = openSecure() ?: run {
            // Chave do Keystore perdida ou arquivo que não abre: recomeça do zero.
            context.deleteSharedPreferences(SECURE_FILE)
            openSecure()
        }
        return secure
    }

    private fun openSecure(): SharedPreferences? =
        runCatching {
            val master = MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build()
            EncryptedSharedPreferences.create(
                context,
                SECURE_FILE,
                master,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
            )
        }.getOrNull()

    private fun packageFile(key: String): File {
        val dir = File(context.filesDir, PACKAGES_DIR).apply { mkdirs() }
        val name = MessageDigest.getInstance("SHA-256").digest(key.toByteArray()).joinToString("") { "%02x".format(it) }
        return File(dir, name)
    }

    private enum class Route { Secure, File, Plain }

    companion object {
        /** O arquivo comum, que o shell também lê antes de o Core existir. */
        const val PLAIN_FILE = "logn_store"
        private const val SECURE_FILE = "logn_secure"
        private const val PACKAGES_DIR = "TrackPackages"
        private const val PREPARED_KEY = "keychain_prepared"

        private fun route(key: String): Route =
            when {
                key == "refresh_token" || key.startsWith("track_key:") -> Route.Secure
                key.startsWith("track_package:") -> Route.File
                else -> Route.Plain
            }

        fun encode(bytes: ByteArray): String = Base64.encodeToString(bytes, Base64.NO_WRAP)

        fun decode(text: String): ByteArray? = runCatching { Base64.decode(text, Base64.NO_WRAP) }.getOrNull()

        private fun value(bytes: ByteArray?): Value = bytes?.let { b -> Value.Bytes(List(b.size) { b[it].toUByte() }) } ?: Value.None

        private fun ok(response: KeyValueResponse) = KeyValueResult.Ok(response)
    }
}
