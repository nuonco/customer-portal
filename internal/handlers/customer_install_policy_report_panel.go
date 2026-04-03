package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
)

// PolicyReportPanel renders the detail view for a single policy evaluation report.
// Route: GET /installs/:install_id/panel/policies/report/:report_id
func (h *Handler) PolicyReportPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}
	install := installInterface.(*models.Install)
	reportID := c.Param("report_id")

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

	report, err := apiClient.GetPolicyReport(c.Request.Context(), reportID)
	if err != nil {
		zap.L().Warn("failed to fetch policy report detail", zap.String("report_id", reportID), zap.Error(err))
		c.String(http.StatusInternalServerError, "Failed to load policy report")
		return
	}

	h.RenderTempl(c, http.StatusOK, partials.PolicyReportDetailPanel(*report))
}
