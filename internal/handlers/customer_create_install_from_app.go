package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) CreateInstallFromApp(c *gin.Context) {
	customer := h.tryGetLoggedInUser(c)
	if customer == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Please log in first"})
		return
	}

	appID := c.Param("app_id")

	var req struct {
		Name     string            `json:"name" binding:"required"`
		Region   string            `json:"region"`
		Location string            `json:"location"`
		Inputs   map[string]string `json:"inputs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Verify app is published for this org
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found or not published"})
		return
	}

	if publishedApp.Status == "coming_soon" {
		c.JSON(http.StatusForbidden, gin.H{"error": "This app is not yet available for installation"})
		return
	}

	region := req.Region
	location := req.Location
	if region == "" && location == "" {
		region = "us-east-1"
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to initialize Nuon client: %v", err)})
		return
	}

	// Get app name for the API call
	appName := appID
	app, appErr := nuonClient.GetApp(c.Request.Context(), appID)
	if appErr == nil && app != nil && app.Name != "" {
		appName = app.Name
	}

	// Merge in defaults for non-customer-facing inputs
	mergedInputs := h.mergeDefaultInputs(c.Request.Context(), nuonClient, appID, org.ID, req.Inputs)

	nuonInstall, err := nuonClient.CreateInstallWithCustomName(c.Request.Context(), appID, appName, req.Name, region, location, mergedInputs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create install via Nuon API: %v", err)})
		return
	}

	// Create local Install record with no install link
	install := &models.Install{
		OrgID:             org.ID,
		UserID:            customer.ID,
		CreatedByVendorID: nil, // no vendor for published-app installs
		InstallLinkID:     nil,
		NuonAppID:         appID,
		AppName:           appName,
		NuonInstallID:     nuonInstall.ID,
		Name:              req.Name,
		Status:            models.StatusPending,
		Region:            region,
		Visibility:        models.VisibilityAccount,
	}

	// Associate with customer's active account
	if activeMember := middleware.GetCustomerAccountMember(c); activeMember != nil {
		install.CustomerAccountID = &activeMember.AccountID
	} else {
		var members []models.CustomerAccountMember
		if err := h.db.Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", customer.ID, org.ID).Find(&members).Error; err == nil && len(members) > 0 {
			selected := middleware.SelectActiveMember(c, members)
			install.CustomerAccountID = &selected.AccountID
		}
	}

	if err := h.db.Create(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store install locally"})
		return
	}

	token, _, err := h.auth.TokenGenerator(customer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Install created successfully",
		"token":   token,
		"install": install,
	})
}
