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

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		h.RenderTempl(c, http.StatusOK, partials.PoliciesPanel(partials.PoliciesPanelProps{}))
		return
	}

	apiClient, err := nuon.NewClientWithURL(
		nuonOrg.APIToken,
		nuonOrg.NuonOrgID,
		h.nuonAPIURLForOrg(nuonOrg),
	)
	if err != nil {
		zap.L().Warn("failed to create nuon client for policies panel", zap.Error(err))
		h.RenderTempl(c, http.StatusOK, partials.PoliciesPanel(partials.PoliciesPanelProps{}))
		return
	}

	ctx := c.Request.Context()
	appID := install.GetAppID()

	policies, err := apiClient.GetLatestAppPoliciesConfigFull(ctx, appID)
	if err != nil {
		zap.L().Warn("failed to fetch policies config", zap.String("app_id", appID), zap.Error(err))
		h.RenderTempl(c, http.StatusOK, partials.PoliciesPanel(partials.PoliciesPanelProps{}))
		return
	}

	var displayPolicies []partials.PoliciesPolicy
	for _, p := range policies {
		displayPolicies = append(displayPolicies, partials.PoliciesPolicy{
			Name:     p.Name,
			Type:     p.Type,
			Engine:   p.Engine,
			Contents: p.Contents,
		})
	}

	h.RenderTempl(c, http.StatusOK, partials.PoliciesPanel(partials.PoliciesPanelProps{
		Policies: displayPolicies,
	}))
}
