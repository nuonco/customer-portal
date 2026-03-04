package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

func (h *Handler) InstallLinkDetail(c *gin.Context) {
	user := h.GetFreshUser(c)
	org := middleware.GetCurrentOrg(c)
	linkID := c.Param("link_id")

	if !shortid.IsValid(linkID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid link ID")
		return
	}

	var link models.InstallLink
	if err := h.db.Preload("Install").Preload("Install.User").Preload("NuonOrg").Where("id = ? AND org_id = ?", linkID, org.ID).First(&link).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Install link not found")
		return
	}

	installURL := link.GetInstallURLWithSubdomain(h.customerBaseURL, h.subdomainBaseDomain)

	// Construct the customer dashboard install URL (with subdomain)
	var customerDashboardInstallURL string
	if link.Install != nil && link.NuonOrg.ID != "" {
		customerDashboardInstallURL = constructCustomerDashboardInstallURL(
			h.customerBaseURL,
			h.subdomainBaseDomain,
			link.NuonOrg.Subdomain,
			link.Install.ID,
		)
	}

	// Determine breadcrumb text (install name or app name)
	breadcrumbText := link.AppName
	if link.Install != nil && link.Install.Name != "" {
		breadcrumbText = link.Install.Name
	}

	// Get user's orgs for org switcher
	userOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.LinkDetailPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            "Install - " + link.AppName,
			ActivePage:       "install-links",
			User:             user,
			CurrentOrg:       org,
			Orgs:             userOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Install Links", Path: fmt.Sprintf("%s/orgs/%s/install-links", h.basePath, org.ID), Active: false}, {Text: breadcrumbText, Path: fmt.Sprintf("%s/orgs/%s/install-links/%s", h.basePath, org.ID, linkID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Link:                        &link,
		InstallURL:                  installURL,
		CustomerDashboardInstallURL: customerDashboardInstallURL,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.LinkDetailPage(props))
}

// InstallLinkStatus returns the link status partial for HTMX polling using Templ
// This enables real-time updates when a customer accepts an install link
