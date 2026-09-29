package io.github.anand34577.shortr.ui.theme

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.anand34577.shortr.data.ThemeMode

// Brand colours, matching the web console (web/src/index.css).
private val Brand = Color(0xFF4766F1)
private val BrandDark = Color(0xFF6487FF)

private val LightColors = lightColorScheme(
    primary = Brand,
    onPrimary = Color.White,
    primaryContainer = Color(0xFFE0E6FF),
    onPrimaryContainer = Color(0xFF0E1C6B),
    secondary = Color(0xFF5B5F76),
    secondaryContainer = Color(0xFFE2E3F5),
    onSecondaryContainer = Color(0xFF181B2E),
    tertiary = Color(0xFF007834),
    background = Color(0xFFFBFBFE),
    surface = Color(0xFFFBFBFE),
    surfaceContainerLowest = Color.White,
    surfaceContainerLow = Color(0xFFF5F5FA),
    surfaceContainer = Color(0xFFEFEFF6),
    surfaceContainerHigh = Color(0xFFE9E9F1),
    surfaceContainerHighest = Color(0xFFE3E3EC),
    error = Color(0xFFD40C1A),
    outlineVariant = Color(0xFFD9DAE6),
)

private val DarkColors = darkColorScheme(
    primary = BrandDark,
    onPrimary = Color(0xFF0A1352),
    primaryContainer = Color(0xFF26347F),
    onPrimaryContainer = Color(0xFFDDE3FF),
    secondary = Color(0xFFC3C5DD),
    secondaryContainer = Color(0xFF2F3246),
    onSecondaryContainer = Color(0xFFDFE1F8),
    tertiary = Color(0xFF63D37D),
    background = Color(0xFF09090A),
    surface = Color(0xFF09090A),
    surfaceContainerLowest = Color(0xFF050506),
    surfaceContainerLow = Color(0xFF121214),
    surfaceContainer = Color(0xFF17171A),
    surfaceContainerHigh = Color(0xFF1E1E22),
    surfaceContainerHighest = Color(0xFF26262B),
    error = Color(0xFFFF7E76),
    outlineVariant = Color(0xFF2E2E35),
)

/** Colours Material 3 doesn't name: success/warning, used for status chips. */
@Immutable
data class ExtraColors(val success: Color, val onSuccessContainer: Color, val successContainer: Color, val warning: Color, val warningContainer: Color)

private val LightExtra = ExtraColors(Color(0xFF007834), Color(0xFF00391A), Color(0xFFD6F5DF), Color(0xFFA05000), Color(0xFFFFE8CC))
private val DarkExtra = ExtraColors(Color(0xFF63D37D), Color(0xFFCFF7D8), Color(0xFF123A21), Color(0xFFFEC348), Color(0xFF3D2C06))

val LocalExtraColors = staticCompositionLocalOf { LightExtra }

private val AppTypography = Typography().run {
    copy(
        headlineMedium = headlineMedium.copy(fontWeight = FontWeight.SemiBold, letterSpacing = (-0.4).sp),
        headlineSmall = headlineSmall.copy(fontWeight = FontWeight.SemiBold, letterSpacing = (-0.2).sp),
        titleLarge = titleLarge.copy(fontWeight = FontWeight.SemiBold),
        titleMedium = titleMedium.copy(fontWeight = FontWeight.SemiBold),
        labelLarge = labelLarge.copy(fontWeight = FontWeight.SemiBold),
    )
}

val MonoStyle = TextStyle(fontFamily = FontFamily.Monospace)

private val AppShapes = Shapes(
    extraSmall = androidx.compose.foundation.shape.RoundedCornerShape(6.dp),
    small = androidx.compose.foundation.shape.RoundedCornerShape(10.dp),
    medium = androidx.compose.foundation.shape.RoundedCornerShape(16.dp),
    large = androidx.compose.foundation.shape.RoundedCornerShape(24.dp),
    extraLarge = androidx.compose.foundation.shape.RoundedCornerShape(32.dp),
)

@Composable
fun ShortrTheme(mode: ThemeMode = ThemeMode.System, dynamicColor: Boolean = true, content: @Composable () -> Unit) {
    val dark = when (mode) {
        ThemeMode.System -> isSystemInDarkTheme()
        ThemeMode.Light -> false
        ThemeMode.Dark -> true
    }
    val ctx = LocalContext.current
    val colors = when {
        dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> if (dark) dynamicDarkColorScheme(ctx) else dynamicLightColorScheme(ctx)
        dark -> DarkColors
        else -> LightColors
    }
    androidx.compose.runtime.CompositionLocalProvider(LocalExtraColors provides if (dark) DarkExtra else LightExtra) {
        MaterialTheme(colorScheme = colors, typography = AppTypography, shapes = AppShapes, content = content)
    }
}
