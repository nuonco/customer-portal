package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) GetAppInputConfig(c *gin.Context) {
	appIDParam := c.Param("app_id")

	// Get optional filter parameter
	filterParam := c.Query("filter")

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	// Initialize Nuon client with the org's credentials and global API URL
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Fetch app details to get platform information
	app, err := nuonClient.GetApp(c.Request.Context(), appIDParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch app details: %v", err)})
		return
	}

	// Fetch app input configuration from Nuon API
	// Note: Input config may not exist for all apps - that's okay, we'll return empty config
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appIDParam)
	if err != nil {
		// Input config doesn't exist or failed to fetch - return empty config rather than error
		// This allows the form to render without inputs for apps that don't have input configs
		inputConfig = nil
	}

	// Apply filtering if requested
	// Convert typed struct to map for filtering (filterInputConfig expects map[string]interface{})
	if filterParam == "vendor" || filterParam == "customer" {
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				if filterParam == "vendor" {
					inputConfig = filterInputConfig(configMap, FilterTypeVendor)

					// Also exclude inputs marked as customer-facing in local config
					var localConfig models.AppInputConfig
					h.db.Where("org_id = ? AND app_id = ?", org.ID, appIDParam).First(&localConfig)
					customerInputNames := localConfig.GetCustomerInputNames()
					if len(customerInputNames) > 0 {
						jsonBytes, err := json.Marshal(inputConfig)
						if err == nil {
							var configMap map[string]interface{}
							if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
								inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeVendor)
							}
						}
					}
				} else {
					inputConfig = filterInputConfig(configMap, FilterTypeCustomer)
				}
			}
		}
	}

	// Extract platform from app runner config
	var platform string
	if app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}

	c.JSON(http.StatusOK, gin.H{
		"platform":     platform,
		"input_config": inputConfig,
	})
}

// DeleteInstallLink handles deleting an install link
