package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

func (h *Handler) OrgsPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	// Get all orgs accessible to this user
	orgs := h.GetUserOrgs(user.ID)

	// If user has orgs, redirect to the first org's customers page
	if len(orgs) > 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/accounts", h.basePath, orgs[0].ID))
		return
	}

	// No orgs - render the OrgsPage template with empty state
	props := vendorpages.OrgsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            "Organizations",
			ActivePage:       "orgs",
			User:             user,
			CurrentOrg:       nil,
			Orgs:             orgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Organizations", Path: h.basePath + "/orgs", Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Orgs: orgs,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.OrgsPage(props))
}

// NOTE: CreateOrg and UpdateOrg are now defined in workspaces.go

// OrgSettingsPage renders the org settings page using Templ
