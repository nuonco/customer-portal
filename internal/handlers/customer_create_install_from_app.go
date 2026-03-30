package handlers

import (
	"encoding/json"
	"fmt"
	"io"
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
			h.RenderTempl(c, http.StatusOK, partials.InstallFormError(msg))
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

	// Read raw body so we can parse both nested and bracket-notation inputs
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondError(http.StatusBadRequest, "Failed to read request body")
		return
	}

	var req struct {
		Name     string            `json:"name" binding:"required"`
		Region   string            `json:"region"`
		Location string            `json:"location"`
		Inputs   map[string]string `json:"inputs"`
	}

	if err := json.Unmarshal(bodyBytes, &req); err != nil || req.Name == "" {
		respondError(http.StatusBadRequest, "Name is required")
		return
	}

	// HTMX json-enc sends inputs as flat "inputs[name]" keys; extract them
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err == nil {
		req.Inputs = extractBracketInputs(raw, req.Inputs)
	}

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		respondError(http.StatusNotFound, "Organization not found")
		return
	}

	// Verify app is published for this org
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ? AND status IN ?", org.ID, appID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).First(&publishedApp).Error; err != nil {
		respondError(http.StatusNotFound, "App not found or not published")
		return
	}

	if publishedApp.Status == "coming_soon" {
		respondError(http.StatusForbidden, "This app is not yet available for installation")
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		respondError(http.StatusInternalServerError, fmt.Sprintf("Failed to initialize Nuon client: %v", err))
		return
	}

	// Get app name and platform for the API call
	appName := appID
	platform := ""
	app, appErr := nuonClient.GetApp(c.Request.Context(), appID)
	if appErr == nil && app != nil {
		if app.Name != "" {
			appName = app.Name
		}
		if app.RunnerConfig != nil {
			platform = string(app.RunnerConfig.AppRunnerType)
		}
	}

	// Only default to us-east-1 for AWS; GCP installs don't need a region
	region := req.Region
	location := req.Location
	if region == "" && location == "" && platform != "gcp" && platform != "azure-aks" && platform != "azure-acs" && platform != "azure" {
		region = "us-east-1"
	}

	// Merge in defaults for non-customer-facing inputs
	mergedInputs := h.mergeDefaultInputs(c.Request.Context(), nuonClient, appID, org.ID, req.Inputs)

	nuonInstall, err := nuonClient.CreateInstallWithCustomName(c.Request.Context(), appID, appName, req.Name, region, location, platform, mergedInputs)
	if err != nil {
		if nuon.IsConflict(err) {
			respondError(http.StatusConflict, "An install with that name already exists. Please choose a different name.")
			return
		}
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
