package middleware

import (
	"net/http"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
)

type LoginCredentials struct {
	Email string `json:"email" binding:"required"`
}

// AuthOptions configures JWT middleware behavior for different user types
type AuthOptions struct {
	AllowSignup   bool            // true for vendor (auto-create accounts), false for customer
	DefaultRole   models.UserRole // role to assign new users (if AllowSignup is true)
	LoginRedirect string          // where to redirect after successful login ("/admin/orgs" or "/installs")
	BasePath      string          // base path prefix for routes (e.g., "/admin" or "" for root)
}

func NewJWTMiddleware(db *gorm.DB, jwtSecret string, opts AuthOptions) (*jwt.GinJWTMiddleware, error) {
	return jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "installer-app",
		Key:         []byte(jwtSecret),
		Timeout:     time.Hour * 24,
		MaxRefresh:  time.Hour * 24,
		IdentityKey: "user_id",

		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if user, ok := data.(*models.User); ok {
				return jwt.MapClaims{
					"user_id": user.ID,
					"name":    user.Name,
					"email":   user.Email,
					"role":    user.Role,
				}
			}
			return jwt.MapClaims{}
		},

		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &models.User{
				ID:    claims["user_id"].(string),
				Name:  getStringClaim(claims, "name"),
				Email: claims["email"].(string),
				Role:  models.UserRole(claims["role"].(string)),
			}
		},

		Authenticator: func(c *gin.Context) (interface{}, error) {
			var creds LoginCredentials
			if err := c.ShouldBind(&creds); err != nil {
				return "", jwt.ErrMissingLoginValues
			}

			// Check if user exists
			var user models.User
			if err := db.Where("email = ?", creds.Email).First(&user).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					if !opts.AllowSignup {
						// Customer port: no signup allowed, must use install-link
						return nil, jwt.ErrFailedAuthentication
					}
					// Vendor port: auto-create new account
					user = models.User{
						Email: creds.Email,
						Role:  opts.DefaultRole,
					}
					if err := db.Create(&user).Error; err != nil {
						return nil, jwt.ErrFailedAuthentication
					}
				} else {
					return nil, jwt.ErrFailedAuthentication
				}
			}

			return &user, nil
		},

		Authorizator: func(data interface{}, c *gin.Context) bool {
			// Basic authorization - user exists and has valid role
			if user, ok := data.(*models.User); ok {
				return user.Role == models.RoleVendor || user.Role == models.RoleCustomer
			}
			return false
		},

		Unauthorized: func(c *gin.Context, code int, message string) {
			// Check if this is an HTML request (browser) or API request
			accept := c.GetHeader("Accept")
			if strings.Contains(accept, "text/html") {
				// Redirect HTML requests to login page (using BasePath prefix)
				c.Redirect(http.StatusFound, opts.BasePath+"/login")
			} else {
				// Return JSON for API requests
				c.JSON(code, gin.H{
					"code":    code,
					"message": message,
				})
			}
		},

		TokenLookup: "header: Authorization, query: token, cookie: jwt",
		TimeFunc:    time.Now,
	})
}

// RequireRole middleware to check specific roles
// Vendors are treated as "super users" and can access customer routes
func RequireRole(role models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		userRole := models.UserRole(claims["role"].(string))

		// Vendors can access all routes
		if userRole == models.RoleVendor {
			c.Next()
			return
		}

		if userRole != role {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireInstallOwnership middleware to verify user has access to the install
// On the customer portal, access is always determined by user_id ownership
func RequireInstallOwnership(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)
		installID := c.Param("install_id")

		if installID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Install ID is required",
			})
			c.Abort()
			return
		}

		// Validate install_id format
		if !shortid.IsValid(installID) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid install ID format",
			})
			c.Abort()
			return
		}

		// Check access - on customer portal, always check user_id ownership
		// (regardless of the user's database role, as users may have accepted
		// installs through install-links which link via user_id)
		var install models.Install
		err := db.Where("id = ? AND user_id = ?", installID, user.ID).First(&install).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Install not found or access denied",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to verify install ownership",
				})
			}
			c.Abort()
			return
		}

		// Store install in context for handler use
		c.Set("install", &install)
		c.Next()
	}
}

// GetCurrentUser helper to extract user from JWT claims
func GetCurrentUser(c *gin.Context) *models.User {
	claims := jwt.ExtractClaims(c)
	return &models.User{
		ID:    claims["user_id"].(string),
		Name:  getStringClaim(claims, "name"),
		Email: claims["email"].(string),
		Role:  models.UserRole(claims["role"].(string)),
	}
}

// getStringClaim safely extracts a string claim from JWT MapClaims
// Returns empty string if claim is missing or not a string
func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok && val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// RequireOrgContext middleware validates org context for vendor users
// Reads org_id from cookie, validates membership, and loads org into context
// If no valid org cookie exists, auto-selects the user's first available org
func RequireOrgContext(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)

		// Only apply to vendor users
		if user.Role != models.RoleVendor {
			c.Next()
			return
		}

		// Get org_id from cookie
		orgID, err := c.Cookie("org_id")

		// Helper function to auto-select first available org
		autoSelectOrg := func() bool {
			var memberships []models.OrgMember
			if err := db.Where("user_id = ? AND status = ?", user.ID, models.MemberStatusActive).
				Preload("Org", "deleted_at IS NULL").
				Find(&memberships).Error; err != nil || len(memberships) == 0 {
				return false
			}
			// Find first membership with a valid (non-deleted) org
			for i := range memberships {
				if memberships[i].Org.ID != "" {
					SetOrgCookie(c, memberships[i].OrgID)
					c.Set("org", &memberships[i].Org)
					return true
				}
			}
			return false
		}

		if err != nil || orgID == "" {
			// No org cookie - try to auto-select first available org
			if autoSelectOrg() {
				c.Next()
				return
			}
			// No orgs available
			accept := c.GetHeader("Accept")
			if strings.Contains(accept, "text/html") {
				c.Redirect(http.StatusFound, "/admin/orgs")
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "No org context",
				})
			}
			c.Abort()
			return
		}

		// Validate org exists
		var org models.NuonOrg
		if err := db.Where("id = ? AND deleted_at IS NULL", orgID).First(&org).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Org not found - clear cookie and try to auto-select another
				ClearOrgCookie(c)
				if autoSelectOrg() {
					c.Next()
					return
				}
				accept := c.GetHeader("Accept")
				if strings.Contains(accept, "text/html") {
					c.Redirect(http.StatusFound, "/admin/orgs")
				} else {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": "Invalid org",
					})
				}
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to load org",
			})
			c.Abort()
			return
		}

		// Verify user is an active member of this org
		var member models.OrgMember
		err = db.Where("org_id = ? AND user_id = ? AND status = ?",
			orgID, user.ID, models.MemberStatusActive).First(&member).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Not a member - clear cookie and try to auto-select another org
				ClearOrgCookie(c)
				if autoSelectOrg() {
					c.Next()
					return
				}
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied to org",
				})
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to verify org membership",
			})
			c.Abort()
			return
		}

		// Store org in context for handlers
		c.Set("org", &org)
		c.Next()
	}
}

// RequireOrgAccessByParam middleware validates access to a specific org by URL parameter
// Must be used after RequireOrgContext - validates the org_id param matches the current org context
// or that the user has membership to the requested org
func RequireOrgAccessByParam(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)
		currentOrg := GetCurrentOrg(c)

		orgID := c.Param("org_id")
		if orgID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Org ID is required",
			})
			c.Abort()
			return
		}

		// Validate org_id format
		if !shortid.IsValid(orgID) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid org ID format",
			})
			c.Abort()
			return
		}

		// If the requested org matches the current context, we're good
		if currentOrg != nil && currentOrg.ID == orgID {
			c.Next()
			return
		}

		// Otherwise, verify user has membership to the requested org
		var member models.OrgMember
		err := db.Where("org_id = ? AND user_id = ? AND status = ?",
			orgID, user.ID, models.MemberStatusActive).First(&member).Error
		if err != nil {
			// For HTML requests, redirect to org list instead of showing error
			accept := c.GetHeader("Accept")
			if strings.Contains(accept, "text/html") {
				c.Redirect(http.StatusFound, "/admin/orgs")
				c.Abort()
				return
			}
			// For API requests, return JSON error
			if err == gorm.ErrRecordNotFound {
				// Return 404 (not 403) for security - don't reveal org existence
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Organization not found",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to verify org access",
				})
			}
			c.Abort()
			return
		}

		// Load the full org and store in context
		var org models.NuonOrg
		if err := db.Where("id = ? AND deleted_at IS NULL", orgID).First(&org).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to load organization",
			})
			c.Abort()
			return
		}

		// Store org in context for handlers (overrides current org context)
		c.Set("org", &org)
		c.Next()
	}
}

// GetCurrentWorkspace is deprecated - use GetCurrentOrg instead
// This function is kept temporarily for backwards compatibility during migration
func GetCurrentWorkspace(c *gin.Context) *models.NuonOrg {
	return GetCurrentOrg(c)
}

// GetCurrentOrg retrieves the org from context
func GetCurrentOrg(c *gin.Context) *models.NuonOrg {
	if org, exists := c.Get("org"); exists {
		if o, ok := org.(*models.NuonOrg); ok {
			return o
		}
	}
	return nil
}

// SetOrgCookie sets the org_id cookie
func SetOrgCookie(c *gin.Context, orgID string) {
	c.SetCookie(
		"org_id",    // name
		orgID,       // value
		60*60*24*30, // maxAge (30 days)
		"/",         // path
		"",          // domain (empty = current domain)
		false,       // secure (set to true in production with HTTPS)
		true,        // httpOnly
	)
}

// ClearOrgCookie removes the org_id cookie
func ClearOrgCookie(c *gin.Context) {
	c.SetCookie(
		"org_id", // name
		"",       // value (empty)
		-1,       // maxAge (negative = delete)
		"/",      // path
		"",       // domain
		false,    // secure
		true,     // httpOnly
	)
}

// SetWorkspaceCookie is deprecated - use SetOrgCookie instead
// Kept temporarily for backwards compatibility during migration
func SetWorkspaceCookie(c *gin.Context, orgID string) {
	SetOrgCookie(c, orgID)
}

// ClearWorkspaceCookie is deprecated - use ClearOrgCookie instead
// Kept temporarily for backwards compatibility during migration
func ClearWorkspaceCookie(c *gin.Context) {
	ClearOrgCookie(c)
}
