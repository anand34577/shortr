package io.github.anand34577.shortr.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonArray
import kotlinx.serialization.json.add
import okhttp3.Call
import okhttp3.Callback
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.util.UUID
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLException
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/** Anything the server or network refused, with a message fit for a snackbar. */
class ApiException(
    val status: Int,
    val code: String,
    override val message: String,
    val fields: Map<String, String> = emptyMap(),
) : Exception(message) {
    val isAuth: Boolean get() = status == 401
}

val ShortrJson = Json {
    ignoreUnknownKeys = true
    explicitNulls = false
    encodeDefaults = true
    coerceInputValues = true
}

private val JSON_TYPE = "application/json".toMediaType()

/**
 * Talks to one Shortr server. [authHeader] supplies "Bearer …" per request
 * (a fixed API key, or a freshly refreshed SSO access token).
 */
class ShortrApi(
    val server: String,
    private val authHeader: suspend () -> String?,
    private val http: OkHttpClient = defaultClient,
) {
    private val base: HttpUrl = server.toHttpUrlOrNull() ?: throw ApiException(0, "BAD_URL", "That server address isn't valid.")

    suspend fun me(): Me = get("api/v1/me")

    suspend fun links(q: String = "", status: String = "", sort: String = "", cursor: String? = null, limit: Int = 30): Page<Link> =
        get("api/v1/links") {
            if (q.isNotBlank()) addQueryParameter("q", q.trim())
            if (status.isNotBlank()) addQueryParameter("status", status)
            if (sort.isNotBlank()) { addQueryParameter("sort", sort); addQueryParameter("order", "desc") }
            if (!cursor.isNullOrBlank()) addQueryParameter("cursor", cursor)
            addQueryParameter("limit", limit.toString())
        }

    suspend fun link(id: String): Link = get("api/v1/links/$id")

    suspend fun createLink(d: LinkDraft): Link {
        val body = buildJsonObject {
            put("targetUrl", d.targetUrl.trim())
            if (d.code.isNotBlank()) put("code", d.code.trim())
            if (d.title.isNotBlank()) put("title", d.title.trim())
            if (d.tags.isNotEmpty()) putJsonArray("tags") { d.tags.forEach { add(it) } }
            d.expiresAt?.let { put("expiresAt", it) }
            d.maxClicks?.let { put("maxClicks", it) }
            if (d.password.isNotEmpty()) put("password", d.password)
            d.redirectStatus?.let { put("redirectStatus", it) }
            put("passQuery", d.passQuery)
        }
        return send("POST", "api/v1/links", body, idempotent = true)
    }

    /** PATCH with only the changed fields; a JSON null clears a value. */
    suspend fun updateLink(id: String, changes: JsonObject): Link = send("PATCH", "api/v1/links/$id", changes)

    suspend fun setLinkStatus(id: String, active: Boolean): Link =
        updateLink(id, buildJsonObject { put("status", if (active) "active" else "disabled") })

    suspend fun deleteLink(id: String) { sendRaw("DELETE", url("api/v1/links/$id"), null).close() }

    suspend fun restoreLink(id: String) { sendRaw("POST", url("api/v1/links/$id/restore"), "{}").close() }

    suspend fun checkAlias(code: String): AliasCheck = get("api/v1/links/check") { addQueryParameter("code", code) }

    suspend fun linkStats(id: String, from: String, to: String): LinkStats =
        get("api/v1/links/$id/stats") { addQueryParameter("from", from); addQueryParameter("to", to) }

    suspend fun overview(from: String, to: String): Overview =
        get("api/v1/stats/overview") { addQueryParameter("from", from); addQueryParameter("to", to) }

    suspend fun recent(): List<RecentClick> = get<Page<RecentClick>>("api/v1/stats/recent") { addQueryParameter("limit", "20") }.items

    suspend fun visitors(id: String, cursor: String? = null, limit: Int = 25): Page<VisitorClick> =
        get("api/v1/links/$id/clicks") {
            addQueryParameter("limit", limit.toString())
            if (!cursor.isNullOrBlank()) addQueryParameter("cursor", cursor)
        }

    suspend fun qrPng(id: String, size: Int = 768): ByteArray {
        val u = base.newBuilder().addPathSegments("api/v1/links/$id/qr.png").addQueryParameter("size", size.toString()).build()
        return sendRaw("GET", u, null).use { it.body.bytes() }
    }

    // --- plumbing ---------------------------------------------------------

    private fun url(path: String, q: HttpUrl.Builder.() -> Unit = {}): HttpUrl =
        base.newBuilder().addPathSegments(path).apply(q).build()

    private suspend inline fun <reified T> get(path: String, noinline q: HttpUrl.Builder.() -> Unit = {}): T =
        decode(sendRaw("GET", url(path, q), null))

    private suspend inline fun <reified T> send(method: String, path: String, body: JsonObject, idempotent: Boolean = false): T =
        decode(sendRaw(method, url(path), body.toString(), idempotent))

    private suspend inline fun <reified T> decode(resp: Response): T = resp.use {
        withContext(Dispatchers.Default) { ShortrJson.decodeFromString<T>(it.body.string()) }
    }

    private suspend fun sendRaw(method: String, url: HttpUrl, json: String?, idempotent: Boolean = false): Response {
        val rb = Request.Builder().url(url).header("Accept", "application/json")
        authHeader()?.let { rb.header("Authorization", it) }
        if (idempotent) rb.header("Idempotency-Key", UUID.randomUUID().toString())
        rb.method(method, json?.toRequestBody(JSON_TYPE) ?: if (method == "GET" || method == "DELETE") null else "".toRequestBody(JSON_TYPE))
        val resp = execute(http, rb.build())
        if (!resp.isSuccessful) throw resp.use { toApiException(it) }
        return resp
    }

    companion object {
        val defaultClient: OkHttpClient = OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(20, TimeUnit.SECONDS)
            .writeTimeout(20, TimeUnit.SECONDS)
            .retryOnConnectionFailure(true)
            .build()

        /**
         * Turns what people type into an origin: adds https:// when no scheme
         * is given and drops a pasted path such as /app/links.
         */
        fun normalizeServer(input: String): String? {
            var s = input.trim().trimEnd('/')
            if (s.isEmpty()) return null
            if (!s.startsWith("http://", true) && !s.startsWith("https://", true)) s = "https://$s"
            val u = s.toHttpUrlOrNull() ?: return null
            return u.newBuilder().encodedPath("/").query(null).fragment(null).build().toString().trimEnd('/')
        }

        /** Reads /api/v1/client-config: proves the address is Shortr before we store anything. */
        suspend fun probe(server: String): ClientConfig {
            val u = "$server/api/v1/client-config".toHttpUrlOrNull()
                ?: throw ApiException(0, "BAD_URL", "That server address isn't valid.")
            val resp = execute(defaultClient, Request.Builder().url(u).header("Accept", "application/json").build())
            return resp.use {
                if (it.code == 404) {
                    throw ApiException(404, "NOT_SHORTR", "No Shortr API at this address. Check the URL, or whether the server sets SHORTR_PUBLIC_API=false.")
                }
                if (!it.isSuccessful) throw toApiException(it)
                val cfg = runCatching { ShortrJson.decodeFromString<ClientConfig>(it.body.string()) }.getOrNull()
                if (cfg == null || cfg.name != "shortr") {
                    throw ApiException(0, "NOT_SHORTR", "That address answered, but it isn't a Shortr server.")
                }
                cfg
            }
        }

        private fun toApiException(resp: Response): ApiException {
            val body = runCatching { ShortrJson.decodeFromString<ApiErrorBody>(resp.body.string()).error }.getOrNull()
            val msg = when {
                resp.code == 401 -> "Your access to this server has ended. Connect again."
                resp.code == 403 && body?.code == "FORBIDDEN" -> body.message.replaceFirstChar { it.uppercase() }
                resp.code == 429 -> "Too many requests. Wait a moment and try again."
                resp.code >= 500 && body == null -> "The server had a problem (HTTP ${resp.code})."
                body != null && body.message.isNotBlank() -> body.message.replaceFirstChar { it.uppercase() }
                else -> "Request failed (HTTP ${resp.code})."
            }
            return ApiException(resp.code, body?.code ?: "HTTP_${resp.code}", msg, body?.fields.orEmpty())
        }

        private suspend fun execute(client: OkHttpClient, req: Request): Response = suspendCancellableCoroutine { cont ->
            val call = client.newCall(req)
            cont.invokeOnCancellation { call.cancel() }
            call.enqueue(object : Callback {
                override fun onResponse(call: Call, response: Response) = cont.resume(response)
                override fun onFailure(call: Call, e: IOException) = cont.resumeWithException(networkError(e))
            })
        }

        private fun networkError(e: IOException): ApiException = ApiException(
            0, "NETWORK",
            when (e) {
                is UnknownHostException -> "Can't find that server. Check the address, or connect to your VPN."
                is ConnectException -> "The server isn't answering. Is it running and reachable from this phone?"
                is SocketTimeoutException -> "The server took too long to answer."
                is SSLException -> "Secure connection failed. If you use your own certificate authority, install it on this phone."
                else -> "Network problem: ${e.message ?: "unknown"}"
            },
        )
    }
}
