package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppTheme_FaviconBase64_DefaultEmpty(t *testing.T) {
	theme := &AppTheme{}
	assert.Equal(t, "", theme.FaviconBase64)
}

func TestAppTheme_FaviconBase64_SetAndGet(t *testing.T) {
	faviconDataURI := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	theme := &AppTheme{
		FaviconBase64: faviconDataURI,
	}

	assert.Equal(t, faviconDataURI, theme.FaviconBase64)
}

func TestGetOrCreateAppTheme_FaviconBase64_DefaultEmpty(t *testing.T) {
	// When no org ID is provided, the returned default theme should have an empty FaviconBase64
	theme, err := GetOrCreateAppTheme(nil, "")
	assert.NoError(t, err)
	assert.NotNil(t, theme)
	assert.Equal(t, "", theme.FaviconBase64)
}

func TestIsValidThemeMode_ValidValues(t *testing.T) {
	assert.True(t, IsValidThemeMode("auto"))
	assert.True(t, IsValidThemeMode("light"))
	assert.True(t, IsValidThemeMode("dark"))
}

func TestIsValidThemeMode_InvalidValues(t *testing.T) {
	assert.False(t, IsValidThemeMode(""))
	assert.False(t, IsValidThemeMode("system"))
	assert.False(t, IsValidThemeMode("DARK"))
}

func TestAppTheme_GetThemeMode_Default(t *testing.T) {
	theme := &AppTheme{}
	assert.Equal(t, "auto", theme.GetThemeMode())
}

func TestAppTheme_GetThemeMode_Set(t *testing.T) {
	theme := &AppTheme{ThemeMode: "dark"}
	assert.Equal(t, "dark", theme.GetThemeMode())

	theme.ThemeMode = "light"
	assert.Equal(t, "light", theme.GetThemeMode())
}

func TestAppTheme_CustomCSS_DefaultEmpty(t *testing.T) {
	theme := &AppTheme{}
	assert.Equal(t, "", theme.CustomCSS)
}

func TestAppTheme_CustomCSS_SetAndGet(t *testing.T) {
	theme := &AppTheme{CustomCSS: ".foo { color: red; }"}
	assert.Equal(t, ".foo { color: red; }", theme.CustomCSS)
}

func TestAppTheme_FaviconBase64_IndependentOfLogo(t *testing.T) {
	// FaviconBase64 and logo fields are independent
	theme := &AppTheme{
		LogoLightBase64: "data:image/svg+xml;base64,logo",
		LogoDarkBase64:  "data:image/svg+xml;base64,logo-dark",
		FaviconBase64:   "data:image/png;base64,favicon",
	}

	assert.Equal(t, "data:image/svg+xml;base64,logo", theme.LogoLightBase64)
	assert.Equal(t, "data:image/svg+xml;base64,logo-dark", theme.LogoDarkBase64)
	assert.Equal(t, "data:image/png;base64,favicon", theme.FaviconBase64)
}
