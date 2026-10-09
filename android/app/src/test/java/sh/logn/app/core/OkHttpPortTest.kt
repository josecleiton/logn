package sh.logn.app.core

import com.novi.serde.Bytes
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import sh.logn.core.LogN.HttpError
import sh.logn.core.LogN.HttpHeader
import sh.logn.core.LogN.HttpRequest
import sh.logn.core.LogN.HttpResult
import java.util.concurrent.TimeUnit

class OkHttpPortTest {
    private val server = MockWebServer()

    @get:Rule val cacheDir = TemporaryFolder()

    @Before fun start() = server.start()

    @After fun stop() = server.close()

    private fun port(client: OkHttpClient = OkHttpClient()) = OkHttpPort(server.url("/").toString(), client)

    @Test
    fun relative_url_resolves_against_the_base_and_headers_come_back() =
        runTest {
            server.enqueue(
                MockResponse
                    .Builder()
                    .code(429)
                    .addHeader("Retry-After", "30")
                    .body("{}")
                    .build(),
            )
            val result =
                port().perform(
                    HttpRequest(
                        "POST",
                        "/api/v1/auth/login",
                        listOf(HttpHeader("Content-Type", "application/json")),
                        Bytes("{\"a\":1}".toByteArray()),
                    ),
                )
            val recorded = server.takeRequest()
            assertEquals("/api/v1/auth/login", recorded.target)
            assertEquals("{\"a\":1}", recorded.body?.utf8())

            val response = (result as HttpResult.Ok).value
            assertEquals(429.toUShort(), response.status)
            assertTrue(response.headers.any { it.name.equals("Retry-After", true) && it.value == "30" })
        }

    @Test
    fun get_goes_without_a_body() =
        runTest {
            val ok = MockResponse.Builder().code(200).body("ok")
            server.enqueue(ok.build())
            val result = port().perform(HttpRequest("GET", "/ready", emptyList(), Bytes(ByteArray(0))))
            val body = (result as HttpResult.Ok).value.body
            assertEquals("ok", body.content.decodeToString())
            assertEquals("GET", server.takeRequest().method)
        }

    @Test
    fun a_server_that_does_not_answer_is_a_timeout() =
        runTest {
            server.enqueue(MockResponse.Builder().headersDelay(2, TimeUnit.SECONDS).build())
            val client = OkHttpClient.Builder().readTimeout(100, TimeUnit.MILLISECONDS).build()
            val result = port(client).perform(HttpRequest("GET", "/slow", emptyList(), Bytes(ByteArray(0))))
            assertEquals(HttpResult.Err(HttpError.Timeout), result)
        }

    @Test
    fun a_malformed_request_is_io_and_does_not_throw() =
        runTest {
            val bad = listOf(HttpHeader("X-Bad", "line\nbreak"))
            val result = port().perform(HttpRequest("GET", "/ready", bad, Bytes(ByteArray(0))))
            assertTrue(result is HttpResult.Err && result.value is HttpError.Io)
        }

    /**
     * Conteúdo da conta: o cache guarda a resposta `private, no-cache` mesmo com
     * `Authorization`, pergunta de novo com o ETag (também depois do refresh, com outro
     * token), e o 304 do servidor chega ao Core como 200 com o corpo guardado.
     */
    @Test
    fun account_content_is_revalidated_by_etag_and_served_from_the_cache() =
        runTest {
            val etag = "W/\"abc\""
            server.enqueue(
                MockResponse
                    .Builder()
                    .code(200)
                    .addHeader("ETag", etag)
                    .addHeader("Cache-Control", "private, no-cache")
                    .addHeader("Vary", "Accept-Language")
                    .body("[\"desafio\"]")
                    .build(),
            )
            server.enqueue(MockResponse.Builder().code(304).addHeader("ETag", etag).build())

            val port = port(OkHttpPort.defaultClient(cacheDir.newFolder("http")))
            val request = { token: String ->
                HttpRequest(
                    "GET",
                    "/api/v1/challenges",
                    listOf(HttpHeader("Authorization", "Bearer $token"), HttpHeader("Accept-Language", "pt-BR")),
                    Bytes(ByteArray(0)),
                )
            }

            port.perform(request("t1"))
            assertEquals(null, server.takeRequest().headers["If-None-Match"])

            val second = (port.perform(request("t2")) as HttpResult.Ok).value
            assertEquals(etag, server.takeRequest().headers["If-None-Match"])
            assertEquals(200.toUShort(), second.status)
            assertEquals("[\"desafio\"]", second.body.content.decodeToString())
        }

    @Test
    fun a_refused_connection_is_io() =
        runTest {
            val base = server.url("/").toString()
            server.close()
            val result = OkHttpPort(base).perform(HttpRequest("GET", "/ready", emptyList(), Bytes(ByteArray(0))))
            assertTrue(result is HttpResult.Err && result.value is HttpError.Io)
        }
}
