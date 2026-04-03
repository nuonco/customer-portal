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

	// Find the install component matching the component ID
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

	// Build VCS info using existing helper
	compInfos := h.buildComponentInfos(ctx, apiClient, install, []*nuonmodels.AppInstallComponent{comp})
	var compInfo partials.ComponentInfo
	if len(compInfos) > 0 {
		compInfo = compInfos[0]
	}

	// Fetch latest build for commit/VCS details
	var build *nuonmodels.AppComponentBuild
	if b, err := apiClient.GetAppComponentLatestBuild(ctx, install.GetAppID(), componentID); err == nil {
		build = b
	} else {
		zap.L().Warn("failed to fetch latest component build", zap.Error(err))
	}

	props := partials.ComponentDetailProps{
		Component: comp,
		Info:      compInfo,
		Build:     build,
	}
	h.RenderTempl(c, http.StatusOK, partials.ComponentDetailPanel(props))
}
