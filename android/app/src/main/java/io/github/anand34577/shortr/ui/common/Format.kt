package io.github.anand34577.shortr.ui.common

import android.text.format.DateUtils
import java.text.NumberFormat
import java.time.Instant
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.time.temporal.ChronoUnit

fun parseInstant(s: String?): Instant? = s?.let { runCatching { OffsetDateTime.parse(it).toInstant() }.getOrNull() }

fun relativeTime(s: String?): String {
    val t = parseInstant(s) ?: return "never"
    val diff = System.currentTimeMillis() - t.toEpochMilli()
    // phone and server clocks rarely agree exactly; a few minutes "in the future" is still now
    if (diff in -300_000..59_000) return "just now"
    return DateUtils.getRelativeTimeSpanString(t.toEpochMilli(), System.currentTimeMillis(), DateUtils.MINUTE_IN_MILLIS).toString()
}

fun shortDateTime(s: String?): String {
    val t = parseInstant(s) ?: return "—"
    return DateTimeFormatter.ofLocalizedDateTime(FormatStyle.MEDIUM, FormatStyle.SHORT).format(t.atZone(ZoneId.systemDefault()))
}

fun compact(n: Long): String = when {
    n >= 1_000_000 -> "%.1fM".format(n / 1_000_000.0).replace(".0M", "M")
    n >= 10_000 -> "%.1fk".format(n / 1_000.0).replace(".0k", "k")
    else -> NumberFormat.getIntegerInstance().format(n)
}

/** Server buckets are UTC "yyyy-MM-dd" (daily) or "yyyy-MM-ddTHH" (hourly). */
fun bucketLabel(b: String): String = if (b.length > 10) {
    val t = LocalDateTime.parse("$b:00").atOffset(ZoneOffset.UTC).atZoneSameInstant(ZoneId.systemDefault())
    DateTimeFormatter.ofPattern("ha").format(t).lowercase()
} else {
    DateTimeFormatter.ofPattern("MMM d").format(LocalDate.parse(b))
}

enum class StatsRange(val label: String, val hours: Long) {
    Day("24h", 24), Week("7d", 24 * 7), Month("30d", 24 * 30), Quarter("90d", 24 * 90);

    fun window(): Pair<String, String> {
        val to = Instant.now()
        return to.minus(hours, ChronoUnit.HOURS).toString() to to.toString()
    }
}

/** Human host of a URL, for list subtitles. */
fun hostOf(url: String): String = runCatching { java.net.URI(url).host?.removePrefix("www.") }.getOrNull() ?: url
