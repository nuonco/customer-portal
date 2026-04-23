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

func (h *Handler) AcceptInstallLink(c *gin.Context) {
	htmx := isHTMXRequest(c)

	respondError := func(status int, msg string) {
		if htmx {
			h.RenderTempl(c, http.StatusOK, partials.InstallFormError(msg))
			return
		}
		c.JSON(status, gin.H{"error": msg})
	}

	// REQUIRE authentication - user must be logged in first
	customer := h.tryGetLoggedInUser(c)
	if customer == nil {
		respondError(http.StatusUnauthorized, "Please log in first")
		return
	}

	// Read raw body so we can parse both nested and bracket-notation inputs
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondError(http.StatusBadRequest, "Failed to read request body")
		return
	}

	var req struct {
		SHA      string            `json:"sha" binding:"required"`
		Region   string            `json:"region"`
		Location string            `json:"location"`
		Inputs   map[string]string `json:"inputs"`
	}

	if err := json.Unmarshal(bodyBytes, &req); err != nil || req.SHA == "" {
		respondError(http.StatusBadRequest, "SHA is required")
		return
	}

	// HTMX json-enc sends inputs as flat "inputs[name]" keys; extract them
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err == nil {
		req.Inputs = extractBracketInputs(raw, req.Inputs)
	}

	// Find the install link
	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Where("sha = ?", req.SHA).First(&link).Error; err != nil {
		respondError(http.StatusNotFound, "Install link not found")
		return
	}

	if link.Used {
		respondError(http.StatusBadRequest, "Install link already used")
		return
	}

	// Get vendor inputs from link
	vendorInputs, err := link.GetVendorInputs()
	if err != nil {
		respondError(http.StatusInternalServerError, "Failed to read vendor inputs")
		return
	}

	// Merge vendor and customer inputs (customer inputs take precedence if overlap)
	mergedInputs := make(map[string]string)
	for k, v := range vendorInputs {
		mergedInputs[k] = v
	}
	for k, v := range req.Inputs {
		mergedInputs[k] = v
	}

	// Initialize Nuon client with the org's credentials
	nuonClient, err := nuon.NewClientWithURL(link.NuonOrg.APIToken, link.NuonOrg.NuonOrgID, h.nuonAPIURLForOrg(&link.NuonOrg))
	if err != nil {
		respondError(http.StatusInternalServerError, fmt.Sprintf("Failed to initialize Nuon client: %v", err))
		return
	}

	// Determine platform to decide region defaulting
	region := req.Region
	location := req.Location
	platform := ""
	if app, err := nuonClient.GetApp(c.Request.Context(), link.AppID); err == nil && app != nil && app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}
	if region == "" && location == "" {
		// Only default to us-east-1 for AWS; GCP installs don't need a region
		if platform != "gcp" && platform != "azure-aks" && platform != "azure-acs" && platform != "azure" {
			region = "us-east-1"
		}
	}

	// Merge in defaults for non-customer-facing inputs
	mergedInputs = h.mergeDefaultInputs(c.Request.Context(), nuonClient, link.AppID, link.OrgID, mergedInputs)

	// Use the vendor-provided install name from the link
	installName := link.Name

	// Create the install via Nuon API with merged inputs
	nuonInstall, err := nuonClient.CreateInstallWithCustomName(c.Request.Context(), link.AppID, link.AppName, installName, region, location, platform, mergedInputs)
	if err != nil {
		if nuon.IsConflict(err) {
			respondError(http.StatusConflict, "An install with that name already exists. Please choose a different name.")
			return
		}
		respondError(http.StatusInternalServerError, fmt.Sprintf("Failed to create install via Nuon API: %v", err))
		return
	}

	// Create local Install record with customer as owner
	linkID := link.ID
	install := &models.Install{
		OrgID:             link.OrgID,   // Link to vendor's org
		UserID:            customer.ID,  // Customer owns the install
		CreatedByVendorID: &link.UserID, // Track original vendor
		InstallLinkID:     &linkID,
		NuonInstallID:     nuonInstall.ID,
		Name:              installName,
		Status:            models.StatusPending,
		Region:            region,
		Visibility:        models.VisibilityAccount,
	}

	// Associate with customer's active account if they have one
	var members []models.CustomerAccountMember
	if err := h.db.Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", customer.ID, link.OrgID).Find(&members).Error; err == nil && len(members) > 0 {
		selected := middleware.SelectActiveMember(c, members)
		install.CustomerAccountID = &selected.AccountID
	}

	if err := h.db.Create(install).Error; err != nil {
		respondError(http.StatusInternalServerError, "Failed to store install locally")
		return
	}

	// Mark link as used
	link.Used = true
	if err := h.db.Save(&link).Error; err != nil {
		respondError(http.StatusInternalServerError, "Failed to update install link")
		return
	}

	// Generate JWT token for the customer (customer is already *models.User)
	token, _, err := h.auth.TokenGenerator(customer)
	if err != nil {
		respondError(http.StatusInternalServerError, "Failed to generate authentication token")
		return
	}

	if htmx {
		basePath := h.basePath
		c.SetCookie("jwt", token, 86400, "/", "", false, false)
		wfs, _, _ := nuonClient.GetInstallWorkflowsV2(c.Request.Context(), nuonInstall.ID, 0, 1)
		c.Header("HX-Redirect", buildWizardURL(basePath, link.AppID, "stack", install.ID, wfs[0].ID))
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Install accepted successfully",
		"token":   token,
		"install": install,
	})
}

// InstallsPage renders the customer installs page with tab-based pagination
