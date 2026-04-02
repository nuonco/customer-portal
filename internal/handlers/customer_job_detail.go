package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
)

// JobDetailPanel renders job detail content for the sliding panel.
// Routes: GET /installs/:install_id/panel/job/:job_type/:job_id
func (h *Handler) JobDetailPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}
	install := installInterface.(*models.Install)
	jobType := c.Param("job_type")
	jobID := c.Param("job_id")

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
	props := partials.JobDetailProps{JobType: jobType}

	switch jobType {
	case "stack":
		runs, err := apiClient.GetInstallStackRuns(ctx, installID)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to fetch stack runs")
			return
		}
		if stack, err := apiClient.GetInstallStack(ctx, installID); err == nil && stack != nil {
			versionStatus := make(map[string]string)
			for _, v := range stack.Versions {
				if v.CompositeStatus != nil {
					versionStatus[v.ID] = string(v.CompositeStatus.Status)
				}
			}
			for i := range runs {
				if s, ok := versionStatus[runs[i].InstallStackVersionID]; ok {
					runs[i].VersionStatus = s
				}
			}
			// Collect stack outputs
			if stack.InstallStackOutputs != nil && len(stack.InstallStackOutputs.Data) > 0 {
				props.StackOutputs = stack.InstallStackOutputs.Data
			}
		}
		for i := range runs {
			if runs[i].ID == jobID {
				props.StackRun = &runs[i]
				break
			}
		}
	case "sandbox":
		runs, err := apiClient.GetInstallSandboxRuns(ctx, installID)
		if err != nil {
			zap.L().Warn("failed to fetch sandbox runs", zap.Error(err))
			c.String(http.StatusInternalServerError, "Failed to fetch sandbox runs")
			return
		}
		for _, run := range runs {
			if run.ID == jobID {
				props.SandboxRun = run
				break
			}
		}
	case "deploy":
		deploys, err := apiClient.GetInstallDeploys(ctx, installID)
		if err != nil {
			zap.L().Warn("failed to fetch deploys", zap.Error(err))
			c.String(http.StatusInternalServerError, "Failed to fetch deploys")
			return
		}
		for _, deploy := range deploys {
			if deploy.ID == jobID {
				props.Deploy = deploy
				break
			}
		}
	case "action":
		workflow, err := apiClient.GetWorkflow(ctx, jobID)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to fetch workflow")
			return
		}
		props.Workflow = workflow
	default:
		c.String(http.StatusBadRequest, "Unknown job type")
		return
	}

	h.RenderTempl(c, http.StatusOK, partials.JobDetailPanel(props))
}
