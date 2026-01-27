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
)

// DNSSettingsPage renders the DNS settings page
func (h *Handler) DNSSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	currentOrg := middleware.GetCurrentOrg(c)
	if currentOrg == nil {
		h.RenderErrorPage(c, http.StatusBadRequest, "Organization context not found")
		return
	}

	// Fetch user's orgs for switcher
	userOrgs := h.GetUserOrgs(user.ID)

	// Build breadcrumb path with org ID
	portalBasePath := h.basePath + "/orgs/" + currentOrg.ID + "/portal"

	props := vendorpages.DNSSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "DNS Settings",
			ActivePage: "portal-dns",
			User:       user,
			CurrentOrg: currentOrg,
			Orgs:       userOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Customer Portal", Path: portalBasePath + "/branding"},
				{Text: "DNS", Path: portalBasePath + "/dns", Active: true},
			},
			BasePath: h.basePath,
			CSSPath:  assets.VendorCSSPath(),
		},
		Org:        currentOrg,
		BaseDomain: h.subdomainBaseDomain,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.DNSSettingsPage(props))
}

// UpdateDNSSettingsRequest represents the request body for updating DNS settings
type UpdateDNSSettingsRequest struct {
	Subdomain string `json:"subdomain" binding:"required"`
}

// UpdateDNSSettings handles PUT request to update the workspace subdomain
func (h *Handler) UpdateDNSSettings(c *gin.Context) {
	var req UpdateDNSSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workspace := middleware.GetCurrentOrg(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace context not found"})
		return
	}

	// Validate subdomain format
	if err := models.ValidateSubdomain(req.Subdomain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check availability (excluding current workspace)
	available, err := models.IsSubdomainAvailable(h.db, req.Subdomain, workspace.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check subdomain availability"})
		return
	}
	if !available {
		c.JSON(http.StatusConflict, gin.H{"error": "This subdomain is already taken"})
		return
	}

	// Update workspace subdomain
	if err := h.db.Model(workspace).Update("subdomain", req.Subdomain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subdomain"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"subdomain": req.Subdomain,
		"url":       fmt.Sprintf("https://%s.%s", req.Subdomain, h.subdomainBaseDomain),
	})
}

// CheckSubdomainAvailability handles GET request to check if a subdomain is available
func (h *Handler) CheckSubdomainAvailability(c *gin.Context) {
	subdomain := c.Query("subdomain")
	if subdomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subdomain query parameter required"})
		return
	}

	org := middleware.GetCurrentOrg(c)
	orgID := ""
	if org != nil {
		orgID = org.ID
	}

	// Validate format first
	if err := models.ValidateSubdomain(subdomain); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"reason":    err.Error(),
		})
		return
	}

	available, err := models.IsSubdomainAvailable(h.db, subdomain, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check availability"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"available": available,
		"reason":    "",
	})
}
