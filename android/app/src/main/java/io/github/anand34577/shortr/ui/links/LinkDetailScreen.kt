package io.github.anand34577.shortr.ui.links

import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.automirrored.rounded.OpenInNew
import androidx.compose.material.icons.rounded.Block
import androidx.compose.material.icons.rounded.CheckCircle
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material.icons.rounded.Delete
import androidx.compose.material.icons.rounded.Edit
import androidx.compose.material.icons.rounded.MoreVert
import androidx.compose.material.icons.rounded.People
import androidx.compose.material.icons.rounded.QrCode2
import androidx.compose.material.icons.rounded.Share
import androidx.compose.material.icons.rounded.SmartToy
import androidx.compose.material.icons.rounded.TouchApp
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.PrimaryScrollableTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarDuration
import androidx.compose.material3.SnackbarResult
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.github.anand34577.shortr.container
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.ui.LocalEditor
import io.github.anand34577.shortr.ui.LocalSnackbar
import io.github.anand34577.shortr.ui.common.BreakdownList
import io.github.anand34577.shortr.ui.common.ClicksChart
import io.github.anand34577.shortr.ui.common.ErrorView
import io.github.anand34577.shortr.ui.common.LinkStatusPill
import io.github.anand34577.shortr.ui.common.LoadingBox
import io.github.anand34577.shortr.ui.common.SectionHeader
import io.github.anand34577.shortr.ui.common.StatTile
import io.github.anand34577.shortr.ui.common.StatsRange
import io.github.anand34577.shortr.ui.common.appViewModel
import io.github.anand34577.shortr.ui.common.compact
import io.github.anand34577.shortr.ui.common.copyText
import io.github.anand34577.shortr.ui.common.countryFlag
import io.github.anand34577.shortr.ui.common.openUrl
import io.github.anand34577.shortr.ui.common.relativeTime
import io.github.anand34577.shortr.ui.common.shareImage
import io.github.anand34577.shortr.ui.common.shareText
import io.github.anand34577.shortr.ui.common.shortDateTime
import io.github.anand34577.shortr.ui.theme.MonoStyle
import kotlinx.coroutines.launch

private enum class Breakdown(val label: String) { Countries("Countries"), Referrers("Referrers"), Devices("Devices"), Browsers("Browsers"), Systems("OS") }

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LinkDetailScreen(id: String, onBack: () -> Unit) {
    val ctx = LocalContext.current
    val c = ctx.container
    val vm = appViewModel(key = "link-$id") { LinkDetailViewModel(c, id) }
    val editor = LocalEditor.current
    val snackbar = LocalSnackbar.current
    val scope = rememberCoroutineScope()
    var menu by remember { mutableStateOf(false) }
    var confirmDelete by remember { mutableStateOf(false) }
    var showQr by remember { mutableStateOf(false) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(vm.link?.let { "/" + it.code } ?: "", style = MonoStyle.merge(MaterialTheme.typography.titleLarge)) },
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Rounded.ArrowBack, "Back") } },
                actions = {
                    vm.link?.let { l ->
                        IconButton(onClick = { editor.edit(l) }) { Icon(Icons.Rounded.Edit, "Edit") }
                        Box {
                            IconButton(onClick = { menu = true }) { Icon(Icons.Rounded.MoreVert, "More actions") }
                            DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                                DropdownMenuItem(
                                    text = { Text(if (l.isActive) "Disable link" else "Enable link") },
                                    leadingIcon = { Icon(if (l.isActive) Icons.Rounded.Block else Icons.Rounded.CheckCircle, null) },
                                    onClick = { menu = false; scope.launch { vm.toggle()?.let { snackbar.showSnackbar(it) } } },
                                )
                                DropdownMenuItem(
                                    text = { Text("Delete", color = MaterialTheme.colorScheme.error) },
                                    leadingIcon = { Icon(Icons.Rounded.Delete, null, tint = MaterialTheme.colorScheme.error) },
                                    onClick = { menu = false; confirmDelete = true },
                                )
                            }
                        }
                    }
                },
            )
        },
    ) { padding ->
        val link = vm.link
        when {
            vm.loading && link == null -> LoadingBox(Modifier.padding(padding))
            link == null -> ErrorView(vm.error ?: "This link couldn't be found.", { vm.load() }, Modifier.padding(padding))
            else -> LazyColumn(
                contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = padding.calculateTopPadding() + 4.dp, bottom = padding.calculateBottomPadding() + 32.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                item { Hero(link, onQr = { vm.loadQr(); showQr = true }) }
                item {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        StatsRange.entries.forEach { r -> FilterChip(selected = vm.range == r, onClick = { vm.selectRange(r) }, label = { Text(r.label) }) }
                    }
                }
                vm.stats?.let { s ->
                    item {
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            StatTile("Clicks", compact(s.totals.clicks), Modifier.weight(1f), Icons.Rounded.TouchApp)
                            StatTile("Visitors", compact(s.totals.uniques), Modifier.weight(1f), Icons.Rounded.People)
                            StatTile("Bots", compact(s.totals.bots), Modifier.weight(1f), Icons.Rounded.SmartToy)
                        }
                    }
                    item {
                        Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) {
                            ClicksChart(s.series, Modifier.padding(16.dp))
                        }
                    }
                    item {
                        var tab by remember { mutableIntStateOf(0) }
                        Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) {
                            PrimaryScrollableTabRow(selectedTabIndex = tab, edgePadding = 8.dp, containerColor = Color.Transparent, divider = {}) {
                                Breakdown.entries.forEachIndexed { i, b -> Tab(selected = tab == i, onClick = { tab = i }, text = { Text(b.label) }) }
                            }
                            Box(Modifier.padding(16.dp)) {
                                when (Breakdown.entries[tab]) {
                                    Breakdown.Countries -> BreakdownList(s.byCountry, "No visits in this range yet.") { countryFlag(it.key) }
                                    Breakdown.Referrers -> BreakdownList(s.byReferrer, "No referrers yet.")
                                    Breakdown.Devices -> BreakdownList(s.byDevice, "No devices yet.")
                                    Breakdown.Browsers -> BreakdownList(s.byBrowser, "No browsers yet.")
                                    Breakdown.Systems -> BreakdownList(s.byOS, "No systems yet.")
                                }
                            }
                        }
                    }
                }
                item { Details(link) }
            }
        }
    }

    if (confirmDelete) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text("Delete this link?") },
            text = { Text("/${vm.link?.code} stops redirecting straight away. You can restore it from Trash.") },
            confirmButton = {
                TextButton(onClick = {
                    confirmDelete = false
                    scope.launch {
                        val err = vm.delete()
                        if (err != null) { snackbar.showSnackbar(err); return@launch }
                        onBack()
                        // this screen is gone now, so the undo runs in the app scope
                        c.appScope.launch {
                            val r = snackbar.showSnackbar("Link moved to trash", actionLabel = "Undo", duration = SnackbarDuration.Long)
                            if (r == SnackbarResult.ActionPerformed) {
                                runCatching { c.api()?.restoreLink(id) }
                                c.notifyLinksChanged()
                            }
                        }
                    }
                }) { Text("Delete", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text("Cancel") } },
        )
    }

    if (showQr) {
        val code = vm.link?.code ?: ""
        AlertDialog(
            onDismissRequest = { showQr = false },
            title = { Text("QR code") },
            text = {
                Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                    val q = vm.qr
                    if (q == null) CircularProgressIndicator()
                    else Image(q.first.asImageBitmap(), "QR code for /$code", Modifier.size(240.dp).background(Color.White, RoundedCornerShape(12.dp)).padding(8.dp))
                }
            },
            confirmButton = {
                TextButton(enabled = vm.qr != null, onClick = { vm.qr?.let { ctx.shareImage(it.second, "shortr-$code") } }) { Text("Share image") }
            },
            dismissButton = { TextButton(onClick = { showQr = false }) { Text("Close") } },
        )
    }
}

@Composable
private fun Hero(link: Link, onQr: () -> Unit) {
    val ctx = LocalContext.current
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                LinkStatusPill(link)
                if (link.title.isNotBlank()) Text(link.title, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onPrimaryContainer, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            Text(link.shortUrl.removePrefix("https://").removePrefix("http://"), style = MonoStyle.merge(MaterialTheme.typography.titleLarge), color = MaterialTheme.colorScheme.onPrimaryContainer)
            Text("→ " + link.targetUrl, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.8f), maxLines = 2, overflow = TextOverflow.Ellipsis)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                FilledTonalButton(onClick = { ctx.copyText("Short link", link.shortUrl) }, modifier = Modifier.weight(1f)) {
                    Icon(Icons.Rounded.ContentCopy, null, Modifier.size(18.dp)); Spacer(Modifier.width(8.dp)); Text("Copy", maxLines = 1)
                }
                FilledTonalButton(onClick = { ctx.shareText(link.shortUrl, link.title.ifBlank { null }) }, modifier = Modifier.weight(1f)) {
                    Icon(Icons.Rounded.Share, null, Modifier.size(18.dp)); Spacer(Modifier.width(8.dp)); Text("Share", maxLines = 1)
                }
                FilledTonalButton(onClick = onQr, contentPadding = PaddingValues(horizontal = 14.dp)) { Icon(Icons.Rounded.QrCode2, "Show QR code") }
                FilledTonalButton(onClick = { ctx.openUrl(link.targetUrl) }, contentPadding = PaddingValues(horizontal = 14.dp)) {
                    Icon(Icons.AutoMirrored.Rounded.OpenInNew, "Open destination")
                }
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Details(link: Link) {
    SectionHeader("Details")
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) {
        Column(Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
            DetailRow("Created", shortDateTime(link.createdAt))
            DetailRow("Last click", if (link.lastClickAt != null) relativeTime(link.lastClickAt) else "No clicks yet")
            DetailRow("Total clicks", compact(link.clickCount))
            DetailRow("Expires", link.expiresAt?.let { shortDateTime(it) } ?: "Never")
            DetailRow("Click limit", link.maxClicks?.let { "${link.clickCount} of $it used" } ?: "None")
            DetailRow("Password", if (link.hasPassword) "Required" else "None")
            DetailRow("Redirect", "${link.redirectStatus} " + if (link.redirectStatus == 301 || link.redirectStatus == 308) "(permanent)" else "(temporary)")
            DetailRow("Pass query string", if (link.passQuery) "Yes" else "No", last = link.tags.isEmpty())
            if (link.tags.isNotEmpty()) {
                FlowRow(Modifier.padding(vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    link.tags.forEach { AssistChip(onClick = {}, label = { Text(it) }) }
                }
            }
        }
    }
}

@Composable
private fun DetailRow(label: String, value: String, last: Boolean = false) {
    Row(Modifier.fillMaxWidth().padding(vertical = 10.dp)) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.weight(1f))
        Text(value, style = MaterialTheme.typography.bodyMedium)
    }
    if (!last) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
}
