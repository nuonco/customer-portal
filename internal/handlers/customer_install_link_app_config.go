package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/pkg/nuon"
)

func (h *Handler) GetInstallLinkAppConfig(c *gin.Context) {
	sha := c.Param("sha")
	if sha == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing SHA parameter"})
		return
	}

	// Get optional filter parameter
	filterParam := c.Query("filter")

	// Find install link with org
	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Where("sha = ?", sha).First(&link).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install link not found"})
		return
	}

	if link.Used {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Install link already used"})
		return
	}

	// Create Nuon client using org credentials
	nuonClient, err := nuon.NewClientWithURL(link.NuonOrg.APIToken, link.NuonOrg.NuonOrgID, h.nuonAPIURLForOrg(&link.NuonOrg))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize client"})
		return
	}

	// Fetch app details to get platform
	app, err := nuonClient.GetApp(c.Request.Context(), link.AppID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch app details"})
		return
	}

	// Extract platform from app runner config
	var platform string
	if app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}

	// Fetch app input config
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), link.AppID)
	if err != nil {
		// Input config may not exist, that's okay
		inputConfig = nil
	}

	// Fetch local customer input config for this app
	var localConfig models.AppInputConfig
	h.db.Where("org_id = ? AND app_id = ?", link.OrgID, link.AppID).First(&localConfig)
	customerInputNames := localConfig.GetCustomerInputNames()
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	// Apply filtering if requested
	// Convert typed struct to map for filtering
	if filterParam == "vendor" || filterParam == "customer" {
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				// Use local config for filtering
				if filterParam == "vendor" {
					inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeVendor)
				} else {
					inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeCustomer)
				}
				// Apply ordering
				inputConfig = applyInputOrdering(inputConfig, groupOrder, inputOrder)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"platform":         platform,
		"input_config":     inputConfig,
		"app_name":         link.AppName,
		"collapsed_groups": localConfig.GetCollapsedGroups(),
	})
}

// CustomerRootRedirect redirects to /installs for logged-in users, /apps for logged-out users.
