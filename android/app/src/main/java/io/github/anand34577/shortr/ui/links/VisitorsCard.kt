package io.github.anand34577.shortr.ui.links

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material.icons.rounded.Public
import androidx.compose.material.icons.rounded.SmartToy
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.github.anand34577.shortr.data.VisitorClick
import io.github.anand34577.shortr.ui.common.SectionHeader
import io.github.anand34577.shortr.ui.common.copyText
import io.github.anand34577.shortr.ui.common.relativeTime
import io.github.anand34577.shortr.ui.theme.MonoStyle

/** Recent visits to a link: visitor IP, place, device and time. Tap a row to copy the IP. */
@Composable
fun VisitorsCard(vm: LinkDetailViewModel) {
    val ctx = LocalContext.current
    Column {
        SectionHeader("Recent visitors")
        Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) {
            when {
                vm.visitorsLoading && vm.visitors.isEmpty() ->
                    Box(Modifier.fillMaxWidth().padding(24.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator(Modifier.size(24.dp)) }
                vm.visitorsError != null && vm.visitors.isEmpty() ->
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(vm.visitorsError!!, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error)
                        OutlinedButton(onClick = { vm.loadVisitors() }) { Text("Try again") }
                    }
                vm.visitors.isEmpty() ->
                    Text("No visits yet.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(16.dp))
                else -> Column {
                    vm.visitors.forEachIndexed { i, v ->
                        if (i > 0) HorizontalDivider(Modifier.padding(horizontal = 16.dp), color = MaterialTheme.colorScheme.outlineVariant)
                        VisitorRow(v, onCopy = { if (v.ip.isNotBlank()) ctx.copyText("Visitor IP", v.ip) })
                    }
                    if (vm.canLoadMoreVisitors || vm.visitorsLoading) {
                        Box(Modifier.fillMaxWidth().padding(8.dp), contentAlignment = Alignment.Center) {
                            if (vm.visitorsLoading) CircularProgressIndicator(Modifier.size(24.dp))
                            else TextButton(onClick = { vm.loadVisitors(reset = false) }) { Text("Load more") }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun VisitorRow(v: VisitorClick, onCopy: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onCopy).padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Icon(if (v.isBot) Icons.Rounded.SmartToy else Icons.Rounded.Public, null, Modifier.size(20.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(
                v.ip.ifBlank { "IP not stored" },
                style = MonoStyle.merge(MaterialTheme.typography.bodyMedium),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            val detail = listOf(v.place, listOf(v.browser, v.os).filter { it.isNotBlank() }.joinToString(" on ")).filter { it.isNotBlank() }.joinToString(" · ")
            if (detail.isNotBlank()) {
                Text(detail, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        Text(relativeTime(v.ts), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (v.ip.isNotBlank()) Icon(Icons.Rounded.ContentCopy, "Copy IP", Modifier.size(16.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
