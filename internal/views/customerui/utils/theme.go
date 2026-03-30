package utils

import (
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// BuildThemeCSS generates CSS custom properties for theme variables.
// This is the unified theme CSS builder used by both layout and login pages.
func BuildThemeCSS(theme *models.AppTheme, includeDefaults bool) string {
	if theme == nil && !includeDefaults {
		return ""
	}

	css := ":root{"

	// Primary color
	if theme != nil && theme.PrimaryColor != "" {
		css += "--theme-primary:" + theme.PrimaryColor + ";"
		css += "--theme-primary-hover:" + DarkenColor(theme.PrimaryColor, 20) + ";"
	} else if includeDefaults {
		css += "--theme-primary:#2563EB;"
		css += "--theme-primary-hover:#1D4ED8;"
	}

	// White/Black background colors
	if theme != nil && theme.WhiteColor != "" {
		css += "--theme-white:" + theme.WhiteColor + ";"
	} else if includeDefaults {
		css += "--theme-white:#ffffff;"
	}
	blackForRoot := ""
	if theme != nil {
		blackForRoot = theme.BlackColor
		if theme.BlackColorLight != "" {
			blackForRoot = theme.BlackColorLight
		}
	}
	if blackForRoot != "" {
		css += "--theme-black:" + blackForRoot + ";"
	} else if includeDefaults {
		css += "--theme-black:#121212;"
	}

	// Heading font
	if theme != nil && theme.HeadingFontBase64 != "" {
		css += "--font-heading:'CustomHeading','Inter',ui-sans-serif,system-ui,sans-serif;"
	} else if theme != nil && theme.HeadingFont != "" {
		css += "--font-heading:'" + theme.HeadingFont + "','Inter',ui-sans-serif,system-ui,sans-serif;"
	} else if includeDefaults {
		css += "--font-heading:'Inter',ui-sans-serif,system-ui,sans-serif;"
	}

	// Body font
	if theme != nil && theme.BodyFontBase64 != "" {
		css += "--font-body:'CustomBody','Inter',ui-sans-serif,system-ui,sans-serif;"
	} else if theme != nil && theme.BodyFont != "" {
		css += "--font-body:'" + theme.BodyFont + "','Inter',ui-sans-serif,system-ui,sans-serif;"
	} else if includeDefaults {
		css += "--font-body:'Inter',ui-sans-serif,system-ui,sans-serif;"
	}

	// Border radius
	if theme != nil {
		switch theme.BorderRadius {
		case "sharp":
			css += "--theme-radius:0;"
		case "subtle":
			css += "--theme-radius:4px;"
		case "very-rounded":
			css += "--theme-radius:12px;"
		default:
			css += "--theme-radius:8px;"
		}
	} else if includeDefaults {
		css += "--theme-radius:8px;"
	}

	css += "}"

	// Append .dark{} block for dark-mode overrides
	darkCSS := ""
	if theme != nil && theme.WhiteColorDark != "" {
		darkCSS += "--theme-white:" + theme.WhiteColorDark + ";"
	}
	if theme != nil && theme.BlackColor != "" {
		darkCSS += "--theme-black:" + theme.BlackColor + ";"
	} else {
		darkCSS += "--theme-black:#121212;"
	}
	if theme != nil && theme.PrimaryColorDark != "" {
		darkCSS += "--theme-primary:" + theme.PrimaryColorDark + ";"
		darkCSS += "--theme-primary-hover:" + DarkenColor(theme.PrimaryColorDark, 20) + ";"
	}

	rootBlock := ""
	if css != ":root{}" {
		rootBlock = css
	}
	return "<style>" + rootBlock + ".dark{" + darkCSS + "}</style>"
}

// BuildLayoutThemeCSS generates theme CSS for the main layout (without defaults).
// This version only includes theme overrides if they are explicitly set.
func BuildLayoutThemeCSS(theme *models.AppTheme) string {
	return BuildThemeCSS(theme, false)
}

// BuildLoginThemeCSS generates theme CSS for the login page (with defaults).
// This version includes sensible defaults even if no theme is provided.
func BuildLoginThemeCSS(theme *models.AppTheme) string {
	return BuildThemeCSS(theme, true)
}

// BuildFontFace generates a @font-face CSS rule for custom fonts.
func BuildFontFace(fontFamily string, fontData string) string {
	if fontData == "" {
		return ""
	}

	return `<style>
@font-face {
	font-family: '` + fontFamily + `';
	src: url(` + fontData + `) format('woff2');
	font-display: swap;
}
</style>`
}

// BuildHeadingFontFace generates the heading font @font-face rule.
func BuildHeadingFontFace(base64Data string) string {
	return BuildFontFace("CustomHeading", base64Data)
}

// BuildBodyFontFace generates the body font @font-face rule.
func BuildBodyFontFace(base64Data string) string {
	return BuildFontFace("CustomBody", base64Data)
}

// BuildGoogleFontsURL generates a Google Fonts URL for the specified fonts.
// This consolidates font loading logic and avoids duplicate font requests.
func BuildGoogleFontsURL(headingFont, bodyFont string, headingFontBase64, bodyFontBase64 string) string {
	// Don't load from Google if we have base64 fonts
	needHeading := headingFont != "" && headingFontBase64 == ""
	needBody := bodyFont != "" && bodyFontBase64 == ""

	if !needHeading && !needBody {
		return ""
	}

	if needHeading && needBody && headingFont != bodyFont {
		return "https://fonts.googleapis.com/css2?family=" + headingFont + ":wght@500;600;700&family=" + bodyFont + ":wght@400;500;600&display=swap"
	} else if needHeading {
		return "https://fonts.googleapis.com/css2?family=" + headingFont + ":wght@400;500;600;700&display=swap"
	} else if needBody {
		return "https://fonts.googleapis.com/css2?family=" + bodyFont + ":wght@400;500;600;700&display=swap"
	}

	return ""
}

// GetRadiusClass returns the CSS class for the specified border radius.
func GetRadiusClass(radius string) string {
	switch radius {
	case "sharp":
		return "radius-sharp"
	case "subtle":
		return "radius-subtle"
	case "very-rounded":
		return "radius-very-rounded"
	default:
		return "radius-rounded"
	}
}

// DarkenColor darkens a hex color by the specified percentage.
// percentage should be between 0 and 100 (e.g., 20 for 20% darker).
func DarkenColor(hexColor string, percent int) string {
	if len(hexColor) != 7 || hexColor[0] != '#' {
		return hexColor
	}

	// Parse RGB values
	r := hexToInt(hexColor[1:3])
	g := hexToInt(hexColor[3:5])
	b := hexToInt(hexColor[5:7])

	// Darken by percentage
	factor := float64(100-percent) / 100.0
	r = int(float64(r) * factor)
	g = int(float64(g) * factor)
	b = int(float64(b) * factor)

	return "#" + intToHex(r) + intToHex(g) + intToHex(b)
}

// hexToInt converts a two-character hex string to an integer (0-255).
func hexToInt(s string) int {
	var result int
	for _, c := range s {
		result *= 16
		if c >= '0' && c <= '9' {
			result += int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			result += int(c - 'a' + 10)
		} else if c >= 'A' && c <= 'F' {
			result += int(c - 'A' + 10)
		}
	}
	return result
}

// intToHex converts an integer (0-255) to a two-character hex string.
func intToHex(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	chars := "0123456789ABCDEF"
	return string(chars[n/16]) + string(chars[n%16])
}
