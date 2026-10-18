package tv.afterglow.client

import androidx.compose.runtime.*
import androidx.compose.ui.graphics.Color

data class TvAppearance(val name:String,val background:Color,val surface:Color,val accent:Color)
val appearanceThemes=listOf(
 TvAppearance("Tidal",Color(0xff0b1018),Color(0xff1b2634),Color(0xffabd9d2)),
 TvAppearance("Aurora",Color(0xff100e1b),Color(0xff272138),Color(0xffd1b7f4)),
 TvAppearance("Ember",Color(0xff160f0d),Color(0xff30231e),Color(0xfff2bf91)),
 TvAppearance("Glacier",Color(0xff09121b),Color(0xff1b2b3c),Color(0xffa6d1ff)),
 TvAppearance("OLED",Color.Black,Color(0xff181818),Color(0xffe0e0e0))
)
val LocalAppearance=staticCompositionLocalOf{appearanceThemes.first()}
val LocalMotion=staticCompositionLocalOf{true}
val LocalCompact=staticCompositionLocalOf{false}
