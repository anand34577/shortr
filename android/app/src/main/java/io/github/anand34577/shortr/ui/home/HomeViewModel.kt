package io.github.anand34577.shortr.ui.home

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import io.github.anand34577.shortr.data.ApiException
import io.github.anand34577.shortr.data.AppContainer
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.data.LinkDraft
import io.github.anand34577.shortr.data.Me
import io.github.anand34577.shortr.data.Overview
import io.github.anand34577.shortr.data.RecentClick
import io.github.anand34577.shortr.ui.common.StatsRange
import kotlinx.coroutines.async
import kotlinx.coroutines.launch

class HomeViewModel(private val c: AppContainer) : ViewModel() {
    var range by mutableStateOf(StatsRange.Week)
        private set
    var me by mutableStateOf<Me?>(null)
        private set
    var overview by mutableStateOf<Overview?>(null)
        private set
    var recent by mutableStateOf<List<RecentClick>>(emptyList())
        private set
    var loading by mutableStateOf(true)
        private set
    var refreshing by mutableStateOf(false)
        private set
    var error by mutableStateOf<String?>(null)
        private set

    // quick shorten
    var quickUrl by mutableStateOf("")
    var quickBusy by mutableStateOf(false)
        private set
    var quickError by mutableStateOf<String?>(null)
    var lastCreated by mutableStateOf<Link?>(null)
        private set

    init {
        load()
        viewModelScope.launch { c.linkChanges.collect { load(quiet = true) } }
    }

    fun selectRange(r: StatsRange) {
        if (r == range) return
        range = r
        load(quiet = true)
    }

    fun refresh() {
        refreshing = true
        load(quiet = true)
    }

    fun load(quiet: Boolean = false) {
        val api = c.api() ?: return
        if (!quiet) loading = true
        viewModelScope.launch {
            try {
                val (from, to) = range.window()
                val o = async { api.overview(from, to) }
                val r = async { runCatching { api.recent() }.getOrDefault(emptyList()) }
                val m = async { runCatching { api.me() }.getOrNull() }
                overview = o.await()
                recent = r.await()
                m.await()?.let { me = it }
                error = null
            } catch (e: ApiException) {
                if (e.isAuth) c.signOut() else error = e.message
            } finally {
                loading = false
                refreshing = false
            }
        }
    }

    fun shorten() {
        val api = c.api() ?: return
        val url = quickUrl.trim()
        if (url.isEmpty()) return
        quickBusy = true
        quickError = null
        viewModelScope.launch {
            try {
                val target = if (url.contains("://")) url else "https://$url"
                lastCreated = api.createLink(LinkDraft(targetUrl = target))
                quickUrl = ""
                c.notifyLinksChanged()
            } catch (e: ApiException) {
                quickError = e.fields["target_url"] ?: e.fields["targetUrl"] ?: e.message
            } finally {
                quickBusy = false
            }
        }
    }

    fun dismissCreated() { lastCreated = null }
}
