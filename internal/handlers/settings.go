package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

// TeamPage renders the combined team members and invites page
func (h *Handler) TeamPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusBadRequest, "Organization context not found")
		return
	}

	// Load org members
	var members []models.OrgMember
	if err := h.db.Where("org_id = ? AND status = ?", org.ID, models.MemberStatusActive).
		Preload("User").
		Find(&members).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load team members")
		return
	}

	// Load active invitations
	var invitations []models.OrgInvitation
	if err := h.db.Where("org_id = ? AND (expires_at IS NULL OR expires_at > ?) AND (max_uses = 0 OR used_count < max_uses)",
		org.ID, time.Now()).
		Find(&invitations).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load invitations")
		return
	}

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.TeamPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Team",
			ActivePage: "settings-team",
			User:       user,
			CurrentOrg: org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Team", Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Org:         org,
		Members:     members,
		Invitations: invitations,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.TeamPage(props))
}

// BrandingSettingsPage renders the branding settings page
func (h *Handler) BrandingSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	org := middleware.GetCurrentOrg(c)

	// Get or create the theme
	theme, err := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load branding settings")
		return
	}

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	// Build breadcrumb path with org ID
	portalBasePath := h.basePath + "/orgs/" + org.ID + "/portal"

	props := vendorpages.BrandingSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Branding",
			ActivePage: "portal-branding",
			User:       user,
			CurrentOrg: org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Customer Portal", Path: portalBasePath + "/branding"},
				{Text: "Branding", Path: portalBasePath + "/branding", Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Theme: theme,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.BrandingSettingsPage(props))
}

// CustomThemeSettingsPage renders the custom theme settings page
func (h *Handler) CustomThemeSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusBadRequest, "Organization context not found")
		return
	}

	// Load GitHub config and overrides
	gitHubConfig, _ := models.GetGitHubRepoConfig(h.db, org.ID)
	templateOverrides, _ := models.GetAllTemplateOverrides(h.db, org.ID)
	assetOverrides, _ := models.GetAllAssetOverrides(h.db, org.ID)

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	// Build breadcrumb path with org ID
	portalBasePath := h.basePath + "/orgs/" + org.ID + "/portal"

	props := vendorpages.CustomThemeSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Custom Theme Settings",
			ActivePage: "portal-custom-theme",
			User:       user,
			CurrentOrg: org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Customer Portal", Path: portalBasePath + "/branding"},
				{Text: "Custom Theme", Path: portalBasePath + "/custom-theme", Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		GitHubConfig: gitHubConfig,
		Templates:    templateOverrides,
		Assets:       assetOverrides,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.CustomThemeSettingsPage(props))
}
