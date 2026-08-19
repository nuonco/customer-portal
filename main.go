package main

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	"github.com/nuonco/mono/services/customer-dashboard/internal/handlers"
	jsonhandlers "github.com/nuonco/mono/services/customer-dashboard/internal/handlers/json"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/spa"
)

func main() {
	// Initialize structured logger
	var logger *zap.Logger
	if os.Getenv("LOG_LEVEL") == "DEBUG" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Initialize asset manifest for cache-busting
	if err := assets.Init("./static"); err != nil {
		logger.Warn("failed to initialize asset manifest", zap.Error(err))
	}

	// Initialize database (uses DATABASE_URL env var or local defaults)
	db, err := models.InitDB()
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
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
		logger.Fatal("vendor JWT initialization failed", zap.Error(err))
	}

	// Initialize customer JWT middleware (no signup, redirects to /installs)
	customerAuth, err := middleware.NewJWTMiddleware(db, jwtSecret, middleware.AuthOptions{
		AllowSignup:   false,
		DefaultRole:   models.RoleCustomer,
		LoginRedirect: "/installs",
		BasePath:      "",
	})
	if err != nil {
		logger.Fatal("customer JWT initialization failed", zap.Error(err))
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

	// Get Nuon dashboard URL for "View Org" links
	dashboardURL := os.Getenv("DASHBOARD_URL")
	if dashboardURL == "" {
		dashboardURL = "https://app.nuon.co"
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

	// Get superuser email domain from environment
	superuserEmailDomain := os.Getenv("SUPERUSER_EMAIL_DOMAIN")
	if superuserEmailDomain == "" {
		superuserEmailDomain = "nuon.co"
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
			logger.Fatal("failed to initialize auth provider", zap.Error(err))
		}
		logger.Info("authentication provider enabled", zap.String("provider", authProvider.Name()))
	} else {
		// No external IdP configured - fall back to local password authentication
		authProvider = auth.NewFallbackLocalProvider(db)
		logger.Info("local password authentication enabled (no IdP configured)")
	}

	// Create single router with shared middleware
	router := gin.Default()

	// Add cache-control middleware for proper browser caching
	router.Use(cacheControlMiddleware())

	// Serve static files (shared across both interfaces).
	// Cache-busting hashes (e.g. css/customer.6491ba3f.css) are stripped from
	// the request path so we serve the canonical base file directly from disk.
	staticFS := http.StripPrefix("/static/", http.FileServer(http.Dir("./static")))
	serveStatic := func(c *gin.Context) {
		c.Request.URL.Path = hashedAssetPattern.ReplaceAllString(c.Request.URL.Path, ".$1")
		staticFS.ServeHTTP(c.Writer, c.Request)
	}
	router.GET("/static/*filepath", serveStatic)
	router.HEAD("/static/*filepath", serveStatic)

	// Dev hot-reload version endpoint (only in local development)
	if os.Getenv("LIVE_RELOAD") == "true" {
		startVersion := fmt.Sprintf("%d", time.Now().UnixNano())
		router.GET("/dev/version", func(c *gin.Context) {
			c.String(200, startVersion)
		})
	}

	// Health check endpoints for Kubernetes probes (at root level)
	router.GET("/livez", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		if err := models.Ping(db); err != nil {
			c.JSON(503, gin.H{"status": "not ready", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	// Set up vendor routes under /admin prefix
	setupVendorRoutes(router.Group("/admin"), db, vendorAuth, authProvider, customerBaseURL, nuonAPIURL, dashboardURL, subdomainBaseDomain, superuserEmailDomain, logger)

	// Create customer auth factory with env var OIDC as fallback
	// Pass the auth config loaded from env vars to use as fallback when no DB config is active
	customerAuthFactory, err := auth.NewCustomerAuthProviderFactory(db, customerBaseURL, &authConfig)
	if err != nil {
		logger.Fatal("failed to initialize customer authentication", zap.Error(err))
	}
	logger.Info("customer authentication configured", zap.String("mode", getCustomerAuthMode(customerAuthFactory)))

	// Set up customer routes at root level (no prefix)
	setupCustomerRoutes(router.Group(""), db, customerAuth, customerAuthFactory, customerBaseURL, nuonAPIURL, subdomainBaseDomain, logger)

	// Serve the React SPA. This MUST come after every other route registration
	// because it installs a NoRoute catch-all for client-side routing.
	spaHandler := spa.NewHandler(spa.Config{
		DistDir:             os.Getenv("DIST_DIR"),
		SubdomainBaseDomain: subdomainBaseDomain,
		CustomerSubdomain:   os.Getenv("CUSTOMER_SUBDOMAIN"),
		NuonAPIURL:          nuonAPIURL,
		Version:             os.Getenv("VERSION"),
		GitRef:              os.Getenv("GIT_REF"),
		NoCacheAssets:       os.Getenv("LIVE_RELOAD") == "true",
		DevReload:           os.Getenv("LIVE_RELOAD") == "true",
	}, logger)
	if err := spaHandler.RegisterRoutes(router); err != nil {
		logger.Fatal("failed to register SPA routes", zap.Error(err))
	}

	// Start the single server
	logger.Info("server starting",
		zap.String("port", port),
		zap.String("admin_ui", "http://localhost:"+port+"/admin/"),
		zap.String("customer_ui", "http://localhost:"+port+"/"),
	)
	if err := router.Run(":" + port); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}

// setupVendorRoutes configures vendor-facing routes on the given router group
func setupVendorRoutes(rg *gin.RouterGroup, db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, authProvider auth.AuthProvider, customerBaseURL, nuonAPIURL, dashboardURL, subdomainBaseDomain, superuserEmailDomain string, logger *zap.Logger) {
	// Initialize handlers with customer base URL for install links, Nuon API URL, and base path
	h := handlers.NewHandler(db, jwtAuth, authProvider, customerBaseURL, nuonAPIURL, dashboardURL, "/admin", subdomainBaseDomain, superuserEmailDomain, logger)
	vendorJSON := jsonhandlers.NewVendorHandler(db, nuonAPIURL, customerBaseURL, subdomainBaseDomain)

	// Public routes - vendor login
	rg.GET("/login/config", h.VendorLoginConfigJSON)
	rg.GET("/logout", h.VendorLogout) // Logout handler

	// Auth routes depend on provider type
	if authProvider != nil && authProvider.Name() == "local" {
		// Local password auth routes
		rg.POST("/login/", h.LocalLogin)
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
		orgMgmtRoutes.POST("/org/create", vendorJSON.CreateOrg) // Create new org
	}

	// Profile settings (user can edit their own profile)
	profile := rg.Group("/profile")
	profile.Use(jwtAuth.MiddlewareFunc())
	profile.Use(middleware.RequireRole(models.RoleVendor))
	{
		profile.GET("/me", vendorJSON.Me)
		profile.GET("/orgs", vendorJSON.MeOrgs)
		profile.PUT("", vendorJSON.UpdateProfile)
		profile.PUT("/", vendorJSON.UpdateProfile)
	}

	// Protected vendor routes
	orgs := rg.Group("/orgs")
	orgs.Use(jwtAuth.MiddlewareFunc())
	orgs.Use(middleware.ProcessPendingOrgInvites(db))
	orgs.Use(middleware.RequireRole(models.RoleVendor))
	{
		// Org-level routes (no specific org selected, no RequireOrgContext)
		// This allows users with no orgs to reach this route without redirect loop

		// Restore archived org (outside orgRoutes because archived orgs fail RequireOrgAccessByParam)
		orgs.POST("/:org_id/restore", h.RestoreOrg)

		// Org-specific routes (require org access by param)
		orgRoutes := orgs.Group("/:org_id")
		orgRoutes.Use(middleware.RequireOrgAccessByParam(db))
		{
			orgRoutes.PUT("/", h.UpdateOrg)
			orgRoutes.DELETE("/", h.DeleteOrg)

			// Organization management (was /admin/org/*)
			orgRoutes.POST("/invitations", h.GenerateOrgInvitation)
			orgRoutes.DELETE("/invitations/:id", h.DeleteOrgInvitation)
			orgRoutes.DELETE("/members/:user_id", h.RemoveOrgMember)

			// Settings (was /admin/settings/*)
			orgRoutes.PUT("/settings", h.UpdateThemeSettings)
			orgRoutes.PUT("/settings/login", h.UpdateLoginSettings)
			orgRoutes.POST("/settings/login/test", h.TestLoginConnection)
			orgRoutes.PUT("/settings/dns", h.UpdateDNSSettings)
			orgRoutes.GET("/settings/dns/check", h.CheckSubdomainAvailability)
			orgRoutes.GET("/settings/github", h.GetGitHubConfig)
			orgRoutes.POST("/settings/github", h.SaveGitHubConfig)
			orgRoutes.POST("/settings/github/sync", h.SyncGitHub)
			orgRoutes.DELETE("/settings/github", h.DeleteGitHubConfig)
			orgRoutes.PUT("/settings/github/templates/:page", h.ToggleTemplateOverride)
			orgRoutes.DELETE("/settings/github/templates/:page", h.DeleteTemplateOverride)
			orgRoutes.PUT("/settings/github/assets/*path", h.ToggleAssetOverride)
			orgRoutes.POST("/settings/github/bulk-toggle", h.BulkToggleOverrides)

			// Apps - configuration pages
			orgRoutes.PUT("/apps/:app_id/logo", h.UpdateAppLogo)                   // Save app logos
			orgRoutes.PUT("/apps/:app_id/overview", h.UpdateAppOverview)           // Save app overview
			orgRoutes.POST("/apps/:app_id/overview/preview", h.PreviewAppOverview) // Preview rendered markdown

			// Apps - publish/unpublish for customer portal catalog
			orgRoutes.POST("/apps/:app_id/publish", h.PublishApp)     // Publish app to customer catalog
			orgRoutes.DELETE("/apps/:app_id/publish", h.UnpublishApp) // Remove app from customer catalog
			orgRoutes.DELETE("/apps/:app_id/forget", h.ForgetApp)     // Soft-delete orphaned app record
			orgRoutes.PUT("/apps/order", h.UpdateAppOrder)            // Update published app catalog order

			// Apps - API endpoints (JSON, used by create link modal)
			orgRoutes.GET("/apps-api", vendorJSON.GetOrgApps)
			orgRoutes.GET("/apps-api/catalog", vendorJSON.AppCatalog)
			orgRoutes.GET("/apps-api/:app_id/input-config", vendorJSON.GetAppInputConfig)

			// Local customer-facing input configuration
			orgRoutes.GET("/apps-api/:app_id/customer-input-config", h.GetAppCustomerInputConfig)
			orgRoutes.PUT("/apps-api/:app_id/customer-input-config", h.UpdateAppCustomerInputConfig)

			orgRoutes.POST("/install-links-api", vendorJSON.CreateInstallLink)
			orgRoutes.GET("/install-links-api", vendorJSON.InstallLinks)
			orgRoutes.GET("/install-links-api/:link_id", vendorJSON.InstallLinkDetail)
			orgRoutes.DELETE("/install-links-api/:link_id", vendorJSON.DeleteInstallLink)
			orgRoutes.GET("/token-status", vendorJSON.OrgTokenStatus)
			orgRoutes.POST("/install-links", h.CreateInstallLink)
			orgRoutes.DELETE("/install-links/:link_id", h.DeleteInstallLink)

			// Customer Installs - admin view of all portal installs
			orgRoutes.GET("/installs-api", vendorJSON.CustomerInstalls)
			orgRoutes.GET("/installs-api/search-nuon", vendorJSON.SearchNuonInstalls)
			orgRoutes.POST("/installs-api/import", vendorJSON.ImportInstall)
			orgRoutes.POST("/installs-api/:install_id/forget", vendorJSON.AdminForgetInstall)
			orgRoutes.GET("/installs/search-nuon", h.SearchNuonInstalls)
			orgRoutes.POST("/installs/import", h.ImportInstall)
			orgRoutes.POST("/installs/:install_id/forget", h.AdminForgetInstall)

			// Customers - view all customers and their installs

			// Customer Accounts - view customer company accounts
			orgRoutes.GET("/accounts-api", vendorJSON.Accounts)
			orgRoutes.GET("/accounts/:account_id/installs/search", h.SearchOrgInstalls)
			orgRoutes.POST("/accounts/:account_id/installs/assign", h.AssignInstallToAccount)
			orgRoutes.POST("/accounts/:account_id/invite", h.VendorInviteAccountMember)
			orgRoutes.DELETE("/accounts/:account_id/invite/:invite_id", h.VendorDeleteAccountInvite)

		}
	}

	// Superuser routes (restricted to superuser email domain)
	if superuserEmailDomain != "" {
		superuser := rg.Group("/superuser")
		superuser.Use(jwtAuth.MiddlewareFunc())
		superuser.Use(middleware.RequireRole(models.RoleVendor))
		superuser.Use(middleware.RequireSuperuser(superuserEmailDomain))
		{
			superuser.GET("/orgs/search", h.SuperuserSearchOrgs)
			superuser.GET("/orgs/:org_id", h.SuperuserOrgDetail)
			superuser.POST("/orgs/:org_id/join", h.SuperuserJoinOrg)
			superuser.DELETE("/orgs/:org_id/leave", h.SuperuserLeaveOrg)

			// JSON API endpoints for the React superuser panel
			superuserAPI := rg.Group("/superuser-api")
			superuserAPI.Use(jwtAuth.MiddlewareFunc())
			superuserAPI.Use(middleware.RequireRole(models.RoleVendor))
			superuserAPI.Use(middleware.RequireSuperuser(superuserEmailDomain))
			{
				superuserAPI.GET("/orgs/search", vendorJSON.SuperuserSearchOrgs)
				superuserAPI.GET("/orgs/:org_id", vendorJSON.SuperuserOrgDetail)
				superuserAPI.POST("/orgs/:org_id/join", vendorJSON.SuperuserJoinOrg)
				superuserAPI.DELETE("/orgs/:org_id/leave", vendorJSON.SuperuserLeaveOrg)
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
func setupCustomerRoutes(rg *gin.RouterGroup, db *gorm.DB, jwtAuth *jwt.GinJWTMiddleware, customerAuthFactory *auth.CustomerAuthProviderFactory, customerBaseURL, nuonAPIURL, subdomainBaseDomain string, logger *zap.Logger) {
	// Add subdomain detection middleware to all routes
	// This extracts subdomain from the host and stores it in context
	rg.Use(middleware.SubdomainContext(subdomainBaseDomain))

	// Redirect customer-facing routes to admin login when accessed without subdomain
	// This prevents users from getting stuck on pages that require org context
	rg.Use(middleware.RedirectBaseDomainCustomerRoutes())

	// Initialize handlers with customer auth factory (OIDC only, no local auth)
	h := handlers.NewHandlerWithCustomerAuth(db, jwtAuth, customerAuthFactory, customerBaseURL, nuonAPIURL, "", subdomainBaseDomain, logger)
	customerJSON := jsonhandlers.NewCustomerAuthHandler(customerBaseURL)
	portalJSON := jsonhandlers.NewCustomerPortalHandler(db, customerBaseURL, subdomainBaseDomain, nuonAPIURL)
	wizardJSON := jsonhandlers.NewCustomerInstallWizardHandler(db, customerBaseURL, subdomainBaseDomain, nuonAPIURL)
	installDetailJSON := jsonhandlers.NewCustomerInstallDetailHandler(db, nuonAPIURL)

	// Root redirect to customer login (with subdomain) or admin login (without subdomain)
	// The RedirectBaseDomainCustomerRoutes middleware handles the base domain case

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

	// JSON auth endpoints for React customer app
	// These allow login/session/logout flows without relying on HTML page redirects.
	authAPI := rg.Group("/auth-api")
	{
		authAPI.GET("/login-url", customerJSON.LoginURL)
		authAPI.POST("/logout", customerJSON.Logout)

		authSession := authAPI.Group("")
		authSession.Use(jwtAuth.MiddlewareFunc())
		authSession.Use(middleware.RequireRole(models.RoleCustomer))
		{
			authSession.GET("/session", customerJSON.Session)
		}
	}

	customerAPI := rg.Group("/portal-api")
	customerAPI.Use(jwtAuth.MiddlewareFunc())
	customerAPI.Use(middleware.RequireRole(models.RoleCustomer))
	{
		customerAPI.GET("/state", portalJSON.State)
		customerAPI.GET("/installs/:install_id/detail", installDetailJSON.Detail)
		customerAPI.GET("/apps/:app_id/install-wizard", wizardJSON.State)
		customerAPI.POST("/apps/:app_id/install-wizard", wizardJSON.Create)

		customerWorkflowActions := customerAPI.Group("/installs/:install_id/workflows/:workflow_id")
		customerWorkflowActions.Use(middleware.RequireCustomerAccount(db))
		{
			customerWorkflowActions.POST("/approve", wizardJSON.ApproveWorkflowStep)
			customerWorkflowActions.POST("/approve-all", wizardJSON.ApproveAllWorkflowSteps)
		}
	}

	// Subdomain completion endpoint (sets JWT cookie on subdomain after base domain auth)
	rg.GET("/auth/complete", h.CompleteSubdomainAuth)

	// Public routes - customer login and logout (OIDC only)
	rg.GET("/logout", h.CustomerLogout)

	// Registration disabled - redirect to login

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

	// Published app config endpoint (unauthenticated - same pattern as install-link app-config)
	rg.GET("/apps/:app_id/config", h.GetPublishedAppConfig)

	// Install form fields partial (returns HTML for HTMX, unauthenticated)

	// Public customer routes for published apps (auth handled in handlers)
	customerApps := rg.Group("/apps")
	{
		customerApps.POST("/:app_id/install", h.CreateInstallFromApp)
	}

	// Public install list routes (auth is optional — unauthenticated visitors see login CTA)
	// Note: /installs/:install_id is handled by installOwnership.GET("/") below

	// Customer account routes (require auth but exempt from account middleware)
	account := rg.Group("/account")
	account.Use(jwtAuth.MiddlewareFunc())
	account.Use(middleware.RequireRole(models.RoleCustomer))
	{
		account.PUT("/", h.UpdateAccount)
		account.POST("/create", h.CreateAccount)
		account.POST("/switch", h.SwitchAccount)
		account.POST("/invite", h.CreateAccountInvite)
		account.DELETE("/invite/:invite_id", h.DeleteAccountInvite)
		account.DELETE("/members/:member_id", h.DeleteAccountMember)
		account.POST("/members/:member_id/transfer-ownership", h.TransferAccountOwnership)
	}

	// Protected customer routes
	installs := rg.Group("/installs")
	installs.Use(jwtAuth.MiddlewareFunc())
	installs.Use(middleware.RequireRole(models.RoleCustomer))
	installs.Use(middleware.RequireCustomerAccount(db))
	{
		// Routes that require install ownership verification
		installOwnership := installs.Group("/:install_id")
		installOwnership.Use(middleware.RequireInstallOwnership(db))
		{
			// Install detail pages (each tab is a separate route)

			// Debug pages (vendor-only, access checked in handler)
			installOwnership.GET("/debug", h.DebugRedirect)
			installOwnership.GET("/debug/workflows", h.DebugWorkflowsPage)
			installOwnership.GET("/debug/workflows/:workflow_id", h.DebugWorkflowDetailPage)
			installOwnership.POST("/debug/workflows/:workflow_id/approve", h.DebugApproveStep)
			installOwnership.POST("/debug/workflows/:workflow_id/approve-all", h.DebugApproveAllSteps)
			installOwnership.POST("/debug/workflows/:workflow_id/cancel", h.DebugCancelWorkflow)
			installOwnership.POST("/debug/workflows/:workflow_id/retry", h.DebugRetryStep)

			// Panel endpoints (HTMX fragments loaded by the tab pages)

			installOwnership.PUT("/", h.UpdateInstall)                  // Customer can update their install
			installOwnership.DELETE("/", h.DeleteInstall)               // Customer can delete (deprovision) their install
			installOwnership.POST("/deprovision", h.DeleteInstall)      // Deprovision via HTML form POST (same handler as DELETE)
			installOwnership.POST("/forget", h.ForgetInstall)           // Customer can forget (remove from DB) their install
			installOwnership.POST("/reprovision", h.ReprovisionInstall) // Customer can reprovision their install

			// Input management
			installOwnership.GET("/inputs", h.GetInstallInputs)    // Customer can view current inputs
			installOwnership.PUT("/inputs", h.UpdateInstallInputs) // Customer can update inputs

			// Workflow actions
			installOwnership.POST("/workflows/:workflow_id/approve", h.ApproveWorkflowStep)
			installOwnership.POST("/workflows/:workflow_id/approve-all", h.ApproveAllWorkflowSteps)
			installOwnership.POST("/workflows/:workflow_id/cancel", h.CancelWorkflow)
			installOwnership.POST("/workflows/:workflow_id/step/:step_id/retry", h.RetryWorkflowStep)
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
