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
	var nuonAPIError string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		checkClient, checkErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if checkErr != nil {
			nuonAPIError = "The app is experiencing network issues, and is not able to access app or install data. Please contact support for assistance."
		} else {
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
				} else {
					nuonAPIError = "The app is experiencing network issues, and is not able to access app or install data. Please contact support for assistance."
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

	// Fetch overview summary card data (stack, sandbox, components)
	var stackStatus, stackRegion, stackAccountID string
	var sandboxStatus, sandboxRepo, sandboxBranch string
	var sandboxRepoPublic bool
	var componentInfos []partials.ComponentInfo
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		summaryClient, summaryErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if summaryErr == nil {
			ctx := c.Request.Context()

			// Stack summary
			if stack, err := summaryClient.GetInstallStack(ctx, install.NuonInstallID); err == nil && stack != nil {
				if len(stack.Versions) > 0 && stack.Versions[0].CompositeStatus != nil {
					stackStatus = string(stack.Versions[0].CompositeStatus.Status)
				}
				if outputs := stack.InstallStackOutputs; outputs != nil && outputs.Aws != nil {
					stackRegion = outputs.Aws.Region
					stackAccountID = outputs.Aws.AccountID
				}
			}

			// Sandbox summary
			if nuonInst, err := summaryClient.GetInstall(ctx, install.NuonInstallID); err == nil && nuonInst != nil && nuonInst.Sandbox != nil {
				sandboxStatus = nuonInst.Sandbox.Status
			}
			if cfg, err := summaryClient.GetAppSandboxLatestConfig(ctx, install.GetAppID()); err == nil && cfg != nil {
				if gh := cfg.ConnectedGithubVcsConfig; gh != nil {
					sandboxRepo = gh.Repo
					sandboxBranch = gh.Branch
				} else if pg := cfg.PublicGitVcsConfig; pg != nil {
					sandboxRepo = pg.Repo
					sandboxBranch = pg.Branch
					sandboxRepoPublic = true
				}
			}

			// Components summary
			if comps, err := summaryClient.GetInstallComponents(ctx, install.NuonInstallID); err == nil {
				componentInfos = h.buildComponentInfos(ctx, summaryClient, install, comps)
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
		StackStatus:            stackStatus,
		StackRegion:            stackRegion,
		StackAccountID:         stackAccountID,
		SandboxStatus:          sandboxStatus,
		SandboxRepo:            sandboxRepo,
		SandboxBranch:          sandboxBranch,
		SandboxRepoPublic:      sandboxRepoPublic,
		Components:             componentInfos,
		NuonAPIError:           nuonAPIError,
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
		status := comp.Status
		if comp.StatusV2 != nil && comp.StatusV2.Status != "" {
			status = string(comp.StatusV2.Status)
		}
		if status == "" && len(comp.InstallDeploys) > 0 {
			latest := comp.InstallDeploys[0]
			if latest.StatusV2 != nil && latest.StatusV2.Status != "" {
				status = string(latest.StatusV2.Status)
			} else if latest.Status != "" {
				status = latest.Status
			}
		}
		ci := partials.ComponentInfo{
			Status: status,
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
