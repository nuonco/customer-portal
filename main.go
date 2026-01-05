package main

import (
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/powertoolsdev/mono/exp/installer/app/internal/background"
	"github.com/powertoolsdev/mono/exp/installer/app/internal/handlers"
	"github.com/powertoolsdev/mono/exp/installer/app/internal/middleware"
	"github.com/powertoolsdev/mono/exp/installer/app/internal/models"
)

// templateFuncs returns custom template functions for all routers
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// safeURL marks a string as safe for use in URL contexts (like img src, font src)
		// This is needed for data URIs which Go's template engine escapes by default
		"safeURL": func(s string) template.URL {
			// Allow data URIs for images and fonts
			// Browsers may use various MIME types for fonts including application/octet-stream
			if strings.HasPrefix(s, "data:image/") ||
				strings.HasPrefix(s, "data:font/") ||
				strings.HasPrefix(s, "data:application/font") ||
				strings.HasPrefix(s, "data:application/octet-stream") {
				return template.URL(s)
			}
			return template.URL("")
		},
		// contains checks if a string contains a substring
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		// dict creates a map from pairs of key-value arguments
		// Used to pass multiple values to partial templates
		// Example: {{template "partial" (dict "key1" .Value1 "key2" .Value2)}}
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					continue
				}
				dict[key] = values[i+1]
			}
			return dict
		},
		// default returns the first non-empty value, or the fallback if all are empty
		// Example: {{default .value "fallback"}}
		"default": func(value, fallback interface{}) interface{} {
			if value == nil {
				return fallback
			}
			// Check for empty string
			if str, ok := value.(string); ok && str == "" {
				return fallback
			}
			// Check for zero int
			if num, ok := value.(int); ok && num == 0 {
				return fallback
			}
			// Check for false bool (only if fallback is also bool)
			if b, ok := value.(bool); ok {
				if fb, ok := fallback.(bool); ok && !b {
					return fb
				}
			}
			return value
		},
		// ternary returns trueVal if condition is true, otherwise falseVal
		// Example: {{ternary .condition "yes" "no"}}
		"ternary": func(condition bool, trueVal, falseVal interface{}) interface{} {
			if condition {
				return trueVal
			}
			return falseVal
		},
		// eq checks if two values are equal (useful for string comparisons in nested contexts)
		// Note: Go templates have built-in eq, but this ensures consistent behavior
		"streq": func(a, b string) bool {
			return a == b
		},
		// hasKey checks if a map has a given key
		// Example: {{if hasKey .dict "mykey"}}...{{end}}
		"hasKey": func(dict map[string]interface{}, key string) bool {
			if dict == nil {
				return false
			}
			_, ok := dict[key]
			return ok
		},
		// upper converts a string to uppercase
		// Example: {{slice .user.Email 0 1 | upper}}
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
	}
}

// loadTemplates loads all HTML templates including those in subdirectories (partials)
func loadTemplates(r *gin.Engine) {
	// Create a new template with custom functions
	tmpl := template.New("").Funcs(templateFuncs())

	// Walk through templates directory and parse all .html files
	err := filepath.Walk("internal/templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			// Parse template with relative name for lookup
			// Convert path separators and strip the base directory
			name := strings.TrimPrefix(path, "internal/templates/")
			name = strings.ReplaceAll(name, string(filepath.Separator), "/")

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}

			_, parseErr := tmpl.New(name).Parse(string(content))
			if parseErr != nil {
				return parseErr
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	r.SetHTMLTemplate(tmpl)
}

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

	// Create single router with shared middleware
	router := gin.Default()

	// Set custom template functions and load HTML templates (including partials)
	router.SetFuncMap(templateFuncs())
	loadTemplates(router)

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
	setupVendorRoutes(router.Group("/admin"), db, vendorAuth, customerBaseURL, nuonAPIURL)

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
func setupVendorRoutes(rg *gin.RouterGroup, db *gorm.DB, auth *jwt.GinJWTMiddleware, customerBaseURL, nuonAPIURL string) {
	// Initialize handlers with customer base URL for install links, Nuon API URL, and base path
	h := handlers.NewHandler(db, auth, customerBaseURL, nuonAPIURL, "/admin")

	// Root redirect to login
	rg.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin/login")
	})

	// Public routes - vendor login
	login := rg.Group("/login")
	{
		login.GET("/", h.VendorLoginPageTempl) // Using Templ
		login.POST("/", auth.LoginHandler)
	}

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
	h := handlers.NewHandler(db, auth, "", nuonAPIURL, "")

	// Public routes - customer login (no signup)
	login := rg.Group("/login")
	{
		login.GET("/", h.LoginPage(handlers.CustomerLoginConfig("")))
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
