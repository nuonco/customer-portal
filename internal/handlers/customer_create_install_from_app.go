package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/wizard"
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
		// Confirmed/Prepared arrive as strings ("true") from htmx json-enc;
		// they're parsed out of the raw map below to avoid bool-unmarshal errors.
		Confirmed bool `json:"-"`
		Prepared  bool `json:"-"`
	}

	if err := json.Unmarshal(bodyBytes, &req); err != nil || req.Name == "" {
		respondError(http.StatusBadRequest, "Name is required")
		return
	}

	// HTMX json-enc sends inputs as flat "inputs[name]" keys; extract them.
	// Also pick up the confirmed flag in string form ("true").
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err == nil {
		req.Inputs = extractBracketInputs(raw, req.Inputs)
		if v, ok := raw["confirmed"]; ok {
			if s, ok := v.(string); ok && s == "true" {
				req.Confirmed = true
			}
			if b, ok := v.(bool); ok {
				req.Confirmed = b
			}
		}
		if v, ok := raw["prepared"]; ok {
			if s, ok := v.(string); ok && s == "true" {
				req.Prepared = true
			}
			if b, ok := v.(bool); ok {
				req.Prepared = b
			}
		}
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

	// Shared builder for the Confirm step props — used both for the initial
	// confirm render and to re-render Confirm with an error banner if the
	// create call fails after the user has confirmed.
	buildConfirmProps := func(errMsg string) wizard.ConfirmStepProps {
		appNameForConfirm := appName
		if app != nil {
			if dn := appDisplayName(app); dn != "" {
				appNameForConfirm = dn
			}
		}
		var inputGroups []partials.InstallFormInputGroup
		if cfg, cfgErr := h.getPublishedAppFormData(c, appID); cfgErr == nil {
			inputGroups = cfg.toTemplConfig().InputGroups
		}
		return wizard.ConfirmStepProps{
			LayoutProps:     customerui.LayoutProps{NuonAPIError: errMsg},
			AppID:           appID,
			AppName:         appNameForConfirm,
			LogoLightBase64: publishedApp.LogoLightBase64,
			LogoDarkBase64:  publishedApp.LogoDarkBase64,
			InstallName:     req.Name,
			Region:          region,
			Location:        location,
			Inputs:          req.Inputs,
			InputGroups:     inputGroups,
			FormAction:      h.basePath + "/apps/" + appID + "/install",
			ConfigureURL:    h.basePath + "/apps/" + appID + "/install",
		}
	}

	// renderConfirmInWizard re-renders the Confirm step inside the wizard
	// shell, preserving step indicator and layout. Used after the user has
	// confirmed but the create call failed.
	renderConfirmInWizard := func(errMsg string) {
		confirmURL := buildAppConfirmURL(h.basePath, appID)
		if htmx {
			c.Header("HX-Retarget", "#wizard-step-wrapper")
			c.Header("HX-Reswap", "innerHTML")
			c.Header("HX-Push-Url", confirmURL)
		}
		h.RenderTempl(c, http.StatusOK, wizard.ConfirmStep(buildConfirmProps(errMsg)))
	}

	// First POST from the Configure step renders the Confirm step rather than
	// creating the install. The Confirm step's form re-POSTs with confirmed=true.
	if !req.Confirmed {
		confirmURL := buildAppConfirmURL(h.basePath, appID)
		if htmx {
			c.Header("HX-Retarget", "#wizard-step-wrapper")
			c.Header("HX-Reswap", "innerHTML")
			c.Header("HX-Push-Url", confirmURL)
		}
		h.RenderTempl(c, http.StatusOK, wizard.ConfirmStep(buildConfirmProps("")))
		return
	}

	// Confirm POST without prepared=true renders the Preparing step. The Preparing
	// step's auto-firing form re-POSTs with both confirmed=true and prepared=true,
	// which falls through to the actual install-creation logic below.
	if !req.Prepared {
		appNameForPreparing := appName
		if app != nil {
			if dn := appDisplayName(app); dn != "" {
				appNameForPreparing = dn
			}
		}
		preparingURL := buildAppPreparingURL(h.basePath, appID)
		configureURL := h.basePath + "/apps/" + appID + "/install"
		if htmx {
			c.Header("HX-Retarget", "#wizard-step-wrapper")
			c.Header("HX-Reswap", "innerHTML")
			c.Header("HX-Push-Url", preparingURL)
		}
		h.RenderTempl(c, http.StatusOK, wizard.PreparingStep(wizard.PreparingStepProps{
			AppID:           appID,
			AppName:         appNameForPreparing,
			LogoLightBase64: publishedApp.LogoLightBase64,
			LogoDarkBase64:  publishedApp.LogoDarkBase64,
			InstallName:     req.Name,
			Region:          region,
			Location:        location,
			Inputs:          req.Inputs,
			FormAction:      h.basePath + "/apps/" + appID + "/install",
			ConfigureURL:    configureURL,
		}))
		return
	}

	// Merge in defaults for non-customer-facing inputs
	mergedInputs := h.mergeDefaultInputs(c.Request.Context(), nuonClient, appID, org.ID, req.Inputs)

	nuonInstall, err := nuonClient.CreateInstallWithCustomName(c.Request.Context(), appID, appName, req.Name, region, location, platform, mergedInputs)
	if err != nil {
		if nuon.IsConflict(err) {
			if htmx {
				renderConfirmInWizard("An install with that name already exists. Please choose a different name.")
				return
			}
			respondError(http.StatusConflict, "An install with that name already exists. Please choose a different name.")
			return
		}
		msg := fmt.Sprintf("Failed to create install via Nuon API: %v", err)
		if htmx {
			renderConfirmInWizard(msg)
			return
		}
		respondError(http.StatusInternalServerError, msg)
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
		if htmx {
			renderConfirmInWizard("Failed to store install locally")
			return
		}
		respondError(http.StatusInternalServerError, "Failed to store install locally")
		return
	}

	token, _, err := h.auth.TokenGenerator(customer)
	if err != nil {
		if htmx {
			renderConfirmInWizard("Failed to generate authentication token")
			return
		}
		respondError(http.StatusInternalServerError, "Failed to generate authentication token")
		return
	}

	if htmx {
		basePath := h.basePath
		c.SetCookie("jwt", token, 86400, "/", "", false, false)
		wfID, _ := waitForWorkflowSteps(c.Request.Context(), nuonClient, nuonInstall.ID, 30*time.Second)
		if wfID == "" {
			renderConfirmInWizard("Install was created, but its provision workflow could not be reached. Please retry from the install page.")
			return
		}
		c.Header("HX-Redirect", buildWizardURL(basePath, appID, "stack", install.ID, wfID))
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Install created successfully",
		"token":   token,
		"install": install,
	})
}

// waitForWorkflowSteps polls until the install's first provision workflow has
// at least one step group with generated steps. Returns the workflow ID and
// whether step groups were observed before the timeout. If the workflow row
// itself never appears, returns an empty workflow ID.
func waitForWorkflowSteps(ctx context.Context, client *nuon.Client, installID string, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	tick := 500 * time.Millisecond

	var wfID string
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return wfID, false
		}
		wfs, _, err := client.GetInstallWorkflowsV2(ctx, installID, 0, 1)
		if err == nil && len(wfs) > 0 && wfs[0] != nil && wfs[0].ID != "" {
			wfID = wfs[0].ID
			break
		}
		time.Sleep(tick)
	}
	if wfID == "" {
		return "", false
	}

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return wfID, false
		}
		groups, err := client.GetWorkflowStepGroups(ctx, wfID)
		if err == nil {
			for _, g := range groups {
				if len(g.Steps) > 0 {
					return wfID, true
				}
			}
		}
		time.Sleep(tick)
	}
	return wfID, false
}
