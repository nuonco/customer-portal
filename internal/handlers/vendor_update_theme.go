package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

func (h *Handler) UpdateThemeSettings(c *gin.Context) {
	var req struct {
		PrimaryColor       string `json:"primary_color"`
		SecondaryColor     string `json:"secondary_color"`
		PrimaryColorDark   string `json:"primary_color_dark"`   // Dark mode primary color
		SecondaryColorDark string `json:"secondary_color_dark"` // Dark mode secondary color
		LogoBase64         string `json:"logo_base64"`          // Kept for backward compatibility (maps to LogoLightBase64)
		LogoLightBase64    string `json:"logo_light_base64"`    // Light mode logo
		LogoDarkBase64     string `json:"logo_dark_base64"`     // Dark mode logo
		FaviconBase64      string `json:"favicon_base64"`
		HeadingFont        string `json:"heading_font"`
		BodyFont           string `json:"body_font"`
		HeadingFontBase64  string `json:"heading_font_base64"`
		BodyFontBase64     string `json:"body_font_base64"`
		HeadingFontName    string `json:"heading_font_name"`
		BodyFontName       string `json:"body_font_name"`
		WhiteColor         string `json:"white_color"`
		BlackColor         string `json:"black_color"`
		WhiteColorDark     string `json:"white_color_dark"`
		BlackColorLight    string `json:"black_color_light"`
		BorderRadius       string `json:"border_radius"`
		ThemeMode          string `json:"theme_mode"`
		CustomCSS          string `json:"custom_css"`
		HeaderTitle        string `json:"header_title"`
		HeaderTitleHidden  *bool  `json:"header_title_hidden"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get or create the global app theme
	theme, err := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load theme settings"})
		return
	}

	// Update fields if provided
	if req.PrimaryColor != "" {
		theme.PrimaryColor = req.PrimaryColor
	}
	if req.SecondaryColor != "" {
		theme.SecondaryColor = req.SecondaryColor
	}

	// Handle dark mode colors - empty string or "REMOVE" clears, valid hex color sets
	// Unlike light mode colors, dark mode colors can be cleared to fall back to light mode
	isValidHexColor := func(s string) bool {
		if len(s) != 7 || s[0] != '#' {
			return false
		}
		for _, c := range s[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}

	// Dark mode primary color: valid hex sets it, empty/REMOVE clears it
	if isValidHexColor(req.PrimaryColorDark) {
		theme.PrimaryColorDark = req.PrimaryColorDark
	} else {
		// Clear dark mode color (falls back to light mode)
		theme.PrimaryColorDark = ""
	}

	// Dark mode secondary color: valid hex sets it, empty/REMOVE clears it
	if isValidHexColor(req.SecondaryColorDark) {
		theme.SecondaryColorDark = req.SecondaryColorDark
	} else {
		// Clear dark mode color (falls back to light mode)
		theme.SecondaryColorDark = ""
	}

	// White/Black background colors: valid hex sets them, empty/invalid clears them
	if isValidHexColor(req.WhiteColor) {
		theme.WhiteColor = req.WhiteColor
	} else {
		theme.WhiteColor = ""
	}
	if isValidHexColor(req.BlackColor) {
		theme.BlackColor = req.BlackColor
	} else {
		theme.BlackColor = ""
	}
	if isValidHexColor(req.WhiteColorDark) {
		theme.WhiteColorDark = req.WhiteColorDark
	} else {
		theme.WhiteColorDark = ""
	}
	if isValidHexColor(req.BlackColorLight) {
		theme.BlackColorLight = req.BlackColorLight
	} else {
		theme.BlackColorLight = ""
	}

	// Handle light mode logo - "REMOVE" clears, valid data URI sets
	if req.LogoLightBase64 != "" {
		if req.LogoLightBase64 == "REMOVE" {
			theme.LogoLightBase64 = ""
		} else if strings.HasPrefix(req.LogoLightBase64, "data:image/") {
			theme.LogoLightBase64 = req.LogoLightBase64
		}
	}

	// Handle dark mode logo - "REMOVE" clears, valid data URI sets
	if req.LogoDarkBase64 != "" {
		if req.LogoDarkBase64 == "REMOVE" {
			theme.LogoDarkBase64 = ""
		} else if strings.HasPrefix(req.LogoDarkBase64, "data:image/") {
			theme.LogoDarkBase64 = req.LogoDarkBase64
		}
	}

	// Backward compatibility: handle legacy LogoBase64 field (maps to light mode logo)
	if req.LogoBase64 != "" {
		if req.LogoBase64 == "REMOVE" {
			theme.LogoLightBase64 = ""
		} else if strings.HasPrefix(req.LogoBase64, "data:image/") {
			theme.LogoLightBase64 = req.LogoBase64
		}
	}

	// Handle favicon - "REMOVE" clears, valid image data URI sets
	if req.FaviconBase64 != "" {
		if req.FaviconBase64 == "REMOVE" {
			theme.FaviconBase64 = ""
		} else if strings.HasPrefix(req.FaviconBase64, "data:image/") {
			theme.FaviconBase64 = req.FaviconBase64
		}
	}

	// Fonts can be set to empty string to use default system fonts
	theme.HeadingFont = req.HeadingFont
	theme.BodyFont = req.BodyFont

	// Handle custom font uploads (base64 data URIs)
	isValidFontDataURI := func(s string) bool {
		return strings.HasPrefix(s, "data:font/woff2") ||
			strings.HasPrefix(s, "data:font/woff") ||
			strings.HasPrefix(s, "data:application/font-woff2") ||
			strings.HasPrefix(s, "data:application/font-woff") ||
			strings.HasPrefix(s, "data:application/x-font-woff") ||
			strings.HasPrefix(s, "data:application/octet-stream")
	}

	// HeadingFontBase64: "REMOVE" clears, valid data URI sets
	if req.HeadingFontBase64 == "REMOVE" {
		theme.HeadingFontBase64 = ""
		theme.HeadingFontName = ""
	} else if len(req.HeadingFontBase64) > 0 && isValidFontDataURI(req.HeadingFontBase64) {
		if len(req.HeadingFontBase64) <= 400000 {
			theme.HeadingFontBase64 = req.HeadingFontBase64
			theme.HeadingFontName = req.HeadingFontName
		}
	}

	// BodyFontBase64: "REMOVE" clears, valid data URI sets
	if req.BodyFontBase64 == "REMOVE" {
		theme.BodyFontBase64 = ""
		theme.BodyFontName = ""
	} else if len(req.BodyFontBase64) > 0 && isValidFontDataURI(req.BodyFontBase64) {
		if len(req.BodyFontBase64) <= 400000 {
			theme.BodyFontBase64 = req.BodyFontBase64
			theme.BodyFontName = req.BodyFontName
		}
	}

	// Update border radius if provided and valid
	if req.BorderRadius != "" {
		if models.IsValidBorderRadius(req.BorderRadius) {
			theme.BorderRadius = req.BorderRadius
		}
	}

	// ThemeMode: "auto", "light", "dark"; empty string clears to auto
	if models.IsValidThemeMode(req.ThemeMode) {
		theme.ThemeMode = req.ThemeMode
	} else {
		theme.ThemeMode = ""
	}

	// CustomCSS: any string value sets it (including empty to clear)
	theme.CustomCSS = req.CustomCSS

	// Header title: allow setting to empty to use default
	theme.HeaderTitle = req.HeaderTitle
	if req.HeaderTitleHidden != nil {
		theme.HeaderTitleHidden = *req.HeaderTitleHidden
	}

	if err := h.db.Save(theme).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save theme settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"theme": theme})
}

// ProfilePanelContent returns the profile edit panel HTML for HTMX lazy loading
