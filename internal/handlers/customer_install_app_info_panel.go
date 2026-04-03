package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/markdown"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) AppInfoPanel(c *gin.Context) {
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

	var nuonClient *nuon.Client
	var nuonClientErr error
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		nuonClient, nuonClientErr = nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	}

	display := h.buildAppDisplay(c, install.GetAppID(), install.OrgID, nuonClient, nuonClientErr)

	// Merge logo and readme from PublishedApp
	var pa models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", install.OrgID, install.GetAppID()).First(&pa).Error; err == nil {
		display.LogoLightBase64 = pa.LogoLightBase64
		display.LogoDarkBase64 = pa.LogoDarkBase64
		if pa.OverviewMarkdown != "" {
			if html, err := markdown.Render([]byte(pa.OverviewMarkdown)); err == nil {
				display.ReadmeHTML = html
			}
		}
	}

	h.RenderTempl(c, http.StatusOK, partials.AppInfoCard(display, h.basePath))
}
