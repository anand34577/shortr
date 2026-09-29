package io.github.anand34577.shortr.ui.links

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AdsClick
import androidx.compose.material.icons.rounded.Block
import androidx.compose.material.icons.rounded.CheckCircle
import androidx.compose.material.icons.rounded.Clear
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material.icons.rounded.Delete
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material.icons.rounded.MoreVert
import androidx.compose.material.icons.rounded.RestoreFromTrash
import androidx.compose.material.icons.rounded.Search
import androidx.compose.material.icons.rounded.SearchOff
import androidx.compose.material.icons.rounded.Share
import androidx.compose.material.icons.automirrored.rounded.Sort
import androidx.compose.material.icons.rounded.AddLink
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SnackbarDuration
import androidx.compose.material3.SnackbarResult
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.github.anand34577.shortr.container
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.ui.LocalEditor
import io.github.anand34577.shortr.ui.LocalSnackbar
import io.github.anand34577.shortr.ui.common.ErrorView
import io.github.anand34577.shortr.ui.common.LinkStatusPill
import io.github.anand34577.shortr.ui.common.LoadingBox
import io.github.anand34577.shortr.ui.common.MessageView
import io.github.anand34577.shortr.ui.common.appViewModel
import io.github.anand34577.shortr.ui.common.compact
import io.github.anand34577.shortr.ui.common.copyText
import io.github.anand34577.shortr.ui.common.hostOf
import io.github.anand34577.shortr.ui.common.relativeTime
import io.github.anand34577.shortr.ui.common.shareText
import io.github.anand34577.shortr.ui.theme.MonoStyle
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LinksScreen(outer: PaddingValues, onOpenLink: (String) -> Unit) {
    val ctx = LocalContext.current
    val c = ctx.container
    val vm = appViewModel { LinksViewModel(c) }
    val editor = LocalEditor.current
    val snackbar = LocalSnackbar.current
    val scope = rememberCoroutineScope()
    val listState = rememberLazyListState()

    // infinite scroll: fetch the next page when the last few rows come into view
    val nearEnd by remember { derivedStateOf { listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index?.let { it >= listState.layoutInfo.totalItemsCount - 5 } == true } }
    LaunchedEffect(nearEnd, vm.canLoadMore) { if (nearEnd && vm.canLoadMore) vm.loadMore() }

    fun act(block: suspend () -> String?) = scope.launch { block()?.let { snackbar.showSnackbar(it) } }

    fun delete(link: Link) = scope.launch {
        val err = vm.delete(link)
        if (err != null) { snackbar.showSnackbar(err); return@launch }
        val res = snackbar.showSnackbar("Moved /${link.code} to trash", actionLabel = "Undo", duration = SnackbarDuration.Long)
        if (res == SnackbarResult.ActionPerformed) vm.restore(link)?.let { snackbar.showSnackbar(it) }
    }

    Column(Modifier.fillMaxSize().padding(bottom = outer.calculateBottomPadding())) {
        Column(Modifier.statusBarsPadding().padding(horizontal = 16.dp).padding(top = 16.dp)) {
            Text("Links", style = MaterialTheme.typography.headlineMedium)
            OutlinedTextField(
                value = vm.query,
                onValueChange = vm::onQuery,
                placeholder = { Text("Search code, title or URL") },
                leadingIcon = { Icon(Icons.Rounded.Search, null) },
                trailingIcon = { if (vm.query.isNotEmpty()) IconButton(onClick = { vm.onQuery("") }) { Icon(Icons.Rounded.Clear, "Clear search") } },
                singleLine = true,
                shape = MaterialTheme.shapes.extraLarge,
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
            )
        }
        Row(Modifier.padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            LazyRow(Modifier.weight(1f), contentPadding = PaddingValues(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(LinkFilter.entries) { f -> FilterChip(selected = vm.filter == f, onClick = { vm.selectFilter(f) }, label = { Text(f.label) }) }
            }
            var sortOpen by remember { mutableStateOf(false) }
            Box {
                IconButton(onClick = { sortOpen = true }) { Icon(Icons.AutoMirrored.Rounded.Sort, "Sort") }
                DropdownMenu(expanded = sortOpen, onDismissRequest = { sortOpen = false }) {
                    LinkSort.entries.forEach { s ->
                        DropdownMenuItem(
                            text = { Text(s.label) },
                            trailingIcon = { if (vm.sort == s) Icon(Icons.Rounded.CheckCircle, null) },
                            onClick = { sortOpen = false; vm.selectSort(s) },
                        )
                    }
                }
            }
        }

        if (vm.working || (vm.loading && vm.links.isNotEmpty())) LinearProgressIndicator(Modifier.fillMaxWidth())
        PullToRefreshBox(isRefreshing = vm.refreshing, onRefresh = vm::refresh, modifier = Modifier.fillMaxSize()) {
            when {
                vm.loading && vm.links.isEmpty() -> LoadingBox()
                vm.error != null && vm.links.isEmpty() -> ErrorView(vm.error!!, { vm.reload() })
                vm.links.isEmpty() -> LazyColumn(Modifier.fillMaxSize()) {
                    item {
                        when {
                            vm.query.isNotBlank() -> MessageView(Icons.Rounded.SearchOff, "Nothing matches “${vm.query}”", "Try a different word, or clear the search.")
                            vm.filter == LinkFilter.Trash -> MessageView(Icons.Rounded.Delete, "Trash is empty", "Deleted links wait here so you can restore them.")
                            vm.filter != LinkFilter.All -> MessageView(Icons.Rounded.SearchOff, "No ${vm.filter.label.lowercase()} links")
                            else -> MessageView(Icons.Rounded.AddLink, "No links yet", "Shorten your first link, or share one into Shortr from any app.") {
                                Button(onClick = { editor.create() }) { Text("New link") }
                            }
                        }
                    }
                }
                else -> LazyColumn(
                    state = listState,
                    contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 96.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier.fillMaxSize(),
                ) {
                    items(vm.links, key = { it.id }) { link ->
                        LinkRow(
                            link,
                            onClick = { if (link.deletedAt == null) onOpenLink(link.id) },
                            onCopy = { ctx.copyText("Short link", link.shortUrl) },
                            onShare = { ctx.shareText(link.shortUrl, link.title.ifBlank { null }) },
                            onToggle = { act { vm.toggle(link) } },
                            onDelete = { delete(link) },
                            onRestore = { act { vm.restore(link) } },
                            modifier = Modifier.animateItem(),
                        )
                    }
                    if (vm.loadingMore) item { Box(Modifier.fillMaxWidth().padding(16.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator(Modifier.size(24.dp)) } }
                }
            }
        }
    }
}

@Composable
private fun LinkRow(
    link: Link,
    onClick: () -> Unit,
    onCopy: () -> Unit,
    onShare: () -> Unit,
    onToggle: () -> Unit,
    onDelete: () -> Unit,
    onRestore: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var menu by remember { mutableStateOf(false) }
    Card(onClick = onClick, modifier = modifier, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) {
        Row(Modifier.padding(start = 16.dp, top = 12.dp, bottom = 12.dp, end = 4.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("/" + link.code, style = MonoStyle.merge(MaterialTheme.typography.titleSmall), color = MaterialTheme.colorScheme.primary, maxLines = 1)
                    if (link.hasPassword) Icon(Icons.Rounded.Lock, "Password protected", Modifier.size(14.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                    LinkStatusPill(link)
                }
                Text(link.displayTitle, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(hostOf(link.targetUrl), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                    Icon(Icons.Rounded.AdsClick, null, Modifier.size(13.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text(compact(link.clickCount), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("· " + relativeTime(link.createdAt), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1)
                }
            }
            if (link.deletedAt != null) {
                IconButton(onClick = onRestore) { Icon(Icons.Rounded.RestoreFromTrash, "Restore") }
            } else {
                IconButton(onClick = onCopy) { Icon(Icons.Rounded.ContentCopy, "Copy short link") }
                Box {
                    IconButton(onClick = { menu = true }) { Icon(Icons.Rounded.MoreVert, "More actions") }
                    DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                        DropdownMenuItem(text = { Text("Share") }, leadingIcon = { Icon(Icons.Rounded.Share, null) }, onClick = { menu = false; onShare() })
                        DropdownMenuItem(
                            text = { Text(if (link.isActive) "Disable" else "Enable") },
                            leadingIcon = { Icon(if (link.isActive) Icons.Rounded.Block else Icons.Rounded.CheckCircle, null) },
                            onClick = { menu = false; onToggle() },
                        )
                        DropdownMenuItem(
                            text = { Text("Delete", color = MaterialTheme.colorScheme.error) },
                            leadingIcon = { Icon(Icons.Rounded.Delete, null, tint = MaterialTheme.colorScheme.error) },
                            onClick = { menu = false; onDelete() },
                        )
                    }
                }
            }
        }
    }
}
