package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

// ComponentDetailPanel renders component detail content for the sliding panel.
// Route: GET /installs/:install_id/panel/component/:component_id
func (h *Handler) ComponentDetailPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}
	install := installInterface.(*models.Install)
	componentID := c.Param("component_id")

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		c.String(http.StatusInternalServerError, "No API credentials")
		return
	}

	apiClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create API client")
		return
	}

	ctx := c.Request.Context()
	installID := install.NuonInstallID

	components, err := apiClient.GetInstallComponents(ctx, installID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch components")
		return
	}

	var comp *nuonmodels.AppInstallComponent
	for _, ic := range components {
		if ic.ComponentID == componentID {
			comp = ic
			break
		}
	}
	if comp == nil {
		c.String(http.StatusNotFound, "Component not found")
		return
	}

	// Build VCS info and collect the component config connection in one app config fetch.
	compInfos := h.buildComponentInfos(ctx, apiClient, install, []*nuonmodels.AppInstallComponent{comp})
	var compInfo partials.ComponentInfo
	if len(compInfos) > 0 {
		compInfo = compInfos[0]
	}

	// Fetch the component config connection for type-specific data (values, variables, manifest, etc.).
	var cc *nuonmodels.AppComponentConfigConnection
	if app, err := apiClient.GetApp(ctx, install.GetAppID()); err == nil && app != nil && len(app.AppConfigs) > 0 {
		if cfg, err := apiClient.GetAppConfigFull(ctx, install.GetAppID(), app.AppConfigs[0].ID); err == nil && cfg != nil {
			for _, conn := range cfg.ComponentConfigConnections {
				if conn != nil && conn.ComponentID == componentID {
					cc = conn
					break
				}
			}
		}
	}

	var build *nuonmodels.AppComponentBuild
	if b, err := apiClient.GetAppComponentLatestBuild(ctx, install.GetAppID(), componentID); err == nil {
		build = b
	} else {
		zap.L().Warn("failed to fetch latest component build", zap.Error(err))
	}

	// Render the type-specific panel.
	switch {
	case cc != nil && cc.TerraformModule != nil:
		props := partials.TerraformComponentDetailProps{
			Component: comp,
			Info:      compInfo,
			Build:     build,
			Variables: cc.TerraformModule.Variables,
		}
		if comp.TerraformWorkspace != nil && comp.TerraformWorkspace.ID != "" {
			workspaceID := comp.TerraformWorkspace.ID
			if states, err := apiClient.GetTerraformWorkspaceStates(ctx, workspaceID); err == nil && len(states) > 0 {
				stateID := states[0].ID
				if o, err := apiClient.GetTerraformWorkspaceStateOutputs(ctx, workspaceID, stateID); err == nil {
					props.Outputs = o
				} else {
					zap.L().Warn("failed to fetch terraform state outputs", zap.Error(err))
				}
				if resources, err := apiClient.GetTerraformWorkspaceStateResources(ctx, workspaceID, stateID); err == nil {
					props.Resources = resources
				} else {
					zap.L().Warn("failed to fetch terraform state resources", zap.Error(err))
				}
			} else if err != nil {
				zap.L().Warn("failed to fetch terraform workspace states", zap.Error(err))
			}
		}
		h.RenderTempl(c, http.StatusOK, partials.TerraformComponentDetailPanel(props))

	default:
		// For all non-terraform types, fetch deploy outputs from the dedicated endpoint.
		var outputs map[string]string
		if o, err := apiClient.GetInstallComponentOutputs(ctx, installID, componentID); err == nil {
			outputs = o
		} else {
			zap.L().Warn("failed to fetch component outputs", zap.Error(err))
		}

		switch {
		case cc != nil && cc.Helm != nil:
			h.RenderTempl(c, http.StatusOK, partials.HelmComponentDetailPanel(partials.HelmComponentDetailProps{
				Component:   comp,
				Info:        compInfo,
				Build:       build,
				Outputs:     outputs,
				Values:      cc.Helm.Values,
				ValuesFiles: cc.Helm.ValuesFiles,
				Namespace:   cc.Helm.Namespace,
				ChartName:   cc.Helm.ChartName,
			}))

		case cc != nil && cc.DockerBuild != nil:
			h.RenderTempl(c, http.StatusOK, partials.DockerComponentDetailPanel(partials.DockerComponentDetailProps{
				Component: comp,
				Info:      compInfo,
				Build:     build,
				Outputs:   outputs,
				EnvVars:   cc.DockerBuild.EnvVars,
			}))

		case cc != nil && cc.ExternalImage != nil:
			h.RenderTempl(c, http.StatusOK, partials.ExternalImageComponentDetailPanel(partials.ExternalImageComponentDetailProps{
				Component: comp,
				Info:      compInfo,
				Build:     build,
				Outputs:   outputs,
				ImageURL:  cc.ExternalImage.ImageURL,
				ImageTag:  cc.ExternalImage.Tag,
			}))

		case cc != nil && cc.Job != nil:
			h.RenderTempl(c, http.StatusOK, partials.JobComponentDetailPanel(partials.JobComponentDetailProps{
				Component: comp,
				Info:      compInfo,
				Build:     build,
				Outputs:   outputs,
				EnvVars:   cc.Job.EnvVars,
				ImageURL:  cc.Job.ImageURL,
				ImageTag:  cc.Job.Tag,
			}))

		case cc != nil && cc.KubernetesManifest != nil:
			h.RenderTempl(c, http.StatusOK, partials.KubernetesComponentDetailPanel(partials.KubernetesComponentDetailProps{
				Component: comp,
				Info:      compInfo,
				Build:     build,
				Outputs:   outputs,
				Manifest:  cc.KubernetesManifest.Manifest,
			}))

		default:
			// Unknown component type — show outputs only.
			h.RenderTempl(c, http.StatusOK, partials.TerraformComponentDetailPanel(partials.TerraformComponentDetailProps{
				Component: comp,
				Info:      compInfo,
				Build:     build,
				Outputs:   outputs,
			}))
		}
	}
}
