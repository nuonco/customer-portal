package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
)

func (h *Handler) UpdateLoginSettings(c *gin.Context) {
	var req struct {
		Enabled                       bool   `json:"enabled"`
		ProviderName                  string `json:"provider_name"`
		ClientID                      string `json:"client_id"`
		ClientSecret                  string `json:"client_secret"`
		IssuerURL                     string `json:"issuer_url"`
		Scopes                        string `json:"scopes"`
		LoginTitle                    string `json:"login_title"`
		LoginSubtitle                 string `json:"login_subtitle"`
		LoginRightSideImageBase64     string `json:"login_right_side_image_base64"`
		LoginRightSideGradient        string `json:"login_right_side_gradient"`
		LoginRightSideImageBase64Dark string `json:"login_right_side_image_base64_dark"`
		LoginRightSideGradientDark    string `json:"login_right_side_gradient_dark"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get or create the customer auth config
	config, err := models.GetOrCreateCustomerAuthConfig(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load login settings"})
		return
	}

	// Update auth fields
	config.Enabled = req.Enabled
	config.ProviderName = req.ProviderName
	config.ClientID = req.ClientID
	config.IssuerURL = req.IssuerURL

	// Only update client secret if provided (not empty)
	// This allows keeping the existing secret when not changing it
	if req.ClientSecret != "" {
		config.ClientSecret = req.ClientSecret
	}

	// Update scopes, using default if empty
	if req.Scopes != "" {
		config.Scopes = req.Scopes
	} else {
		config.Scopes = models.DefaultScopes
	}

	if err := h.db.Save(config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save login settings"})
		return
	}

	// Update login page appearance in theme
	theme, err := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
	if err == nil {
		theme.LoginTitle = req.LoginTitle
		theme.LoginSubtitle = req.LoginSubtitle

		// Handle login right side image - "REMOVE" clears, valid data URI sets
		if req.LoginRightSideImageBase64 != "" {
			if req.LoginRightSideImageBase64 == "REMOVE" {
				theme.LoginRightSideImageBase64 = ""
			} else if strings.HasPrefix(req.LoginRightSideImageBase64, "data:image/") {
				theme.LoginRightSideImageBase64 = req.LoginRightSideImageBase64
			}
		}

		// Handle login right side gradient - always update (matches dark mode color pattern)
		// Valid CSS gradient sets it, empty string clears it
		if req.LoginRightSideGradient != "" && strings.HasPrefix(req.LoginRightSideGradient, "linear-gradient") {
			theme.LoginRightSideGradient = req.LoginRightSideGradient
		} else {
			// Clear gradient (falls back to primary/secondary colors)
			theme.LoginRightSideGradient = ""
		}

		// Handle dark mode login right side image
		if req.LoginRightSideImageBase64Dark != "" {
			if req.LoginRightSideImageBase64Dark == "REMOVE" {
				theme.LoginRightSideImageBase64Dark = ""
			} else if strings.HasPrefix(req.LoginRightSideImageBase64Dark, "data:image/") {
				theme.LoginRightSideImageBase64Dark = req.LoginRightSideImageBase64Dark
			}
		}

		// Handle dark mode login right side gradient
		if req.LoginRightSideGradientDark != "" && strings.HasPrefix(req.LoginRightSideGradientDark, "linear-gradient") {
			theme.LoginRightSideGradientDark = req.LoginRightSideGradientDark
		} else {
			theme.LoginRightSideGradientDark = ""
		}

		h.db.Save(theme)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config": gin.H{
			"id":                config.ID,
			"enabled":           config.Enabled,
			"provider_name":     config.ProviderName,
			"client_id":         config.ClientID,
			"issuer_url":        config.IssuerURL,
			"scopes":            config.Scopes,
			"has_client_secret": config.HasClientSecret(),
		},
	})
}

// TestLoginConnection tests the OIDC connection with current settings
