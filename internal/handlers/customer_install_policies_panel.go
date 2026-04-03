package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
)

func (h *Handler) PoliciesPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	activeSubTab := c.DefaultQuery("sub", "policies")

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		h.RenderTempl(c, http.StatusOK, partials.PoliciesPanel(partials.PoliciesPanelProps{ActiveSubTab: activeSubTab}))
		return
	}

	apiClient, err := nuon.NewClientWithURL(
		nuonOrg.APIToken,
		nuonOrg.NuonOrgID,
		h.nuonAPIURLForOrg(nuonOrg),
	)
	if err != nil {
		zap.L().Warn("failed to create nuon client for policies panel", zap.Error(err))
		h.RenderTempl(c, http.StatusOK, partials.PoliciesPanel(partials.PoliciesPanelProps{ActiveSubTab: activeSubTab}))
		return
	}

	ctx := c.Request.Context()
	appID := install.GetAppID()
	installID := install.NuonInstallID

	props := partials.PoliciesPanelProps{
		ActiveSubTab: activeSubTab,
		BasePath:     h.basePath,
		InstallID:    install.ID,
	}

	policies, err := apiClient.GetLatestAppPoliciesConfigFull(ctx, appID)
	if err != nil {
		zap.L().Warn("failed to fetch policies config", zap.String("app_id", appID), zap.Error(err))
	} else {
		for _, p := range policies {
			props.Policies = append(props.Policies, partials.PoliciesPolicy{
				Name:     p.Name,
				Type:     p.Type,
				Engine:   p.Engine,
				Contents: p.Contents,
			})
		}
	}

	reports, err := apiClient.GetInstallPolicyReports(ctx, installID, "")
	if err != nil {
		zap.L().Warn("failed to fetch policy reports", zap.String("install_id", installID), zap.Error(err))
	} else {
		resolvePolicyReportNames(ctx, apiClient, appID, reports)
		props.PolicyReports = reports
	}

	h.RenderTempl(c, http.StatusOK, partials.PoliciesPanel(props))
}
