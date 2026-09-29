package io.github.anand34577.shortr

import android.content.Intent
import android.os.Bundle
import android.os.SystemClock
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.getValue
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.DefaultLifecycleObserver
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ProcessLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.lifecycleScope
import io.github.anand34577.shortr.data.AppPrefs
import io.github.anand34577.shortr.data.SessionState
import io.github.anand34577.shortr.ui.AppRoot
import io.github.anand34577.shortr.ui.common.extractUrl
import io.github.anand34577.shortr.ui.theme.ShortrTheme
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

/** FragmentActivity (not plain ComponentActivity) because BiometricPrompt needs it. */
class MainActivity : FragmentActivity() {
    private var backgroundedAt = 0L

    override fun onCreate(savedInstanceState: Bundle?) {
        val splash = installSplashScreen()
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val c = container
        splash.setKeepOnScreenCondition { c.state.value is SessionState.Loading }

        if (savedInstanceState == null) {
            handleIntent(intent)
            lifecycleScope.launch { if (c.storage.prefs.first().appLock) c.locked.value = true }
        }
        ProcessLifecycleOwner.get().lifecycle.addObserver(lockObserver)

        setContent {
            val prefs by c.storage.prefs.collectAsStateWithLifecycle(AppPrefs())
            ShortrTheme(mode = prefs.theme, dynamicColor = prefs.dynamicColor) {
                AppRoot()
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIntent(intent)
    }

    override fun onDestroy() {
        ProcessLifecycleOwner.get().lifecycle.removeObserver(lockObserver)
        super.onDestroy()
    }

    private fun handleIntent(intent: Intent?) {
        val c = container
        when {
            intent == null -> Unit
            intent.action == Intent.ACTION_VIEW && intent.data?.scheme == "shortr" -> c.pendingPairing.value = intent.data
            intent.action == Intent.ACTION_SEND -> extractUrl(intent.getStringExtra(Intent.EXTRA_TEXT))?.let { c.pendingShare.value = it }
            intent.action == ACTION_NEW_LINK -> c.pendingNewLink.value = true
        }
    }

    // Re-lock after the app has been in the background for a little while.
    private val lockObserver = object : DefaultLifecycleObserver {
        override fun onStop(owner: LifecycleOwner) {
            backgroundedAt = SystemClock.elapsedRealtime()
        }

        override fun onStart(owner: LifecycleOwner) {
            if (backgroundedAt == 0L) return
            val away = SystemClock.elapsedRealtime() - backgroundedAt
            lifecycleScope.launch {
                if (away > LOCK_AFTER_MS && container.storage.prefs.first().appLock) container.locked.value = true
            }
        }
    }

    companion object {
        const val ACTION_NEW_LINK = "io.github.anand34577.shortr.NEW_LINK"
        private const val LOCK_AFTER_MS = 30_000L
    }
}
