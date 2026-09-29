package io.github.anand34577.shortr.ui.common

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.PersistableBundle
import android.widget.Toast
import androidx.browser.customtabs.CustomTabsIntent
import androidx.core.content.FileProvider
import java.io.File

fun Context.copyText(label: String, text: String, sensitive: Boolean = false) {
    val cm = getSystemService(ClipboardManager::class.java)
    val clip = ClipData.newPlainText(label, text)
    if (sensitive && Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
        clip.description.extras = PersistableBundle().apply { putBoolean("android.content.extra.IS_SENSITIVE", true) }
    }
    cm.setPrimaryClip(clip)
    // Android 13+ shows its own "Copied" confirmation
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) Toast.makeText(this, "Copied", Toast.LENGTH_SHORT).show()
}

fun Context.shareText(text: String, title: String? = null) {
    val send = Intent(Intent.ACTION_SEND).apply {
        type = "text/plain"
        putExtra(Intent.EXTRA_TEXT, text)
        title?.let { putExtra(Intent.EXTRA_TITLE, it) }
    }
    startActivity(Intent.createChooser(send, null))
}

fun Context.shareImage(png: ByteArray, name: String) {
    val dir = File(cacheDir, "shared").apply { mkdirs() }
    val f = File(dir, "$name.png").apply { writeBytes(png) }
    val uri = FileProvider.getUriForFile(this, "$packageName.files", f)
    val send = Intent(Intent.ACTION_SEND).apply {
        type = "image/png"
        putExtra(Intent.EXTRA_STREAM, uri)
        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
    startActivity(Intent.createChooser(send, null))
}

fun Context.openUrl(url: String) {
    runCatching { CustomTabsIntent.Builder().setShowTitle(true).build().launchUrl(this, Uri.parse(url)) }
        .onFailure { runCatching { startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url))) } }
}

/** First http(s) URL inside shared text ("Look at this https://…"). */
fun extractUrl(text: String?): String? =
    text?.let { Regex("""https?://[^\s<>"']+""", RegexOption.IGNORE_CASE).find(it)?.value?.trimEnd('.', ',', ')', ']') }

/** Reads the clipboard only when it holds a URL (used to prefill "new link"). */
fun Context.clipboardUrl(): String? {
    val cm = getSystemService(ClipboardManager::class.java)
    val text = cm.primaryClip?.takeIf { it.itemCount > 0 }?.getItemAt(0)?.coerceToText(this)?.toString()
    return extractUrl(text)
}
