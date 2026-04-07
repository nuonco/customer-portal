package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

// SandboxRunDetailPanel renders sandbox run detail content for the sliding panel.
// Route: GET /installs/:install_id/panel/sandbox-run/:run_id
func (h *Handler) SandboxRunDetailPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}
	install := installInterface.(*models.Install)
	runID := c.Param("run_id")

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

	runs, err := apiClient.GetInstallSandboxRuns(ctx, installID)
	if err != nil {
		zap.L().Warn("failed to fetch sandbox runs", zap.Error(err))
		c.String(http.StatusInternalServerError, "Failed to fetch sandbox runs")
		return
	}

	var run *nuonmodels.AppInstallSandboxRun
	for _, r := range runs {
		if r.ID == runID {
			run = r
			break
		}
	}
	if run == nil {
		c.String(http.StatusNotFound, "Sandbox run not found")
		return
	}

	// Extract outputs from the run.
	var outputs map[string]string
	if outputMap, ok := run.Outputs.(map[string]interface{}); ok && len(outputMap) > 0 {
		outputs = make(map[string]string, len(outputMap))
		for k, v := range outputMap {
			switch s := v.(type) {
			case string:
				outputs[k] = prettyIfJSON(s)
			default:
				if b, err := json.MarshalIndent(s, "", "  "); err == nil {
					outputs[k] = string(b)
				} else {
					outputs[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

	// Fetch resources from the sandbox terraform workspace.
	props := partials.SandboxRunDetailProps{
		Run:     run,
		Outputs: outputs,
	}
	if nuonInstall, err := apiClient.GetInstall(ctx, installID); err == nil &&
		nuonInstall != nil && nuonInstall.Sandbox != nil &&
		nuonInstall.Sandbox.TerraformWorkspace != nil {
		workspaceID := nuonInstall.Sandbox.TerraformWorkspace.ID
		if states, err := apiClient.GetTerraformWorkspaceStates(ctx, workspaceID); err == nil && len(states) > 0 {
			if resources, err := apiClient.GetTerraformWorkspaceStateResources(ctx, workspaceID, states[0].ID); err == nil {
				props.Resources = resources
			} else {
				zap.L().Warn("failed to fetch terraform state resources", zap.Error(err))
			}
		} else if err != nil {
			zap.L().Warn("failed to fetch terraform workspace states", zap.Error(err))
		}
	}

	h.RenderTempl(c, http.StatusOK, partials.SandboxRunDetailPanel(props))
}
