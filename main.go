package main

import (
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	"github.com/nuonco/mono/services/customer-dashboard/internal/background"
	"github.com/nuonco/mono/services/customer-dashboard/internal/handlers"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

func main() {
	// Initialize asset manifest for cache-busting
	if err := assets.Init("./static"); err != nil {
		log.Printf("Warning: Failed to initialize asset manifest: %v", err)
	}

	// Initialize database (uses DATABASE_URL env var or local defaults)
	db, err := models.InitDB()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Get JWT secret from environment or use default
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key" // In production, this should always be from env
	}

	// Initialize vendor JWT middleware (allows signup, redirects to /admin/orgs)
	vendorAuth, err := middleware.NewJWTMiddleware(db, jwtSecret, middleware.AuthOptions{
		AllowSignup:   true,
		DefaultRole:   models.RoleVendor,
		LoginRedirect: "/admin/orgs",
		BasePath:      "/admin",
	})
	if err != nil {
		log.Fatal("Vendor JWT Error:", err.Error())
	}

	// Initialize customer JWT middleware (no signup, redirects to /installs)
	customerAuth, err := middleware.NewJWTMiddleware(db, jwtSecret, middleware.AuthOptions{
		AllowSignup:   false,
		DefaultRole:   models.RoleCustomer,
		LoginRedirect: "/installs",
		BasePath:      "",
	})
	if err != nil {
		log.Fatal("Customer JWT Error:", err.Error())
	}

	// Get port from environment (single server now)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Get Nuon API URL from environment (global setting for all orgs)
	nuonAPIURL := os.Getenv("NUON_API_URL")
	if nuonAPIURL == "" {
		nuonAPIURL = "https://api.nuon.co"
	}

	// Build customer base URL for install links
	// Customer routes are now at root level (no /customer prefix)
	customerBaseURL := os.Getenv("CUSTOMER_BASE_URL")
	if customerBaseURL == "" {
		customerBaseURL = "http://localhost:" + port
	}

	// Get subdomain base domain for org subdomains
	subdomainBaseDomain := os.Getenv("SUBDOMAIN_BASE_DOMAIN")
	if subdomainBaseDomain == "" {
		subdomainBaseDomain = "localhost:" + port
	}

	// Load auth provider configuration from environment
	authConfig := auth.LoadConfigFromEnv()

	// Set default redirect URI if not specified
	if authConfig.RedirectURI == "" {
		authConfig.RedirectURI = "http://localhost:" + port + "/admin/callback"
	}

	// Initialize auth provider
	var authProvider auth.AuthProvider
	if authConfig.IsConfigured() {
		// External IdP configured (OIDC or SAML)
		var err error
		authProvider, err = auth.NewProvider(authConfig, db)
		if err != nil {
			log.Fatalf("Failed to initialize auth provider: %v", err)
		}
		log.Printf("Authentication provider enabled: %s", authProvider.Name())
	} else {
		// No external IdP configured - fall back to local password authentication
		authProvider = auth.NewFallbackLocalProvider(db)
		log.Printf("Local password authentication enabled (no IdP configured)")
	}

	// Create single router with shared middleware
	router := gin.Default()

	// Add cache-control middleware for proper browser caching
	router.Use(cacheControlMiddleware())

	// Serve static files (shared across both interfaces)
	router.Static("/static", "./static")

	// Health check endpoints for Kubernetes probes (at root level)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/ready", func(c *gin.Context) {
		if err := models.Ping(db); err != nil {
			c.JSON(503, gin.H{"status": "not ready", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	// Set up vendor routes under /admin prefix
	setupVendorRoutes(router.Group("/admin"), db, vendorAuth, authProvider, customerBaseURL, nuonAPIURL, subdomainBaseDomain)

	// Create customer auth factory with env var OIDC as fallback
	// Pass the auth config loaded from env vars to use as fallback when no DB config is active
	customerAuthFactory, err := auth.NewCustomerAuthProviderFactory(db, customerBaseURL, &authConfig)
	if err != nil {
		log.Fatalf("Failed to initialize customer authentication: %v", err)
	}
	log.Printf("Customer authentication: %s", getCustomerAuthMode(customerAuthFactory))

	// Set up customer routes at root level (no prefix)
	setupCustomerRoutes(router.Group(""), db, customerAuth, customerAuthFactory, customerBaseURL, nuonAPIURL, subdomainBaseDomain)

	// Start background health check runner (needs Nuon API URL for status checks)
	healthCheckRunner := background.NewHealthCheckRunner(db, 30*time.Second, nuonAPIURL)
	healthCheckRunner.Start()

	// Start the single server
	log.Printf("Server starting on port %s", port)
	log.Printf("  Admin UI:    http://localhost:%s/admin/", port)
	log.Printf("  Customer UI: http://localhost:%s/", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// setupVendorRoutes configures vendor-facing routes on the given router group
func setupVendorRoutes(rg *gin.RouterGroup, db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, authProvider auth.AuthProvider, customerBaseURL, nuonAPIURL, subdomainBaseDomain string) {
	// Initialize handlers with customer base URL for install links, Nuon API URL, and base path
	h := handlers.NewHandler(db, jwtAuth, authProvider, customerBaseURL, nuonAPIURL, "/admin", subdomainBaseDomain)

	// Root redirect to login
	rg.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin/login")
	})

	// Public routes - vendor login
	rg.GET("/login/", h.VendorLoginPageTempl)
	rg.GET("/logout", h.VendorLogout) // Logout handler

	// Invitation acceptance route (public - redirects to login if not authenticated)
	rg.GET("/invite", h.AcceptInvitationPage)

	// Auth routes depend on provider type
	if authProvider != nil && authProvider.Name() == "local" {
		// Local password auth routes
		rg.POST("/login/", h.LocalLogin)
		rg.GET("/register", h.VendorRegisterPageTempl)
		rg.POST("/register", h.LocalRegister)
	} else {
		// IdP callback routes (OIDC/SAML)
		rg.GET("/callback", h.AuthCallback)  // OIDC callback (GET with code)
		rg.POST("/callback", h.AuthCallback) // SAML callback (POST with SAMLResponse)
	}

	// JWT refresh endpoint
	rg.POST("/refresh_token", jwtAuth.RefreshHandler)

	// Organization management routes (no org context required)
	orgMgmtRoutes := rg.Group("/")
	orgMgmtRoutes.Use(jwtAuth.MiddlewareFunc())
	orgMgmtRoutes.Use(middleware.RequireRole(models.RoleVendor))
	{
		orgMgmtRoutes.POST("/org/create", h.CreateOrg) // Create new org
		orgMgmtRoutes.POST("/org/switch", h.SwitchOrg) // Switch org context
	}

	// Organization management routes (org context required)
	orgContextRoutes := rg.Group("/org")
	orgContextRoutes.Use(jwtAuth.MiddlewareFunc())
	orgContextRoutes.Use(middleware.RequireRole(models.RoleVendor))
	orgContextRoutes.Use(middleware.RequireOrgContext(db))
	{
		orgContextRoutes.PUT("", h.UpdateOrg)
		orgContextRoutes.POST("/invitations", h.GenerateOrgInvitation)
		orgContextRoutes.DELETE("/invitations/:id", h.DeleteOrgInvitation)
		orgContextRoutes.DELETE("/members/:user_id", h.RemoveOrgMember)
	}

	// Settings API endpoints (org-scoped, pages are under /orgs/:org_id/settings/)
	settings := rg.Group("/settings")
	settings.Use(jwtAuth.MiddlewareFunc())
	settings.Use(middleware.RequireRole(models.RoleVendor))
	settings.Use(middleware.RequireOrgContext(db))
	{
		settings.PUT("/", h.UpdateThemeSettings)

		// Login settings API endpoints
		settings.PUT("/login", h.UpdateLoginSettings)
		settings.POST("/login/test", h.TestLoginConnection)

		// DNS settings
		settings.PUT("/dns", h.UpdateDNSSettings)
		settings.GET("/dns/check", h.CheckSubdomainAvailability)

		// GitHub template customization settings
		settings.GET("/github", h.GetGitHubConfig)
		settings.POST("/github", h.SaveGitHubConfig)
		settings.POST("/github/sync", h.SyncGitHub)
		settings.DELETE("/github", h.DeleteGitHubConfig)
		settings.PUT("/github/templates/:page", h.ToggleTemplateOverride)
		settings.DELETE("/github/templates/:page", h.DeleteTemplateOverride)
		settings.PUT("/github/assets/*path", h.ToggleAssetOverride)
		settings.POST("/github/bulk-toggle", h.BulkToggleOverrides)
	}

	// Profile settings (user can edit their own profile)
	profile := rg.Group("/profile")
	profile.Use(jwtAuth.MiddlewareFunc())
	profile.Use(middleware.RequireRole(models.RoleVendor))
	{
		profile.GET("/panel", h.ProfilePanelContent)
		profile.PUT("/", h.UpdateProfile)
	}

	// Protected vendor routes
	orgs := rg.Group("/orgs")
	orgs.Use(jwtAuth.MiddlewareFunc())
	orgs.Use(middleware.RequireRole(models.RoleVendor))
	{
		// Org-level routes (no specific org selected, no RequireOrgContext)
		// This allows users with no orgs to reach this route without redirect loop
		orgs.GET("/", h.OrgsPage)

		// Org-specific routes (require org context and access by param)
		orgRoutes := orgs.Group("/:org_id")
		orgRoutes.Use(middleware.RequireOrgContext(db))
		orgRoutes.Use(middleware.RequireOrgAccessByParam(db))
		{
			orgRoutes.GET("/install-links", h.OrgDetailPage)
			orgRoutes.GET("/connection", h.OrgSettingsPage)
			orgRoutes.PUT("/", h.UpdateOrg)
			orgRoutes.DELETE("/", h.DeleteOrg)

			// Apps - configuration pages
			orgRoutes.GET("/apps", h.AppsPage)                                    // Apps list page (HTML)
			orgRoutes.GET("/apps/:app_id", h.AppDetailRedirect)                   // Redirect to inputs
			orgRoutes.GET("/apps/:app_id/inputs", h.AppInputsPage)                // Inputs config page
			orgRoutes.GET("/apps/:app_id/health-checks", h.AppHealthChecksPage)   // Health checks config page
			orgRoutes.PUT("/apps/:app_id/health-checks", h.UpdateAppHealthChecks) // Update health checks

			// Apps - API endpoints (JSON, used by create link modal)
			orgRoutes.GET("/apps-api", h.GetOrgApps)
			orgRoutes.GET("/apps-api/:app_id/input-config", h.GetAppInputConfig)
			orgRoutes.GET("/apps-api/:app_id/actions", h.GetAppActions)

			// Local customer-facing input configuration
			orgRoutes.GET("/apps-api/:app_id/customer-input-config", h.GetAppCustomerInputConfig)
			orgRoutes.PUT("/apps-api/:app_id/customer-input-config", h.UpdateAppCustomerInputConfig)

			orgRoutes.POST("/install-links", h.CreateInstallLink)
			orgRoutes.GET("/install-links/:link_id", h.InstallLinkDetail)
			orgRoutes.GET("/install-links/:link_id/status", h.InstallLinkStatus) // HTMX polling endpoint
			orgRoutes.DELETE("/install-links/:link_id", h.DeleteInstallLink)

			// Customers - view all customers and their installs
			orgRoutes.GET("/customers", h.CustomersPage)
			orgRoutes.GET("/customers/:customer_id", h.CustomerDetailPage)

			// Team pages (org-scoped)
			orgRoutes.GET("/team", func(c *gin.Context) {
				c.Redirect(http.StatusFound, c.Request.URL.Path+"/members")
			})
			orgRoutes.GET("/team/members", h.TeamMembersPage)
			orgRoutes.GET("/team/invites", h.TeamInvitesPage)

			// Customer Portal pages (org-scoped)
			orgPortal := orgRoutes.Group("/portal")
			{
				orgPortal.GET("/branding", h.BrandingSettingsPage)
				orgPortal.GET("/custom-theme", h.CustomThemeSettingsPage)
				orgPortal.GET("/login", h.LoginSettingsPage)
				orgPortal.GET("/dns", h.DNSSettingsPage)
			}
		}
	}

	// Debug endpoint for troubleshooting
	debug := rg.Group("/debug")
	debug.Use(jwtAuth.MiddlewareFunc())
	{
		debug.GET("/user-orgs", h.DebugUserOrgs)
	}
}

// setupCustomerRoutes configures customer-facing routes on the given router group
func setupCustomerRoutes(rg *gin.RouterGroup, db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, customerAuthFactory *auth.CustomerAuthProviderFactory, customerBaseURL, nuonAPIURL, subdomainBaseDomain string) {
	// Add subdomain detection middleware to all routes
	// This extracts subdomain from the host and stores it in context
	rg.Use(middleware.SubdomainContext(subdomainBaseDomain))

	// Redirect customer-facing routes to admin login when accessed without subdomain
	// This prevents users from getting stuck on pages that require org context
	rg.Use(middleware.RedirectBaseDomainCustomerRoutes())

	// Initialize handlers with customer auth factory (OIDC only, no local auth)
	h := handlers.NewHandlerWithCustomerAuth(db, jwtAuth, customerAuthFactory, customerBaseURL, nuonAPIURL, "", subdomainBaseDomain)

	// Root redirect to customer login (with subdomain) or admin login (without subdomain)
	// The RedirectBaseDomainCustomerRoutes middleware handles the base domain case
	rg.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/login")
	})

	// Base domain auth routes (for OIDC flow without subdomain cookie issues)
	// These handle the actual OIDC authentication on the base domain to avoid
	// subdomain cookie scoping problems
	baseAuth := rg.Group("/auth")
	baseAuth.Use(middleware.AllowBaseDomainOnly()) // Only accessible from base domain
	{
		baseAuth.GET("/login", h.BaseDomainLogin)       // Initiate OIDC from base domain
		baseAuth.GET("/callback", h.BaseDomainCallback) // OIDC callback handler
		baseAuth.GET("/error", h.AuthErrorPage)         // Auth error page
	}

	// Subdomain completion endpoint (sets JWT cookie on subdomain after base domain auth)
	rg.GET("/auth/complete", h.CompleteSubdomainAuth)

	// Public routes - customer login (OIDC only)
	rg.GET("/login", h.CustomerLoginPageTempl)
	rg.GET("/login/", h.CustomerLoginPageTempl) // Handle both with and without trailing slash

	// Registration disabled - redirect to login
	rg.GET("/register", h.CustomerRegisterPage)

	// Install link acceptance (customer signup flow)
	installLinks := rg.Group("/install-link")
	{
		installLinks.GET("/", h.InstallLinkPage)
		installLinks.POST("/", h.AcceptInstallLink)
		installLinks.GET("/:sha/app-config", h.GetInstallLinkAppConfig)
	}

	// Custom asset serving (org-specific CSS and images from GitHub sync)
	rg.GET("/custom/css/:org_id", h.ServeCustomCSS)
	rg.GET("/custom/assets/:org_id/*path", h.ServeCustomAsset)

	// JWT refresh endpoint
	rg.POST("/refresh_token", jwtAuth.RefreshHandler)

	// Protected customer routes
	installs := rg.Group("/installs")
	installs.Use(jwtAuth.MiddlewareFunc())
	installs.Use(middleware.RequireRole(models.RoleCustomer))
	{
		installs.GET("/", h.InstallsPage)

		// Routes that require install ownership verification
		installOwnership := installs.Group("/:install_id")
		installOwnership.Use(middleware.RequireInstallOwnership(db))
		{
			// Panel endpoints (for sliding panel content)
			installOwnership.GET("/panel", h.InstallDetailPanel)     // Panel manage tab
			installOwnership.GET("/panel/history", h.WorkflowsPanel) // Panel history tab
			installOwnership.GET("/panel/audit", h.AuditLogsPanel)   // Panel audit tab

			installOwnership.PUT("/", h.UpdateInstall)                         // Customer can update their install
			installOwnership.DELETE("/", h.DeleteInstall)                      // Customer can delete (deprovision) their install
			installOwnership.POST("/forget", h.ForgetInstall)                  // Customer can forget (remove from DB) their install
			installOwnership.POST("/health-checks/run", h.TriggerHealthChecks) // Customer can manually trigger health checks

			// Input management
			installOwnership.GET("/inputs", h.GetInstallInputs)    // Customer can view current inputs
			installOwnership.PUT("/inputs", h.UpdateInstallInputs) // Customer can update inputs

			// Workflow actions
			installOwnership.POST("/workflows/:workflow_id/approve", h.ApproveWorkflowStep)
			installOwnership.POST("/workflows/:workflow_id/approve-all", h.ApproveAllWorkflowSteps)
			installOwnership.POST("/workflows/:workflow_id/cancel", h.CancelWorkflow)
		}
	}
}

// getCustomerAuthMode returns a description of the current customer auth configuration
func getCustomerAuthMode(factory *auth.CustomerAuthProviderFactory) string {
	source := factory.GetOIDCSource()
	if source == auth.OIDCSourceDatabase {
		config, _ := factory.GetConfig()
		if config != nil && config.ProviderName != "" {
			return "OIDC (" + config.ProviderName + ") [database]"
		}
		return "OIDC [database]"
	}
	return "OIDC [environment fallback]"
}

// hashedAssetPattern matches filenames with 8-char hex hash before extension
// e.g., vendor.a1b2c3d4.css, customer.deadbeef.css
var hashedAssetPattern = regexp.MustCompile(`\.[a-f0-9]{8}\.(css|js)$`)

// cacheControlMiddleware sets appropriate Cache-Control headers for responses
func cacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Long cache for hashed static assets (immutable)
		// These files have content hashes in their names, so they can be cached forever
		if strings.HasPrefix(path, "/static/") && hashedAssetPattern.MatchString(path) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.Next()
			return
		}

		// Process the request first
		c.Next()

		// For HTML responses, ensure no caching so users always get fresh content
		contentType := c.Writer.Header().Get("Content-Type")
		if strings.Contains(contentType, "text/html") {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		}
	}
}
