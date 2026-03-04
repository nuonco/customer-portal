package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

func (h *Handler) TestLoginConnection(c *gin.Context) {
	// Get the current config
	config, err := models.GetOrCreateCustomerAuthConfig(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load login settings"})
		return
	}

	// Check if config is complete enough to test
	if !config.IsConfigured() {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "OIDC is not fully configured. Please provide Client ID, Client Secret, and Issuer URL.",
		})
		return
	}

	// Try to create an OIDC provider to test the connection
	// This will perform OIDC discovery and validate the configuration
	providerConfig := auth.ProviderConfig{
		Type:         auth.ProviderTypeOIDC,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		IssuerURL:    config.IssuerURL,
		RedirectURI:  h.customerBaseURL + "/auth/callback",
		Scopes:       config.GetScopes(),
	}

	_, err = auth.NewOIDCProvider(providerConfig, h.db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to connect to OIDC provider: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully connected to OIDC provider",
	})
}

// GetAppCustomerInputConfig returns the local customer-facing input configuration for an app
