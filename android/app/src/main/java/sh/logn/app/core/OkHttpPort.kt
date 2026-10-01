package sh.logn.app.core

import com.novi.serde.Bytes
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import sh.logn.core.LogN.HttpError
import sh.logn.core.LogN.HttpHeader
import sh.logn.core.LogN.HttpRequest
import sh.logn.core.LogN.HttpResponse
import sh.logn.core.LogN.HttpResult
import sh.logn.coreshell.HttpPort
import java.io.InterruptedIOException
import java.util.concurrent.TimeUnit
import kotlin.coroutines.cancellation.CancellationException

/**
 * `Effect.Http` com OkHttp. Espelho de `handleHttp` em CoreWrapper.swift: a URL relativa
 * resolve contra a base do build, os cabeçalhos da resposta voltam inteiros (o Core lê
 * `Retry-After` de um 429 e `Content-Language`), e falha de rede vira `Io`, que o Core
 * trata como sem rede.
 *
 * Nada do pedido vai para o log: corpo, token e e-mail passam por aqui (regra 9).
 */
class OkHttpPort(
    private val baseUrl: String,
    private val client: OkHttpClient = defaultClient(),
) : HttpPort {
    override suspend fun perform(request: HttpRequest): HttpResult =
        withContext(Dispatchers.IO) {
            val base = baseUrl.toHttpUrlOrNull() ?: return@withContext HttpResult.Err(HttpError.Io(NO_BASE_URL))
            val url = base.resolve(request.url) ?: return@withContext HttpResult.Err(HttpError.Url(request.url))

            val contentType =
                request.headers
                    .firstOrNull { it.name.equals("content-type", ignoreCase = true) }
                    ?.value
                    ?.toMediaTypeOrNull()
            val bodyBytes = request.body.content
            val body =
                when {
                    bodyBytes.isEmpty() && request.method in listOf("GET", "HEAD") -> null
                    else -> bodyBytes.toRequestBody(contentType)
                }

            try {
                // Dentro do try: método com corpo que não aceita, ou cabeçalho com caractere
                // inválido, lançam aqui, e viram `Io` como qualquer falha.
                val builder = Request.Builder().url(url).method(request.method, body)
                for (header in request.headers) builder.addHeader(header.name, header.value)
                client.newCall(builder.build()).execute().use { response ->
                    val headers =
                        response.headers.names().flatMap { name ->
                            response.headers.values(name).map { HttpHeader(name, it) }
                        }
                    HttpResult.Ok(
                        HttpResponse(
                            status = response.code.toUShort(),
                            headers = headers,
                            body = Bytes(response.body.bytes()),
                        ),
                    )
                }
            } catch (_: InterruptedIOException) {
                // Leitura parada (`SocketTimeoutException`) ou o teto da chamada.
                HttpResult.Err(HttpError.Timeout)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                // Só a classe: a mensagem de OkHttp pode trazer a URL com e-mail na query.
                HttpResult.Err(HttpError.Io(e.javaClass.simpleName))
            }
        }

    private companion object {
        const val NO_BASE_URL = "no API base URL configured"

        fun defaultClient(): OkHttpClient =
            OkHttpClient
                .Builder()
                .connectTimeout(10, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                // Teto da chamada inteira: resposta que goteja não segura o Core para sempre.
                .callTimeout(60, TimeUnit.SECONDS)
                .build()
    }
}
