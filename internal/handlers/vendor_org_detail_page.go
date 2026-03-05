package handlers

import (
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

func (h *Handler) OrgDetailPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}
	orgID := org.ID

	// Tab and pagination parameters
	const linksPerPage = 10
	currentTab := c.DefaultQuery("tab", "available")
	page := getPageFromQuery(c)
	offset := (page - 1) * linksPerPage

	// Build base query for tab filtering (org access already validated by middleware)
	baseQuery := h.db.Where("org_id = ?", orgID)
	switch currentTab {
	case "used":
		baseQuery = baseQuery.Where("used = ?", true)
	default: // "available"
		baseQuery = baseQuery.Where("used = ?", false)
	}

	// Get total count for pagination (filtered by current tab)
	var totalCount int64
	if err := baseQuery.Model(&models.InstallLink{}).Count(&totalCount).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to count install links")
		return
	}

	// Get separate counts for each tab (for tab headers)
	var availableCount, usedCount int64
	if err := h.db.Model(&models.InstallLink{}).Where("org_id = ? AND used = ?", orgID, false).Count(&availableCount).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to count available install links")
		return
	}
	if err := h.db.Model(&models.InstallLink{}).Where("org_id = ? AND used = ?", orgID, true).Count(&usedCount).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to count used install links")
		return
	}

	// Get paginated links for current tab
	var links []models.InstallLink
	query := h.db.Preload("Install").Where("org_id = ?", orgID)
	switch currentTab {
	case "used":
		query = query.Where("used = ?", true)
	default: // "available"
		query = query.Where("used = ?", false)
	}

	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(linksPerPage).
		Find(&links).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load install links")
		return
	}

	// Calculate pagination metadata
	totalPages := int(math.Ceil(float64(totalCount) / float64(linksPerPage)))
	if totalPages == 0 {
		totalPages = 1 // Ensure at least 1 page for empty state
	}

	// Ensure current page is valid
	if page > totalPages {
		page = totalPages
	}

	// Calculate showing range
	showingFrom := offset + 1
	showingTo := offset + len(links)
	if totalCount == 0 {
		showingFrom = 0
	}

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.OrgDetailPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            org.Name + " - Install Links",
			ActivePage:       "install-links",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Install Links", Path: fmt.Sprintf("%s/orgs/%s/install-links", h.basePath, org.ID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Org:   *org,
		Links: links,
		Pagination: vendorpages.PaginationData{
			CurrentPage:    page,
			TotalPages:     totalPages,
			HasPrevious:    page > 1,
			HasNext:        page < totalPages,
			PreviousPage:   page - 1,
			NextPage:       page + 1,
			TotalCount:     totalCount,
			PerPage:        linksPerPage,
			ShowingFrom:    showingFrom,
			ShowingTo:      showingTo,
			CurrentTab:     currentTab,
			AvailableCount: availableCount,
			UsedCount:      usedCount,
		},
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.OrgDetailPage(props))
}

// generateSHA creates a random SHA for install links
