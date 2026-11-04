// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package model

import "strings"

// Theme values stored in User.Theme are a color scheme
// followed by "_serif" or "_sans_serif", or "none" (custom CSS only).
const (
	ThemeDefault = "dark_serif"
	ThemeNone    = "none"
)

// Color schemes returned by SplitTheme and User.ThemeAt.
const (
	SchemeLight        = "light"
	SchemeDark         = "dark"
	SchemeOLED         = "oled"              // True black variant of the dark scheme, for OLED screens.
	SchemeReadability  = "readability_light" // Light scheme after the Readability iOS app.
	SchemeTime         = "time"              // Light between ThemeLightHour and ThemeDarkHour, dark otherwise.
	SchemeTimeOLED     = "time_oled"         // Like "time", but the OLED scheme at night.
	SchemeTimeDarkOLED = "time_dark_oled"    // Like "time", but dark by day and OLED at night.
)

// Themes returns the available themes and their labels.
func Themes() map[string]string {
	return map[string]string{
		"light_serif":                  "Light – Serif",
		"light_sans_serif":             "Light – Sans-serif",
		"dark_serif":                   "Dark – Serif",
		"dark_sans_serif":              "Dark – Sans-serif",
		"oled_serif":                   "OLED – Serif",
		"oled_sans_serif":              "OLED – Sans-serif",
		"readability_light_serif":      "Readability light – Serif",
		"readability_light_sans_serif": "Readability light – Sans-serif",
		"system_serif":                 "System – Serif",
		"system_sans_serif":            "System – Sans-serif",
		"time_serif":                   "Light by day, dark by night – Serif",
		"time_sans_serif":              "Light by day, dark by night – Sans-serif",
		"time_oled_serif":              "Light by day, OLED by night – Serif",
		"time_oled_sans_serif":         "Light by day, OLED by night – Sans-serif",
		"time_dark_oled_serif":         "Dark by day, OLED by night – Serif",
		"time_dark_oled_sans_serif":    "Dark by day, OLED by night – Sans-serif",
		ThemeNone:                      "None (custom CSS only)",
	}
}

// SplitTheme returns the color scheme of theme and whether it uses the serif
// font. "none" is returned as scheme "none".
func SplitTheme(theme string) (scheme string, serif bool) {
	if scheme, ok := strings.CutSuffix(theme, "_sans_serif"); ok {
		return scheme, false
	}
	if scheme, ok := strings.CutSuffix(theme, "_serif"); ok {
		return scheme, true
	}
	return theme, false
}
