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

// TeamSettingsPage renders the team settings page
func (h *Handler) TeamSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		h.RenderErrorPage(c, http.StatusBadRequest, "Workspace context not found")
		return
	}

	// Load workspace members
	var members []models.WorkspaceMember
	if err := h.db.Where("workspace_id = ? AND status = ?", workspace.ID, models.MemberStatusActive).
		Preload("User").
		Find(&members).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load team members")
		return
	}

	// Load active invitations
	var invitations []models.WorkspaceInvitation
	if err := h.db.Where("workspace_id = ? AND (expires_at IS NULL OR expires_at > ?) AND (max_uses = 0 OR used_count < max_uses)",
		workspace.ID, time.Now()).
		Find(&invitations).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load invitations")
		return
	}

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("workspace_id = ?", workspace.ID).Find(&allOrgs)

	// Get current org from middleware context (set by RequireOrgAccess)
	currentOrg := middleware.GetCurrentOrg(c)

	// Fetch user's workspaces for switcher
	userWorkspaces := h.GetUserWorkspaces(user.ID)

	// Load theme for styling
	theme, _ := models.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)

	// Build breadcrumb path with org ID
	settingsBasePath := h.basePath + "/orgs/" + currentOrg.ID + "/settings"

	props := vendorpages.TeamSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Team",
			ActivePage: "settings-team",
			User:       user,
			CurrentOrg: currentOrg,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Settings", Path: settingsBasePath + "/team"},
				{Text: "Team", Path: settingsBasePath + "/team", Active: true},
			},
			BasePath:         h.basePath,
			CurrentWorkspace: workspace,
			Workspaces:       userWorkspaces,
			PrimaryColor:     primaryColor,
			PrimaryColorDark: primaryColorDark,
			CSSPath:          assets.VendorCSSPath(),
		},
		Workspace:       workspace,
		Members:         members,
		Invitations:     invitations,
		CustomerBaseURL: h.customerBaseURL,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.TeamSettingsPage(props))
}

// BrandingSettingsPage renders the branding settings page
func (h *Handler) BrandingSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	workspace := middleware.GetCurrentWorkspace(c)

	// Get or create the theme
	theme, err := models.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load branding settings")
		return
	}

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("workspace_id = ?", workspace.ID).Find(&allOrgs)

	// Get current org from middleware context (set by RequireOrgAccess)
	currentOrg := middleware.GetCurrentOrg(c)

	// Fetch user's workspaces for switcher
	userWorkspaces := h.GetUserWorkspaces(user.ID)

	// Load theme colors for styling
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)

	// Build breadcrumb path with org ID
	portalBasePath := h.basePath + "/orgs/" + currentOrg.ID + "/portal"

	props := vendorpages.BrandingSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Branding",
			ActivePage: "portal-branding",
			User:       user,
			CurrentOrg: currentOrg,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Customer Portal", Path: portalBasePath + "/branding"},
				{Text: "Branding", Path: portalBasePath + "/branding", Active: true},
			},
			BasePath:         h.basePath,
			CurrentWorkspace: workspace,
			Workspaces:       userWorkspaces,
			PrimaryColor:     primaryColor,
			PrimaryColorDark: primaryColorDark,
			CSSPath:          assets.VendorCSSPath(),
		},
		Theme: theme,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.BrandingSettingsPage(props))
}

// CustomThemeSettingsPage renders the custom theme settings page
func (h *Handler) CustomThemeSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		h.RenderErrorPage(c, http.StatusBadRequest, "Workspace context not found")
		return
	}

	// Load GitHub config and overrides
	gitHubConfig, _ := models.GetGitHubRepoConfig(h.db, workspace.ID)
	templateOverrides, _ := models.GetAllTemplateOverrides(h.db, workspace.ID)
	assetOverrides, _ := models.GetAllAssetOverrides(h.db, workspace.ID)

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("workspace_id = ?", workspace.ID).Find(&allOrgs)

	// Get current org from middleware context (set by RequireOrgAccess)
	currentOrg := middleware.GetCurrentOrg(c)

	// Fetch user's workspaces for switcher
	userWorkspaces := h.GetUserWorkspaces(user.ID)

	// Load theme for styling
	theme, _ := models.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)

	// Build breadcrumb path with org ID
	portalBasePath := h.basePath + "/orgs/" + currentOrg.ID + "/portal"

	props := vendorpages.CustomThemeSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Custom Theme Settings",
			ActivePage: "portal-custom-theme",
			User:       user,
			CurrentOrg: currentOrg,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Customer Portal", Path: portalBasePath + "/branding"},
				{Text: "Custom Theme", Path: portalBasePath + "/custom-theme", Active: true},
			},
			BasePath:         h.basePath,
			CurrentWorkspace: workspace,
			Workspaces:       userWorkspaces,
			PrimaryColor:     primaryColor,
			PrimaryColorDark: primaryColorDark,
			CSSPath:          assets.VendorCSSPath(),
		},
		GitHubConfig: gitHubConfig,
		Templates:    templateOverrides,
		Assets:       assetOverrides,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.CustomThemeSettingsPage(props))
}
