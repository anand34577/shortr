package io.github.anand34577.shortr.ui.onboarding

import android.content.Intent
import android.net.Uri
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import io.github.anand34577.shortr.data.ApiException
import io.github.anand34577.shortr.data.AppContainer
import io.github.anand34577.shortr.data.ClientConfig
import io.github.anand34577.shortr.data.ShortrApi
import kotlinx.coroutines.launch

enum class OnboardingStep { Welcome, Server, Scan }

class OnboardingViewModel(private val c: AppContainer) : ViewModel() {
    var step by mutableStateOf(OnboardingStep.Welcome)
    var serverInput by mutableStateOf("")
    var keyInput by mutableStateOf("")
    var config by mutableStateOf<ClientConfig?>(null)
        private set
    /** The origin we probed; SSO and keys are used against this. */
    var server by mutableStateOf<String?>(null)
        private set
    var busy by mutableStateOf(false)
        private set
    var error by mutableStateOf<String?>(null)

    fun probe() {
        val normalized = ShortrApi.normalizeServer(serverInput)
        if (normalized == null) {
            error = "Enter your server's address, for example https://sho.rt"
            return
        }
        task {
            val cfg = ShortrApi.probe(normalized)
            config = cfg
            server = normalized // keep what the user typed, e.g. a VPN address
            serverInput = normalized
        }
    }

    fun changeServer() {
        config = null
        server = null
        error = null
    }

    fun connectWithKey() = task { c.connectWithKey(server ?: serverInput, keyInput) }

    /** A shortr://connect?server=…&key=… link, from the QR scanner or a tapped link. */
    fun pair(uri: Uri) {
        val s = uri.getQueryParameter("server")
        val k = uri.getQueryParameter("key")
        if (s.isNullOrBlank() || k.isNullOrBlank()) {
            error = "That pairing code is incomplete. Create a new API key in the web console and scan its code."
            step = OnboardingStep.Welcome
            return
        }
        serverInput = s
        keyInput = k
        step = OnboardingStep.Server
        task { c.connectWithKey(s, k) }
    }

    /** Anything else a QR code might hold: a plain server address is still useful. */
    fun scanned(text: String) {
        val uri = Uri.parse(text)
        when {
            uri.scheme == "shortr" -> pair(uri)
            uri.scheme == "http" || uri.scheme == "https" -> {
                serverInput = text
                step = OnboardingStep.Server
                probe()
            }
            else -> {
                error = "That QR code isn't a Shortr pairing code."
                step = OnboardingStep.Welcome
            }
        }
    }

    suspend fun ssoIntent(): Intent? {
        val oidc = config?.oidc ?: return null
        return try {
            c.ssoIntent(oidc)
        } catch (e: ApiException) {
            error = e.message
            null
        }
    }

    fun completeSso(result: Intent?) = task {
        c.completeSso(server ?: return@task, config?.siteName ?: "Shortr", result)
    }

    private fun task(block: suspend () -> Unit) {
        if (busy) return
        busy = true
        error = null
        viewModelScope.launch {
            try {
                block()
            } catch (e: ApiException) {
                error = e.message
            } catch (e: Exception) {
                error = e.message ?: "Something went wrong."
            } finally {
                busy = false
            }
        }
    }
}
