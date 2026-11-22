package dev.furax.runyx

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

/**
 * Same palette and shape language as web/src/lib/theme.css: warm,
 * serif headings, one quiet ink-blue accent, no gradients/neon.
 */
private val LightColors = lightColorScheme(
    primary = Color(0xFF28527A),
    onPrimary = Color(0xFFFFFFFF),
    secondary = Color(0xFF6B6258),
    background = Color(0xFFFAF7F2),
    surface = Color(0xFFFFFFFF),
    surfaceVariant = Color(0xFFF1ECE3),
    onBackground = Color(0xFF2A2521),
    onSurface = Color(0xFF2A2521),
    onSurfaceVariant = Color(0xFF6B6258),
    outline = Color(0xFFE4DCD0),
    error = Color(0xFFB3492F),
)

private val DarkColors = darkColorScheme(
    primary = Color(0xFF6FA0C9),
    onPrimary = Color(0xFF15232E),
    secondary = Color(0xFFA89D8E),
    background = Color(0xFF211E1B),
    surface = Color(0xFF2A2622),
    surfaceVariant = Color(0xFF1A1815),
    onBackground = Color(0xFFEDE7DE),
    onSurface = Color(0xFFEDE7DE),
    onSurfaceVariant = Color(0xFFA89D8E),
    outline = Color(0xFF3A352E),
    error = Color(0xFFE08066),
)

private val serif = FontFamily.Serif
private val sans = FontFamily.SansSerif

private val RunyxTypography = Typography().let { base ->
    base.copy(
        displaySmall = base.displaySmall.copy(fontFamily = serif, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.3).sp),
        headlineSmall = base.headlineSmall.copy(fontFamily = serif, fontWeight = FontWeight.SemiBold),
        titleLarge = base.titleLarge.copy(fontFamily = serif, fontWeight = FontWeight.SemiBold),
        titleMedium = base.titleMedium.copy(fontFamily = sans, fontWeight = FontWeight.Medium),
        bodyLarge = base.bodyLarge.copy(fontFamily = sans),
        bodyMedium = base.bodyMedium.copy(fontFamily = sans),
        labelLarge = base.labelLarge.copy(fontFamily = sans, fontWeight = FontWeight.Medium),
    )
}

/** Easing used for every transition/animation, mirroring --ease-fluid in the web app. */
val RunyxEasing = androidx.compose.animation.core.CubicBezierEasing(0.22f, 1f, 0.36f, 1f)

@Composable
fun RunyxTheme(content: @Composable () -> Unit) {
    val colors = if (isSystemInDarkTheme()) DarkColors else LightColors
    MaterialTheme(colorScheme = colors, typography = RunyxTypography, content = content)
}

/** Placeholder text style for the mono-space pairing code field. */
val MonoTextStyle = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 14.sp)
