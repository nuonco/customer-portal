package handlers

import (
	"context"

	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/views/customerui/theme/partials"
	"github.com/nuonco/customer-portal/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon/sdks/nuon-go/models"
)

// mergeDeployStatuses fills in any missing ComponentInfo.Status values by
// looking up the latest deploy for each component, falling back to
// "not deployed" when no deploy exists. Mirrors the audit panel logic so
// the overview and components pages agree on per-component status.
func (h *Handler) mergeDeployStatuses(ctx context.Context, apiClient *nuon.Client, installID string, comps []*nuonmodels.AppInstallComponent, infos []partials.ComponentInfo) {
	deployStatus := make(map[string]string)
	if deploys, err := apiClient.GetInstallDeploys(ctx, installID); err == nil {
		for _, d := range deploys {
			if d.ComponentID == "" {
				continue
			}
			if _, exists := deployStatus[d.ComponentID]; exists {
				continue
			}
			if d.StatusV2 != nil && d.StatusV2.Status != "" {
				deployStatus[d.ComponentID] = string(d.StatusV2.Status)
			} else if d.Status != "" {
				deployStatus[d.ComponentID] = d.Status
			}
		}
	}
	for i := range infos {
		if infos[i].Status != "" || i >= len(comps) {
			continue
		}
		if s, ok := deployStatus[comps[i].ComponentID]; ok {
			infos[i].Status = s
		} else {
			infos[i].Status = "not deployed"
		}
	}
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
