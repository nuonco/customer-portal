package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
)

// StackRunDetailPanel renders stack run detail content for the sliding panel.
// Route: GET /installs/:install_id/panel/stack-run/:run_id
func (h *Handler) StackRunDetailPanel(c *gin.Context) {
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

	runs, err := apiClient.GetInstallStackRuns(ctx, installID)
	if err != nil {
		zap.L().Warn("failed to fetch stack runs", zap.Error(err))
		c.String(http.StatusInternalServerError, "Failed to fetch stack runs")
		return
	}

	// Enrich runs with version status and template URL from the parent stack.
	var outputs map[string]string
	if stack, err := apiClient.GetInstallStack(ctx, installID); err == nil && stack != nil {
		versionStatus := make(map[string]string)
		versionTemplateURL := make(map[string]string)
		for _, v := range stack.Versions {
			if v.CompositeStatus != nil {
				versionStatus[v.ID] = string(v.CompositeStatus.Status)
			}
			if v.TemplateURL != "" {
				versionTemplateURL[v.ID] = v.TemplateURL
			}
		}
		for i := range runs {
			if s, ok := versionStatus[runs[i].InstallStackVersionID]; ok {
				runs[i].VersionStatus = s
			}
			if u, ok := versionTemplateURL[runs[i].InstallStackVersionID]; ok {
				runs[i].TemplateURL = u
			}
		}

		// Extract stack outputs — prefer DataContents for nested JSON values.
		if stack.InstallStackOutputs != nil {
			if dc, ok := stack.InstallStackOutputs.DataContents.(map[string]interface{}); ok && len(dc) > 0 {
				outputs = make(map[string]string, len(dc))
				for k, v := range dc {
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
			} else if len(stack.InstallStackOutputs.Data) > 0 {
				outputs = stack.InstallStackOutputs.Data
			}
		}
	} else if err != nil {
		zap.L().Warn("failed to fetch install stack", zap.Error(err))
	}

	var run nuon.StackRun
	found := false
	for _, r := range runs {
		if r.ID == runID {
			run = r
			found = true
			break
		}
	}
	if !found {
		c.String(http.StatusNotFound, "Stack run not found")
		return
	}

	h.RenderTempl(c, http.StatusOK, partials.StackRunDetailPanel(partials.StackRunDetailProps{
		Run:     run,
		Outputs: outputs,
	}))
}
