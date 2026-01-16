package main

import (
	"log"
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

	// Root redirect to customer login (primary interface)
	router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/login")
	})

	// Set up vendor routes under /admin prefix
	setupVendorRoutes(router.Group("/admin"), db, vendorAuth, authProvider, customerBaseURL, nuonAPIURL)

	// Create customer auth factory with env var OIDC as fallback
	// Pass the auth config loaded from env vars to use as fallback when no DB config is active
	customerAuthFactory, err := auth.NewCustomerAuthProviderFactory(db, customerBaseURL, &authConfig)
	if err != nil {
		log.Fatalf("Failed to initialize customer authentication: %v", err)
	}
	log.Printf("Customer authentication: %s", getCustomerAuthMode(customerAuthFactory))

	// Set up customer routes at root level (no prefix)
	setupCustomerRoutes(router.Group(""), db, customerAuth, customerAuthFactory, customerBaseURL, nuonAPIURL)

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
func setupVendorRoutes(rg *gin.RouterGroup, db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, authProvider auth.AuthProvider, customerBaseURL, nuonAPIURL string) {
	// Initialize handlers with customer base URL for install links, Nuon API URL, and base path
	h := handlers.NewHandler(db, jwtAuth, authProvider, customerBaseURL, nuonAPIURL, "/admin")

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

	// Workspace management routes (no workspace context required)
	workspaceRoutes := rg.Group("/")
	workspaceRoutes.Use(jwtAuth.MiddlewareFunc())
	workspaceRoutes.Use(middleware.RequireRole(models.RoleVendor))
	{
		workspaceRoutes.POST("/workspaces", h.CreateWorkspace)
		workspaceRoutes.GET("/workspaces/select", h.WorkspaceSelectorPage)
		workspaceRoutes.POST("/workspace/switch", h.SwitchWorkspace)
	}

	// Workspace management routes (workspace context required)
	workspaceContextRoutes := rg.Group("/workspace")
	workspaceContextRoutes.Use(jwtAuth.MiddlewareFunc())
	workspaceContextRoutes.Use(middleware.RequireRole(models.RoleVendor))
	workspaceContextRoutes.Use(middleware.RequireWorkspaceContext(db))
	{
		workspaceContextRoutes.GET("/settings/panel", h.WorkspaceSettingsPanel)
		workspaceContextRoutes.PUT("", h.UpdateWorkspace)
		workspaceContextRoutes.POST("/invitations", h.GenerateInvitation)
		workspaceContextRoutes.DELETE("/invitations/:id", h.DeleteInvitation)
		workspaceContextRoutes.DELETE("/members/:user_id", h.RemoveMember)
	}

	// Settings API endpoints (workspace-scoped, pages are under /orgs/:org_id/settings/)
	settings := rg.Group("/settings")
	settings.Use(jwtAuth.MiddlewareFunc())
	settings.Use(middleware.RequireRole(models.RoleVendor))
	settings.Use(middleware.RequireWorkspaceContext(db))
	{
		// Panel endpoints (for HTMX lazy loading)
		settings.GET("/panel", h.ThemeSettingsPanelContent)
		settings.GET("/customization/panel", h.CustomThemePanelContent)
		settings.PUT("/", h.UpdateThemeSettings)

		// Login settings API endpoints
		settings.GET("/login/panel", h.LoginSettingsPanelContent)
		settings.PUT("/login", h.UpdateLoginSettings)
		settings.POST("/login/test", h.TestLoginConnection)

		// GitHub template customization settings
		settings.GET("/github", h.GetGitHubConfig)
		settings.POST("/github", h.SaveGitHubConfig)
		settings.POST("/github/sync", h.SyncGitHub)
		settings.DELETE("/github", h.DeleteGitHubConfig)
		settings.PUT("/github/templates/:page", h.ToggleTemplateOverride)
		settings.DELETE("/github/templates/:page", h.DeleteTemplateOverride)
	}

	// Profile settings (user can edit their own profile)
	profile := rg.Group("/profile")
	profile.Use(jwtAuth.MiddlewareFunc())
	profile.Use(middleware.RequireRole(models.RoleVendor))
	{
		profile.GET("/panel", h.ProfilePanelContent)
		profile.PUT("/", h.UpdateProfile)
	}

	// Protected vendor routes (require workspace context)
	orgs := rg.Group("/orgs")
	orgs.Use(jwtAuth.MiddlewareFunc())
	orgs.Use(middleware.RequireRole(models.RoleVendor))
	orgs.Use(middleware.RequireWorkspaceContext(db))
	{
		// Workspace-level routes (no specific org)
		orgs.GET("/", h.OrgsPage)
		orgs.POST("/", h.CreateOrg)

		// Org-specific routes (require org access)
		orgRoutes := orgs.Group("/:org_id")
		orgRoutes.Use(middleware.RequireOrgAccess(db))
		{
			orgRoutes.GET("/install-links", h.OrgDetailPage)
			orgRoutes.GET("/settings", h.OrgSettingsPage)
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

			// Settings pages (org-scoped to preserve org context in URL)
			orgSettings := orgRoutes.Group("/settings")
			{
				orgSettings.GET("/team", h.TeamSettingsPage)
			}

			// Customer Portal pages (org-scoped)
			orgPortal := orgRoutes.Group("/portal")
			{
				orgPortal.GET("/branding", h.BrandingSettingsPage)
				orgPortal.GET("/custom-theme", h.CustomThemeSettingsPage)
				orgPortal.GET("/login", h.LoginSettingsPage)
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
func setupCustomerRoutes(rg *gin.RouterGroup, db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, customerAuthFactory *auth.CustomerAuthProviderFactory, customerBaseURL, nuonAPIURL string) {
	// Initialize handlers with customer auth factory (OIDC only, no local auth)
	h := handlers.NewHandlerWithCustomerAuth(db, jwtAuth, customerAuthFactory, customerBaseURL, nuonAPIURL, "")

	// Public routes - customer login (OIDC only)
	rg.GET("/login", h.CustomerLoginPageTempl)
	rg.GET("/login/", h.CustomerLoginPageTempl) // Handle both with and without trailing slash

	// OIDC callback
	rg.GET("/callback", h.CustomerOIDCCallback)

	// Registration disabled - redirect to login
	rg.GET("/register", h.CustomerRegisterPage)

	// Install link acceptance (customer signup flow)
	installLinks := rg.Group("/install-link")
	{
		installLinks.GET("/", h.InstallLinkPage)
		installLinks.POST("/", h.AcceptInstallLink)
		installLinks.GET("/:sha/app-config", h.GetInstallLinkAppConfig)
	}

	// Custom asset serving (workspace-specific CSS and images from GitHub sync)
	rg.GET("/custom/css/:workspace_id", h.ServeCustomCSS)
	rg.GET("/custom/assets/:workspace_id/*path", h.ServeCustomAsset)

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
			installOwnership.GET("/", h.InstallDetail)
			installOwnership.PUT("/", h.UpdateInstall)                         // Customer can update their install
			installOwnership.DELETE("/", h.DeleteInstall)                      // Customer can delete (deprovision) their install
			installOwnership.POST("/forget", h.ForgetInstall)                  // Customer can forget (remove from DB) their install
			installOwnership.POST("/health-checks/run", h.TriggerHealthChecks) // Customer can manually trigger health checks

			// Input management
			installOwnership.GET("/inputs", h.GetInstallInputs)    // Customer can view current inputs
			installOwnership.PUT("/inputs", h.UpdateInstallInputs) // Customer can update inputs

			// Workflow history and actions
			installOwnership.GET("/workflows", h.WorkflowsPage)
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
