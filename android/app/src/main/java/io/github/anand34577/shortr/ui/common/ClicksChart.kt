package io.github.anand34577.shortr.ui.common

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import io.github.anand34577.shortr.data.SeriesPoint

/**
 * Clicks over time as a smooth area chart. Touch and drag to read a value.
 * The server zero-fills gaps, so every bucket in the range is present.
 */
@Composable
fun ClicksChart(series: List<SeriesPoint>, modifier: Modifier = Modifier, height: Dp = 180.dp) {
    val primary = MaterialTheme.colorScheme.primary
    val grid = MaterialTheme.colorScheme.outlineVariant
    val progress = remember(series) { Animatable(0f) }
    LaunchedEffect(series) { progress.animateTo(1f, tween(700)) }
    var selected by remember(series) { mutableStateOf<Int?>(null) }
    val total = series.sumOf { it.clicks }

    Column(modifier) {
        val sel = selected?.let { series.getOrNull(it) }
        Row {
            Text(
                if (sel != null) "${compact(sel.clicks)} clicks · ${bucketLabel(sel.bucket)}" else "${compact(total)} clicks in this range",
                style = MaterialTheme.typography.labelLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Canvas(
            Modifier
                .fillMaxWidth()
                .height(height)
                .semantics { contentDescription = "Clicks chart, $total clicks in this range" }
                .pointerInput(series) {
                    fun pick(x: Float) {
                        if (series.size > 1) selected = ((x / size.width) * (series.size - 1)).toInt().coerceIn(0, series.size - 1)
                    }
                    detectTapGestures(onPress = { pick(it.x); tryAwaitRelease(); selected = null })
                }
                .pointerInput(series) {
                    detectDragGestures(
                        onDragStart = { if (series.size > 1) selected = ((it.x / size.width) * (series.size - 1)).toInt().coerceIn(0, series.size - 1) },
                        onDragEnd = { selected = null },
                        onDragCancel = { selected = null },
                    ) { change, _ ->
                        if (series.size > 1) selected = ((change.position.x / size.width) * (series.size - 1)).toInt().coerceIn(0, series.size - 1)
                    }
                },
        ) {
            val top = 12.dp.toPx()
            val h = size.height - top
            // three faint guide lines
            for (i in 0..2) {
                val y = top + h * i / 2f
                drawLine(grid, Offset(0f, y), Offset(size.width, y), 1.dp.toPx(), pathEffect = PathEffect.dashPathEffect(floatArrayOf(8f, 8f)))
            }
            if (series.isEmpty()) return@Canvas
            val max = series.maxOf { it.clicks }.coerceAtLeast(1).toFloat()
            val step = if (series.size > 1) size.width / (series.size - 1) else 0f
            val pts = series.mapIndexed { i, p -> Offset(i * step, top + h - h * (p.clicks / max) * progress.value) }

            val line = Path().apply {
                moveTo(pts.first().x, pts.first().y)
                for (i in 1 until pts.size) {
                    val a = pts[i - 1]; val b = pts[i]
                    val mid = (a.x + b.x) / 2
                    cubicTo(mid, a.y, mid, b.y, b.x, b.y)
                }
            }
            val area = Path().apply {
                addPath(line)
                lineTo(pts.last().x, size.height)
                lineTo(pts.first().x, size.height)
                close()
            }
            drawPath(area, Brush.verticalGradient(listOf(primary.copy(alpha = 0.28f), primary.copy(alpha = 0f)), startY = top, endY = size.height))
            drawPath(line, primary, style = Stroke(2.5.dp.toPx(), cap = StrokeCap.Round, join = StrokeJoin.Round))

            selected?.let { i ->
                val p = pts[i]
                drawLine(primary.copy(alpha = 0.4f), Offset(p.x, top), Offset(p.x, size.height), 1.dp.toPx())
                drawCircle(primary, 5.dp.toPx(), p)
                drawCircle(androidx.compose.ui.graphics.Color.White, 2.dp.toPx(), p)
            }
        }
        if (series.size > 1) {
            Row(Modifier.fillMaxWidth()) {
                Text(bucketLabel(series.first().bucket), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.weight(1f))
                Text(bucketLabel(series.last().bucket), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}
