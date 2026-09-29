package io.github.anand34577.shortr.ui.home

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowForward
import androidx.compose.material.icons.rounded.AdsClick
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material.icons.rounded.Language
import androidx.compose.material.icons.rounded.Link
import androidx.compose.material.icons.rounded.People
import androidx.compose.material.icons.rounded.Public
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material.icons.rounded.Share
import androidx.compose.material.icons.rounded.TouchApp
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.github.anand34577.shortr.container
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.data.RecentClick
import io.github.anand34577.shortr.data.SessionState
import io.github.anand34577.shortr.ui.common.ClicksChart
import io.github.anand34577.shortr.ui.common.ErrorView
import io.github.anand34577.shortr.ui.common.LoadingBox
import io.github.anand34577.shortr.ui.common.SectionHeader
import io.github.anand34577.shortr.ui.common.StatTile
import io.github.anand34577.shortr.ui.common.StatsRange
import io.github.anand34577.shortr.ui.common.appViewModel
import io.github.anand34577.shortr.ui.common.compact
import io.github.anand34577.shortr.ui.common.copyText
import io.github.anand34577.shortr.ui.common.hostOf
import io.github.anand34577.shortr.ui.common.relativeTime
import io.github.anand34577.shortr.ui.common.shareText
import io.github.anand34577.shortr.ui.theme.MonoStyle
import java.time.LocalTime

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(outer: PaddingValues, onOpenLink: (String) -> Unit, onSeeAll: () -> Unit) {
    val ctx = LocalContext.current
    val c = ctx.container
    val vm = appViewModel { HomeViewModel(c) }
    val siteName = (c.state.value as? SessionState.SignedIn)?.session?.siteName ?: "Shortr"

    PullToRefreshBox(isRefreshing = vm.refreshing, onRefresh = vm::refresh, modifier = Modifier.fillMaxSize().padding(bottom = outer.calculateBottomPadding())) {
        when {
            vm.loading && vm.overview == null -> LoadingBox()
            vm.overview == null && vm.error != null -> ErrorView(vm.error!!, vm::load, Modifier.statusBarsPadding())
            else -> LazyColumn(
                contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 96.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
                modifier = Modifier.fillMaxSize(),
            ) {
                item {
                    Column(Modifier.statusBarsPadding().padding(top = 16.dp, bottom = 4.dp)) {
                        Text(greeting() + (vm.me?.let { ", ${it.displayName}" } ?: ""), style = MaterialTheme.typography.headlineMedium)
                        Text(siteName, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
                item { QuickShorten(vm) }
                if (vm.updating && !vm.refreshing) item { LinearProgressIndicator(Modifier.fillMaxWidth()) }
                item {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        StatsRange.entries.forEach { r ->
                            FilterChip(selected = vm.range == r, onClick = { vm.selectRange(r) }, label = { Text(r.label) })
                        }
                    }
                }
                val o = vm.overview
                if (o != null) {
                    item {
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            StatTile("Clicks", compact(o.totals.clicks), Modifier.weight(1f), Icons.Rounded.TouchApp, o.deltaPct)
                            StatTile("Visitors", compact(o.totals.uniques), Modifier.weight(1f), Icons.Rounded.People)
                        }
                    }
                    item {
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            StatTile("Active links", compact(o.totals.activeLinks), Modifier.weight(1f), Icons.Rounded.Link)
                            StatTile("Top referrer", o.topReferrer ?: "Direct", Modifier.weight(1f), Icons.Rounded.Language)
                        }
                    }
                    item {
                        Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) {
                            ClicksChart(o.series, Modifier.padding(16.dp))
                        }
                    }
                    if (o.topLinks.isNotEmpty()) {
                        item {
                            SectionHeader("Top links") { TextButton(onClick = onSeeAll) { Text("All links") } }
                        }
                        items(o.topLinks, key = { "top-" + it.id }) { l -> TopLinkRow(l, onClick = { onOpenLink(l.id) }) }
                    }
                }
                if (vm.recent.isNotEmpty()) {
                    item { SectionHeader("Recent clicks") }
                    items(groupRecent(vm.recent), key = { "r-" + it.first.id }) { (click, count) ->
                        RecentRow(click, count, onClick = { onOpenLink(click.linkId) })
                    }
                }
            }
        }
    }
}

private fun greeting(): String = when (LocalTime.now().hour) {
    in 5..11 -> "Good morning"
    in 12..16 -> "Good afternoon"
    in 17..21 -> "Good evening"
    else -> "Hello"
}

@Composable
private fun QuickShorten(vm: HomeViewModel) {
    val ctx = LocalContext.current
    val focus = LocalFocusManager.current
    val submit = { focus.clearFocus(); vm.shorten() }
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Text("Shorten a link", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.onPrimaryContainer)
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = vm.quickUrl,
                    onValueChange = { vm.quickUrl = it; vm.quickError = null },
                    placeholder = { Text("Paste a long URL") },
                    singleLine = true,
                    isError = vm.quickError != null,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go, autoCorrectEnabled = false),
                    keyboardActions = KeyboardActions(onGo = { submit() }),
                    modifier = Modifier.weight(1f),
                )
                FilledIconButton(onClick = submit, enabled = vm.quickUrl.isNotBlank() && !vm.quickBusy, modifier = Modifier.size(52.dp)) {
                    if (vm.quickBusy) CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                    else Icon(Icons.AutoMirrored.Rounded.ArrowForward, "Shorten")
                }
            }
            vm.quickError?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
            AnimatedVisibility(vm.lastCreated != null) {
                val l = vm.lastCreated ?: return@AnimatedVisibility
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(l.shortUrl.removePrefix("https://").removePrefix("http://"), style = MonoStyle.merge(MaterialTheme.typography.bodyLarge),
                        color = MaterialTheme.colorScheme.onPrimaryContainer, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    IconButton(onClick = { ctx.copyText("Short link", l.shortUrl) }) { Icon(Icons.Rounded.ContentCopy, "Copy") }
                    IconButton(onClick = { ctx.shareText(l.shortUrl) }) { Icon(Icons.Rounded.Share, "Share") }
                }
            }
        }
    }
}

@Composable
private fun TopLinkRow(l: Link, onClick: () -> Unit) {
    Card(onClick = onClick, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) {
        Row(Modifier.padding(horizontal = 16.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text("/" + l.code, style = MonoStyle.merge(MaterialTheme.typography.titleSmall), color = MaterialTheme.colorScheme.primary)
                Text(hostOf(l.targetUrl), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            Icon(Icons.Rounded.AdsClick, null, Modifier.size(16.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.width(4.dp))
            Text(compact(l.clickCount), style = MaterialTheme.typography.titleSmall)
        }
    }
}

/** Back-to-back clicks on the same link from the same place collapse into one row. */
private fun groupRecent(items: List<RecentClick>): List<Pair<RecentClick, Int>> {
    val out = mutableListOf<Pair<RecentClick, Int>>()
    for (it in items) {
        val last = out.lastOrNull()
        if (last != null && last.first.linkId == it.linkId && last.first.referrerHost == it.referrerHost && last.first.country == it.country) {
            out[out.lastIndex] = last.first to last.second + 1
        } else out += it to 1
    }
    return out
}

@Composable
private fun RecentRow(click: RecentClick, count: Int, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Icon(Icons.Rounded.Public, null, Modifier.size(22.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("/" + click.code, style = MonoStyle.merge(MaterialTheme.typography.bodyMedium), color = MaterialTheme.colorScheme.primary)
                if (count > 1) Text("×$count", modifier = Modifier.padding(start = 6.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Text("from " + click.referrerHost.ifBlank { "direct" }, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Text(relativeTime(click.ts), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
