package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) GetOrgApps(c *gin.Context) {
	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	// Initialize Nuon client with the org's credentials and global API URL
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Fetch apps from Nuon API
	apps, err := nuonClient.ListApps(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch apps: %v", err)})
		return
	}

	c.JSON(http.StatusOK, apps)
}

// GetAppInputConfig fetches app input configuration for dynamic form rendering
// Accepts optional ?filter=vendor|customer query parameter to filter inputs:
// - filter=vendor: returns inputs where user_configurable != true
// - filter=customer: returns inputs where user_configurable == true
// - no filter: returns all inputs (backward compatibility)
