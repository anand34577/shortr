package io.github.anand34577.shortr.ui.links

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import io.github.anand34577.shortr.data.ApiException
import io.github.anand34577.shortr.data.AppContainer
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.data.LinkDraft
import io.github.anand34577.shortr.ui.common.parseInstant
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonArray
import kotlinx.serialization.json.add
import java.time.Instant

enum class AliasState { Idle, Checking, Available, Taken, Invalid }
enum class PasswordMode { Keep, Set, Remove }

class LinkEditorViewModel(private val c: AppContainer, private val existing: Link?, initialUrl: String?) : ViewModel() {
    val isEdit = existing != null
    val hadPassword = existing?.hasPassword == true

    var targetUrl by mutableStateOf(existing?.targetUrl ?: initialUrl.orEmpty())
    var code by mutableStateOf(existing?.code.orEmpty())
        private set
    var title by mutableStateOf(existing?.title.orEmpty())
    var tags by mutableStateOf(existing?.tags.orEmpty())
        private set
    var tagInput by mutableStateOf("")
    var expiresAt by mutableStateOf(parseInstant(existing?.expiresAt))
    var maxClicks by mutableStateOf(existing?.maxClicks?.toString().orEmpty())
    var password by mutableStateOf("")
    var passwordMode by mutableStateOf(if (existing?.hasPassword == true) PasswordMode.Keep else PasswordMode.Set)
    var permanent by mutableStateOf(existing?.redirectStatus == 301 || existing?.redirectStatus == 308)
    var passQuery by mutableStateOf(existing?.passQuery ?: true)
    var showAdvanced by mutableStateOf(existing != null && (existing.expiresAt != null || existing.maxClicks != null || existing.hasPassword || existing.redirectStatus != 302))

    var aliasState by mutableStateOf(AliasState.Idle)
        private set
    var saving by mutableStateOf(false)
        private set
    var error by mutableStateOf<String?>(null)
    var fieldErrors by mutableStateOf<Map<String, String>>(emptyMap())
        private set
    private var aliasJob: Job? = null

    fun onCode(v: String) {
        code = v.filter { it.isLetterOrDigit() || it == '-' || it == '_' }.take(64)
        fieldErrors = fieldErrors - "code"
        aliasJob?.cancel()
        if (code.isBlank() || code == existing?.code) { aliasState = AliasState.Idle; return }
        aliasState = AliasState.Checking
        aliasJob = viewModelScope.launch {
            delay(400)
            aliasState = runCatching { c.api()!!.checkAlias(code) }.fold(
                { if (it.available) AliasState.Available else if (it.reason != null) AliasState.Invalid else AliasState.Taken },
                { AliasState.Idle },
            )
        }
    }

    fun addTag() {
        val t = tagInput.trim().trim(',')
        if (t.isNotEmpty() && tags.none { it.equals(t, true) } && tags.size < 10) tags = tags + t.take(32)
        tagInput = ""
    }

    fun removeTag(t: String) { tags = tags - t }

    fun save(onSaved: (Link, Boolean) -> Unit) {
        val api = c.api() ?: return
        if (tagInput.isNotBlank()) addTag()
        val url = targetUrl.trim().let { if (it.isNotEmpty() && !it.contains("://")) "https://$it" else it }
        val clicks = maxClicks.trim().takeIf { it.isNotEmpty() }?.toIntOrNull()
        val errs = buildMap {
            if (url.isEmpty()) put("targetUrl", "Paste the long URL to shorten")
            if (maxClicks.isNotBlank() && (clicks == null || clicks < 1)) put("maxClicks", "Use a whole number of 1 or more")
            if (expiresAt?.isBefore(Instant.now().plusSeconds(60)) == true) put("expiresAt", "Pick a time in the future")
            if (aliasState == AliasState.Taken) put("code", "This alias is already taken")
            if (aliasState == AliasState.Invalid) put("code", "This alias can't be used")
            if (passwordMode == PasswordMode.Set && showAdvanced && password.isNotEmpty() && password.length > 128) put("password", "Max 128 characters")
        }
        if (errs.isNotEmpty()) { fieldErrors = errs; return }

        saving = true
        error = null
        viewModelScope.launch {
            try {
                val status = if (permanent) 301 else 302
                val saved = if (existing == null) {
                    api.createLink(
                        LinkDraft(
                            targetUrl = url, code = code, title = title, tags = tags,
                            expiresAt = expiresAt?.toString(), maxClicks = clicks,
                            password = if (passwordMode == PasswordMode.Set) password else "",
                            redirectStatus = status, passQuery = passQuery,
                        ),
                    )
                } else {
                    val e = existing
                    val changes = buildJsonObject {
                        if (url != e.targetUrl) put("targetUrl", url)
                        if (code.isNotBlank() && code != e.code) put("code", code)
                        if (title != e.title) put("title", title)
                        if (tags != e.tags) putJsonArray("tags") { tags.forEach { add(it) } }
                        if (expiresAt != parseInstant(e.expiresAt)) put("expiresAt", expiresAt?.toString()?.let { kotlinx.serialization.json.JsonPrimitive(it) } ?: JsonNull)
                        if (clicks != e.maxClicks) put("maxClicks", clicks?.let { kotlinx.serialization.json.JsonPrimitive(it) } ?: JsonNull)
                        when (passwordMode) {
                            PasswordMode.Set -> if (password.isNotEmpty()) put("password", password)
                            PasswordMode.Remove -> if (e.hasPassword) put("password", "")
                            PasswordMode.Keep -> Unit
                        }
                        if (status != e.redirectStatus && !(permanent && e.redirectStatus == 308) && !(!permanent && e.redirectStatus == 307)) put("redirectStatus", status)
                        if (passQuery != e.passQuery) put("passQuery", passQuery)
                    }
                    if (changes.isEmpty()) e else api.updateLink(e.id, changes)
                }
                onSaved(saved, existing == null)
            } catch (e: ApiException) {
                // server field names -> ours
                fieldErrors = e.fields.mapKeys { (k, _) ->
                    when (k) { "target_url" -> "targetUrl"; "expires_at" -> "expiresAt"; "max_clicks" -> "maxClicks"; else -> k }
                }
                error = if (fieldErrors.isEmpty()) e.message else null
            } finally {
                saving = false
            }
        }
    }
}
