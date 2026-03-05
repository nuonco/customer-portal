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

func (h *Handler) CustomerDetailPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	// Get customer ID from URL
	customerID := c.Param("customer_id")

	// Get customer user (role filter removed - if they own installs in this org, they're a customer)
	var customer models.User
	if err := h.db.Where("id = ?", customerID).First(&customer).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Customer not found")
		return
	}

	// Get all installs for this customer in this org
	var installs []models.Install
	if err := h.db.Where("user_id = ? AND org_id = ?", customerID, org.ID).
		Preload("InstallLink").
		Preload("InstallLink.NuonOrg").
		Order("created_at DESC").
		Find(&installs).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch installs: %v", err))
		return
	}

	// Verify customer has installs in this org
	if len(installs) == 0 {
		h.RenderErrorPage(c, http.StatusNotFound, "Customer has no installs in this organization")
		return
	}

	// Fetch apps from Nuon API to build platform and name maps; degrade gracefully on failure
	platformMap := make(map[string]string)
	nameMap := make(map[string]string)
	var customerDetailNuonAPIError string
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		customerDetailNuonAPIError = "Your API token may be expired or invalid."
	} else {
		apps, appsErr := nuonClient.ListApps(c.Request.Context())
		if appsErr != nil {
			customerDetailNuonAPIError = "Your API token may be expired or invalid."
		} else {
			for _, app := range apps {
				platform := "aws" // default
				if app.RunnerConfig != nil {
					runnerType := string(app.RunnerConfig.AppRunnerType)
					if runnerType == "azure" {
						platform = "azure"
					}
				}
				platformMap[app.ID] = platform
				nameMap[app.ID] = app.Name
			}
		}
	}

	// Convert to template type
	customerInstalls := make([]vendorpages.CustomerInstall, len(installs))
	for i, install := range installs {
		appName := nameMap[install.InstallLink.AppID]
		if appName == "" {
			appName = install.InstallLink.AppName
		}
		if appName == "" {
			appName = "Unknown App"
		}

		nuonOrgID := install.InstallLink.NuonOrg.NuonOrgID

		// Get platform from map, default to "aws"
		platform := "aws"
		if p, ok := platformMap[install.InstallLink.AppID]; ok {
			platform = p
		}

		installLinkIDStr := ""
		if install.InstallLinkID != nil {
			installLinkIDStr = *install.InstallLinkID
		}
		customerInstalls[i] = vendorpages.CustomerInstall{
			ID:            install.ID,
			InstallLinkID: installLinkIDStr,
			NuonInstallID: install.NuonInstallID,
			NuonOrgID:     nuonOrgID,
			AppID:         install.InstallLink.AppID,
			Name:          install.Name,
			AppName:       appName,
			Platform:      platform,
			Status:        string(install.Status),
			Region:        install.Region,
			CreatedAt:     install.CreatedAt.Format("Jan 2, 2006"),
		}
	}

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.CustomerDetailPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      customer.Name + " - Customer Details",
			ActivePage: "customers",
			User:       user,
			CurrentOrg: org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Customers", Path: fmt.Sprintf("%s/orgs/%s/customers", h.basePath, org.ID), Active: false},
				{Text: customer.Name, Path: fmt.Sprintf("%s/orgs/%s/customers/%s", h.basePath, org.ID, customer.ID), Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
			NuonAPIError:     customerDetailNuonAPIError,
		},
		Org:      *org,
		Customer: &customer,
		Installs: customerInstalls,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.CustomerDetailPage(props))
}

// CustomerInstallsPage displays all installs tracked in the portal for an org.
