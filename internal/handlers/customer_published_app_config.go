package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/pkg/nuon"
)

func (h *Handler) GetPublishedAppConfig(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing app_id parameter"})
		return
	}

	filterParam := c.Query("filter")

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Verify app is published for this org
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ? AND status IN ?", org.ID, appID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).First(&publishedApp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found or not published"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize client"})
		return
	}

	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch app details"})
		return
	}

	var platform string
	if app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}

	appName := ""
	if app.Name != "" {
		appName = app.Name
	}

	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appID)
	if err != nil {
		inputConfig = nil
	}

	var localConfig models.AppInputConfig
	h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&localConfig)
	customerInputNames := localConfig.GetCustomerInputNames()
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	if filterParam == "vendor" || filterParam == "customer" {
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				if filterParam == "vendor" {
					inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeVendor)
				} else {
					inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeCustomer)
				}
				inputConfig = applyInputOrdering(inputConfig, groupOrder, inputOrder)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"platform":         platform,
		"input_config":     inputConfig,
		"app_name":         appName,
		"collapsed_groups": localConfig.GetCollapsedGroups(),
	})
}

// CustomerAppsPage renders the customer-facing app catalog

func (h *Handler) getOrgForCustomerPage(c *gin.Context) (*models.NuonOrg, error) {
	orgID := h.getOrgIDForTheme(c)
	if orgID == "" {
		return nil, fmt.Errorf("org not found for this subdomain")
	}
	var org models.NuonOrg
	if err := h.db.Where("id = ?", orgID).First(&org).Error; err != nil {
		return nil, fmt.Errorf("org not found: %w", err)
	}
	return &org, nil
}

// GetPublishedAppConfig returns app config for a published app (unauthenticated endpoint)
