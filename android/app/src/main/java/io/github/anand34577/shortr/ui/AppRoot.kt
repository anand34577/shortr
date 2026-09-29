package io.github.anand34577.shortr.ui

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.Home
import androidx.compose.material.icons.rounded.Link
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.Link
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import io.github.anand34577.shortr.container
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.data.SessionState
import io.github.anand34577.shortr.ui.home.HomeScreen
import io.github.anand34577.shortr.ui.links.LinkDetailScreen
import io.github.anand34577.shortr.ui.links.LinkEditorSheet
import io.github.anand34577.shortr.ui.links.LinksScreen
import io.github.anand34577.shortr.ui.lock.LockScreen
import io.github.anand34577.shortr.ui.onboarding.OnboardingFlow
import io.github.anand34577.shortr.ui.settings.SettingsScreen
import kotlinx.serialization.Serializable

@Serializable object HomeRoute
@Serializable object LinksRoute
@Serializable object SettingsRoute
@Serializable data class LinkRoute(val id: String)

/** Opens the create/edit sheet from anywhere (FAB, share sheet, shortcut, detail screen). */
class EditorController {
    var request by mutableStateOf<EditorRequest?>(null)
        private set

    fun create(url: String? = null) { request = EditorRequest(null, url) }
    fun edit(link: Link) { request = EditorRequest(link, null) }
    fun close() { request = null }
}

data class EditorRequest(val existing: Link?, val initialUrl: String?)

val LocalEditor = staticCompositionLocalOf { EditorController() }
val LocalSnackbar = staticCompositionLocalOf { SnackbarHostState() }

@Composable
fun AppRoot() {
    val c = LocalContext.current.container
    val state by c.state.collectAsStateWithLifecycle()
    val locked by c.locked.collectAsStateWithLifecycle()

    Box(Modifier.fillMaxSize()) {
        AnimatedContent(targetState = state::class, transitionSpec = { fadeIn() togetherWith fadeOut() }, label = "root") { kind ->
            when (kind) {
                SessionState.SignedIn::class -> MainShell()
                SessionState.SignedOut::class -> OnboardingFlow()
                else -> Box(Modifier.fillMaxSize())
            }
        }
        if (locked && state is SessionState.SignedIn) LockScreen(onUnlocked = { c.locked.value = false })
    }
}

private data class Tab(val route: Any, val label: String, val icon: ImageVector, val selectedIcon: ImageVector)

private val tabs = listOf(
    Tab(HomeRoute, "Home", Icons.Outlined.Home, Icons.Rounded.Home),
    Tab(LinksRoute, "Links", Icons.Outlined.Link, Icons.Rounded.Link),
    Tab(SettingsRoute, "Settings", Icons.Outlined.Settings, Icons.Rounded.Settings),
)

@Composable
private fun MainShell() {
    val c = LocalContext.current.container
    val nav = rememberNavController()
    val editor = remember { EditorController() }
    val snackbar = remember { SnackbarHostState() }
    val entry by nav.currentBackStackEntryAsState()
    val dest = entry?.destination
    val onTab = tabs.any { t -> dest?.hasRoute(t.route::class) == true }
    val showFab = dest?.hasRoute(HomeRoute::class) == true || dest?.hasRoute(LinksRoute::class) == true

    // Things that arrive from outside the app
    val shared by c.pendingShare.collectAsStateWithLifecycle()
    val newLink by c.pendingNewLink.collectAsStateWithLifecycle()
    LaunchedEffect(shared) { shared?.let { editor.create(it); c.pendingShare.value = null } }
    LaunchedEffect(newLink) { if (newLink) { editor.create(); c.pendingNewLink.value = false } }
    // A pairing link while already connected is ignored (sign out first to switch servers).
    LaunchedEffect(Unit) { c.pendingPairing.value = null }


    CompositionLocalProvider(LocalEditor provides editor, LocalSnackbar provides snackbar) {
        Scaffold(
            snackbarHost = { SnackbarHost(snackbar) },
            floatingActionButton = {
                if (showFab) {
                    ExtendedFloatingActionButton(
                        onClick = { editor.create() },
                        icon = { Icon(Icons.Rounded.Add, null) },
                        text = { Text("New link") },
                    )
                }
            },
            bottomBar = {
                if (onTab) {
                    NavigationBar {
                        tabs.forEach { t ->
                            val sel = dest?.hasRoute(t.route::class) == true
                            NavigationBarItem(
                                selected = sel,
                                onClick = {
                                    nav.navigate(t.route) {
                                        popUpTo(nav.graph.findStartDestination().id) { saveState = true }
                                        launchSingleTop = true
                                        restoreState = true
                                    }
                                },
                                icon = { Icon(if (sel) t.selectedIcon else t.icon, null) },
                                label = { Text(t.label) },
                            )
                        }
                    }
                }
            },
        ) { padding ->
            NavHost(nav, startDestination = HomeRoute) {
                composable<HomeRoute> {
                    HomeScreen(padding, onOpenLink = { nav.navigate(LinkRoute(it)) }, onSeeAll = {
                        nav.navigate(LinksRoute) { popUpTo(nav.graph.findStartDestination().id) { saveState = true }; launchSingleTop = true; restoreState = true }
                    })
                }
                composable<LinksRoute> { LinksScreen(padding, onOpenLink = { nav.navigate(LinkRoute(it)) }) }
                composable<SettingsRoute> { SettingsScreen(padding) }
                composable<LinkRoute> { e -> LinkDetailScreen(e.toRoute<LinkRoute>().id, onBack = { nav.popBackStack() }) }
            }
        }

        editor.request?.let { req ->
            LinkEditorSheet(
                request = req,
                onDismiss = editor::close,
                onSaved = { saved, created ->
                    editor.close()
                    c.notifyLinksChanged()
                    if (created) nav.navigate(LinkRoute(saved.id))
                },
            )
        }
    }
}
