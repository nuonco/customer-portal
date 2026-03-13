package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) CreateInstallFromApp(c *gin.Context) {
	htmx := isHTMXRequest(c)

	respondError := func(status int, msg string) {
		if htmx {
			h.RenderTempl(c, status, partials.InstallFormError(msg))
			return
		}
		c.JSON(status, gin.H{"error": msg})
	}

	customer := h.tryGetLoggedInUser(c)
	if customer == nil {
		respondError(http.StatusUnauthorized, "Please log in first")
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
		respondError(http.StatusBadRequest, err.Error())
		return
	}

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		respondError(http.StatusNotFound, "Organization not found")
		return
	}

	// Verify app is published for this org
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp).Error; err != nil {
		respondError(http.StatusNotFound, "App not found or not published")
		return
	}

	if publishedApp.Status == "coming_soon" {
		respondError(http.StatusForbidden, "This app is not yet available for installation")
		return
	}

	region := req.Region
	location := req.Location
	if region == "" && location == "" {
		region = "us-east-1"
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		respondError(http.StatusInternalServerError, fmt.Sprintf("Failed to initialize Nuon client: %v", err))
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
		respondError(http.StatusInternalServerError, fmt.Sprintf("Failed to create install via Nuon API: %v", err))
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
		respondError(http.StatusInternalServerError, "Failed to store install locally")
		return
	}

	token, _, err := h.auth.TokenGenerator(customer)
	if err != nil {
		respondError(http.StatusInternalServerError, "Failed to generate authentication token")
		return
	}

	if htmx {
		basePath := h.basePath
		c.SetCookie("jwt", token, 86400, "/", "", false, false)
		c.Header("HX-Redirect", basePath+"/installs/"+install.ID)
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Install created successfully",
		"token":   token,
		"install": install,
	})
}
