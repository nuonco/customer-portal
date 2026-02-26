package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// DefaultPrimaryColor is the default blue color (Tailwind blue-600)
const DefaultPrimaryColor = "#2563EB"

// isHTMXRequest checks if the request was made by HTMX (for partial responses)
func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
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
	db                  *gorm.DB
	auth                *jwt.GinJWTMiddleware
	authProvider        auth.AuthProvider                 // Authentication provider (OIDC, SAML) for vendors
	customerAuthFactory *auth.CustomerAuthProviderFactory // Dynamic auth factory for customers
	customerBaseURL     string                            // Base URL for customer-facing install links
	nuonAPIURL          string                            // Global Nuon API URL for all orgs
	dashboardURL        string                            // URL of the Nuon dashboard-ui (e.g., "https://app.nuon.co")
	basePath            string                            // Base path prefix for routes (e.g., "/admin" or "" for root)
	templateRenderer    *overrides.TemplateRenderer       // Template renderer for customer page overrides
	subdomainBaseDomain string                            // Base domain for workspace subdomains (e.g., "portal.nuon.co")
	logger              *zap.Logger
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

func NewHandler(db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, authProvider auth.AuthProvider, customerBaseURL, nuonAPIURL, dashboardURL, basePath, subdomainBaseDomain string, logger *zap.Logger) *Handler {
	return &Handler{
		db:                  db,
		auth:                jwtAuth,
		authProvider:        authProvider,
		customerBaseURL:     customerBaseURL,
		nuonAPIURL:          nuonAPIURL,
		dashboardURL:        dashboardURL,
		basePath:            basePath,
		templateRenderer:    overrides.NewTemplateRenderer(db, logger),
		subdomainBaseDomain: subdomainBaseDomain,
		logger:              logger,
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

// schemeFromBaseURL returns the URL scheme ("http://" or "https://") based on the customerBaseURL.
func (h *Handler) schemeFromBaseURL() string {
	if strings.HasPrefix(h.customerBaseURL, "http://") {
		return "http://"
	}
	return "https://"
}

// checkOrgStatus validates API connectivity for the given org.
// Returns status ("active" or "error") and a human-readable message.
func (h *Handler) checkOrgStatus(ctx context.Context, org *models.NuonOrg) (string, string) {
	if org == nil || org.APIToken == "" {
		return "error", "No API token configured"
	}

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		return "error", "Unable to reach Nuon API"
	}

	if err := client.ValidateOrgAccess(checkCtx); err != nil {
		return "error", "Unable to reach Nuon API"
	}

	return "active", "Connected"
}

// enrichLayoutWithOrgStatus sets OrgStatus and OrgStatusMessage on the layout props
// by checking live API connectivity for the current org.
func (h *Handler) enrichLayoutWithOrgStatus(ctx context.Context, layout *vendorui.LayoutProps) {
	status, msg := h.checkOrgStatus(ctx, layout.CurrentOrg)
	layout.OrgStatus = status
	layout.OrgStatusMessage = msg
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
func (h *Handler) HandlePostLoginRedirect(c *gin.Context, user *models.User) {
	h.logger.Debug("HandlePostLoginRedirect",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
		zap.String("role", string(user.Role)),
	)

	// Check for return_url cookie (set by invitation flow)
	if returnURL, err := c.Cookie("return_url"); err == nil && returnURL != "" {
		h.logger.Debug("HandlePostLoginRedirect: found return_url cookie, redirecting", zap.String("return_url", returnURL))
		// Clear the cookie
		c.SetCookie("return_url", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, returnURL)
		return
	}
	h.logger.Debug("HandlePostLoginRedirect: no return_url cookie found")

	// Only apply org logic for vendor users
	if user.Role != models.RoleVendor {
		c.Redirect(http.StatusFound, h.basePath+"/orgs")
		return
	}

	// Get user's active orgs (filter out soft-deleted orgs)
	var memberships []models.OrgMember
	if err := h.db.Where("user_id = ? AND status = ?", user.ID, models.MemberStatusActive).
		Preload("Org", "deleted_at IS NULL").
		Find(&memberships).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load organizations")
		return
	}

	// Extract orgs (only those with valid non-deleted orgs)
	orgs := make([]models.NuonOrg, 0, len(memberships))
	for _, m := range memberships {
		if m.Org.ID != "" {
			orgs = append(orgs, m.Org)
		}
	}

	// Case 1: No orgs - redirect to org creation page
	// (Personal orgs are no longer auto-created; users must connect a Nuon org)
	if len(orgs) == 0 {
		c.Redirect(http.StatusFound, h.basePath+"/orgs")
		return
	}

	// Case 2: One or more orgs - redirect to org list page
	// The /orgs page will redirect to the first org
	c.Redirect(http.StatusFound, h.basePath+"/orgs")
}

// RenderErrorPage renders a templ error page (for vendor pages)
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
		LayoutProps: h.buildCustomerLayoutProps(title, user, theme, nil),
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
func (h *Handler) RootRedirect(c *gin.Context) {
	c.Redirect(http.StatusFound, "/login")
}

// LoginPageConfig holds configuration for the login page
type LoginPageConfig struct {
	Title       string
	ButtonText  string
	HelpText    string
	RedirectURL string
	Template    string // Template path (e.g., "vendor/login.html" or "customer/login.html")
}

// VendorLoginConfig returns the login page config for vendors
func VendorLoginConfig(basePath string) LoginPageConfig {
	return LoginPageConfig{
		Title:       "Vendor Login",
		ButtonText:  "Login / Sign Up",
		HelpText:    "",
		RedirectURL: basePath + "/orgs",
		Template:    "vendor/login.html",
	}
}

// CustomerLoginConfig returns the login page config for customers
func CustomerLoginConfig(basePath string) LoginPageConfig {
	return LoginPageConfig{
		Title:       "Customer Login",
		ButtonText:  "Login",
		HelpText:    "Don't have an account? Accept an install link from your vendor to get started.",
		RedirectURL: basePath + "/installs",
		Template:    "customer/login.html",
	}
}

// LoginPage renders the login page with the given configuration
func (h *Handler) LoginPage(config LoginPageConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, config.Template, h.MergeData(h.BaseData(), gin.H{
			"title":           config.Title + " - Installer App",
			"Title":           config.Title,
			"ButtonText":      config.ButtonText,
			"HelpText":        config.HelpText,
			"RedirectURL":     config.RedirectURL,
			"contentTemplate": "login_content",
		}))
	}
}

// CustomerLoginPageTempl renders the customer login page with vendor theming
func (h *Handler) CustomerLoginPageTempl(c *gin.Context) {
	// Check for error message in query params
	errorMsg := c.Query("error")

	// Get vendor theme for customer-facing pages
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)

	// Check if we're on a subdomain
	subdomain, _ := c.Get("subdomain")
	var authURL string

	// Get redirect URL from query params (e.g., from install link page when user is not logged in)
	redirectURL := c.Query("redirect")

	if subdomain != nil && subdomain.(string) != "" {
		// On subdomain: buttons should link to base domain auth endpoint
		// This initiates the base domain auth flow to avoid cookie scoping issues
		authURL = fmt.Sprintf("%s/auth/login?return_to=%s",
			h.customerBaseURL, subdomain.(string))

		// Preserve redirect URL through the auth flow
		if redirectURL != "" {
			authURL = fmt.Sprintf("%s&redirect=%s", authURL, url.QueryEscape(redirectURL))
		}
	} else {
		// On base domain: show error or fallback behavior
		if errorMsg == "" {
			errorMsg = "Please access login from your workspace subdomain"
		}

		// Generate fallback OIDC URL for base domain (legacy behavior)
		state, err := auth.GenerateState()
		if err != nil {
			errorMsg = "Failed to generate security token"
		} else {
			// Store state in cookie for validation on callback
			c.SetCookie("auth_state", state, 600, "/", "", false, true)

			fallbackAuthURL, err := h.customerAuthFactory.GetAuthURL(state)
			if err != nil {
				errorMsg = "Failed to generate login URL"
			} else {
				authURL = fallbackAuthURL
			}
		}
	}

	props := customerpages.CustomerLoginPageProps{
		BasePath: h.basePath,
		Error:    errorMsg,
		AuthURL:  authURL,
		Theme:    theme,
		CSSPath:  assets.CustomerCSSPath(),
	}

	// Try template override first
	pageData := overrides.LoginPageData{
		AuthURL: authURL,
		Error:   errorMsg,
	}
	ctx := h.buildTemplateContext(orgID, theme.GetLoginTitle(), nil, theme, pageData)
	if h.tryRenderOverride(c, orgID, "login", ctx) {
		return
	}

	// Fall back to default Templ template
	h.RenderTempl(c, http.StatusOK, customerpages.CustomerLoginPage(props))
}

// CustomerLocalLogin is disabled - local email/password login is not supported for customers.
// Customers must use OIDC authentication.
func (h *Handler) CustomerLocalLogin(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{"message": "Local login is not supported. Please use SSO."})
}

// CustomerRegisterPage redirects to login - registration is not available for customers.
// Customer accounts are created via OIDC authentication.
func (h *Handler) CustomerRegisterPage(c *gin.Context) {
	redirect := c.Query("redirect")
	if redirect != "" {
		c.Redirect(http.StatusFound, "/login?redirect="+redirect)
	} else {
		c.Redirect(http.StatusFound, "/login")
	}
}

// CustomerRegister is disabled - registration is not available for customers.
// Customer accounts are created via OIDC authentication.
func (h *Handler) CustomerRegister(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"message": "Registration is not available. Please use SSO."})
}

// VendorLoginPageTempl renders the vendor login page
func (h *Handler) VendorLoginPageTempl(c *gin.Context) {
	// Check for error message in query params (from failed login/register)
	errorMsg := c.Query("error")

	props := vendorpages.VendorLoginPageProps{
		Title:    "Customer Portal",
		BasePath: h.basePath,
		Error:    errorMsg,
		CSSPath:  assets.CustomerCSSPath(),
	}

	// Check if we have an auth provider configured
	if h.authProvider == nil {
		props.Error = "Authentication not configured"
		h.RenderTempl(c, http.StatusInternalServerError, vendorpages.VendorLoginPage(props))
		return
	}

	// Local auth is no longer supported - require OIDC
	if h.authProvider.Name() == "local" {
		props.Error = "OIDC authentication required. Please configure AUTH_* environment variables."
		h.RenderTempl(c, http.StatusInternalServerError, vendorpages.VendorLoginPage(props))
		return
	}

	// Generate state for CSRF protection
	state, err := auth.GenerateState()
	if err != nil {
		props.Error = "Failed to generate security token"
		h.RenderTempl(c, http.StatusInternalServerError, vendorpages.VendorLoginPage(props))
		return
	}

	// Store state in cookie for validation on callback
	c.SetCookie("auth_state", state, 600, "/", "", false, true)

	authURL, err := h.authProvider.GetAuthorizationURL(state)
	if err != nil {
		props.Error = "Failed to generate login URL"
		h.RenderTempl(c, http.StatusInternalServerError, vendorpages.VendorLoginPage(props))
		return
	}

	props.AuthURL = authURL
	h.RenderTempl(c, http.StatusOK, vendorpages.VendorLoginPage(props))
}

// AuthCallback handles the authentication callback from OIDC or SAML providers
func (h *Handler) AuthCallback(c *gin.Context) {
	if h.authProvider == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Authentication not configured")
		return
	}

	// Validate state parameter (CSRF protection)
	expectedState, _ := c.Cookie("auth_state")
	receivedState := c.Query("state")
	if receivedState == "" {
		// SAML uses RelayState
		receivedState = c.PostForm("RelayState")
	}

	if expectedState != "" && !auth.ValidateState(expectedState, receivedState) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid state parameter")
		return
	}

	// Clear state cookie
	c.SetCookie("auth_state", "", -1, "/", "", false, true)

	// Build callback request (supports both OIDC and SAML)
	req := auth.CallbackRequest{
		Code:         c.Query("code"),
		State:        receivedState,
		SAMLResponse: c.PostForm("SAMLResponse"),
		RelayState:   c.PostForm("RelayState"),
	}

	// Validate we have either code (OIDC) or SAMLResponse (SAML)
	if req.Code == "" && req.SAMLResponse == "" {
		h.RenderErrorPage(c, http.StatusBadRequest, "Missing authentication response")
		return
	}

	// Handle the callback
	authResult, err := h.authProvider.HandleCallback(c.Request.Context(), req)
	if err != nil {
		h.RenderErrorPage(c, http.StatusUnauthorized, fmt.Sprintf("Authentication failed: %v", err))
		return
	}

	// Generate JWT token using the existing auth middleware
	token, _, err := h.auth.TokenGenerator(authResult.User)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to generate session token")
		return
	}

	// Set the JWT cookie
	maxAge := int(h.auth.Timeout.Seconds())
	c.SetCookie("jwt", token, maxAge, "/", "", false, true)

	// Store session ID for logout (if available)
	if authResult.SessionID != "" {
		c.SetCookie("auth_session", authResult.SessionID, maxAge, "/", "", false, true)
	}

	// Handle post-login workspace selection
	h.HandlePostLoginRedirect(c, authResult.User)
}

// VendorLogout handles logout for vendor users
func (h *Handler) VendorLogout(c *gin.Context) {
	// Clear the JWT cookie
	c.SetCookie("jwt", "", -1, "/", "", false, true)

	// Get session ID from cookie
	sessionID, _ := c.Cookie("auth_session")

	// Clear the session cookie
	c.SetCookie("auth_session", "", -1, "/", "", false, true)

	// If provider supports logout, redirect to IdP logout endpoint
	if h.authProvider != nil && h.authProvider.SupportsLogout() {
		logoutURL, err := h.authProvider.GetLogoutURL(sessionID)
		if err == nil && logoutURL != "" {
			c.Redirect(http.StatusFound, logoutURL)
			return
		}
	}

	// Fallback: redirect to login page
	c.Redirect(http.StatusFound, h.basePath+"/login/")
}

// CustomerLogout handles logout for customer users
func (h *Handler) CustomerLogout(c *gin.Context) {
	// Clear the JWT cookie server-side (same params as when it was set)
	c.SetCookie("jwt", "", -1, "/", "", false, true)

	// Clear the session cookie if present
	c.SetCookie("auth_session", "", -1, "/", "", false, true)

	// Redirect to customer login
	c.Redirect(http.StatusFound, "/login")
}

// LocalLogin handles POST /admin/login/ for email/password authentication
func (h *Handler) LocalLogin(c *gin.Context) {
	localProvider, ok := h.authProvider.(*auth.LocalProvider)
	if !ok {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Local authentication not configured")
		return
	}

	email := c.PostForm("email")
	password := c.PostForm("password")

	authResult, err := localProvider.Login(email, password)
	if err != nil {
		// Redirect back to login with error
		c.Redirect(http.StatusFound, h.basePath+"/login/?error="+err.Error())
		return
	}

	// Generate JWT token
	token, _, err := h.auth.TokenGenerator(authResult.User)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to generate session token")
		return
	}

	// Set the JWT cookie
	maxAge := int(h.auth.Timeout.Seconds())
	c.SetCookie("jwt", token, maxAge, "/", "", false, true)

	// Store session ID for logout
	if authResult.SessionID != "" {
		c.SetCookie("auth_session", authResult.SessionID, maxAge, "/", "", false, true)
	}

	// Handle post-login workspace selection
	h.HandlePostLoginRedirect(c, authResult.User)
}

// VendorRegisterPageTempl renders the vendor registration page using Templ
func (h *Handler) VendorRegisterPageTempl(c *gin.Context) {
	// Check for error message in query params
	errorMsg := c.Query("error")

	props := vendorpages.RegisterPageProps{
		Title:      "Create Account",
		ButtonText: "Sign Up",
		BasePath:   h.basePath,
		Error:      errorMsg,
		CSSPath:    assets.VendorCSSPath(),
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.RegisterPage(props))
}

// LocalRegister handles POST /admin/register for new user registration
func (h *Handler) LocalRegister(c *gin.Context) {
	localProvider, ok := h.authProvider.(*auth.LocalProvider)
	if !ok {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Local authentication not configured")
		return
	}

	name := c.PostForm("name")
	email := c.PostForm("email")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")

	// Validate password confirmation
	if password != confirmPassword {
		c.Redirect(http.StatusFound, h.basePath+"/register?error=Passwords do not match")
		return
	}

	authResult, err := localProvider.Register(email, password, name)
	if err != nil {
		// Redirect back to register with error
		c.Redirect(http.StatusFound, h.basePath+"/register?error="+err.Error())
		return
	}

	// Generate JWT token
	token, _, err := h.auth.TokenGenerator(authResult.User)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to generate session token")
		return
	}

	// Set the JWT cookie
	maxAge := int(h.auth.Timeout.Seconds())
	c.SetCookie("jwt", token, maxAge, "/", "", false, true)

	// Store session ID for logout
	if authResult.SessionID != "" {
		c.SetCookie("auth_session", authResult.SessionID, maxAge, "/", "", false, true)
	}

	// Redirect to the orgs page
	c.Redirect(http.StatusFound, h.basePath+"/orgs")
}

// OrgsPage renders the organizations list page for a org.
// Each workspace has exactly one connected org (one-to-one relationship).
// New users may have a workspace but no org yet - they need to see the empty state
// with the "Connect Your First Org" button rather than an error page.
func (h *Handler) OrgsPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	// Get all orgs accessible to this user
	orgs := h.GetUserOrgs(user.ID)

	// If user has orgs, redirect to the first org's customers page
	if len(orgs) > 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/customers", h.basePath, orgs[0].ID))
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
		},
		Orgs: orgs,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.OrgsPage(props))
}

// NOTE: CreateOrg and UpdateOrg are now defined in workspaces.go

// OrgSettingsPage renders the org settings page using Templ
func (h *Handler) OrgSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	// Fetch all orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.OrgSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            org.Name + " - Settings",
			ActivePage:       "settings",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Org Connection", Path: fmt.Sprintf("%s/orgs/%s/connection", h.basePath, org.ID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Org: *org,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.OrgSettingsPage(props))
}

// OrgRedirect redirects /:org_id to the customers page.
func (h *Handler) OrgRedirect(c *gin.Context) {
	orgID := c.Param("org_id")
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/customers", h.basePath, orgID))
}

// OrgDetailPage renders the org detail page with paginated install links using Templ
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
func generateSHA() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateInstallLink handles creating a new install link
// Vendor provides vendor-facing inputs which are stored for later use when customer accepts
// The Install is created when customer accepts the link (not here)
func (h *Handler) CreateInstallLink(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetCurrentOrg(c)

	var req struct {
		AppID   string            `json:"app_id" binding:"required"`
		AppName string            `json:"app_name" binding:"required"`
		Name    string            `json:"name" binding:"required"` // Required install name
		Inputs  map[string]string `json:"inputs"`                  // Vendor-facing inputs (stored for later)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Normalize and validate install name
	normalizedName := models.NormalizeInstallName(req.Name)
	if err := models.ValidateInstallName(normalizedName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check local uniqueness (unused install links with same name for this app)
	localAvailable, err := models.IsInstallNameAvailableLocally(h.db, org.ID, req.AppID, normalizedName, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check install name availability"})
		return
	}
	if !localAvailable {
		c.JSON(http.StatusConflict, gin.H{"error": "An install link with this name already exists"})
		return
	}

	// Check Nuon API uniqueness (existing installs with same name)
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	nuonAvailable, err := nuonClient.IsInstallNameAvailable(c.Request.Context(), req.AppID, normalizedName)
	if err != nil {
		// Log error but don't fail - Nuon API check is best-effort
		h.logger.Warn("failed to check Nuon API for install name availability", zap.Error(err))
	} else if !nuonAvailable {
		c.JSON(http.StatusConflict, gin.H{"error": "An install with this name already exists"})
		return
	}

	// Generate unique SHA for the link
	sha, err := generateSHA()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate link"})
		return
	}

	// Create install link with vendor inputs stored
	link := models.InstallLink{
		UserID:  user.ID,
		OrgID:   org.ID,
		AppID:   req.AppID,
		AppName: req.AppName,
		Name:    normalizedName,
		SHA:     sha,
		Used:    false,
	}

	// Store vendor inputs for later use when customer accepts
	if req.Inputs != nil && len(req.Inputs) > 0 {
		if err := link.SetVendorInputs(req.Inputs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store vendor inputs"})
			return
		}
	}

	if err := h.db.Create(&link).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create install link"})
		return
	}

	// Note: Install is NOT created here - it will be created when customer accepts the link
	// This allows customer to provide required customer inputs and choose region/location

	c.JSON(http.StatusCreated, gin.H{
		"link": link,
	})
}

// InstallLinkDetail shows details about a specific install link using Templ
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
func (h *Handler) GetOrgApps(c *gin.Context) {
	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	// Initialize Nuon client with the org's credentials and global API URL
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Fetch apps from Nuon API
	apps, err := nuonClient.ListApps(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch apps: %v", err)})
		return
	}

	c.JSON(http.StatusOK, apps)
}

// GetAppInputConfig fetches app input configuration for dynamic form rendering
// Accepts optional ?filter=vendor|customer query parameter to filter inputs:
// - filter=vendor: returns inputs where user_configurable != true
// - filter=customer: returns inputs where user_configurable == true
// - no filter: returns all inputs (backward compatibility)
func (h *Handler) GetAppInputConfig(c *gin.Context) {
	appIDParam := c.Param("app_id")

	// Get optional filter parameter
	filterParam := c.Query("filter")

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	// Initialize Nuon client with the org's credentials and global API URL
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Fetch app details to get platform information
	app, err := nuonClient.GetApp(c.Request.Context(), appIDParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch app details: %v", err)})
		return
	}

	// Fetch app input configuration from Nuon API
	// Note: Input config may not exist for all apps - that's okay, we'll return empty config
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appIDParam)
	if err != nil {
		// Input config doesn't exist or failed to fetch - return empty config rather than error
		// This allows the form to render without inputs for apps that don't have input configs
		inputConfig = nil
	}

	// Apply filtering if requested
	// Convert typed struct to map for filtering (filterInputConfig expects map[string]interface{})
	if filterParam == "vendor" || filterParam == "customer" {
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				if filterParam == "vendor" {
					inputConfig = filterInputConfig(configMap, FilterTypeVendor)

					// Also exclude inputs marked as customer-facing in local config
					var localConfig models.AppInputConfig
					h.db.Where("org_id = ? AND app_id = ?", org.ID, appIDParam).First(&localConfig)
					customerInputNames := localConfig.GetCustomerInputNames()
					if len(customerInputNames) > 0 {
						jsonBytes, err := json.Marshal(inputConfig)
						if err == nil {
							var configMap map[string]interface{}
							if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
								inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeVendor)
							}
						}
					}
				} else {
					inputConfig = filterInputConfig(configMap, FilterTypeCustomer)
				}
			}
		}
	}

	// Extract platform from app runner config
	var platform string
	if app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}

	c.JSON(http.StatusOK, gin.H{
		"platform":     platform,
		"input_config": inputConfig,
	})
}

// DeleteInstallLink handles deleting an install link
func (h *Handler) DeleteInstallLink(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	linkID := c.Param("link_id")

	if !shortid.IsValid(linkID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid link ID"})
		return
	}

	result := h.db.Where("id = ? AND org_id = ?", linkID, org.ID).Delete(&models.InstallLink{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete install"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install link not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Install deleted successfully"})
}

// DeleteOrg is disabled - org deletion is no longer supported.
// With one-to-one workspace-org relationship, users should delete the workspace instead.
func (h *Handler) DeleteOrg(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":     "Org deletion is no longer supported. Each workspace has exactly one connected org.",
		"migration": "To remove this org, delete the workspace from workspace settings.",
	})
}

// UpdateThemeSettings handles PUT request to update global theme settings
func (h *Handler) UpdateThemeSettings(c *gin.Context) {
	var req struct {
		PrimaryColor              string `json:"primary_color"`
		SecondaryColor            string `json:"secondary_color"`
		PrimaryColorDark          string `json:"primary_color_dark"`   // Dark mode primary color
		SecondaryColorDark        string `json:"secondary_color_dark"` // Dark mode secondary color
		LogoBase64                string `json:"logo_base64"`          // Kept for backward compatibility (maps to LogoLightBase64)
		LogoLightBase64           string `json:"logo_light_base64"`    // Light mode logo
		LogoDarkBase64            string `json:"logo_dark_base64"`     // Dark mode logo
		FaviconBase64             string `json:"favicon_base64"`
		HeadingFont               string `json:"heading_font"`
		BodyFont                  string `json:"body_font"`
		HeadingFontBase64         string `json:"heading_font_base64"`
		BodyFontBase64            string `json:"body_font_base64"`
		HeadingFontName           string `json:"heading_font_name"`
		BodyFontName              string `json:"body_font_name"`
		WhiteColor                string `json:"white_color"`
		BlackColor                string `json:"black_color"`
		WhiteColorDark            string `json:"white_color_dark"`
		BlackColorLight           string `json:"black_color_light"`
		BorderRadius              string `json:"border_radius"`
		ThemeMode                 string `json:"theme_mode"`
		LoginTitle                string `json:"login_title"`
		LoginSubtitle             string `json:"login_subtitle"`
		LoginRightSideImageBase64 string `json:"login_right_side_image_base64"`
		LoginRightSideGradient    string `json:"login_right_side_gradient"`
		CustomCSS                 string `json:"custom_css"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get or create the global app theme
	theme, err := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load theme settings"})
		return
	}

	// Update fields if provided
	if req.PrimaryColor != "" {
		theme.PrimaryColor = req.PrimaryColor
	}
	if req.SecondaryColor != "" {
		theme.SecondaryColor = req.SecondaryColor
	}

	// Handle dark mode colors - empty string or "REMOVE" clears, valid hex color sets
	// Unlike light mode colors, dark mode colors can be cleared to fall back to light mode
	isValidHexColor := func(s string) bool {
		if len(s) != 7 || s[0] != '#' {
			return false
		}
		for _, c := range s[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}

	// Dark mode primary color: valid hex sets it, empty/REMOVE clears it
	if isValidHexColor(req.PrimaryColorDark) {
		theme.PrimaryColorDark = req.PrimaryColorDark
	} else {
		// Clear dark mode color (falls back to light mode)
		theme.PrimaryColorDark = ""
	}

	// Dark mode secondary color: valid hex sets it, empty/REMOVE clears it
	if isValidHexColor(req.SecondaryColorDark) {
		theme.SecondaryColorDark = req.SecondaryColorDark
	} else {
		// Clear dark mode color (falls back to light mode)
		theme.SecondaryColorDark = ""
	}

	// White/Black background colors: valid hex sets them, empty/invalid clears them
	if isValidHexColor(req.WhiteColor) {
		theme.WhiteColor = req.WhiteColor
	} else {
		theme.WhiteColor = ""
	}
	if isValidHexColor(req.BlackColor) {
		theme.BlackColor = req.BlackColor
	} else {
		theme.BlackColor = ""
	}
	if isValidHexColor(req.WhiteColorDark) {
		theme.WhiteColorDark = req.WhiteColorDark
	} else {
		theme.WhiteColorDark = ""
	}
	if isValidHexColor(req.BlackColorLight) {
		theme.BlackColorLight = req.BlackColorLight
	} else {
		theme.BlackColorLight = ""
	}

	// Handle light mode logo - "REMOVE" clears, valid data URI sets
	if req.LogoLightBase64 != "" {
		if req.LogoLightBase64 == "REMOVE" {
			theme.LogoLightBase64 = ""
		} else if strings.HasPrefix(req.LogoLightBase64, "data:image/") {
			theme.LogoLightBase64 = req.LogoLightBase64
		}
	}

	// Handle dark mode logo - "REMOVE" clears, valid data URI sets
	if req.LogoDarkBase64 != "" {
		if req.LogoDarkBase64 == "REMOVE" {
			theme.LogoDarkBase64 = ""
		} else if strings.HasPrefix(req.LogoDarkBase64, "data:image/") {
			theme.LogoDarkBase64 = req.LogoDarkBase64
		}
	}

	// Backward compatibility: handle legacy LogoBase64 field (maps to light mode logo)
	if req.LogoBase64 != "" {
		if req.LogoBase64 == "REMOVE" {
			theme.LogoLightBase64 = ""
		} else if strings.HasPrefix(req.LogoBase64, "data:image/") {
			theme.LogoLightBase64 = req.LogoBase64
		}
	}

	// Handle favicon - "REMOVE" clears, valid image data URI sets
	if req.FaviconBase64 != "" {
		if req.FaviconBase64 == "REMOVE" {
			theme.FaviconBase64 = ""
		} else if strings.HasPrefix(req.FaviconBase64, "data:image/") {
			theme.FaviconBase64 = req.FaviconBase64
		}
	}

	// Fonts can be set to empty string to use default system fonts
	theme.HeadingFont = req.HeadingFont
	theme.BodyFont = req.BodyFont

	// Handle custom font uploads (base64 data URIs)
	isValidFontDataURI := func(s string) bool {
		return strings.HasPrefix(s, "data:font/woff2") ||
			strings.HasPrefix(s, "data:font/woff") ||
			strings.HasPrefix(s, "data:application/font-woff2") ||
			strings.HasPrefix(s, "data:application/font-woff") ||
			strings.HasPrefix(s, "data:application/x-font-woff") ||
			strings.HasPrefix(s, "data:application/octet-stream")
	}

	// HeadingFontBase64: "REMOVE" clears, valid data URI sets
	if req.HeadingFontBase64 == "REMOVE" {
		theme.HeadingFontBase64 = ""
		theme.HeadingFontName = ""
	} else if len(req.HeadingFontBase64) > 0 && isValidFontDataURI(req.HeadingFontBase64) {
		if len(req.HeadingFontBase64) <= 400000 {
			theme.HeadingFontBase64 = req.HeadingFontBase64
			theme.HeadingFontName = req.HeadingFontName
		}
	}

	// BodyFontBase64: "REMOVE" clears, valid data URI sets
	if req.BodyFontBase64 == "REMOVE" {
		theme.BodyFontBase64 = ""
		theme.BodyFontName = ""
	} else if len(req.BodyFontBase64) > 0 && isValidFontDataURI(req.BodyFontBase64) {
		if len(req.BodyFontBase64) <= 400000 {
			theme.BodyFontBase64 = req.BodyFontBase64
			theme.BodyFontName = req.BodyFontName
		}
	}

	// Update border radius if provided and valid
	if req.BorderRadius != "" {
		if models.IsValidBorderRadius(req.BorderRadius) {
			theme.BorderRadius = req.BorderRadius
		}
	}

	// ThemeMode: "auto", "light", "dark"; empty string clears to auto
	if models.IsValidThemeMode(req.ThemeMode) {
		theme.ThemeMode = req.ThemeMode
	} else {
		theme.ThemeMode = ""
	}

	// Update login page text - allow setting to empty to use defaults
	theme.LoginTitle = req.LoginTitle
	theme.LoginSubtitle = req.LoginSubtitle

	// Handle login right side image - "REMOVE" clears, valid data URI sets
	if req.LoginRightSideImageBase64 != "" {
		if req.LoginRightSideImageBase64 == "REMOVE" {
			theme.LoginRightSideImageBase64 = ""
		} else if strings.HasPrefix(req.LoginRightSideImageBase64, "data:image/") {
			theme.LoginRightSideImageBase64 = req.LoginRightSideImageBase64
		}
	}

	// Handle login right side gradient - "REMOVE" clears, otherwise sets CSS gradient
	if req.LoginRightSideGradient != "" {
		if req.LoginRightSideGradient == "REMOVE" {
			theme.LoginRightSideGradient = ""
		} else {
			theme.LoginRightSideGradient = req.LoginRightSideGradient
		}
	}

	// CustomCSS: any string value sets it (including empty to clear)
	theme.CustomCSS = req.CustomCSS

	if err := h.db.Save(theme).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save theme settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"theme": theme})
}

// ProfilePanelContent returns the profile edit panel HTML for HTMX lazy loading
func (h *Handler) ProfilePanelContent(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Load full user from DB to get current name
	var dbUser models.User
	if err := h.db.First(&dbUser, "id = ?", user.ID).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to load user profile")
		return
	}

	props := partials.ProfilePanelProps{
		User:     &dbUser,
		BasePath: h.basePath,
	}

	h.RenderTempl(c, http.StatusOK, partials.ProfilePanel(props))
}

// UpdateProfile handles PUT request to update user profile (name)
func (h *Handler) UpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update user in database
	if err := h.db.Model(&models.User{}).Where("id = ?", user.ID).Update("name", req.Name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

// DebugUserOrgs returns debug information about user's organizations
func (h *Handler) DebugUserOrgs(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetCurrentOrg(c)

	h.logger.Debug("DebugUserOrgs",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
		zap.String("org_id", org.ID),
	)

	var orgs []models.NuonOrg
	if err := h.db.Where("org_id = ?", org.ID).Find(&orgs).Error; err != nil {
		h.logger.Error("DebugUserOrgs: error querying orgs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	h.logger.Debug("DebugUserOrgs: found organizations",
		zap.Int("count", len(orgs)),
		zap.String("org_id", org.ID),
	)

	// Format debug response
	debugOrgs := make([]gin.H, len(orgs))
	for i, org := range orgs {
		debugOrgs[i] = gin.H{
			"id":      org.ID,
			"name":    org.Name,
			"org_id":  org.NuonOrgID,
			"user_id": org.UserID,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":             user.ID,
		"user_email":          user.Email,
		"organizations_count": len(orgs),
		"organizations":       debugOrgs,
		"database_connection": "ok",
	})
}

// AppInfo represents basic app information for the apps list
type AppInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Platform        string `json:"platform"` // "aws" or "azure"
	LogoLightBase64 string `json:"logo_light_base64"`
	LogoDarkBase64  string `json:"logo_dark_base64"`
}

// orgHasPublishedApps checks if an org has any published apps
func (h *Handler) orgHasPublishedApps(orgID string) bool {
	var count int64
	h.db.Model(&models.PublishedApp{}).Where("org_id = ?", orgID).Count(&count)
	return count > 0
}

// AppsPage displays the vendor apps list
func (h *Handler) AppsPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		showCTA := nuon.IsUnauthorized(err)
		allOrgs := h.GetUserOrgs(user.ID)
		props := vendorpages.AppsPageProps{
			LayoutProps: vendorui.LayoutProps{
				Title:                org.Name + " - Apps",
				ActivePage:           "apps",
				User:                 user,
				CurrentOrg:           org,
				Orgs:                 allOrgs,
				Breadcrumbs:          []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: true}},
				BasePath:             h.basePath,
				PortalScheme:         h.schemeFromBaseURL(),
				DashboardURL:         h.dashboardURL,
				PortalBaseDomain:     h.subdomainBaseDomain,
				CSSPath:              assets.VendorCSSPath(),
				NuonAPIError:         err.Error(),
				NuonAPIShowUpdateCTA: showCTA,
			},
			Org: *org,
		}
		h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
		h.RenderTempl(c, http.StatusOK, vendorpages.AppsPage(props))
		return
	}

	// Fetch apps from Nuon API
	apps, err := nuonClient.ListApps(c.Request.Context())
	if err != nil {
		showCTA := nuon.IsUnauthorized(err)
		allOrgs := h.GetUserOrgs(user.ID)
		props := vendorpages.AppsPageProps{
			LayoutProps: vendorui.LayoutProps{
				Title:                org.Name + " - Apps",
				ActivePage:           "apps",
				User:                 user,
				CurrentOrg:           org,
				Orgs:                 allOrgs,
				Breadcrumbs:          []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: true}},
				BasePath:             h.basePath,
				PortalScheme:         h.schemeFromBaseURL(),
				DashboardURL:         h.dashboardURL,
				PortalBaseDomain:     h.subdomainBaseDomain,
				CSSPath:              assets.VendorCSSPath(),
				NuonAPIError:         err.Error(),
				NuonAPIShowUpdateCTA: showCTA,
			},
			Org: *org,
		}
		h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
		h.RenderTempl(c, http.StatusOK, vendorpages.AppsPage(props))
		return
	}

	// Get all published apps for this org
	var publishedApps []models.PublishedApp
	h.db.Where("org_id = ?", org.ID).Order("sort_order ASC, created_at ASC").Find(&publishedApps)
	publishedAppIDs := make(map[string]bool)
	sortOrderMap := make(map[string]int)
	logoLightMap := make(map[string]string)
	logoDarkMap := make(map[string]string)
	for _, pa := range publishedApps {
		publishedAppIDs[pa.AppID] = true
		sortOrderMap[pa.AppID] = pa.SortOrder
		if pa.LogoLightBase64 != "" {
			logoLightMap[pa.AppID] = pa.LogoLightBase64
		}
		if pa.LogoDarkBase64 != "" {
			logoDarkMap[pa.AppID] = pa.LogoDarkBase64
		}
	}

	// Build app list
	appsWithStatus := make([]AppInfo, len(apps))
	for i, app := range apps {
		// Extract platform from runner config (same as GetAppInputConfig)
		platform := "aws" // default
		if app.RunnerConfig != nil {
			runnerType := string(app.RunnerConfig.AppRunnerType)
			if runnerType == "azure" {
				platform = "azure"
			}
		}

		appsWithStatus[i] = AppInfo{
			ID:              app.ID,
			Name:            app.Name,
			Platform:        platform,
			LogoLightBase64: logoLightMap[app.ID],
			LogoDarkBase64:  logoDarkMap[app.ID],
		}
	}

	// Sort appsWithStatus by sort_order (published apps first, then unpublished)
	sort.SliceStable(appsWithStatus, func(i, j int) bool {
		oi, iHas := sortOrderMap[appsWithStatus[i].ID]
		oj, jHas := sortOrderMap[appsWithStatus[j].ID]
		if !iHas {
			oi = math.MaxInt32
		}
		if !jHas {
			oj = math.MaxInt32
		}
		return oi < oj
	})

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	// Convert to templ type
	templApps := make([]vendorpages.AppWithHealthCheckStatus, len(appsWithStatus))
	for i, app := range appsWithStatus {
		templApps[i] = vendorpages.AppWithHealthCheckStatus{
			ID:              app.ID,
			Name:            app.Name,
			Platform:        app.Platform,
			LogoLightBase64: app.LogoLightBase64,
			LogoDarkBase64:  app.LogoDarkBase64,
		}
	}

	props := vendorpages.AppsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            org.Name + " - Apps",
			ActivePage:       "apps",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Org:             *org,
		Apps:            templApps,
		PublishedAppIDs: publishedAppIDs,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.AppsPage(props))
}

// PublishApp publishes an app so customers can discover and install it without a link
func (h *Handler) PublishApp(c *gin.Context) {
	appID := c.Param("app_id")
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	// Verify app exists in Nuon API
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	_, err = nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
		return
	}

	// Find or create the PublishedApp record (idempotent)
	var publishedApp models.PublishedApp
	result := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp)
	if result.Error != nil {
		// Not found - create new
		publishedApp = models.PublishedApp{
			OrgID: org.ID,
			AppID: appID,
		}
		if err := h.db.Create(&publishedApp).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish app"})
			return
		}
	}

	c.JSON(http.StatusOK, publishedApp)
}

// UnpublishApp removes a published app so customers can no longer discover it
func (h *Handler) UnpublishApp(c *gin.Context) {
	appID := c.Param("app_id")
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	// Soft-delete the PublishedApp record
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).Delete(&models.PublishedApp{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpublish app"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "App unpublished successfully"})
}

// UpdateAppOrder updates the sort_order of published apps for an org
func (h *Handler) UpdateAppOrder(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	var body struct {
		AppIDs          []string `json:"app_ids"`
		PublishedAppIDs []string `json:"published_app_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	publishedSet := make(map[string]bool, len(body.PublishedAppIDs))
	for _, id := range body.PublishedAppIDs {
		publishedSet[id] = true
	}

	for i, appID := range body.AppIDs {
		if publishedSet[appID] {
			var pa models.PublishedApp
			result := h.db.Unscoped().Where("org_id = ? AND app_id = ?", org.ID, appID).First(&pa)
			updates := map[string]interface{}{"deleted_at": nil, "sort_order": i}
			if result.Error != nil {
				newPA := models.PublishedApp{OrgID: org.ID, AppID: appID, SortOrder: i}
				h.db.Create(&newPA)
			} else {
				h.db.Unscoped().Model(&pa).Updates(updates)
			}
		} else {
			h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).Delete(&models.PublishedApp{})
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated"})
}

// AppDetailRedirect redirects app detail to inputs page
func (h *Handler) AppDetailRedirect(c *gin.Context) {
	orgID := c.Param("org_id")
	appID := c.Param("app_id")
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/apps/%s/inputs", h.basePath, orgID, appID))
}

// AppInputsPage displays the input configuration for an app with vendor/customer tabs
func (h *Handler) AppInputsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	appID := c.Param("app_id")

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}
	orgID := org.ID

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		showCTA := nuon.IsUnauthorized(err)
		allOrgs := h.GetUserOrgs(user.ID)
		props := vendorpages.AppInputsPageProps{
			LayoutProps: vendorui.LayoutProps{
				Title:                "App - Inputs",
				ActivePage:           "apps",
				User:                 user,
				CurrentOrg:           org,
				Orgs:                 allOrgs,
				Breadcrumbs:          []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false}, {Text: appID, Active: true}},
				BasePath:             h.basePath,
				PortalScheme:         h.schemeFromBaseURL(),
				DashboardURL:         h.dashboardURL,
				PortalBaseDomain:     h.subdomainBaseDomain,
				CSSPath:              assets.VendorCSSPath(),
				NuonAPIError:         err.Error(),
				NuonAPIShowUpdateCTA: showCTA,
			},
			Org:   *org,
			AppID: appID,
			App:   vendorpages.AppInfo{ID: appID},
		}
		h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
		h.RenderTempl(c, http.StatusOK, vendorpages.AppInputsPage(props))
		return
	}

	// Fetch app details from Nuon API
	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		showCTA := nuon.IsUnauthorized(err)
		allOrgs := h.GetUserOrgs(user.ID)
		props := vendorpages.AppInputsPageProps{
			LayoutProps: vendorui.LayoutProps{
				Title:                "App - Inputs",
				ActivePage:           "apps",
				User:                 user,
				CurrentOrg:           org,
				Orgs:                 allOrgs,
				Breadcrumbs:          []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false}, {Text: appID, Active: true}},
				BasePath:             h.basePath,
				PortalScheme:         h.schemeFromBaseURL(),
				DashboardURL:         h.dashboardURL,
				PortalBaseDomain:     h.subdomainBaseDomain,
				CSSPath:              assets.VendorCSSPath(),
				NuonAPIError:         err.Error(),
				NuonAPIShowUpdateCTA: showCTA,
			},
			Org:   *org,
			AppID: appID,
			App:   vendorpages.AppInfo{ID: appID},
		}
		h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
		h.RenderTempl(c, http.StatusOK, vendorpages.AppInputsPage(props))
		return
	}

	// Fetch app input config from Nuon API
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appID)
	if err != nil {
		// Input config may not exist, that's okay - we'll show empty tables
		inputConfig = nil
	}

	// Fetch local customer input config
	var localConfig models.AppInputConfig
	h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&localConfig)
	customerInputNames := localConfig.GetCustomerInputNames()

	// Create a set for fast lookup
	customerInputSet := make(map[string]bool)
	for _, name := range customerInputNames {
		customerInputSet[name] = true
	}

	// Create a set for collapsed groups
	collapsedGroups := localConfig.GetCollapsedGroups()
	collapsedGroupSet := make(map[string]bool)
	for _, name := range collapsedGroups {
		collapsedGroupSet[name] = true
	}

	// Parse inputs into grouped structure
	var inputGroups []vendorpages.AppInputGroup

	if inputConfig != nil {
		// Convert the typed struct to JSON then back to map for flexible field access
		// This handles the case where user_configurable might exist in the API response
		// but not in the SDK struct
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				if apiGroups, ok := configMap["input_groups"].([]interface{}); ok {
					for _, group := range apiGroups {
						if groupMap, ok := group.(map[string]interface{}); ok {
							groupName, _ := groupMap["name"].(string)
							groupDisplayName, _ := groupMap["display_name"].(string)
							if groupDisplayName == "" {
								groupDisplayName = groupName
							}

							inputGroup := vendorpages.AppInputGroup{
								Name:             groupName,
								DisplayName:      groupDisplayName,
								Inputs:           []vendorpages.AppInputInfo{},
								CollapsedDefault: collapsedGroupSet[groupName],
							}

							if appInputs, ok := groupMap["app_inputs"].([]interface{}); ok {
								for _, input := range appInputs {
									if inputMap, ok := input.(map[string]interface{}); ok {
										inputName := getMapString(inputMap, "name")
										inputInfo := vendorpages.AppInputInfo{
											Name:           inputName,
											DisplayName:    getMapString(inputMap, "display_name"),
											Description:    getMapString(inputMap, "description"),
											Type:           getMapString(inputMap, "type"),
											Required:       getMapBool(inputMap, "required"),
											Sensitive:      getMapBool(inputMap, "sensitive"),
											Default:        getStringFromAny(inputMap["default"]),
											Source:         getMapString(inputMap, "source"),
											CustomerFacing: customerInputSet[inputName],
										}

										if inputInfo.DisplayName == "" {
											inputInfo.DisplayName = inputInfo.Name
										}
										if inputInfo.Type == "" {
											inputInfo.Type = "string"
										}

										inputGroup.Inputs = append(inputGroup.Inputs, inputInfo)
									}
								}
							}

							inputGroups = append(inputGroups, inputGroup)
						}
					}
				}
			}
		}
	}

	// Apply saved ordering
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	// Sort groups if ordering is saved
	if len(groupOrder) > 0 {
		groupOrderMap := make(map[string]int)
		for i, name := range groupOrder {
			groupOrderMap[name] = i
		}
		sort.SliceStable(inputGroups, func(i, j int) bool {
			orderI, okI := groupOrderMap[inputGroups[i].Name]
			orderJ, okJ := groupOrderMap[inputGroups[j].Name]
			if !okI && !okJ {
				return false // Keep original order for unordered items
			}
			if !okI {
				return false // Unordered items go after ordered ones
			}
			if !okJ {
				return true // Ordered items go before unordered ones
			}
			return orderI < orderJ
		})
	}

	// Sort inputs within each group if ordering is saved
	for i := range inputGroups {
		groupName := inputGroups[i].Name
		if order, ok := inputOrder[groupName]; ok && len(order) > 0 {
			inputOrderMap := make(map[string]int)
			for j, name := range order {
				inputOrderMap[name] = j
			}
			sort.SliceStable(inputGroups[i].Inputs, func(a, b int) bool {
				orderA, okA := inputOrderMap[inputGroups[i].Inputs[a].Name]
				orderB, okB := inputOrderMap[inputGroups[i].Inputs[b].Name]
				if !okA && !okB {
					return false
				}
				if !okA {
					return false
				}
				if !okB {
					return true
				}
				return orderA < orderB
			})
		}
	}

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.AppInputsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      app.Name + " - Inputs",
			ActivePage: "apps",
			User:       user,
			CurrentOrg: org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: org.Name, Path: fmt.Sprintf("%s/orgs/%s", h.basePath, org.ID), Active: false},
				{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false},
				{Text: app.Name, Path: fmt.Sprintf("%s/orgs/%s/apps/%s", h.basePath, org.ID, appID), Active: false},
				{Text: "Inputs", Path: "", Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Org:   *org,
		AppID: appID,
		App: vendorpages.AppInfo{
			ID:   app.ID,
			Name: app.Name,
		},
		InputGroups: inputGroups,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.AppInputsPage(props))
}

// getMapString safely gets a string from a map[string]interface{}
func getMapString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getMapBool safely gets a bool from a map[string]interface{}
func getMapBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getStringFromAny converts various types to string
func getStringFromAny(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// AppLogoPage displays the app logo upload page
func (h *Handler) AppLogoPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	appID := c.Param("app_id")

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	// Load app name from Nuon API (best effort)
	appInfo := vendorpages.AppInfo{ID: appID}
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err == nil {
		if app, err := nuonClient.GetApp(c.Request.Context(), appID); err == nil && app != nil {
			appInfo.Name = app.Name
		}
	}

	// Load existing logos from PublishedApp record (may not exist yet)
	var publishedApp models.PublishedApp
	h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp)

	allOrgs := h.GetUserOrgs(user.ID)
	props := vendorpages.AppLogoPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            "App - Logo",
			ActivePage:       "apps",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false}, {Text: appID, Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Org:             *org,
		App:             appInfo,
		LogoLightBase64: publishedApp.LogoLightBase64,
		LogoDarkBase64:  publishedApp.LogoDarkBase64,
	}
	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.AppLogoPage(props))
}

// UpdateAppLogo saves light and dark logos for a published app
func (h *Handler) UpdateAppLogo(c *gin.Context) {
	appID := c.Param("app_id")

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	var req struct {
		LogoLightBase64 string `json:"logo_light_base64"`
		LogoDarkBase64  string `json:"logo_dark_base64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Find or create the PublishedApp record
	var pa models.PublishedApp
	result := h.db.Unscoped().Where("org_id = ? AND app_id = ?", org.ID, appID).First(&pa)
	if result.Error != nil {
		pa = models.PublishedApp{OrgID: org.ID, AppID: appID}
	}

	if req.LogoLightBase64 == "REMOVE" {
		pa.LogoLightBase64 = ""
	} else if strings.HasPrefix(req.LogoLightBase64, "data:image/") {
		pa.LogoLightBase64 = req.LogoLightBase64
	}

	if req.LogoDarkBase64 == "REMOVE" {
		pa.LogoDarkBase64 = ""
	} else if strings.HasPrefix(req.LogoDarkBase64, "data:image/") {
		pa.LogoDarkBase64 = req.LogoDarkBase64
	}

	if result.Error != nil {
		h.db.Create(&pa)
	} else {
		h.db.Unscoped().Model(&pa).Updates(map[string]interface{}{
			"logo_light_base64": pa.LogoLightBase64,
			"logo_dark_base64":  pa.LogoDarkBase64,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// LoginSettingsPage renders the login settings page
func (h *Handler) LoginSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	// Get or create the customer auth config
	config, err := models.GetOrCreateCustomerAuthConfig(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load login settings")
		return
	}

	// Get current org from middleware context (set by RequireOrgAccess)
	currentOrg := middleware.GetCurrentOrg(c)

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	// Load theme for login page settings (theme data is passed to the page, not for vendor UI styling)
	theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))

	// Build breadcrumb path with org ID
	portalBasePath := h.basePath + "/orgs/" + currentOrg.ID + "/portal"

	props := vendorpages.LoginSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Login",
			ActivePage: "portal-login",
			User:       user,
			CurrentOrg: currentOrg,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Customer Portal", Path: portalBasePath + "/branding"},
				{Text: "Login", Path: portalBasePath + "/login", Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Config: config,
		Theme:  theme,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.LoginSettingsPage(props))
}

// UpdateLoginSettings handles PUT request to update login settings
func (h *Handler) UpdateLoginSettings(c *gin.Context) {
	var req struct {
		Enabled                       bool   `json:"enabled"`
		ProviderName                  string `json:"provider_name"`
		ClientID                      string `json:"client_id"`
		ClientSecret                  string `json:"client_secret"`
		IssuerURL                     string `json:"issuer_url"`
		Scopes                        string `json:"scopes"`
		LoginTitle                    string `json:"login_title"`
		LoginSubtitle                 string `json:"login_subtitle"`
		LoginRightSideImageBase64     string `json:"login_right_side_image_base64"`
		LoginRightSideGradient        string `json:"login_right_side_gradient"`
		LoginRightSideImageBase64Dark string `json:"login_right_side_image_base64_dark"`
		LoginRightSideGradientDark    string `json:"login_right_side_gradient_dark"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get or create the customer auth config
	config, err := models.GetOrCreateCustomerAuthConfig(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load login settings"})
		return
	}

	// Update auth fields
	config.Enabled = req.Enabled
	config.ProviderName = req.ProviderName
	config.ClientID = req.ClientID
	config.IssuerURL = req.IssuerURL

	// Only update client secret if provided (not empty)
	// This allows keeping the existing secret when not changing it
	if req.ClientSecret != "" {
		config.ClientSecret = req.ClientSecret
	}

	// Update scopes, using default if empty
	if req.Scopes != "" {
		config.Scopes = req.Scopes
	} else {
		config.Scopes = models.DefaultScopes
	}

	if err := h.db.Save(config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save login settings"})
		return
	}

	// Update login page appearance in theme
	theme, err := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
	if err == nil {
		theme.LoginTitle = req.LoginTitle
		theme.LoginSubtitle = req.LoginSubtitle

		// Handle login right side image - "REMOVE" clears, valid data URI sets
		if req.LoginRightSideImageBase64 != "" {
			if req.LoginRightSideImageBase64 == "REMOVE" {
				theme.LoginRightSideImageBase64 = ""
			} else if strings.HasPrefix(req.LoginRightSideImageBase64, "data:image/") {
				theme.LoginRightSideImageBase64 = req.LoginRightSideImageBase64
			}
		}

		// Handle login right side gradient - always update (matches dark mode color pattern)
		// Valid CSS gradient sets it, empty string clears it
		if req.LoginRightSideGradient != "" && strings.HasPrefix(req.LoginRightSideGradient, "linear-gradient") {
			theme.LoginRightSideGradient = req.LoginRightSideGradient
		} else {
			// Clear gradient (falls back to primary/secondary colors)
			theme.LoginRightSideGradient = ""
		}

		// Handle dark mode login right side image
		if req.LoginRightSideImageBase64Dark != "" {
			if req.LoginRightSideImageBase64Dark == "REMOVE" {
				theme.LoginRightSideImageBase64Dark = ""
			} else if strings.HasPrefix(req.LoginRightSideImageBase64Dark, "data:image/") {
				theme.LoginRightSideImageBase64Dark = req.LoginRightSideImageBase64Dark
			}
		}

		// Handle dark mode login right side gradient
		if req.LoginRightSideGradientDark != "" && strings.HasPrefix(req.LoginRightSideGradientDark, "linear-gradient") {
			theme.LoginRightSideGradientDark = req.LoginRightSideGradientDark
		} else {
			theme.LoginRightSideGradientDark = ""
		}

		h.db.Save(theme)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config": gin.H{
			"id":                config.ID,
			"enabled":           config.Enabled,
			"provider_name":     config.ProviderName,
			"client_id":         config.ClientID,
			"issuer_url":        config.IssuerURL,
			"scopes":            config.Scopes,
			"has_client_secret": config.HasClientSecret(),
		},
	})
}

// TestLoginConnection tests the OIDC connection with current settings
func (h *Handler) TestLoginConnection(c *gin.Context) {
	// Get the current config
	config, err := models.GetOrCreateCustomerAuthConfig(h.db, h.getOrgIDForTheme(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load login settings"})
		return
	}

	// Check if config is complete enough to test
	if !config.IsConfigured() {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "OIDC is not fully configured. Please provide Client ID, Client Secret, and Issuer URL.",
		})
		return
	}

	// Try to create an OIDC provider to test the connection
	// This will perform OIDC discovery and validate the configuration
	providerConfig := auth.ProviderConfig{
		Type:         auth.ProviderTypeOIDC,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		IssuerURL:    config.IssuerURL,
		RedirectURI:  h.customerBaseURL + "/auth/callback",
		Scopes:       config.GetScopes(),
	}

	_, err = auth.NewOIDCProvider(providerConfig, h.db)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to connect to OIDC provider: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully connected to OIDC provider",
	})
}

// GetAppCustomerInputConfig returns the local customer-facing input configuration for an app
func (h *Handler) GetAppCustomerInputConfig(c *gin.Context) {
	appID := c.Param("app_id")

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}
	orgID := org.ID

	// Fetch or create the AppInputConfig
	var config models.AppInputConfig
	result := h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&config)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Return empty config if none exists
			c.JSON(http.StatusOK, gin.H{
				"customer_input_names": []string{},
				"group_order":          []string{},
				"input_order":          map[string][]string{},
				"collapsed_groups":     []string{},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch input config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"customer_input_names": config.GetCustomerInputNames(),
		"group_order":          config.GetGroupOrder(),
		"input_order":          config.GetInputOrder(),
		"collapsed_groups":     config.GetCollapsedGroups(),
	})
}

// UpdateAppCustomerInputConfig updates the local customer-facing input configuration for an app
func (h *Handler) UpdateAppCustomerInputConfig(c *gin.Context) {
	appID := c.Param("app_id")

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}
	orgID := org.ID

	// Parse request body
	var req struct {
		CustomerInputNames []string            `json:"customer_input_names"`
		GroupOrder         []string            `json:"group_order"`
		InputOrder         map[string][]string `json:"input_order"`
		CollapsedGroups    []string            `json:"collapsed_groups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Find or create the AppInputConfig
	var config models.AppInputConfig
	result := h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&config)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create new config
			config = models.AppInputConfig{
				OrgID: orgID,
				AppID: appID,
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch input config"})
			return
		}
	}

	// Update customer input names
	if err := config.SetCustomerInputNames(req.CustomerInputNames); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set customer input names"})
		return
	}

	// Update group order
	if err := config.SetGroupOrder(req.GroupOrder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set group order"})
		return
	}

	// Update input order
	if err := config.SetInputOrder(req.InputOrder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set input order"})
		return
	}

	// Update collapsed groups
	if err := config.SetCollapsedGroups(req.CollapsedGroups); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set collapsed groups"})
		return
	}

	// Save the config
	if result.Error == gorm.ErrRecordNotFound {
		if err := h.db.Create(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create input config"})
			return
		}
	} else {
		if err := h.db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update input config"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"customer_input_names": config.GetCustomerInputNames(),
		"group_order":          config.GetGroupOrder(),
		"input_order":          config.GetInputOrder(),
		"collapsed_groups":     config.GetCollapsedGroups(),
	})
}

// CustomersPage displays all customers who have installed apps from this org
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
			Title:            org.Name + " - Customers",
			ActivePage:       "customers",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Customers", Path: fmt.Sprintf("%s/orgs/%s/customers", h.basePath, org.ID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Org:         *org,
		Customers:   customers,
		SearchQuery: searchQuery,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.CustomersPage(props))
}

// CustomerDetailPage displays details for a specific customer and their installs
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
func (h *Handler) CustomerInstallsPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	filterEmail := c.Query("email")
	filterApp := c.Query("app")
	filterPlatform := c.Query("platform")

	// Build query for installs joined with their owning user
	type InstallRow struct {
		ID                 string
		NuonInstallID      string
		NuonOrgID          string
		Name               string
		UserID             string
		CustomerEmail      string
		CustomerName       string
		AppID              string
		AppName            string
		Status             string
		Region             string
		CreatedAt          time.Time
		InstallLinkID      string
		InstallLinkAppID   string
		InstallLinkAppName string
	}

	query := h.db.Table("installs").
		Select(`installs.id, installs.nuon_install_id, installs.user_id,
			installs.name, installs.app_id, installs.app_name,
			installs.status, installs.region, installs.created_at,
			COALESCE(installs.install_link_id::text, '') as install_link_id,
			users.email as customer_email, users.name as customer_name,
			COALESCE(install_links.app_id, '') as install_link_app_id,
			COALESCE(install_links.app_name, '') as install_link_app_name`).
		Joins("JOIN users ON users.id = installs.user_id").
		Joins("LEFT JOIN install_links ON install_links.id = installs.install_link_id").
		Where("installs.org_id = ? AND installs.deleted_at IS NULL", org.ID).
		Order("installs.created_at DESC")

	if filterEmail != "" {
		query = query.Where("users.email ILIKE ?", "%"+filterEmail+"%")
	}
	if filterApp != "" {
		pattern := "%" + filterApp + "%"
		query = query.Where("(installs.app_name ILIKE ? OR install_links.app_name ILIKE ?)", pattern, pattern)
	}

	var rows []InstallRow
	if err := query.Find(&rows).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch installs: %v", err))
		return
	}

	// Fetch apps from Nuon API to build platform map
	platformMap := make(map[string]string)
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err == nil {
		apps, appsErr := nuonClient.ListApps(c.Request.Context())
		if appsErr == nil {
			for _, app := range apps {
				platform := "aws"
				if app.RunnerConfig != nil && string(app.RunnerConfig.AppRunnerType) == "azure" {
					platform = "azure"
				}
				platformMap[app.ID] = platform
			}
		}
	}

	// Convert rows to AdminInstall, applying platform filter in Go
	adminInstalls := make([]vendorpages.AdminInstall, 0, len(rows))
	for _, row := range rows {
		// Resolve app ID and name
		effectiveAppID := row.AppID
		if effectiveAppID == "" {
			effectiveAppID = row.InstallLinkAppID
		}
		effectiveAppName := row.AppName
		if effectiveAppName == "" {
			effectiveAppName = row.InstallLinkAppName
		}
		if effectiveAppName == "" {
			effectiveAppName = "Unknown App"
		}

		// Resolve platform
		platform := "aws"
		if p, ok := platformMap[effectiveAppID]; ok {
			platform = p
		}

		// Apply platform filter
		if filterPlatform != "" && platform != filterPlatform {
			continue
		}

		adminInstalls = append(adminInstalls, vendorpages.AdminInstall{
			ID:            row.ID,
			NuonInstallID: row.NuonInstallID,
			NuonOrgID:     org.NuonOrgID,
			Name:          row.Name,
			CustomerID:    row.UserID,
			CustomerEmail: row.CustomerEmail,
			CustomerName:  row.CustomerName,
			AppID:         effectiveAppID,
			AppName:       effectiveAppName,
			Platform:      platform,
			Status:        row.Status,
			InstallLinkID: row.InstallLinkID,
			CreatedAt:     row.CreatedAt.Format("Jan 2, 2006"),
		})
	}

	// Return partial for HTMX filter requests
	if isHTMXRequest(c) {
		h.RenderTempl(c, http.StatusOK, vendorpages.InstallsTableBody(adminInstalls, org.ID, h.basePath, org.NuonOrgID))
		return
	}

	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.CustomerInstallsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      org.Name + " - Installs",
			ActivePage: "installs",
			User:       user,
			CurrentOrg: org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Installs", Path: fmt.Sprintf("%s/orgs/%s/installs", h.basePath, org.ID), Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Org:            *org,
		Installs:       adminInstalls,
		FilterEmail:    filterEmail,
		FilterApp:      filterApp,
		FilterPlatform: filterPlatform,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.CustomerInstallsPage(props))
}

// SearchNuonInstalls searches the Nuon API for installs matching a query string.
// Returns an HTMX HTML fragment of result rows for the import modal.
func (h *Handler) SearchNuonInstalls(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.String(http.StatusBadRequest, "Organization context not found")
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 3 {
		c.String(http.StatusOK, "")
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	apps, err := nuonClient.ListApps(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch apps")
		return
	}

	type searchResult struct {
		InstallID   string
		InstallName string
		AppID       string
		AppName     string
		Status      string
		Region      string
	}

	var results []searchResult
	for _, app := range apps {
		if len(results) >= 20 {
			break
		}
		installs, err := nuonClient.ListAppInstalls(c.Request.Context(), app.ID, q)
		if err != nil {
			zap.L().Warn("failed to search installs for app",
				zap.String("app_id", app.ID),
				zap.Error(err),
			)
			continue
		}
		for _, install := range installs {
			if len(results) >= 20 {
				break
			}
			installName := install.Name
			status := install.Status
			region := ""
			if install.AwsAccount != nil && install.AwsAccount.Region != "" {
				region = install.AwsAccount.Region
			} else if install.AzureAccount != nil && install.AzureAccount.Location != "" {
				region = install.AzureAccount.Location
			}
			results = append(results, searchResult{
				InstallID:   install.ID,
				InstallName: installName,
				AppID:       app.ID,
				AppName:     app.Name,
				Status:      status,
				Region:      region,
			})
		}
	}

	// Build HTML fragment
	if len(results) == 0 {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="px-4 py-3 text-sm text-cool-grey-500 dark:text-cool-grey-400">No installs found.</div>`))
		return
	}

	html := ""
	for _, r := range results {
		displayText := r.InstallName + "  •  " + r.AppName
		if r.Status != "" {
			displayText += "  •  " + r.Status
		}
		html += fmt.Sprintf(`<div class="px-4 py-3 hover:bg-cool-grey-50 dark:hover:bg-dark-grey-800">
			<div class="flex items-center justify-between gap-3">
				<div>
					<div class="text-sm font-medium text-cool-grey-900 dark:text-white">%s</div>
					<div class="text-xs text-cool-grey-500 dark:text-cool-grey-400">%s &middot; %s</div>
				</div>
				<button
					type="button"
					onclick="selectImportInstall(%q, %q, %q)"
					class="shrink-0 text-xs font-medium text-primary-600 dark:text-primary-400 hover:underline"
				>
					Select &rarr;
				</button>
			</div>
		</div>`,
			r.InstallName, r.AppName, r.Region,
			r.InstallID, r.AppID, displayText,
		)
	}

	c.Data(http.StatusOK, "text/html", []byte(html))
}

// ImportInstall imports an existing Nuon install into the portal and assigns it to a customer.
func (h *Handler) ImportInstall(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.String(http.StatusBadRequest, "Organization context not found")
		return
	}

	vendor := h.GetFreshUser(c)

	nuonInstallID := strings.TrimSpace(c.PostForm("nuon_install_id"))
	appID := strings.TrimSpace(c.PostForm("app_id"))
	customerEmail := strings.TrimSpace(c.PostForm("customer_email"))

	if nuonInstallID == "" || customerEmail == "" {
		c.String(http.StatusBadRequest, "Install ID and customer email are required.")
		return
	}

	// Check if already imported
	var existing models.Install
	if err := h.db.Where("nuon_install_id = ? AND org_id = ?", nuonInstallID, org.ID).First(&existing).Error; err == nil {
		c.String(http.StatusConflict, "This install has already been imported.")
		return
	}

	// Fetch install from Nuon API
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client.")
		return
	}

	nuonInstall, err := nuonClient.GetInstall(c.Request.Context(), nuonInstallID)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Install not found in Nuon API: %v", err))
		return
	}

	// Fetch app name from Nuon API
	appName := ""
	if appID != "" {
		nuonApp, appErr := nuonClient.GetApp(c.Request.Context(), appID)
		if appErr == nil {
			appName = nuonApp.Name
		}
	}

	// Resolve install name, region and status
	installName := nuonInstall.Name
	region := ""
	if nuonInstall.AwsAccount != nil && nuonInstall.AwsAccount.Region != "" {
		region = nuonInstall.AwsAccount.Region
	} else if nuonInstall.AzureAccount != nil && nuonInstall.AzureAccount.Location != "" {
		region = nuonInstall.AzureAccount.Location
	}
	installStatus := models.StatusActive
	switch nuonInstall.Status {
	case "provisioning":
		installStatus = models.StatusProvisioning
	case "failed":
		installStatus = models.StatusFailed
	case "deprovisioning":
		installStatus = models.StatusDeprovisioning
	}

	// Find or create customer user
	var customerUser models.User
	if err := h.db.Where("email = ?", customerEmail).First(&customerUser).Error; err != nil {
		// Create new customer record
		customerUser = models.User{
			Email: customerEmail,
			Name:  customerEmail,
			Role:  models.RoleCustomer,
		}
		if createErr := h.db.Create(&customerUser).Error; createErr != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create customer: %v", createErr))
			return
		}
	}

	// Create local install record
	vendorID := vendor.ID
	install := models.Install{
		OrgID:             org.ID,
		UserID:            customerUser.ID,
		CreatedByVendorID: &vendorID,
		InstallLinkID:     nil,
		NuonInstallID:     nuonInstallID,
		AppID:             appID,
		AppName:           appName,
		Name:              installName,
		Status:            installStatus,
		Region:            region,
	}
	if err := h.db.Create(&install).Error; err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to save install: %v", err))
		return
	}

	// Redirect to installs page
	redirectURL := fmt.Sprintf("%s/orgs/%s/installs", h.basePath, org.ID)
	if isHTMXRequest(c) {
		c.Header("HX-Redirect", redirectURL)
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusFound, redirectURL)
}
