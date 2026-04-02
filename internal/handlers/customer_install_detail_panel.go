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
	nuonmodels "github.com/nuonco/nuon-go/models"
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
			h.nuonAPIURLForOrg(nuonOrg),
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
			h.nuonAPIURLForOrg(nuonOrg),
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
	var stackSetup partials.StackSetupData
	var provisionPhases []partials.ProvisionPhase
	var hasActiveProvision bool
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		provClient, provErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if provErr == nil {
			ctx := c.Request.Context()
			bestWf, bestPanel := h.findMostRecentProvisionWorkflow(ctx, provClient, install.NuonInstallID)
			if bestPanel != nil {
				provisionWorkflow = bestPanel
				if bestWf != nil {
					if isActiveWorkflowStatus(provisionWorkflow.Status) {
						stackSetup = h.getStackSetupData(ctx, provClient, install, bestWf)
					}
					if isActiveWorkflowStatus(provisionWorkflow.Status) || provisionWorkflow.Status == "error" {
						provisionPhases = groupStepsIntoPhases(bestWf, provisionWorkflow.IsReprovision)
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

	// Fetch app logo from PublishedApp
	var appLogoLight, appLogoDark string
	var pa models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", install.OrgID, install.GetAppID()).First(&pa).Error; err == nil {
		appLogoLight = pa.LogoLightBase64
		appLogoDark = pa.LogoDarkBase64
	}

	// Fetch stack and sandbox info for overview cards
	var stackInfo *partials.StackInfo
	var latestStackRuns []nuon.StackRun
	var sandboxInfo *partials.SandboxInfo
	var latestSandboxRuns []*nuonmodels.AppInstallSandboxRun
	var componentInfos []partials.ComponentInfo
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		apiClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
		if err == nil {
			ctx := c.Request.Context()
			stackInfo = h.buildStackInfo(ctx, apiClient, install)
			sandboxInfo = h.buildSandboxInfo(ctx, apiClient, install)
			if comps, err := apiClient.GetInstallComponents(ctx, install.NuonInstallID); err == nil {
				componentInfos = h.buildComponentInfos(ctx, apiClient, install, comps)
			}
			if runs, err := apiClient.GetInstallStackRuns(ctx, install.NuonInstallID); err == nil && len(runs) > 0 {
				// Populate VersionStatus from the stack version if available
				if stack, err := apiClient.GetInstallStack(ctx, install.NuonInstallID); err == nil && stack != nil && len(stack.Versions) > 0 {
					for i := range runs {
						for _, v := range stack.Versions {
							if runs[i].InstallStackVersionID == v.ID && v.CompositeStatus != nil {
								runs[i].VersionStatus = string(v.CompositeStatus.Status)
							}
						}
					}
				}
				if len(runs) > 5 {
					runs = runs[:5]
				}
				latestStackRuns = runs
			}
			if runs, err := apiClient.GetInstallSandboxRuns(ctx, install.NuonInstallID); err == nil && len(runs) > 0 {
				if len(runs) > 5 {
					runs = runs[:5]
				}
				latestSandboxRuns = runs
			}
		}
	}

	props := partials.InstallDetailPanelProps{
		Install:                install,
		AppName:                appName,
		AppLogoLightBase64:     appLogoLight,
		AppLogoDarkBase64:      appLogoDark,
		APIDeletedError:        apiDeletedError,
		BasePath:               h.basePath,
		PrimaryColor:           primaryColor,
		AppConfigVersion:       appConfigVersion,
		AppConfigUpdatedAt:     appConfigUpdatedAt,
		InstallConfigVersion:   installConfigVersion,
		InstallConfigUpdatedAt: installConfigUpdatedAt,
		ProvisionWorkflow:      provisionWorkflow,
		StackSetup:             stackSetup,
		ProvisionPhases:        provisionPhases,
		HasActiveProvision:     hasActiveProvision,
		StackInfo:              stackInfo,
		LatestStackRuns:        latestStackRuns,
		SandboxInfo:            sandboxInfo,
		LatestSandboxRuns:      latestSandboxRuns,
		Components:             componentInfos,
	}
	h.RenderTempl(c, http.StatusOK, partials.InstallDetailPanel(props))
}

// buildComponentInfos builds display info for each install component, including repo data from the app config.
func (h *Handler) buildComponentInfos(ctx context.Context, apiClient *nuon.Client, install *models.Install, comps []*nuonmodels.AppInstallComponent) []partials.ComponentInfo {
	// Build a map of component_id → config connection from the app config
	ccMap := map[string]*nuonmodels.AppComponentConfigConnection{}
	app, err := apiClient.GetApp(ctx, install.GetAppID())
	if err == nil && app != nil && len(app.AppConfigs) > 0 {
		appConfigID := app.AppConfigs[0].ID
		if cfg, err := apiClient.GetAppConfigFull(ctx, install.GetAppID(), appConfigID); err == nil && cfg != nil {
			for _, cc := range cfg.ComponentConfigConnections {
				if cc != nil && cc.ComponentID != "" {
					ccMap[cc.ComponentID] = cc
				}
			}
		}
	}

	var infos []partials.ComponentInfo
	for _, comp := range comps {
		ci := partials.ComponentInfo{
			Status: comp.Status,
			Name:   "Component",
		}
		if comp.Component != nil {
			if comp.Component.Name != "" {
				ci.Name = comp.Component.Name
			}
			ci.Type = string(comp.Component.Type)
		}

		// Extract VCS info from the component config connection
		if cc, ok := ccMap[comp.ComponentID]; ok {
			type vcsSource struct {
				Public *nuonmodels.AppPublicGitVCSConfig
				GitHub *nuonmodels.AppConnectedGithubVCSConfig
			}
			var vcs vcsSource
			switch {
			case cc.TerraformModule != nil:
				vcs.Public = cc.TerraformModule.PublicGitVcsConfig
				vcs.GitHub = cc.TerraformModule.ConnectedGithubVcsConfig
			case cc.Helm != nil:
				vcs.Public = cc.Helm.PublicGitVcsConfig
				vcs.GitHub = cc.Helm.ConnectedGithubVcsConfig
			case cc.DockerBuild != nil:
				vcs.Public = cc.DockerBuild.PublicGitVcsConfig
				vcs.GitHub = cc.DockerBuild.ConnectedGithubVcsConfig
			}
			if vcs.Public != nil {
				ci.Repo = vcs.Public.Repo
				ci.Directory = vcs.Public.Directory
				ci.Branch = vcs.Public.Branch
				ci.RepoPublic = true
			} else if vcs.GitHub != nil {
				ci.Repo = vcs.GitHub.Repo
				ci.Directory = vcs.GitHub.Directory
				ci.Branch = vcs.GitHub.Branch
			}
		}
		infos = append(infos, ci)
	}
	return infos
}

// InstallWorkflowStatus returns the active provision workflow banner for HTMX polling
// This endpoint is called by HTMX polling every 5 seconds
// It must return HTML (ProvisionAccordion or empty polling div) for HTMX to swap
// Authentication is handled by JWT middleware which returns HX-Trigger: auth-error on failure
