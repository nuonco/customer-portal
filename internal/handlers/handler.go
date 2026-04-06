package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const DefaultPrimaryColor = "#2563EB"

// isHTMXRequest checks if the request was made by HTMX (for partial responses)

func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

// isHTMXPartialRequest returns true for HTMX requests that expect a partial response
// (e.g., hx-get on a filter input), but false for hx-boost navigations which expect
// a full page response.
func isHTMXPartialRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true" && c.GetHeader("HX-Boosted") == ""
}

// constructCustomerDashboardInstallURL builds the URL for viewing an install in the customer dashboard.
// If a subdomain is provided, it constructs a full URL with the subdomain (e.g., https://acme.portal.nuon.co/installs/{id}).
// Otherwise, it falls back to the base URL path.

func constructCustomerDashboardInstallURL(baseURL, baseDomain, subdomain, installID string) string {
	if subdomain != "" && baseDomain != "" {
		scheme := "https://"
		if strings.HasPrefix(baseURL, "http://") {
			scheme = "http://"
		}
		return fmt.Sprintf("%s%s.%s/installs/%s", scheme, subdomain, baseDomain, installID)
	}
	return fmt.Sprintf("%s/installs/%s", baseURL, installID)
}

// DarkenColor takes a hex color and returns a darker version (for hover states)

func DarkenColor(hexColor string, factor float64) string {
	// Remove # prefix if present
	hexColor = strings.TrimPrefix(hexColor, "#")
	if len(hexColor) != 6 {
		return "#1D4ED8" // Default dark blue
	}

	// Parse RGB values
	r, _ := strconv.ParseInt(hexColor[0:2], 16, 64)
	g, _ := strconv.ParseInt(hexColor[2:4], 16, 64)
	b, _ := strconv.ParseInt(hexColor[4:6], 16, 64)

	// Darken by reducing each component
	r = int64(float64(r) * factor)
	g = int64(float64(g) * factor)
	b = int64(float64(b) * factor)

	// Clamp values
	if r < 0 {
		r = 0
	}
	if g < 0 {
		g = 0
	}
	if b < 0 {
		b = 0
	}

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// GetPrimaryColors returns the primary color and its dark variant for templates

func GetPrimaryColors(primaryColor string) (string, string) {
	if primaryColor == "" {
		primaryColor = DefaultPrimaryColor
	}
	return primaryColor, DarkenColor(primaryColor, 0.8)
}

type Handler struct {
	db                   *gorm.DB
	auth                 *jwt.GinJWTMiddleware
	authProvider         auth.AuthProvider                 // Authentication provider (OIDC, SAML) for vendors
	customerAuthFactory  *auth.CustomerAuthProviderFactory // Dynamic auth factory for customers
	customerBaseURL      string                            // Base URL for customer-facing install links
	nuonAPIURL           string                            // Global Nuon API URL for all orgs
	dashboardURL         string                            // URL of the Nuon dashboard-ui (e.g., "https://app.nuon.co")
	basePath             string                            // Base path prefix for routes (e.g., "/admin" or "" for root)
	templateRenderer     *overrides.TemplateRenderer       // Template renderer for customer page overrides
	subdomainBaseDomain  string                            // Base domain for workspace subdomains (e.g., "portal.nuon.co")
	superuserEmailDomain string                            // Email domain for superuser access (e.g., "nuon.co")
	logger               *zap.Logger
}

// PaginationData holds pagination metadata for templates

type PaginationData struct {
	Links          []models.InstallLink `json:"links"`
	CurrentPage    int                  `json:"current_page"`
	TotalPages     int                  `json:"total_pages"`
	HasPrevious    bool                 `json:"has_previous"`
	HasNext        bool                 `json:"has_next"`
	PreviousPage   int                  `json:"previous_page"`
	NextPage       int                  `json:"next_page"`
	TotalCount     int64                `json:"total_count"`
	PerPage        int                  `json:"per_page"`
	ShowingFrom    int                  `json:"showing_from"`
	ShowingTo      int                  `json:"showing_to"`
	CurrentTab     string               `json:"current_tab"`
	AvailableCount int64                `json:"available_count"`
	UsedCount      int64                `json:"used_count"`
}

// Breadcrumb represents a single breadcrumb item for navigation

type Breadcrumb struct {
	Text   string `json:"text"`
	Path   string `json:"path"`
	Active bool   `json:"active"`
}

func NewHandler(db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, authProvider auth.AuthProvider, customerBaseURL, nuonAPIURL, dashboardURL, basePath, subdomainBaseDomain, superuserEmailDomain string, logger *zap.Logger) *Handler {
	return &Handler{
		db:                   db,
		auth:                 jwtAuth,
		authProvider:         authProvider,
		customerBaseURL:      customerBaseURL,
		nuonAPIURL:           nuonAPIURL,
		dashboardURL:         dashboardURL,
		basePath:             basePath,
		templateRenderer:     overrides.NewTemplateRenderer(db, logger),
		subdomainBaseDomain:  subdomainBaseDomain,
		superuserEmailDomain: superuserEmailDomain,
		logger:               logger,
	}
}

// NewHandlerWithCustomerAuth creates a handler with customer authentication support

func NewHandlerWithCustomerAuth(db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, customerAuthFactory *auth.CustomerAuthProviderFactory, customerBaseURL, nuonAPIURL, basePath, subdomainBaseDomain string, logger *zap.Logger) *Handler {
	return &Handler{
		db:                  db,
		auth:                jwtAuth,
		customerAuthFactory: customerAuthFactory,
		customerBaseURL:     customerBaseURL,
		nuonAPIURL:          nuonAPIURL,
		basePath:            basePath,
		templateRenderer:    overrides.NewTemplateRenderer(db, logger),
		subdomainBaseDomain: subdomainBaseDomain,
		logger:              logger,
	}
}

// isSuperuser checks if a user has superuser access based on email domain.
func (h *Handler) isSuperuser(user *models.User) bool {
	if user == nil || h.superuserEmailDomain == "" {
		return false
	}
	return middleware.IsSuperuserEmail(user.Email, h.superuserEmailDomain)
}

// nuonAPIURLForOrg returns the per-org API URL if set, otherwise the global default.
func (h *Handler) nuonAPIURLForOrg(org *models.NuonOrg) string {
	if org != nil && org.APIURL != "" {
		return org.APIURL
	}
	return h.nuonAPIURL
}

// schemeFromBaseURL returns the URL scheme ("http://" or "https://") based on the customerBaseURL.

func (h *Handler) schemeFromBaseURL() string {
	if strings.HasPrefix(h.customerBaseURL, "http://") {
		return "http://"
	}
	return "https://"
}

// checkOrgStatus validates API connectivity for the given org.
// Returns status ("active" or "error"), a short status message, and an optional parsed API error.

func (h *Handler) checkOrgStatus(ctx context.Context, org *models.NuonOrg) (string, string, *nuon.APIError) {
	if org == nil || org.APIToken == "" {
		return "error", "No API token configured", nil
	}

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		return "error", "Unable to reach Nuon API", nil
	}

	if err := client.ValidateOrgAccess(checkCtx); err != nil {
		apiErr := nuon.ParseAPIError(err)
		return "error", "Unable to reach Nuon API", &apiErr
	}

	return "active", "Connected", nil
}

// enrichLayoutWithOrgStatus sets OrgStatus, OrgStatusMessage, and API error banner fields
// by checking live API connectivity for the current org.

func (h *Handler) enrichLayoutWithOrgStatus(ctx context.Context, layout *vendorui.LayoutProps) {
	status, msg, apiErr := h.checkOrgStatus(ctx, layout.CurrentOrg)
	layout.OrgStatus = status
	layout.OrgStatusMessage = msg
	if status == "error" && apiErr != nil {
		if layout.NuonAPIErrorTitle == "" {
			layout.NuonAPIErrorTitle = apiErr.Title
		}
		if layout.NuonAPIError == "" {
			layout.NuonAPIError = apiErr.Description
		}
		layout.NuonAPIShowUpdateCTA = strings.EqualFold(apiErr.Title, "Token Is Expired")
	} else if status == "error" {
		if layout.NuonAPIError == "" {
			layout.NuonAPIError = msg
		}
	}
}

// GetUserOrgs returns all active organizations for a user

func (h *Handler) GetUserOrgs(userID string) []models.NuonOrg {
	var memberships []models.OrgMember
	h.db.Where("user_id = ? AND status = ?", userID, models.MemberStatusActive).
		Preload("Org", "deleted_at IS NULL").Find(&memberships)

	orgs := make([]models.NuonOrg, 0, len(memberships))
	for _, m := range memberships {
		// Only include memberships with valid (non-deleted) orgs
		if m.Org.ID != "" {
			orgs = append(orgs, m.Org)
		}
	}
	return orgs
}

// RenderTempl renders a Templ component to the response

func (h *Handler) RenderTempl(c *gin.Context, status int, component templ.Component) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("templ-skip-modify", "true")
	c.Status(status)
	if err := component.Render(c.Request.Context(), c.Writer); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// buildTemplateContext creates a TemplateContext for override templates from existing data

func (h *Handler) buildTemplateContext(orgID, title string, user *models.User, theme *models.AppTheme, pageData interface{}) *overrides.TemplateContext {
	ctx := &overrides.TemplateContext{
		OrgID:         orgID,
		Title:         title,
		BasePath:      h.basePath,
		CSSPath:       assets.CustomerCSSPath(),
		CustomCSSPath: h.getCustomCSSPath(orgID),
		Theme:         overrides.NewThemeData(theme),
		PageData:      pageData,
	}

	if user != nil {
		ctx.User = overrides.NewUserData(user)
	}

	return ctx
}

// getCustomCSSPath returns the custom CSS path for an org, or empty string if none

func (h *Handler) getCustomCSSPath(orgID string) string {
	if orgID == "" {
		return ""
	}
	// Check if org has any enabled CSS overrides
	cssAssets, err := models.GetCSSOverrides(h.db, orgID)
	hasOverrides := err == nil && len(cssAssets) > 0

	// Also check for theme custom CSS
	theme, themeErr := models.GetOrCreateAppTheme(h.db, orgID)
	hasThemeCSS := themeErr == nil && theme.CustomCSS != ""

	if !hasOverrides && !hasThemeCSS {
		return ""
	}
	version := theme.UpdatedAt.Unix()
	return fmt.Sprintf("%s/custom/css/%s.css?v=%d", h.basePath, orgID, version)
}

// tryRenderOverride attempts to render a template override for the given page
// Returns true if an override was rendered, false if fallback to default template is needed

func (h *Handler) tryRenderOverride(c *gin.Context, orgID, pageName string, ctx *overrides.TemplateContext) bool {
	subdomain, _ := c.Get("subdomain")
	h.logger.Debug("tryRenderOverride",
		zap.String("page", pageName),
		zap.String("org_id", orgID),
		zap.Any("subdomain", subdomain),
	)

	if orgID == "" {
		h.logger.Debug("tryRenderOverride: skipping, orgID empty")
		return false
	}

	// Add custom CSS path if available
	ctx.CustomCSSPath = h.getCustomCSSPath(orgID)

	result := h.templateRenderer.TryRender(c, orgID, pageName, ctx)

	h.logger.Debug("tryRenderOverride result",
		zap.Bool("rendered", result.Rendered),
		zap.Error(result.Error),
	)

	if result.Error != nil {
		h.logger.Error("template override error",
			zap.String("org_id", orgID),
			zap.String("page", pageName),
			zap.Error(result.Error),
		)
	}
	return result.Rendered
}

// HandlePostLoginRedirect handles org selection logic after successful authentication
// This is called from both LocalLogin and AuthCallback handlers

func (h *Handler) RenderErrorPage(c *gin.Context, status int, errorMsg string) {
	props := vendorpages.ErrorPageProps{
		Error:    errorMsg,
		BasePath: h.basePath,
		CSSPath:  assets.VendorCSSPath(),
	}

	h.RenderTempl(c, status, vendorpages.ErrorPage(props))
}

// RenderCustomerErrorPage renders an error page for customer-facing pages with override support

func (h *Handler) RenderCustomerErrorPage(c *gin.Context, status int, title, errorMsg string, user *models.User) {
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)

	// Try template override first
	pageData := overrides.ErrorPageData{
		Title:   title,
		Message: errorMsg,
		Code:    status,
	}
	ctx := h.buildTemplateContext(orgID, title, user, theme, pageData)
	if h.tryRenderOverride(c, orgID, "error", ctx) {
		return
	}

	// Fall back to default Templ template
	props := customerpages.ErrorPageProps{
		LayoutProps: h.buildCustomerLayoutProps(title, user, theme, nil, nil, nil),
		Error:       errorMsg,
	}
	h.RenderTempl(c, status, customerpages.ErrorPage(props))
}

// getOrgIDForTheme safely gets org ID for theme operations
// For vendor pages with org context, uses that. For customer pages with subdomain, looks up org.

func (h *Handler) getOrgIDForTheme(c *gin.Context) string {
	// Try to get org from context (vendor routes with middleware)
	if org := middleware.GetCurrentOrg(c); org != nil {
		return org.ID
	}

	// For customer-facing routes, try to resolve org from subdomain
	subdomain, exists := c.Get("subdomain")
	if exists && subdomain != nil && subdomain.(string) != "" {
		var org models.NuonOrg
		if err := h.db.Where("subdomain = ?", subdomain.(string)).First(&org).Error; err == nil {
			return org.ID
		}
	}

	// For routes without org context and no subdomain, return empty
	// The GetOrCreateAppTheme function will need to handle empty org ID
	return ""
}

// getOrgForLayout returns the NuonOrg for use in customer layout props (admin bar context).
// Returns nil when no org context is available (e.g. auth error pages, base-domain routes).

func (h *Handler) getOrgForLayout(c *gin.Context) *models.NuonOrg {
	// Try vendor route middleware context first
	if org := middleware.GetCurrentOrg(c); org != nil {
		return org
	}

	// For customer-facing routes, resolve from subdomain
	subdomain, exists := c.Get("subdomain")
	if exists && subdomain != nil && subdomain.(string) != "" {
		var org models.NuonOrg
		if err := h.db.Where("subdomain = ?", subdomain.(string)).First(&org).Error; err == nil {
			return &org
		}
	}

	return nil
}

// BaseData returns a gin.H map with common template data including BasePath
// Use this as the base for all c.HTML calls to ensure BasePath is always available

func (h *Handler) BaseData() gin.H {
	return gin.H{
		"BasePath": h.basePath,
	}
}

// GetFreshUser loads the current user from the database (not JWT claims)
// Use this for page renders where the topbar needs to show fresh user data (e.g., updated name)
// JWT claims cache user data at login time, so updates won't appear until re-login

func (h *Handler) GetFreshUser(c *gin.Context) *models.User {
	jwtUser := middleware.GetCurrentUser(c)

	var dbUser models.User
	if err := h.db.First(&dbUser, "id = ?", jwtUser.ID).Error; err != nil {
		// Fall back to JWT user if DB lookup fails
		return jwtUser
	}
	return &dbUser
}

// MergeData merges additional data into a base gin.H map
// Use with BaseData: h.MergeData(h.BaseData(), gin.H{"title": "..."})

func (h *Handler) MergeData(base gin.H, additional gin.H) gin.H {
	for k, v := range additional {
		base[k] = v
	}
	return base
}

// getThemeData fetches the global theme settings for use in vendor pages
// Returns a gin.H map with theme data for the template

func (h *Handler) getThemeData(c *gin.Context) gin.H {
	theme, err := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		// Return empty theme on error
		return gin.H{
			"theme": &models.AppTheme{},
		}
	}
	return gin.H{
		"theme": theme,
	}
}

// getPageFromQuery extracts and validates page parameter from query string

func getPageFromQuery(c *gin.Context) int {
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// RootRedirect redirects to login page

type LoginPageConfig struct {
	Title       string
	ButtonText  string
	HelpText    string
	RedirectURL string
	Template    string // Template path (e.g., "vendor/login.html" or "customer/login.html")
}

// VendorLoginConfig returns the login page config for vendors

type AppInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Platform        string `json:"platform"` // "aws" or "azure"
	LogoLightBase64 string `json:"logo_light_base64"`
	LogoDarkBase64  string `json:"logo_dark_base64"`
	Deleted         bool   `json:"deleted"`
}

// orgHasPublishedApps checks if an org has any published apps
