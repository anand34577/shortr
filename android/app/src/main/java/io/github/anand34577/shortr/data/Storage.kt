package io.github.anand34577.shortr.data

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private val Context.dataStore by preferencesDataStore(name = "shortr")

/** How this device is connected to a server. */
@Serializable
data class Session(
    val server: String,          // API origin, e.g. https://sho.rt
    val siteName: String = "Shortr",
    val method: AuthMethod,
    val apiKey: String? = null,  // for AuthMethod.ApiKey
    val authState: String? = null, // AppAuth AuthState JSON, for AuthMethod.Sso
    val account: String? = null, // email shown in Settings
)

@Serializable
enum class AuthMethod { ApiKey, Sso }

enum class ThemeMode { System, Light, Dark }

data class AppPrefs(
    val theme: ThemeMode = ThemeMode.System,
    val dynamicColor: Boolean = false,
    val appLock: Boolean = false,
)

/**
 * Session is sealed with an AES-256-GCM key that lives in the Android
 * Keystore and never leaves it, so copying the app's files off the device
 * gives an attacker nothing usable.
 */
class Storage(private val context: Context) {
    private val json = Json { ignoreUnknownKeys = true }
    private val sessionKey = stringPreferencesKey("session_sealed")
    private val themeKey = stringPreferencesKey("theme")
    private val dynamicKey = booleanPreferencesKey("dynamic_color")
    private val lockKey = booleanPreferencesKey("app_lock")

    val prefs: Flow<AppPrefs> = context.dataStore.data.map { p ->
        AppPrefs(
            theme = p[themeKey]?.let { runCatching { ThemeMode.valueOf(it) }.getOrNull() } ?: ThemeMode.System,
            dynamicColor = p[dynamicKey] ?: false,
            appLock = p[lockKey] ?: false,
        )
    }

    suspend fun setTheme(mode: ThemeMode) = context.dataStore.edit { it[themeKey] = mode.name }
    suspend fun setDynamicColor(on: Boolean) = context.dataStore.edit { it[dynamicKey] = on }
    suspend fun setAppLock(on: Boolean) = context.dataStore.edit { it[lockKey] = on }

    suspend fun loadSession(): Session? {
        val sealed = context.dataStore.data.first()[sessionKey] ?: return null
        return runCatching { json.decodeFromString<Session>(open(sealed)) }.getOrElse {
            // key was wiped (e.g. screen lock removed) — start over rather than crash
            clearSession()
            null
        }
    }

    suspend fun saveSession(session: Session) {
        val sealed = seal(json.encodeToString(Session.serializer(), session))
        context.dataStore.edit { it[sessionKey] = sealed }
    }

    suspend fun clearSession() {
        context.dataStore.edit { it.remove(sessionKey) }
    }

    // --- Keystore sealing -------------------------------------------------

    private fun key(): SecretKey {
        val ks = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        (ks.getEntry(ALIAS, null) as? KeyStore.SecretKeyEntry)?.let { return it.secretKey }
        val gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE)
        gen.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return gen.generateKey()
    }

    private fun seal(plain: String): String {
        val c = Cipher.getInstance(TRANSFORM).apply { init(Cipher.ENCRYPT_MODE, key()) }
        val out = c.iv + c.doFinal(plain.toByteArray())
        return Base64.encodeToString(out, Base64.NO_WRAP)
    }

    private fun open(sealed: String): String {
        val raw = Base64.decode(sealed, Base64.NO_WRAP)
        val c = Cipher.getInstance(TRANSFORM).apply {
            init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, raw, 0, IV_LEN))
        }
        return String(c.doFinal(raw, IV_LEN, raw.size - IV_LEN))
    }

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val ALIAS = "shortr.session.v1"
        const val TRANSFORM = "AES/GCM/NoPadding"
        const val IV_LEN = 12
    }
}
