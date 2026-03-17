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

func (h *Handler) AppOverviewPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	appID := c.Param("app_id")

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	appInfo := vendorpages.AppInfo{ID: appID}
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err == nil {
		if app, err := nuonClient.GetApp(c.Request.Context(), appID); err == nil && app != nil {
			appInfo.Name = appDisplayName(app)
		}
	}

	var publishedApp models.PublishedApp
	h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp)

	allOrgs := h.GetUserOrgs(user.ID)
	props := vendorpages.AppOverviewPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            "App - Overview",
			ActivePage:       "apps",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false}, {Text: appInfoDisplayName(appInfo), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Org:              *org,
		App:              appInfo,
		OverviewMarkdown: publishedApp.OverviewMarkdown,
	}
	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.AppOverviewPage(props))
}
