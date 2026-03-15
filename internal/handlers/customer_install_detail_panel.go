package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"github.com/nuonco/nuon-go/client/operations"
	"go.uber.org/zap"
)

func (h *Handler) InstallDetailPanel(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with both InstallLink.NuonOrg and Org (handles published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()

	// Check if install still exists in Nuon API
	var apiDeletedError bool
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		checkClient, checkErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if checkErr == nil {
			_, apiErr := checkClient.GetInstall(context.Background(), install.NuonInstallID)
			if apiErr != nil {
				// Check if error is specifically a 404 NotFound
				var notFoundErr *operations.GetInstallNotFound
				if errors.As(apiErr, &notFoundErr) {
					apiDeletedError = true
					h.logger.Warn("install deleted from API but exists locally",
						zap.String("install_id", install.ID),
						zap.String("nuon_install_id", install.NuonInstallID),
					)
					if !install.APIDeleted {
						h.db.Model(install).Update("api_deleted", true)
					}
				}
			} else if install.APIDeleted {
				h.db.Model(install).Update("api_deleted", false)
			}
		}
	}

	// Fetch app config version info and app name
	var appName string
	var appConfigVersion int64
	var appConfigUpdatedAt string
	var installConfigVersion int64
	var installConfigUpdatedAt string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		appClient, err := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if err == nil {
			app, err := appClient.GetApp(context.Background(), install.GetAppID())
			if err == nil && app != nil {
				appName = appDisplayName(app)
				if len(app.AppConfigs) > 0 {
					appConfigVersion = app.AppConfigs[0].Version
					appConfigUpdatedAt = app.AppConfigs[0].UpdatedAt

					// Get install's current config version by matching AppConfigID
					nuonInstall, instErr := appClient.GetInstall(context.Background(), install.NuonInstallID)
					if instErr == nil && nuonInstall.AppConfigID != "" {
						for _, cfg := range app.AppConfigs {
							if cfg.ID == nuonInstall.AppConfigID {
								installConfigVersion = cfg.Version
								installConfigUpdatedAt = cfg.UpdatedAt
								break
							}
						}
					}
				}
			}
		}
	}

	// Fetch most recent provision workflow (any status)
	var provisionWorkflow *partials.WorkflowDataPanel
	var cloudFormationLink string
	var provisionPhases []partials.ProvisionPhase
	var hasActiveProvision bool
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		provClient, provErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if provErr == nil {
			ctx := c.Request.Context()
			bestWf, bestPanel := h.findMostRecentProvisionWorkflow(ctx, provClient, install.NuonInstallID)
			if bestPanel != nil {
				provisionWorkflow = bestPanel
				if bestWf != nil {
					if isActiveWorkflowStatus(provisionWorkflow.Status) {
						cloudFormationLink = h.getCloudFormationLink(ctx, provClient, install, bestWf)
					}
					if isActiveWorkflowStatus(provisionWorkflow.Status) || provisionWorkflow.Status == "error" {
						provisionPhases = groupStepsIntoPhases(bestWf)
						hasActiveProvision = len(provisionPhases) > 0
					}
				}
			}
		}
	}

	// If the most recent provision workflow reached a terminal state, transition install status
	if provisionWorkflow != nil && (install.Status == models.StatusPending || install.Status == models.StatusProvisioning) {
		switch provisionWorkflow.Status {
		case "completed", "success":
			h.db.Model(install).Update("status", models.StatusActive)
			install.Status = models.StatusActive
		case "cancelled":
			h.db.Model(install).Update("status", models.StatusFailed)
			install.Status = models.StatusFailed
		}
	}

	// If install is pending and no active provision accordion, show placeholder
	if install.Status == models.StatusPending && !hasActiveProvision {
		provisionWorkflow = &partials.WorkflowDataPanel{
			Name:            "Provision",
			Status:          "pending",
			StatusClass:     "badge-neutral",
			CurrentStepName: "Preparing to provision",
			CurrentStepType: "initializing",
		}
		provisionPhases = []partials.ProvisionPhase{
			{Name: "Install stack", Status: "not_started"},
			{Name: "Provision sandbox", Status: "not_started"},
			{Name: "Deploy app", Status: "not_started"},
		}
		hasActiveProvision = true
	}

	// Get theme colors
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)

	props := partials.InstallDetailPanelProps{
		Install:                install,
		AppName:                appName,
		APIDeletedError:        apiDeletedError,
		BasePath:               h.basePath,
		PrimaryColor:           primaryColor,
		AppConfigVersion:       appConfigVersion,
		AppConfigUpdatedAt:     appConfigUpdatedAt,
		InstallConfigVersion:   installConfigVersion,
		InstallConfigUpdatedAt: installConfigUpdatedAt,
		ProvisionWorkflow:      provisionWorkflow,
		CloudFormationLink:     cloudFormationLink,
		ProvisionPhases:        provisionPhases,
		HasActiveProvision:     hasActiveProvision,
	}
	h.RenderTempl(c, http.StatusOK, partials.InstallDetailPanel(props))
}

// InstallWorkflowStatus returns the active provision workflow banner for HTMX polling
// This endpoint is called by HTMX polling every 5 seconds
// It must return HTML (ProvisionAccordion or empty polling div) for HTMX to swap
// Authentication is handled by JWT middleware which returns HX-Trigger: auth-error on failure
