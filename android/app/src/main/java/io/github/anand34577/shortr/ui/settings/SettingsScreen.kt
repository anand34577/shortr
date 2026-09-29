package io.github.anand34577.shortr.ui.settings

import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.width
import android.os.Build
import androidx.biometric.BiometricManager
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.selection.toggleable
import androidx.compose.ui.semantics.Role
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.Logout
import androidx.compose.material.icons.automirrored.rounded.OpenInNew
import androidx.compose.material.icons.rounded.Dns
import androidx.compose.material.icons.rounded.Fingerprint
import androidx.compose.material.icons.rounded.Key
import androidx.compose.material.icons.rounded.Palette
import androidx.compose.material.icons.rounded.VerifiedUser
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.anand34577.shortr.BuildConfig
import io.github.anand34577.shortr.container
import io.github.anand34577.shortr.data.AppPrefs
import io.github.anand34577.shortr.data.AuthMethod
import io.github.anand34577.shortr.data.SessionState
import io.github.anand34577.shortr.data.ThemeMode
import io.github.anand34577.shortr.ui.LocalSnackbar
import io.github.anand34577.shortr.ui.common.openUrl
import kotlinx.coroutines.launch

private const val SOURCE_URL = "https://github.com/anand34577/shortr"

@Composable
fun SettingsScreen(outer: PaddingValues) {
    val ctx = LocalContext.current
    val c = ctx.container
    val state by c.state.collectAsStateWithLifecycle()
    val prefs by c.storage.prefs.collectAsStateWithLifecycle(AppPrefs())
    val session = (state as? SessionState.SignedIn)?.session
    val scope = rememberCoroutineScope()
    val snackbar = LocalSnackbar.current
    var confirmSignOut by remember { mutableStateOf(false) }

    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).statusBarsPadding()
            .padding(start = 16.dp, end = 16.dp, top = 16.dp, bottom = outer.calculateBottomPadding() + 24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Settings", style = MaterialTheme.typography.headlineMedium)

        Group("Server") {
            Item(Icons.Rounded.Dns, session?.siteName ?: "Shortr", session?.server ?: "")
            HorizontalDivider()
            Item(
                if (session?.method == AuthMethod.Sso) Icons.Rounded.VerifiedUser else Icons.Rounded.Key,
                session?.account ?: "Connected",
                if (session?.method == AuthMethod.Sso) "Signed in with single sign-on" else "Connected with an API key",
            )
        }

        Group("Appearance") {
            Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                    ThemeMode.entries.forEachIndexed { i, m ->
                        SegmentedButton(
                            selected = prefs.theme == m,
                            onClick = { scope.launch { c.storage.setTheme(m) } },
                            shape = SegmentedButtonDefaults.itemShape(i, ThemeMode.entries.size),
                        ) { Text(when (m) { ThemeMode.System -> "System"; ThemeMode.Light -> "Light"; ThemeMode.Dark -> "Dark" }) }
                    }
                }
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                HorizontalDivider()
                SwitchItem(Icons.Rounded.Palette, "Match wallpaper colours", "Use Material You colours instead of Shortr blue", prefs.dynamicColor) {
                    scope.launch { c.storage.setDynamicColor(it) }
                }
            }
        }

        Group("Security") {
            SwitchItem(Icons.Rounded.Fingerprint, "App lock", "Ask for your fingerprint, face or screen lock when opening Shortr", prefs.appLock) { on ->
                val bm = BiometricManager.from(ctx)
                val can = bm.canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_WEAK or BiometricManager.Authenticators.DEVICE_CREDENTIAL)
                if (on && can != BiometricManager.BIOMETRIC_SUCCESS) {
                    scope.launch { snackbar.showSnackbar("Set up a screen lock on this phone first.") }
                } else {
                    scope.launch { c.storage.setAppLock(on) }
                }
            }
        }

        Group("About") {
            Item(null, "Version", BuildConfig.VERSION_NAME)
            HorizontalDivider()
            ListItem(
                headlineContent = { Text("Source code and docs") },
                supportingContent = { Text("Open source, MIT licence") },
                trailingContent = { Icon(Icons.AutoMirrored.Rounded.OpenInNew, null) },
                colors = ListItemDefaults.colors(containerColor = Color.Transparent),
                modifier = Modifier.clickable { ctx.openUrl(SOURCE_URL) },
            )
        }

        OutlinedButton(onClick = { confirmSignOut = true }, modifier = Modifier.fillMaxWidth()) {
            Icon(Icons.AutoMirrored.Rounded.Logout, null)
            Spacer(Modifier.width(8.dp))
            Text("Disconnect from this server")
        }
    }

    if (confirmSignOut) {
        AlertDialog(
            onDismissRequest = { confirmSignOut = false },
            title = { Text("Disconnect?") },
            text = {
                Text(
                    if (session?.method == AuthMethod.ApiKey) "The API key is removed from this phone. It still works elsewhere; revoke it in the web console if you won't use it again."
                    else "You'll need to sign in again to use Shortr on this phone.",
                )
            },
            confirmButton = { TextButton(onClick = { confirmSignOut = false; c.signOut() }) { Text("Disconnect", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton(onClick = { confirmSignOut = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun Group(title: String, content: @Composable () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(title, style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary, modifier = Modifier.padding(start = 4.dp))
        Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) { content() }
    }
}

@Composable
private fun Item(icon: androidx.compose.ui.graphics.vector.ImageVector?, title: String, subtitle: String) {
    ListItem(
        headlineContent = { Text(title) },
        supportingContent = { Text(subtitle) },
        leadingContent = icon?.let { { Icon(it, null) } },
        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
    )
}

@Composable
private fun SwitchItem(icon: androidx.compose.ui.graphics.vector.ImageVector, title: String, subtitle: String, checked: Boolean, onChange: (Boolean) -> Unit) {
    ListItem(
        headlineContent = { Text(title) },
        supportingContent = { Text(subtitle) },
        leadingContent = { Icon(icon, null) },
        trailingContent = { Switch(checked = checked, onCheckedChange = null) },
        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
        modifier = Modifier.toggleable(value = checked, role = Role.Switch, onValueChange = onChange),
    )
}
