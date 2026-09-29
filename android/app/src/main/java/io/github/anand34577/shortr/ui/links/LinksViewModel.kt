package io.github.anand34577.shortr.ui.links

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import io.github.anand34577.shortr.data.ApiException
import io.github.anand34577.shortr.data.AppContainer
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.data.ShortrApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

enum class LinkFilter(val label: String, val status: String) { All("All", ""), Active("Active", "active"), Disabled("Disabled", "disabled"), Trash("Trash", "deleted") }
enum class LinkSort(val label: String, val param: String) { Newest("Newest", ""), Clicks("Most clicked", "clicks") }

class LinksViewModel(private val c: AppContainer) : ViewModel() {
    var query by mutableStateOf("")
        private set
    var filter by mutableStateOf(LinkFilter.All)
        private set
    var sort by mutableStateOf(LinkSort.Newest)
        private set
    var links by mutableStateOf<List<Link>>(emptyList())
        private set
    var loading by mutableStateOf(true)
        private set
    var refreshing by mutableStateOf(false)
        private set
    var loadingMore by mutableStateOf(false)
        private set
    var error by mutableStateOf<String?>(null)
        private set
    private var cursor: String? = null
    val canLoadMore: Boolean get() = cursor != null
    private var job: Job? = null

    init {
        reload()
        viewModelScope.launch { c.linkChanges.collect { reload(quiet = true) } }
    }

    fun onQuery(q: String) {
        query = q
        job?.cancel()
        job = viewModelScope.launch {
            delay(300) // debounce typing
            fetch(reset = true)
        }
    }

    fun selectFilter(f: LinkFilter) { filter = f; reload() }
    fun selectSort(s: LinkSort) { sort = s; reload() }

    fun refresh() {
        refreshing = true
        reload(quiet = true)
    }

    fun reload(quiet: Boolean = false) {
        if (!quiet) loading = true
        job?.cancel()
        job = viewModelScope.launch { fetch(reset = true) }
    }

    fun loadMore() {
        if (loadingMore || cursor == null) return
        loadingMore = true
        viewModelScope.launch { fetch(reset = false) }
    }

    private suspend fun fetch(reset: Boolean) {
        val api = c.api() ?: return
        try {
            val page = api.links(q = query, status = filter.status, sort = sort.param, cursor = if (reset) null else cursor)
            links = if (reset) page.items else (links + page.items).distinctBy { it.id }
            cursor = page.nextCursor?.takeIf { it.isNotBlank() }
            error = null
        } catch (e: ApiException) {
            if (e.isAuth) c.signOut() else error = e.message
        } finally {
            loading = false
            refreshing = false
            loadingMore = false
        }
    }

    /** Returns an error message, or null on success. */
    suspend fun toggle(link: Link): String? = mutate {
        val updated = it.setLinkStatus(link.id, !link.isActive)
        links = links.map { l -> if (l.id == updated.id) updated else l }
    }

    suspend fun delete(link: Link): String? = mutate {
        it.deleteLink(link.id)
        links = links.filterNot { l -> l.id == link.id }
    }

    suspend fun restore(link: Link): String? = mutate {
        it.restoreLink(link.id)
        if (filter == LinkFilter.Trash) links = links.filterNot { l -> l.id == link.id }
    }

    private suspend fun mutate(block: suspend (ShortrApi) -> Unit): String? {
        val api = c.api() ?: return "Not connected"
        return try {
            block(api)
            c.notifyLinksChanged()
            null
        } catch (e: ApiException) {
            e.message
        }
    }
}
