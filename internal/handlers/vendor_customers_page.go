package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

func (h *Handler) CustomersPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	// Get search query
	searchQuery := c.Query("q")

	// Query for customers with their install counts
	type CustomerResult struct {
		UserID       string
		Name         string
		Email        string
		InstallCount int64
	}

	query := h.db.Table("installs").
		Select("users.id as user_id, users.name, users.email, COUNT(*) as install_count").
		Joins("JOIN users ON users.id = installs.user_id").
		Where("installs.org_id = ?", org.ID).
		Group("users.id, users.name, users.email").
		Order("users.name ASC")

	// Apply search filter if provided
	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		query = query.Where("users.name ILIKE ? OR users.email ILIKE ?", searchPattern, searchPattern)
	}

	var results []CustomerResult
	if err := query.Find(&results).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch customers: %v", err))
		return
	}

	// Convert to template type
	customers := make([]vendorpages.CustomerWithInstallCount, len(results))
	for i, result := range results {
		customers[i] = vendorpages.CustomerWithInstallCount{
			ID:           result.UserID,
			Name:         result.Name,
			Email:        result.Email,
			InstallCount: result.InstallCount,
		}
	}

	// Check if this is an HTMX request (search)
	if c.GetHeader("HX-Request") == "true" {
		// Render only the table body for HTMX updates
		h.RenderTempl(c, http.StatusOK, vendorpages.CustomersTableBody(customers, org.ID, h.basePath, searchQuery))
		return
	}

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.CustomersPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            org.Name + " - Accounts",
			ActivePage:       "accounts",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Accounts", Path: fmt.Sprintf("%s/orgs/%s/customers", h.basePath, org.ID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Org:         *org,
		Customers:   customers,
		SearchQuery: searchQuery,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.CustomersPage(props))
}
