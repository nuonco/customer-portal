package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

func (h *Handler) LoginSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	// Get or create the customer auth config
	config, err := models.GetOrCreateCustomerAuthConfig(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load login settings")
		return
	}

	// Get current org from middleware context (set by RequireOrgAccess)
	currentOrg := middleware.GetCurrentOrg(c)

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	// Load theme for login page settings (theme data is passed to the page, not for vendor UI styling)
	theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))

	// Build breadcrumb path with org ID
	portalBasePath := h.basePath + "/orgs/" + currentOrg.ID + "/portal"

	props := vendorpages.LoginSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Login",
			ActivePage: "portal-login",
			User:       user,
			CurrentOrg: currentOrg,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Customer Portal", Path: portalBasePath + "/branding"},
				{Text: "Login", Path: portalBasePath + "/login", Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Config: config,
		Theme:  theme,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.LoginSettingsPage(props))
}

// UpdateLoginSettings handles PUT request to update login settings
