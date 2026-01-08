package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/pages"
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
	db              *gorm.DB
	auth            *jwt.GinJWTMiddleware
	authProvider    auth.AuthProvider // Authentication provider (OIDC, SAML)
	customerBaseURL string            // Base URL for customer-facing install links
	nuonAPIURL      string            // Global Nuon API URL for all orgs
	basePath        string            // Base path prefix for routes (e.g., "/admin" or "" for root)
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

func NewHandler(db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, authProvider auth.AuthProvider, customerBaseURL, nuonAPIURL, basePath string) *Handler {
	return &Handler{
		db:              db,
		auth:            jwtAuth,
		authProvider:    authProvider,
		customerBaseURL: customerBaseURL,
		nuonAPIURL:      nuonAPIURL,
		basePath:        basePath,
	}
}

// RenderTempl renders a Templ component to the response
func (h *Handler) RenderTempl(c *gin.Context, status int, component templ.Component) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	if err := component.Render(c.Request.Context(), c.Writer); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// RenderErrorPage renders a templ error page
func (h *Handler) RenderErrorPage(c *gin.Context, status int, errorMsg string) {
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor := ""
	if theme != nil {
		primaryColor = theme.PrimaryColor
	}

	props := vendorpages.ErrorPageProps{
		Error:        errorMsg,
		BasePath:     h.basePath,
		PrimaryColor: primaryColor,
	}

	h.RenderTempl(c, status, vendorpages.ErrorPage(props))
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
func (h *Handler) getThemeData() gin.H {
	theme, err := models.GetOrCreateAppTheme(h.db)
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

// CustomerLoginPageTempl renders the customer login page using Templ
func (h *Handler) CustomerLoginPageTempl(c *gin.Context) {
	theme, _ := models.GetOrCreateAppTheme(h.db)

	props := customerpages.LoginPageProps{
		Title:       "Customer Login",
		ButtonText:  "Login",
		HelpText:    "Don't have an account? Accept an install link from your vendor to get started.",
		BasePath:    h.basePath,
		RedirectURL: h.basePath + "/installs",
		Theme:       theme,
	}

	h.RenderTempl(c, http.StatusOK, customerpages.LoginPage(props))
}

// VendorLoginPageTempl renders the vendor login page using Templ
func (h *Handler) VendorLoginPageTempl(c *gin.Context) {
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	// Determine auth mode and get auth URL
	authURL := ""
	useLocalAuth := false

	if h.authProvider != nil {
		if h.authProvider.Name() == "local" {
			// Local password auth - show email/password form
			useLocalAuth = true
		} else {
			// External IdP - generate state for CSRF protection
			state, err := auth.GenerateState()
			if err != nil {
				h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to generate security token")
				return
			}

			// Store state in cookie for validation on callback
			c.SetCookie("auth_state", state, 600, "/", "", false, true)

			authURL, err = h.authProvider.GetAuthorizationURL(state)
			if err != nil {
				h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to generate login URL")
				return
			}
		}
	}

	// Check for error message in query params (from failed login/register)
	errorMsg := c.Query("error")

	props := vendorpages.LoginPageProps{
		Title:              "Vendor Login",
		ButtonText:         "Login / Sign Up",
		HelpText:           "",
		BasePath:           h.basePath,
		AuthURL:            authURL,
		UseLocalAuth:       useLocalAuth,
		Error:              errorMsg,
		PrimaryColor:       primaryColor,
		PrimaryColorDark:   primaryColorDark,
		SecondaryColor:     secondaryColor,
		SecondaryColorDark: secondaryColorDark,
		HeadingFont:        theme.HeadingFont,
		BodyFont:           theme.BodyFont,
		HeadingFontBase64:  theme.HeadingFontBase64,
		BodyFontBase64:     theme.BodyFontBase64,
		LogoBase64:         theme.LogoBase64,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.LoginPage(props))
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

	// Redirect to the orgs page
	c.Redirect(http.StatusFound, h.basePath+"/orgs")
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

	// Redirect to the orgs page
	c.Redirect(http.StatusFound, h.basePath+"/orgs")
}

// VendorRegisterPageTempl renders the vendor registration page using Templ
func (h *Handler) VendorRegisterPageTempl(c *gin.Context) {
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	// Check for error message in query params
	errorMsg := c.Query("error")

	props := vendorpages.RegisterPageProps{
		Title:              "Create Account",
		ButtonText:         "Sign Up",
		BasePath:           h.basePath,
		Error:              errorMsg,
		PrimaryColor:       primaryColor,
		PrimaryColorDark:   primaryColorDark,
		SecondaryColor:     secondaryColor,
		SecondaryColorDark: secondaryColorDark,
		HeadingFont:        theme.HeadingFont,
		BodyFont:           theme.BodyFont,
		HeadingFontBase64:  theme.HeadingFontBase64,
		BodyFontBase64:     theme.BodyFontBase64,
		LogoBase64:         theme.LogoBase64,
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

// OrgsPage renders the orgs page for vendors using Templ
// If user has orgs, redirects to first org's links page
// Otherwise shows the connect org form
func (h *Handler) OrgsPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display

	var orgs []models.NuonOrg
	if err := h.db.Where("user_id = ?", user.ID).Find(&orgs).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load organizations")
		return
	}

	// If user has orgs, redirect to first org's links page
	if len(orgs) > 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/links", h.basePath, orgs[0].ID))
		return
	}

	// Show the connect org page (user has no orgs yet)
	// Load theme for styling
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	props := vendorpages.OrgsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:              "Connect Organization",
			ActivePage:         "orgs",
			User:               user,
			CurrentOrg:         nil,
			Orgs:               orgs,
			Breadcrumbs:        []partials.Breadcrumb{},
			BasePath:           h.basePath,
			PrimaryColor:       primaryColor,
			PrimaryColorDark:   primaryColorDark,
			SecondaryColor:     secondaryColor,
			SecondaryColorDark: secondaryColorDark,
			HeadingFont:        theme.HeadingFont,
			BodyFont:           theme.BodyFont,
			HeadingFontBase64:  theme.HeadingFontBase64,
			BodyFontBase64:     theme.BodyFontBase64,
			LogoBase64:         theme.LogoBase64,
		},
		Orgs: orgs,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.OrgsPage(props))
}

// CreateOrg handles POST request to create/connect a new org
func (h *Handler) CreateOrg(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		OrgID    string `json:"org_id" binding:"required"`
		APIToken string `json:"api_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate API token with Nuon API using global API URL
	nuonClient, err := nuon.NewClientWithURL(req.APIToken, req.OrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	if err := nuonClient.ValidateOrgAccess(c.Request.Context()); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API token or organization access"})
		return
	}

	// Fetch org name from Nuon API
	nuonOrg, err := nuonClient.GetOrg(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch organization details from Nuon API"})
		return
	}

	orgName := nuonOrg.Name
	if orgName == "" {
		orgName = req.OrgID // Fallback to org ID if name is empty
	}

	org := models.NuonOrg{
		UserID:    user.ID,
		NuonOrgID: req.OrgID,
		APIToken:  req.APIToken,
		Name:      orgName,
	}

	if err := h.db.Create(&org).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"org": org})
}

// UpdateOrg handles PUT request to update org settings (name, api_token)
func (h *Handler) UpdateOrg(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	orgID := c.Param("org_id")

	if !shortid.IsValid(orgID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Find org owned by user
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var req struct {
		APIToken string `json:"api_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update API token if provided
	if req.APIToken != "" {
		org.APIToken = req.APIToken
	}

	if err := h.db.Save(&org).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update organization"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"org": org})
}

// OrgSettingsPage renders the org settings page using Templ
func (h *Handler) OrgSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	orgID := c.Param("org_id")

	if !shortid.IsValid(orgID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Organization not found")
		return
	}

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("user_id = ?", user.ID).Find(&allOrgs)

	// Load theme for styling
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	props := vendorpages.OrgSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:              org.Name + " - Settings",
			ActivePage:         "settings",
			User:               user,
			CurrentOrg:         &org,
			Orgs:               allOrgs,
			Breadcrumbs:        []partials.Breadcrumb{{Text: "Links", Path: fmt.Sprintf("%s/orgs/%s/links", h.basePath, org.ID), Active: false}, {Text: "Settings", Path: fmt.Sprintf("%s/orgs/%s/settings", h.basePath, org.ID), Active: true}},
			BasePath:           h.basePath,
			PrimaryColor:       primaryColor,
			PrimaryColorDark:   primaryColorDark,
			SecondaryColor:     secondaryColor,
			SecondaryColorDark: secondaryColorDark,
			HeadingFont:        theme.HeadingFont,
			BodyFont:           theme.BodyFont,
			HeadingFontBase64:  theme.HeadingFontBase64,
			BodyFontBase64:     theme.BodyFontBase64,
			LogoBase64:         theme.LogoBase64,
		},
		Org: org,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.OrgSettingsPage(props))
}

// OrgDetailPage renders the org detail page with paginated install links using Templ
func (h *Handler) OrgDetailPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display
	orgID := c.Param("org_id")

	if !shortid.IsValid(orgID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Organization not found")
		return
	}

	// Tab and pagination parameters
	const linksPerPage = 10
	currentTab := c.DefaultQuery("tab", "available")
	page := getPageFromQuery(c)
	offset := (page - 1) * linksPerPage

	// Build base query for tab filtering
	baseQuery := h.db.Where("org_id = ? AND user_id = ?", orgID, user.ID)
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
	if err := h.db.Model(&models.InstallLink{}).Where("org_id = ? AND user_id = ? AND used = ?", orgID, user.ID, false).Count(&availableCount).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to count available install links")
		return
	}
	if err := h.db.Model(&models.InstallLink{}).Where("org_id = ? AND user_id = ? AND used = ?", orgID, user.ID, true).Count(&usedCount).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to count used install links")
		return
	}

	// Get paginated links for current tab
	var links []models.InstallLink
	query := h.db.Preload("Install").Where("org_id = ? AND user_id = ?", orgID, user.ID)
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

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("user_id = ?", user.ID).Find(&allOrgs)

	// Load theme for styling
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	props := vendorpages.OrgDetailPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:              org.Name + " - Install Links",
			ActivePage:         "links",
			User:               user,
			CurrentOrg:         &org,
			Orgs:               allOrgs,
			Breadcrumbs:        []partials.Breadcrumb{{Text: "Links", Path: fmt.Sprintf("%s/orgs/%s/links", h.basePath, org.ID), Active: true}},
			BasePath:           h.basePath,
			PrimaryColor:       primaryColor,
			PrimaryColorDark:   primaryColorDark,
			SecondaryColor:     secondaryColor,
			SecondaryColorDark: secondaryColorDark,
			HeadingFont:        theme.HeadingFont,
			BodyFont:           theme.BodyFont,
			HeadingFontBase64:  theme.HeadingFontBase64,
			BodyFontBase64:     theme.BodyFontBase64,
			LogoBase64:         theme.LogoBase64,
		},
		Org:   org,
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
	orgID := c.Param("org_id")

	if !shortid.IsValid(orgID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Verify user owns the org
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var req struct {
		AppID   string            `json:"app_id" binding:"required"`
		AppName string            `json:"app_name" binding:"required"`
		Inputs  map[string]string `json:"inputs"` // Vendor-facing inputs (stored for later)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		OrgID:   orgID,
		AppID:   req.AppID,
		AppName: req.AppName,
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

	// Auto-apply health checks from app config (if configured)
	var healthConfig models.AppHealthCheckConfig
	if err := h.db.Where("app_id = ?", req.AppID).First(&healthConfig).Error; err == nil {
		// Found an app health check config, use those IDs
		healthCheckIDs := healthConfig.GetHealthCheckActionIDs()
		if len(healthCheckIDs) > 0 {
			link.SetHealthCheckActionIDs(healthCheckIDs)
			fmt.Printf("Auto-applied %d health checks from app config for app %s\n", len(healthCheckIDs), req.AppID)
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
	orgID := c.Param("org_id")
	linkID := c.Param("link_id")

	if !shortid.IsValid(orgID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	if !shortid.IsValid(linkID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid link ID")
		return
	}

	var link models.InstallLink
	if err := h.db.Preload("Install").Preload("NuonOrg").Where("id = ? AND org_id = ? AND user_id = ?", linkID, orgID, user.ID).First(&link).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Install link not found")
		return
	}

	installURL := link.GetInstallURL(h.customerBaseURL)

	// Get org for breadcrumb and sidebar
	var org models.NuonOrg
	h.db.Where("id = ?", orgID).First(&org)

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("user_id = ?", user.ID).Find(&allOrgs)

	// Load theme for styling
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	// Determine breadcrumb text (install name or app name)
	breadcrumbText := link.AppName
	if link.Install != nil && link.Install.Name != "" {
		breadcrumbText = link.Install.Name
	}

	props := vendorpages.LinkDetailPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:              "Install - " + link.AppName,
			ActivePage:         "links",
			User:               user,
			CurrentOrg:         &org,
			Orgs:               allOrgs,
			Breadcrumbs:        []partials.Breadcrumb{{Text: "Links", Path: fmt.Sprintf("%s/orgs/%s/links", h.basePath, orgID), Active: false}, {Text: breadcrumbText, Path: fmt.Sprintf("%s/orgs/%s/links/%s", h.basePath, orgID, linkID), Active: true}},
			BasePath:           h.basePath,
			PrimaryColor:       primaryColor,
			PrimaryColorDark:   primaryColorDark,
			SecondaryColor:     secondaryColor,
			SecondaryColorDark: secondaryColorDark,
			HeadingFont:        theme.HeadingFont,
			BodyFont:           theme.BodyFont,
			HeadingFontBase64:  theme.HeadingFontBase64,
			BodyFontBase64:     theme.BodyFontBase64,
			LogoBase64:         theme.LogoBase64,
		},
		Link:       &link,
		InstallURL: installURL,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.LinkDetailPage(props))
}

// InstallLinkStatus returns the link status partial for HTMX polling using Templ
// This enables real-time updates when a customer accepts an install link
func (h *Handler) InstallLinkStatus(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	orgID := c.Param("org_id")
	linkID := c.Param("link_id")

	if !shortid.IsValid(orgID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	if !shortid.IsValid(linkID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid link ID")
		return
	}

	var link models.InstallLink
	if err := h.db.Preload("Install").Preload("NuonOrg").Where("id = ? AND org_id = ? AND user_id = ?", linkID, orgID, user.ID).First(&link).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Install link not found")
		return
	}

	installURL := link.GetInstallURL(h.customerBaseURL)

	// For HTMX requests, return only the status partial
	if isHTMXRequest(c) {
		theme, _ := models.GetOrCreateAppTheme(h.db)
		primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)
		h.RenderTempl(c, http.StatusOK, partials.LinkStatus(partials.LinkStatusProps{
			Link:         &link,
			InstallURL:   installURL,
			PrimaryColor: primaryColor,
			BasePath:     h.basePath,
		}))
		return
	}

	// For full page requests, redirect to the detail page
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/links/%s", h.basePath, orgID, linkID))
}

// GetOrgApps fetches apps for an organization
func (h *Handler) GetOrgApps(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	orgID := c.Param("org_id")

	if !shortid.IsValid(orgID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Verify user owns the org
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
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
	user := middleware.GetCurrentUser(c)
	orgID := c.Param("org_id")
	appIDParam := c.Param("app_id")

	if !shortid.IsValid(orgID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Get optional filter parameter
	filterParam := c.Query("filter")

	// Verify user owns the org
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
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
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appIDParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch app input config: %v", err)})
		return
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

// GetAppActions fetches available actions for an app (for health check selection)
func (h *Handler) GetAppActions(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	orgID := c.Param("org_id")
	appIDParam := c.Param("app_id")

	if !shortid.IsValid(orgID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Verify user owns the org
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Initialize Nuon client with the org's credentials and global API URL
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Fetch app actions from Nuon API
	actions, err := nuonClient.GetAppActionWorkflows(c.Request.Context(), appIDParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch app actions: %v", err)})
		return
	}

	c.JSON(http.StatusOK, actions)
}

// DeleteInstallLink handles deleting an install link
func (h *Handler) DeleteInstallLink(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	orgID := c.Param("org_id")
	linkID := c.Param("link_id")

	if !shortid.IsValid(orgID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	if !shortid.IsValid(linkID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid link ID"})
		return
	}

	result := h.db.Where("id = ? AND org_id = ? AND user_id = ?", linkID, orgID, user.ID).Delete(&models.InstallLink{})
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

// DeleteOrg handles deleting an organization and its associated data
func (h *Handler) DeleteOrg(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	orgID := c.Param("org_id")

	fmt.Printf("DeleteOrg: Request from user %s (%s) for org_id=%s\n", user.ID, user.Email, orgID)

	if !shortid.IsValid(orgID) {
		fmt.Printf("DeleteOrg: Invalid org ID parameter: %s\n", orgID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Start a database transaction for atomic operations
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Debug: Check what organizations exist for this user
	var userOrgs []models.NuonOrg
	if err := tx.Where("user_id = ?", user.ID).Find(&userOrgs).Error; err != nil {
		fmt.Printf("DeleteOrg: Error querying user orgs for user_id=%s: %v\n", user.ID, err)
	} else {
		fmt.Printf("DeleteOrg: User %s has %d organizations: ", user.ID, len(userOrgs))
		for _, o := range userOrgs {
			fmt.Printf("[id=%s, name=%s] ", o.ID, o.Name)
		}
		fmt.Printf("\n")
	}

	// Debug: Check if org exists regardless of user
	var anyOrg models.NuonOrg
	if err := tx.Where("id = ?", orgID).First(&anyOrg).Error; err != nil {
		fmt.Printf("DeleteOrg: Organization with id=%s does not exist in database: %v\n", orgID, err)
	} else {
		fmt.Printf("DeleteOrg: Organization with id=%s exists but owned by user_id=%s (not %s)\n", orgID, anyOrg.UserID, user.ID)
	}

	// Verify user owns the org
	fmt.Printf("DeleteOrg: Attempting to find org with id=%s AND user_id=%s\n", orgID, user.ID)
	var org models.NuonOrg
	if err := tx.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		fmt.Printf("DeleteOrg: Organization not found or access denied for org_id=%s, user_id=%s, error: %v\n", orgID, user.ID, err)
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found or already deleted"})
		return
	}

	fmt.Printf("DeleteOrg: Found organization '%s' (id=%s) owned by user %s\n", org.Name, org.ID, user.ID)

	// Find all install links associated with this org
	var links []models.InstallLink
	if err := tx.Where("org_id = ?", orgID).Find(&links).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query install links"})
		return
	}

	// Find all installs associated with these install links
	var installLinkIDs []string
	for _, link := range links {
		installLinkIDs = append(installLinkIDs, link.ID)
	}

	var installs []models.Install
	if len(installLinkIDs) > 0 {
		if err := tx.Where("install_link_id IN ?", installLinkIDs).Find(&installs).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query installs"})
			return
		}
	}

	// Delete local install records (no API calls - just disconnect from installer app)
	fmt.Printf("DeleteOrg: Deleting %d local install records\n", len(installs))
	if len(installs) > 0 {
		for _, install := range installs {
			if err := tx.Delete(&install).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete local install record %s", install.NuonInstallID)})
				return
			}
			fmt.Printf("DeleteOrg: Deleted local install record for %s\n", install.NuonInstallID)
		}
	}

	// Delete install links
	if len(links) > 0 {
		if err := tx.Where("org_id = ?", orgID).Delete(&models.InstallLink{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete install links"})
			return
		}
	}

	// Delete the organization
	if err := tx.Delete(&org).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete organization"})
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit deletion transaction"})
		return
	}

	// Return success with summary of what was disconnected
	fmt.Printf("DeleteOrg: Successfully disconnected organization '%s' (id=%s) - %d links, %d installs\n",
		org.Name, org.ID, len(links), len(installs))

	response := gin.H{
		"message":               "Organization disconnected successfully",
		"org_name":              org.Name,
		"install_links_removed": len(links),
		"installs_removed":      len(installs),
	}

	c.JSON(http.StatusOK, response)
}

// ThemeSettingsPage renders the global theme settings page
func (h *Handler) ThemeSettingsPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display

	// Get or create the global app theme
	theme, err := models.GetOrCreateAppTheme(h.db)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load theme settings")
		return
	}

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("user_id = ?", user.ID).Find(&allOrgs)

	// Get primary colors for styling
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)

	props := vendorpages.ThemeSettingsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      "Theme Settings - Installer App",
			ActivePage: "settings",
			User:       user,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Theme Settings", Path: h.basePath + "/settings", Active: true},
			},
			BasePath:         h.basePath,
			PrimaryColor:     primaryColor,
			PrimaryColorDark: primaryColorDark,
		},
		Theme: theme,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.ThemeSettingsPage(props))
}

// ThemeSettingsPanelContent returns just the panel HTML for HTMX lazy loading
func (h *Handler) ThemeSettingsPanelContent(c *gin.Context) {
	theme, err := models.GetOrCreateAppTheme(h.db)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load theme settings")
		return
	}

	props := partials.ThemePanelProps{
		Theme:    theme,
		BasePath: h.basePath,
	}

	h.RenderTempl(c, http.StatusOK, partials.ThemePanel(props))
}

// UpdateThemeSettings handles PUT request to update global theme settings
func (h *Handler) UpdateThemeSettings(c *gin.Context) {
	var req struct {
		PrimaryColor      string `json:"primary_color"`
		SecondaryColor    string `json:"secondary_color"`
		LogoBase64        string `json:"logo_base64"`
		SupportContact    string `json:"support_contact"`
		HeadingFont       string `json:"heading_font"`
		BodyFont          string `json:"body_font"`
		HeadingFontBase64 string `json:"heading_font_base64"`
		BodyFontBase64    string `json:"body_font_base64"`
		BorderRadius      string `json:"border_radius"`
		SpacingDensity    string `json:"spacing_density"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get or create the global app theme
	theme, err := models.GetOrCreateAppTheme(h.db)
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

	// Handle logo - "REMOVE" clears, valid data URI sets
	if req.LogoBase64 != "" {
		if req.LogoBase64 == "REMOVE" {
			theme.LogoBase64 = ""
		} else if strings.HasPrefix(req.LogoBase64, "data:image/") {
			theme.LogoBase64 = req.LogoBase64
		}
	}

	// SupportContact can be set to empty string intentionally
	theme.SupportContact = req.SupportContact

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
	} else if len(req.HeadingFontBase64) > 0 && isValidFontDataURI(req.HeadingFontBase64) {
		if len(req.HeadingFontBase64) <= 280000 {
			theme.HeadingFontBase64 = req.HeadingFontBase64
		}
	}

	// BodyFontBase64: "REMOVE" clears, valid data URI sets
	if req.BodyFontBase64 == "REMOVE" {
		theme.BodyFontBase64 = ""
	} else if len(req.BodyFontBase64) > 0 && isValidFontDataURI(req.BodyFontBase64) {
		if len(req.BodyFontBase64) <= 280000 {
			theme.BodyFontBase64 = req.BodyFontBase64
		}
	}

	// Update border radius if provided and valid
	if req.BorderRadius != "" {
		if models.IsValidBorderRadius(req.BorderRadius) {
			theme.BorderRadius = req.BorderRadius
		}
	}

	// Update spacing density if provided and valid
	if req.SpacingDensity != "" {
		if models.IsValidSpacingDensity(req.SpacingDensity) {
			theme.SpacingDensity = req.SpacingDensity
		}
	}

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

	fmt.Printf("DebugUserOrgs: Request from user %s (%s)\n", user.ID, user.Email)

	var orgs []models.NuonOrg
	if err := h.db.Where("user_id = ?", user.ID).Find(&orgs).Error; err != nil {
		fmt.Printf("DebugUserOrgs: Error querying orgs: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	fmt.Printf("DebugUserOrgs: Found %d organizations for user %s\n", len(orgs), user.ID)

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

// AppWithHealthCheckStatus represents an app with its health check configuration status
type AppWithHealthCheckStatus struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Platform         string `json:"platform"` // "aws" or "azure"
	HasHealthChecks  bool   `json:"has_health_checks"`
	HealthCheckCount int    `json:"health_check_count"`
}

// ActionForHealthCheck represents a flattened action for the health checks template
type ActionForHealthCheck struct {
	ID                string `json:"id"`                   // Workflow ID
	AppActionConfigID string `json:"app_action_config_id"` // Config ID from first config
	Name              string `json:"name"`
	Description       string `json:"description"`
}

// AppsPage displays the apps list with health check configuration status
func (h *Handler) AppsPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display
	orgID := c.Param("org_id")

	if !shortid.IsValid(orgID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	// Verify user owns the org
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Organization not found")
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	// Fetch apps from Nuon API
	apps, err := nuonClient.ListApps(c.Request.Context())
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch apps: %v", err))
		return
	}

	// Get all health check configs for these apps
	var healthConfigs []models.AppHealthCheckConfig
	appIDs := make([]string, len(apps))
	for i, app := range apps {
		appIDs[i] = app.ID
	}
	h.db.Where("app_id IN ?", appIDs).Find(&healthConfigs)

	// Create a map for quick lookup
	configMap := make(map[string]*models.AppHealthCheckConfig)
	for i := range healthConfigs {
		configMap[healthConfigs[i].AppID] = &healthConfigs[i]
	}

	// Build app list with health check status
	appsWithStatus := make([]AppWithHealthCheckStatus, len(apps))
	for i, app := range apps {
		// Extract platform from runner config (same as GetAppInputConfig)
		platform := "aws" // default
		if app.RunnerConfig != nil {
			runnerType := string(app.RunnerConfig.AppRunnerType)
			if runnerType == "azure" {
				platform = "azure"
			}
		}

		appStatus := AppWithHealthCheckStatus{
			ID:       app.ID,
			Name:     app.Name,
			Platform: platform,
		}

		if config, ok := configMap[app.ID]; ok {
			appStatus.HasHealthChecks = config.HasHealthChecks()
			appStatus.HealthCheckCount = config.HealthCheckCount()
		}

		appsWithStatus[i] = appStatus
	}

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("user_id = ?", user.ID).Find(&allOrgs)

	// Load theme for styling
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	// Convert to templ type
	templApps := make([]vendorpages.AppWithHealthCheckStatus, len(appsWithStatus))
	for i, app := range appsWithStatus {
		templApps[i] = vendorpages.AppWithHealthCheckStatus{
			ID:               app.ID,
			Name:             app.Name,
			Platform:         app.Platform,
			HasHealthChecks:  app.HasHealthChecks,
			HealthCheckCount: app.HealthCheckCount,
		}
	}

	props := vendorpages.AppsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:              org.Name + " - Apps",
			ActivePage:         "apps",
			User:               user,
			CurrentOrg:         &org,
			Orgs:               allOrgs,
			Breadcrumbs:        []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: true}},
			BasePath:           h.basePath,
			PrimaryColor:       primaryColor,
			PrimaryColorDark:   primaryColorDark,
			SecondaryColor:     secondaryColor,
			SecondaryColorDark: secondaryColorDark,
			HeadingFont:        theme.HeadingFont,
			BodyFont:           theme.BodyFont,
			HeadingFontBase64:  theme.HeadingFontBase64,
			BodyFontBase64:     theme.BodyFontBase64,
			LogoBase64:         theme.LogoBase64,
		},
		Org:  org,
		Apps: templApps,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.AppsPage(props))
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
	orgID := c.Param("org_id")
	appID := c.Param("app_id")

	if !shortid.IsValid(orgID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	// Verify user owns the org
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Organization not found")
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	// Fetch app details from Nuon API
	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch app: %v", err))
		return
	}

	// Fetch app input config from Nuon API
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appID)
	if err != nil {
		// Input config may not exist, that's okay - we'll show empty tables
		inputConfig = nil
	}

	// Parse inputs into vendor and customer categories
	var vendorInputs, customerInputs []vendorpages.AppInputInfo

	if inputConfig != nil {
		// Convert the typed struct to JSON then back to map for flexible field access
		// This handles the case where user_configurable might exist in the API response
		// but not in the SDK struct
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				if inputGroups, ok := configMap["input_groups"].([]interface{}); ok {
					for _, group := range inputGroups {
						if groupMap, ok := group.(map[string]interface{}); ok {
							groupName, _ := groupMap["name"].(string)
							groupDisplayName, _ := groupMap["display_name"].(string)
							if groupDisplayName == "" {
								groupDisplayName = groupName
							}

							if appInputs, ok := groupMap["app_inputs"].([]interface{}); ok {
								for _, input := range appInputs {
									if inputMap, ok := input.(map[string]interface{}); ok {
										inputInfo := vendorpages.AppInputInfo{
											Name:        getMapString(inputMap, "name"),
											DisplayName: getMapString(inputMap, "display_name"),
											Description: getMapString(inputMap, "description"),
											Type:        getMapString(inputMap, "type"),
											Required:    getMapBool(inputMap, "required"),
											Sensitive:   getMapBool(inputMap, "sensitive"),
											Default:     getStringFromAny(inputMap["default"]),
											Group:       groupDisplayName,
											Source:      getMapString(inputMap, "source"),
										}

										if inputInfo.DisplayName == "" {
											inputInfo.DisplayName = inputInfo.Name
										}
										if inputInfo.Type == "" {
											inputInfo.Type = "string"
										}

										// Categorize by source field
										// Note: if source is not in the API response or is "vendor",
										// inputs will be treated as vendor inputs (the default)
										if inputInfo.Source == "customer" {
											customerInputs = append(customerInputs, inputInfo)
										} else {
											vendorInputs = append(vendorInputs, inputInfo)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("user_id = ?", user.ID).Find(&allOrgs)

	// Load theme
	theme, _ := models.GetOrCreateAppTheme(h.db)
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)

	// Get active tab from query param (default to vendor)
	activeTab := c.DefaultQuery("tab", "vendor")

	props := vendorpages.AppInputsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      app.Name + " - Inputs",
			ActivePage: "apps",
			User:       user,
			CurrentOrg: &org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: org.Name, Path: fmt.Sprintf("%s/orgs/%s", h.basePath, org.ID), Active: false},
				{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false},
				{Text: app.Name, Path: fmt.Sprintf("%s/orgs/%s/apps/%s", h.basePath, org.ID, appID), Active: false},
				{Text: "Inputs", Path: "", Active: true},
			},
			BasePath:         h.basePath,
			PrimaryColor:     primaryColor,
			PrimaryColorDark: primaryColorDark,
		},
		Org: org,
		App: vendorpages.AppInfo{
			ID:   app.ID,
			Name: app.Name,
		},
		VendorInputs:   vendorInputs,
		CustomerInputs: customerInputs,
		ActiveTab:      activeTab,
	}

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

// AppHealthChecksPage displays the health check configuration for an app
func (h *Handler) AppHealthChecksPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display
	orgID := c.Param("org_id")
	appID := c.Param("app_id")

	if !shortid.IsValid(orgID) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	// Verify user owns the org
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Organization not found")
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	// Fetch app details from Nuon API
	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch app: %v", err))
		return
	}

	// Fetch available actions for this app
	rawActions, err := nuonClient.GetAppActionWorkflows(c.Request.Context(), appID)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch app actions: %v", err))
		return
	}

	// Transform actions to flatten the config ID for the template
	// The API returns actions with nested configs, but we need AppActionConfigID at the top level
	var actions []ActionForHealthCheck
	for _, action := range rawActions {
		// Skip actions without configs (they can't be run)
		if action.Configs == nil || len(action.Configs) == 0 {
			continue
		}
		// Use the first (most recent) config's ID
		configID := action.Configs[0].ID

		flatAction := ActionForHealthCheck{
			ID:                action.ID,
			AppActionConfigID: configID,
			Name:              action.Name,
		}
		actions = append(actions, flatAction)
	}

	// Load existing health check config (if any)
	var healthConfig models.AppHealthCheckConfig
	h.db.Where("app_id = ?", appID).First(&healthConfig)

	// Create a set of selected action IDs for easy lookup
	selectedIDs := make(map[string]bool)
	for _, id := range healthConfig.GetHealthCheckActionIDs() {
		selectedIDs[id] = true
	}

	// Fetch all orgs for sidebar dropdown
	var allOrgs []models.NuonOrg
	h.db.Where("user_id = ?", user.ID).Find(&allOrgs)

	// Load theme
	theme, _ := models.GetOrCreateAppTheme(h.db)

	// Get primary colors
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)

	// Convert actions to templ type
	templActions := make([]vendorpages.ActionForHealthCheck, len(actions))
	for i, a := range actions {
		templActions[i] = vendorpages.ActionForHealthCheck{
			ID:                a.ID,
			AppActionConfigID: a.AppActionConfigID,
			Name:              a.Name,
			Description:       a.Description,
		}
	}

	props := vendorpages.AppHealthChecksPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      app.Name + " - Health Checks",
			ActivePage: "apps",
			User:       user,
			CurrentOrg: &org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: org.Name, Path: fmt.Sprintf("%s/orgs/%s", h.basePath, org.ID), Active: false},
				{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false},
				{Text: app.Name, Path: fmt.Sprintf("%s/orgs/%s/apps/%s", h.basePath, org.ID, appID), Active: false},
				{Text: "Health Checks", Path: "", Active: true},
			},
			BasePath:         h.basePath,
			PrimaryColor:     primaryColor,
			PrimaryColorDark: primaryColorDark,
		},
		Org: org,
		App: vendorpages.AppInfo{
			ID:   app.ID,
			Name: app.Name,
		},
		Actions:     templActions,
		SelectedIDs: selectedIDs,
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.AppHealthChecksPage(props))
}

// UpdateAppHealthChecks saves the health check configuration for an app
func (h *Handler) UpdateAppHealthChecks(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	orgID := c.Param("org_id")
	appID := c.Param("app_id")

	if !shortid.IsValid(orgID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Verify user owns the org
	var org models.NuonOrg
	if err := h.db.Where("id = ? AND user_id = ?", orgID, user.ID).First(&org).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Initialize Nuon client to get app name
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Fetch app to get its name for caching
	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch app: %v", err)})
		return
	}

	var req struct {
		HealthCheckActionIDs []string `json:"health_check_action_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Find or create health check config
	var healthConfig models.AppHealthCheckConfig
	result := h.db.Where("app_id = ?", appID).First(&healthConfig)
	if result.Error != nil {
		// Create new config
		healthConfig = models.AppHealthCheckConfig{
			AppID:   appID,
			AppName: app.Name,
		}
	}

	// Update the health check action IDs
	healthConfig.SetHealthCheckActionIDs(req.HealthCheckActionIDs)
	healthConfig.AppName = app.Name // Update name in case it changed

	if err := h.db.Save(&healthConfig).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save health check configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  healthConfig,
	})
}
