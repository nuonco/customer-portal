package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

func (h *Handler) InstallLinkStatus(c *gin.Context) {
	linkID := c.Param("link_id")

	if !shortid.IsValid(linkID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid link ID")
		return
	}

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
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

	// For HTMX requests, return only the status partial
	if isHTMXRequest(c) {
		h.RenderTempl(c, http.StatusOK, partials.LinkStatus(partials.LinkStatusProps{
			Link:                        &link,
			InstallURL:                  installURL,
			CustomerDashboardInstallURL: customerDashboardInstallURL,
			BasePath:                    h.basePath,
		}))
		return
	}

	// For full page requests, redirect to the detail page
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/install-links/%s", h.basePath, org.ID, linkID))
}

// GetOrgApps fetches apps for an organization
