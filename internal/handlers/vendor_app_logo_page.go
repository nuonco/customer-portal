package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) AppLogoPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	appID := c.Param("app_id")

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	// Load app name from Nuon API (best effort)
	appInfo := vendorpages.AppInfo{ID: appID}
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err == nil {
		if app, err := nuonClient.GetApp(c.Request.Context(), appID); err == nil && app != nil {
			appInfo.Name = app.Name
		}
	}

	// Load existing logos from PublishedApp record (may not exist yet)
	var publishedApp models.PublishedApp
	h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp)

	allOrgs := h.GetUserOrgs(user.ID)
	props := vendorpages.AppLogoPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            "App - Logo",
			ActivePage:       "apps",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false}, {Text: appID, Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Org:             *org,
		App:             appInfo,
		LogoLightBase64: publishedApp.LogoLightBase64,
		LogoDarkBase64:  publishedApp.LogoDarkBase64,
	}
	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.AppLogoPage(props))
}

// UpdateAppLogo saves light and dark logos for a published app
