package io.github.anand34577.shortr.ui.links

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import io.github.anand34577.shortr.data.ApiException
import io.github.anand34577.shortr.data.AppContainer
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.data.LinkStats
import io.github.anand34577.shortr.ui.common.StatsRange
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class LinkDetailViewModel(private val c: AppContainer, private val id: String) : ViewModel() {
    var link by mutableStateOf<Link?>(null)
        private set
    var stats by mutableStateOf<LinkStats?>(null)
        private set
    var range by mutableStateOf(StatsRange.Week)
        private set
    var loading by mutableStateOf(true)
        private set
    var error by mutableStateOf<String?>(null)
        private set
    var qr by mutableStateOf<Pair<Bitmap, ByteArray>?>(null)
        private set
    var busy by mutableStateOf(false)
        private set

    init {
        load()
        viewModelScope.launch { c.linkChanges.collect { load(quiet = true) } }
    }

    fun selectRange(r: StatsRange) {
        range = r
        loadStats()
    }

    fun load(quiet: Boolean = false) {
        val api = c.api() ?: return
        if (!quiet) loading = true
        viewModelScope.launch {
            try {
                val (from, to) = range.window()
                val l = async { api.link(id) }
                val s = async { runCatching { api.linkStats(id, from, to) }.getOrNull() }
                link = l.await()
                stats = s.await()
                error = null
            } catch (e: ApiException) {
                if (e.isAuth) c.signOut() else error = e.message
            } finally {
                loading = false
            }
        }
    }

    private fun loadStats() {
        val api = c.api() ?: return
        viewModelScope.launch {
            val (from, to) = range.window()
            runCatching { api.linkStats(id, from, to) }.onSuccess { stats = it }
        }
    }

    fun loadQr() {
        if (qr != null) return
        val api = c.api() ?: return
        viewModelScope.launch {
            runCatching {
                val png = api.qrPng(id)
                val bmp = withContext(Dispatchers.Default) { BitmapFactory.decodeByteArray(png, 0, png.size) }
                qr = bmp to png
            }
        }
    }

    suspend fun toggle(): String? = act { link = it.setLinkStatus(id, !(link?.isActive ?: true)) }
    suspend fun delete(): String? = act { it.deleteLink(id) }

    private suspend fun act(block: suspend (io.github.anand34577.shortr.data.ShortrApi) -> Unit): String? {
        val api = c.api() ?: return "Not connected"
        busy = true
        return try {
            block(api)
            c.notifyLinksChanged()
            null
        } catch (e: ApiException) {
            e.message
        } finally {
            busy = false
        }
    }
}
