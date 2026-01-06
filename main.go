package main

import (
	"log"
	"os"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/powertoolsdev/mono/exp/installer/app/internal/background"
	"github.com/powertoolsdev/mono/exp/installer/app/internal/handlers"
	"github.com/powertoolsdev/mono/exp/installer/app/internal/middleware"
	"github.com/powertoolsdev/mono/exp/installer/app/internal/models"
)

func main() {
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

	// WorkOS configuration for vendor authentication
	workosAPIKey := os.Getenv("WORKOS_API_KEY")
	workosClientID := os.Getenv("WORKOS_CLIENT_ID")
	workosRedirectURI := os.Getenv("WORKOS_REDIRECT_URI")
	if workosRedirectURI == "" {
		workosRedirectURI = "http://localhost:" + port + "/admin/callback"
	}

	// Initialize WorkOS auth (optional - only if credentials provided)
	var workosAuth *middleware.WorkOSAuth
	if workosAPIKey != "" && workosClientID != "" {
		workosAuth = middleware.NewWorkOSAuth(workosAPIKey, workosClientID, workosRedirectURI, db)
		log.Printf("WorkOS authentication enabled")
	} else {
		log.Printf("WorkOS authentication disabled (WORKOS_API_KEY and WORKOS_CLIENT_ID not set)")
	}

	// Create single router with shared middleware
	router := gin.Default()

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
	setupVendorRoutes(router.Group("/admin"), db, vendorAuth, workosAuth, customerBaseURL, nuonAPIURL)

	// Set up customer routes at root level (no prefix)
	setupCustomerRoutes(router.Group(""), db, customerAuth, nuonAPIURL)

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
func setupVendorRoutes(rg *gin.RouterGroup, db *gorm.DB, auth *jwt.GinJWTMiddleware, workosAuth *middleware.WorkOSAuth, customerBaseURL, nuonAPIURL string) {
	// Initialize handlers with customer base URL for install links, Nuon API URL, and base path
	h := handlers.NewHandler(db, auth, workosAuth, customerBaseURL, nuonAPIURL, "/admin")

	// Root redirect to login
	rg.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin/login")
	})

	// Public routes - vendor login (WorkOS AuthKit)
	rg.GET("/login/", h.VendorLoginPageTempl)
	rg.GET("/callback", h.WorkOSCallback) // OAuth callback from WorkOS
	rg.GET("/logout", h.VendorLogout)     // Logout handler (clears JWT + WorkOS session)

	// JWT refresh endpoint
	rg.POST("/refresh_token", auth.RefreshHandler)

	// Global theme settings (any vendor can edit)
	settings := rg.Group("/settings")
	settings.Use(auth.MiddlewareFunc())
	settings.Use(middleware.RequireRole(models.RoleVendor))
	{
		settings.GET("/", h.ThemeSettingsPage)
		settings.GET("/panel", h.ThemeSettingsPanelContent)
		settings.PUT("/", h.UpdateThemeSettings)
	}

	// Profile settings (user can edit their own profile)
	profile := rg.Group("/profile")
	profile.Use(auth.MiddlewareFunc())
	profile.Use(middleware.RequireRole(models.RoleVendor))
	{
		profile.GET("/panel", h.ProfilePanelContent)
		profile.PUT("/", h.UpdateProfile)
	}

	// Protected vendor routes
	orgs := rg.Group("/orgs")
	orgs.Use(auth.MiddlewareFunc())
	orgs.Use(middleware.RequireRole(models.RoleVendor))
	{
		orgs.GET("/", h.OrgsPage)
		orgs.POST("/", h.CreateOrg)
		orgs.GET("/:org_id/links", h.OrgDetailPage)
		orgs.GET("/:org_id/settings", h.OrgSettingsPage)
		orgs.PUT("/:org_id", h.UpdateOrg)
		orgs.DELETE("/:org_id", h.DeleteOrg)

		// Apps - health check configuration pages
		orgs.GET("/:org_id/apps", h.AppsPage)                                    // Apps list page (HTML)
		orgs.GET("/:org_id/apps/:app_id", h.AppDetailRedirect)                   // Redirect to health-checks
		orgs.GET("/:org_id/apps/:app_id/health-checks", h.AppHealthChecksPage)   // Health checks config page
		orgs.PUT("/:org_id/apps/:app_id/health-checks", h.UpdateAppHealthChecks) // Update health checks

		// Apps - API endpoints (JSON, used by create link modal)
		orgs.GET("/:org_id/apps-api", h.GetOrgApps)
		orgs.GET("/:org_id/apps-api/:app_id/input-config", h.GetAppInputConfig)
		orgs.GET("/:org_id/apps-api/:app_id/actions", h.GetAppActions)

		orgs.POST("/:org_id/links", h.CreateInstallLink)
		orgs.GET("/:org_id/links/:link_id", h.InstallLinkDetail)
		orgs.GET("/:org_id/links/:link_id/status", h.InstallLinkStatus) // HTMX polling endpoint
		orgs.DELETE("/:org_id/links/:link_id", h.DeleteInstallLink)
	}

	// Debug endpoint for troubleshooting
	debug := rg.Group("/debug")
	debug.Use(auth.MiddlewareFunc())
	{
		debug.GET("/user-orgs", h.DebugUserOrgs)
	}
}

// setupCustomerRoutes configures customer-facing routes on the given router group
func setupCustomerRoutes(rg *gin.RouterGroup, db *gorm.DB, auth *jwt.GinJWTMiddleware, nuonAPIURL string) {
	// Initialize handlers (customer doesn't generate install links, so base URL not needed)
	// Empty basePath since customer routes are at root level
	// nil for workosAuth since customers don't use WorkOS
	h := handlers.NewHandler(db, auth, nil, "", nuonAPIURL, "")

	// Public routes - customer login (no signup)
	login := rg.Group("/login")
	{
		login.GET("/", h.CustomerLoginPageTempl)
		login.POST("/", auth.LoginHandler)
	}

	// Install link acceptance (customer signup flow)
	installLinks := rg.Group("/install-link")
	{
		installLinks.GET("/", h.InstallLinkPage)
		installLinks.POST("/", h.AcceptInstallLink)
	}

	// JWT refresh endpoint
	rg.POST("/refresh_token", auth.RefreshHandler)

	// Protected customer routes
	installs := rg.Group("/installs")
	installs.Use(auth.MiddlewareFunc())
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

			// Workflow history and actions
			installOwnership.GET("/workflows", h.WorkflowsPage)
			installOwnership.POST("/workflows/:workflow_id/approve", h.ApproveWorkflowStep)
			installOwnership.POST("/workflows/:workflow_id/approve-all", h.ApproveAllWorkflowSteps)
			installOwnership.POST("/workflows/:workflow_id/cancel", h.CancelWorkflow)
		}
	}
}
