package io.github.anand34577.shortr.data

import android.content.Context
import android.content.Intent
import android.net.Uri
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import net.openid.appauth.AppAuthConfiguration
import net.openid.appauth.AuthState
import net.openid.appauth.AuthorizationException
import net.openid.appauth.AuthorizationRequest
import net.openid.appauth.AuthorizationResponse
import net.openid.appauth.AuthorizationService
import net.openid.appauth.AuthorizationServiceConfiguration
import net.openid.appauth.ResponseTypeValues
import net.openid.appauth.connectivity.ConnectionBuilder
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.TimeUnit
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

sealed interface SessionState {
    data object Loading : SessionState
    data object SignedOut : SessionState
    data class SignedIn(val session: Session, val api: ShortrApi) : SessionState
}

/** Where SSO sends the browser back to; register it on your Keycloak client. */
const val OAUTH_REDIRECT = "io.github.anand34577.shortr:/oauth2redirect"

/** Process-wide objects, created once in [io.github.anand34577.shortr.ShortrApp]. */
class AppContainer(private val context: Context) {
    val storage = Storage(context)
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    /** Outlives any screen: for work like "Undo" that must finish after navigating away. */
    val appScope: CoroutineScope get() = scope
    private val _state = MutableStateFlow<SessionState>(SessionState.Loading)
    val state: StateFlow<SessionState> = _state

    /** A link the user shared into the app, waiting to be shortened. */
    val pendingShare = MutableStateFlow<String?>(null)
    /** A shortr://connect link, waiting for the connect screen. */
    val pendingPairing = MutableStateFlow<Uri?>(null)
    /** Launcher shortcut "New link". */
    val pendingNewLink = MutableStateFlow(false)

    /** Fires after any link is created, edited or deleted, so open screens reload. */
    val linkChanges = MutableSharedFlow<Unit>(extraBufferCapacity = 4)
    fun notifyLinksChanged() { linkChanges.tryEmit(Unit) }

    /** App lock: true while the unlock screen should cover the app. */
    val locked = MutableStateFlow(false)

    // Homelab identity providers often run on plain http inside the LAN;
    // AppAuth only allows https unless told otherwise.
    private val authConfig = AppAuthConfiguration.Builder()
        .setConnectionBuilder(AnySchemeConnectionBuilder)
        .setSkipIssuerHttpsCheck(true)
        .build()
    val authService: AuthorizationService by lazy { AuthorizationService(context, authConfig) }

    private val refreshLock = Mutex()

    init {
        scope.launch {
            val s = storage.loadSession()
            _state.value = if (s == null) SessionState.SignedOut else SessionState.SignedIn(s, apiFor(s))
        }
    }

    fun api(): ShortrApi? = (state.value as? SessionState.SignedIn)?.api

    /** Pair with an API key (typed in, or scanned from the web console). */
    suspend fun connectWithKey(serverInput: String, key: String): Session {
        val server = ShortrApi.normalizeServer(serverInput) ?: throw ApiException(0, "BAD_URL", "Enter the server address, e.g. https://sho.rt")
        val cleanKey = key.trim()
        if (!cleanKey.startsWith("sk_")) throw ApiException(0, "BAD_KEY", "API keys start with sk_. Create one under Settings → API keys in the web console.")
        val cfg = ShortrApi.probe(server)
        // Use the address the user chose (it may be a VPN/LAN one), not the
        // server's advertised public URL.
        val api = ShortrApi(server, { "Bearer $cleanKey" })
        val me = api.me()
        val session = Session(server = api.server, siteName = cfg.siteName, method = AuthMethod.ApiKey, apiKey = cleanKey, account = me.email)
        storage.saveSession(session)
        _state.value = SessionState.SignedIn(session, api)
        return session
    }

    /** Step 1 of SSO: an intent that opens the identity provider in a Custom Tab. */
    suspend fun ssoIntent(oidc: OidcConfig): Intent {
        val cfg = suspendCancellableCoroutine { cont ->
            AuthorizationServiceConfiguration.fetchFromIssuer(Uri.parse(oidc.issuer), { c, ex ->
                if (c != null) cont.resume(c) else cont.resumeWithException(
                    ApiException(0, "SSO_DISCOVERY", "Couldn't reach your sign-in provider (${ex?.errorDescription ?: ex?.message ?: "unknown error"})."),
                )
            }, AnySchemeConnectionBuilder)
        }
        val req = AuthorizationRequest.Builder(cfg, oidc.clientId, ResponseTypeValues.CODE, Uri.parse(OAUTH_REDIRECT))
            .setScopes(oidc.scopes)
            .build()
        return authService.getAuthorizationRequestIntent(req)
    }

    /** Step 2 of SSO: trade the returned code for tokens, then confirm with the server. */
    suspend fun completeSso(server: String, siteName: String, result: Intent?): Session {
        val resp = result?.let { AuthorizationResponse.fromIntent(it) }
        val err = result?.let { AuthorizationException.fromIntent(it) }
        if (resp == null) {
            throw ApiException(0, "SSO_CANCELLED", err?.errorDescription ?: "Sign-in was cancelled.")
        }
        val authState = AuthState(resp, err)
        val tokens = suspendCancellableCoroutine { cont ->
            authService.performTokenRequest(resp.createTokenExchangeRequest()) { t, ex ->
                if (t != null) cont.resume(t) else cont.resumeWithException(
                    ApiException(0, "SSO_TOKEN", "Sign-in failed at the token step (${ex?.errorDescription ?: ex?.message ?: "unknown error"})."),
                )
            }
        }
        authState.update(tokens, null)
        val probeApi = ShortrApi(server, { tokens.accessToken?.let { "Bearer $it" } })
        val me = try {
            probeApi.me()
        } catch (e: ApiException) {
            if (e.isAuth) throw ApiException(401, "SSO_NOT_LINKED",
                "Signed in, but the server didn't accept the token. Sign in to the web console with SSO once, and ask your admin to add this app's client ID to SHORTR_OIDC_API_AUDIENCES.")
            throw e
        }
        val session = Session(server = server, siteName = siteName, method = AuthMethod.Sso, authState = authState.jsonSerializeString(), account = me.email)
        storage.saveSession(session)
        _state.value = SessionState.SignedIn(session, apiFor(session))
        return session
    }

    fun signOut() {
        scope.launch {
            storage.clearSession()
            _state.value = SessionState.SignedOut
        }
    }

    private fun apiFor(s: Session): ShortrApi = when (s.method) {
        AuthMethod.ApiKey -> ShortrApi(s.server, { "Bearer ${s.apiKey}" })
        AuthMethod.Sso -> {
            val auth = AuthState.jsonDeserialize(s.authState ?: "{}")
            ShortrApi(s.server, { freshToken(s, auth)?.let { "Bearer $it" } })
        }
    }

    /** Returns a valid access token, refreshing (and persisting) it when needed. */
    private suspend fun freshToken(s: Session, auth: AuthState): String? = refreshLock.withLock {
        val before = auth.accessToken
        val token = suspendCancellableCoroutine { cont ->
            auth.performActionWithFreshTokens(authService) { access, _, ex ->
                if (ex != null) cont.resume(null) else cont.resume(access)
            }
        }
        if (token != null && token != before) storage.saveSession(s.copy(authState = auth.jsonSerializeString()))
        token
    }
}

/** AppAuth connection builder that also accepts http:// issuers (LAN Keycloak). */
private object AnySchemeConnectionBuilder : ConnectionBuilder {
    override fun openConnection(uri: Uri): HttpURLConnection =
        (URL(uri.toString()).openConnection() as HttpURLConnection).apply {
            connectTimeout = TimeUnit.SECONDS.toMillis(15).toInt()
            readTimeout = TimeUnit.SECONDS.toMillis(15).toInt()
            instanceFollowRedirects = false
        }
}
