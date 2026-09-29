package io.github.anand34577.shortr.ui.onboarding

import androidx.compose.foundation.layout.width
import android.app.Activity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Dns
import androidx.compose.material.icons.rounded.Key
import androidx.compose.material.icons.rounded.QrCodeScanner
import androidx.compose.material.icons.rounded.Visibility
import androidx.compose.material.icons.rounded.VisibilityOff
import androidx.compose.material.icons.rounded.WarningAmber
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.anand34577.shortr.R
import io.github.anand34577.shortr.container
import io.github.anand34577.shortr.ui.common.PrimaryWideButton
import io.github.anand34577.shortr.ui.common.appViewModel
import io.github.anand34577.shortr.ui.theme.LocalExtraColors
import io.github.anand34577.shortr.ui.theme.MonoStyle
import kotlinx.coroutines.launch

@Composable
fun OnboardingFlow() {
    val c = LocalContext.current.container
    val vm = appViewModel { OnboardingViewModel(c) }
    val pairing by c.pendingPairing.collectAsStateWithLifecycle()
    LaunchedEffect(pairing) {
        pairing?.let { vm.pair(it); c.pendingPairing.value = null }
    }
    BackHandler(enabled = vm.step != OnboardingStep.Welcome) { vm.step = OnboardingStep.Welcome }

    AnimatedContent(
        targetState = vm.step,
        transitionSpec = {
            val forward = targetState.ordinal > initialState.ordinal
            slideInHorizontally { if (forward) it / 3 else -it / 3 } togetherWith slideOutHorizontally { if (forward) -it / 3 else it / 3 }
        },
        label = "onboarding",
    ) { step ->
        when (step) {
            OnboardingStep.Welcome -> WelcomeScreen(vm)
            OnboardingStep.Server -> ServerScreen(vm)
            OnboardingStep.Scan -> ScanScreen(onResult = vm::scanned, onClose = { vm.step = OnboardingStep.Welcome })
        }
    }
}

@Composable
private fun BrandMark(size: Int = 72) {
    Box(
        Modifier.size(size.dp).background(MaterialTheme.colorScheme.primary, RoundedCornerShape((size / 3.4f).dp)),
        contentAlignment = Alignment.Center,
    ) {
        Image(painterResource(R.drawable.ic_link_mark), null, Modifier.size((size * 0.55f).dp), colorFilter = ColorFilter.tint(MaterialTheme.colorScheme.onPrimary))
    }
}

@Composable
private fun WelcomeScreen(vm: OnboardingViewModel) {
    Column(
        Modifier.fillMaxSize().safeDrawingPadding().padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(Modifier.weight(1f))
        BrandMark()
        Spacer(Modifier.height(24.dp))
        Text("Shortr", style = MaterialTheme.typography.headlineMedium)
        Spacer(Modifier.height(8.dp))
        Text(
            "Short links and their stats, from your own server.",
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
        )
        Spacer(Modifier.weight(1.2f))
        vm.error?.let { ErrorNote(it); Spacer(Modifier.height(16.dp)) }
        PrimaryWideButton("Scan pairing code", onClick = { vm.error = null; vm.step = OnboardingStep.Scan })
        Spacer(Modifier.height(10.dp))
        OutlinedButton(onClick = { vm.error = null; vm.step = OnboardingStep.Server }, modifier = Modifier.fillMaxWidth().height(52.dp)) {
            Text("Enter server address")
        }
        Spacer(Modifier.height(16.dp))
        Text(
            "Pairing code: open the web console → Settings → API keys → New key.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
        )
    }
}

@Composable
private fun ServerScreen(vm: OnboardingViewModel) {
    val scope = rememberCoroutineScope()
    val ssoLauncher = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) { r ->
        if (r.resultCode == Activity.RESULT_OK || r.data != null) vm.completeSso(r.data)
    }
    var showKey by remember { mutableStateOf(false) }
    val cfg = vm.config

    Column(
        Modifier.fillMaxSize().safeDrawingPadding().imePadding().verticalScroll(rememberScrollState()).padding(horizontal = 24.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        IconButton(onClick = { vm.step = OnboardingStep.Welcome }) { Icon(Icons.AutoMirrored.Rounded.ArrowBack, "Back") }
        Text(if (cfg == null) "Connect to your server" else "Choose how to sign in", style = MaterialTheme.typography.headlineSmall)

        if (cfg == null) {
            Text(
                "Use the public address your short links use, for example https://sho.rt, or a LAN/VPN address like http://10.0.0.5:8080.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            OutlinedTextField(
                value = vm.serverInput,
                onValueChange = { vm.serverInput = it; vm.error = null },
                label = { Text("Server address") },
                leadingIcon = { Icon(Icons.Rounded.Dns, null) },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go, autoCorrectEnabled = false),
                keyboardActions = KeyboardActions(onGo = { vm.probe() }),
                modifier = Modifier.fillMaxWidth(),
            )
            vm.error?.let { ErrorNote(it) }
            PrimaryWideButton("Continue", onClick = vm::probe, enabled = vm.serverInput.isNotBlank(), loading = vm.busy)
            return@Column
        }

        ServerCard(cfg.siteName, vm.server.orEmpty(), onChange = vm::changeServer)

        cfg.oidc?.let { oidc ->
            PrimaryWideButton("Sign in with ${oidc.displayName}", loading = vm.busy, onClick = {
                scope.launch { vm.ssoIntent()?.let(ssoLauncher::launch) }
            })
            Row(verticalAlignment = Alignment.CenterVertically) {
                HorizontalDivider(Modifier.weight(1f))
                Text("or use an API key", modifier = Modifier.padding(horizontal = 12.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                HorizontalDivider(Modifier.weight(1f))
            }
        }

        OutlinedTextField(
            value = vm.keyInput,
            onValueChange = { vm.keyInput = it.trim(); vm.error = null },
            label = { Text("API key") },
            placeholder = { Text("sk_…") },
            leadingIcon = { Icon(Icons.Rounded.Key, null) },
            trailingIcon = {
                IconButton(onClick = { showKey = !showKey }) {
                    Icon(if (showKey) Icons.Rounded.VisibilityOff else Icons.Rounded.Visibility, if (showKey) "Hide key" else "Show key")
                }
            },
            visualTransformation = if (showKey) VisualTransformation.None else PasswordVisualTransformation(),
            textStyle = MonoStyle,
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done, autoCorrectEnabled = false),
            keyboardActions = KeyboardActions(onDone = { vm.connectWithKey() }),
            modifier = Modifier.fillMaxWidth(),
        )
        Text(
            "Create one in the web console under Settings → API keys. Give it links and stats access; it doesn't need admin.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        vm.error?.let { ErrorNote(it) }
        if (cfg.oidc == null) {
            PrimaryWideButton("Connect", onClick = vm::connectWithKey, enabled = vm.keyInput.isNotBlank(), loading = vm.busy)
        } else {
            OutlinedButton(onClick = vm::connectWithKey, enabled = vm.keyInput.isNotBlank() && !vm.busy, modifier = Modifier.fillMaxWidth().height(52.dp)) {
                Text("Connect with key")
            }
        }
        TextButton(onClick = { vm.step = OnboardingStep.Scan }, modifier = Modifier.align(Alignment.CenterHorizontally)) {
            Icon(Icons.Rounded.QrCodeScanner, null, Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
            Text("Scan a pairing code instead")
        }
    }
}

@Composable
private fun ServerCard(name: String, url: String, onChange: () -> Unit) {
    val insecure = url.startsWith("http://") && !isPrivateHost(url)
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)) {
        Row(Modifier.padding(16.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            BrandMark(40)
            Column(Modifier.weight(1f)) {
                Text(name, style = MaterialTheme.typography.titleMedium)
                Text(url, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            TextButton(onClick = onChange) { Text("Change") }
        }
        if (insecure) {
            Row(Modifier.padding(start = 16.dp, end = 16.dp, bottom = 14.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Icon(Icons.Rounded.WarningAmber, null, Modifier.size(18.dp), tint = LocalExtraColors.current.warning)
                Text(
                    "This public address uses http, so your key travels unencrypted. Use https if you can.",
                    style = MaterialTheme.typography.bodySmall,
                    color = LocalExtraColors.current.warning,
                )
            }
        }
    }
}

@Composable
private fun ErrorNote(text: String) {
    Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth()) {
        Text(text, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onErrorContainer, modifier = Modifier.padding(12.dp))
    }
}

/** LAN, VPN (CGNAT range used by Tailscale/Netbird) and .lan/.local names. */
internal fun isPrivateHost(url: String): Boolean {
    val host = runCatching { java.net.URI(url).host }.getOrNull()?.lowercase() ?: return false
    if (host == "localhost" || host.endsWith(".lan") || host.endsWith(".local") || host.endsWith(".home.arpa") || host.endsWith(".internal")) return true
    val p = host.split('.').mapNotNull { it.toIntOrNull() }
    if (p.size != 4) return false
    return p[0] == 10 || p[0] == 127 || (p[0] == 192 && p[1] == 168) || (p[0] == 172 && p[1] in 16..31) || (p[0] == 100 && p[1] in 64..127)
}
