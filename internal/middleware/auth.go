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
// Customers must own the install (user_id), vendors must have created it (created_by_vendor_id)
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

		// Check access based on role
		var install models.Install
		var err error
		if user.Role == models.RoleVendor {
			// Vendors can access installs they created
			err = db.Where("id = ? AND created_by_vendor_id = ?", installID, user.ID).First(&install).Error
		} else {
			// Customers can access installs they own
			err = db.Where("id = ? AND user_id = ?", installID, user.ID).First(&install).Error
		}

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

// RequireWorkspaceContext middleware validates workspace context for vendor users
// Reads workspace_id from cookie, validates membership, and loads workspace into context
func RequireWorkspaceContext(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)

		// Only apply to vendor users
		if user.Role != models.RoleVendor {
			c.Next()
			return
		}

		// Get workspace_id from cookie
		workspaceID, err := c.Cookie("workspace_id")
		if err != nil || workspaceID == "" {
			// No workspace cookie - redirect to workspace selector
			accept := c.GetHeader("Accept")
			if strings.Contains(accept, "text/html") {
				c.Redirect(http.StatusFound, "/admin/workspaces/select")
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "No workspace context",
				})
			}
			c.Abort()
			return
		}

		// Validate workspace exists
		var workspace models.Workspace
		if err := db.Where("id = ? AND deleted_at IS NULL", workspaceID).First(&workspace).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Workspace not found - clear cookie and redirect
				ClearWorkspaceCookie(c)
				accept := c.GetHeader("Accept")
				if strings.Contains(accept, "text/html") {
					c.Redirect(http.StatusFound, "/admin/workspaces/select")
				} else {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": "Invalid workspace",
					})
				}
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to load workspace",
			})
			c.Abort()
			return
		}

		// Verify user is an active member of this workspace
		var member models.WorkspaceMember
		err = db.Where("workspace_id = ? AND user_id = ? AND status = ?",
			workspaceID, user.ID, models.MemberStatusActive).First(&member).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Not a member - clear cookie and show forbidden
				ClearWorkspaceCookie(c)
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied to workspace",
				})
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to verify workspace membership",
			})
			c.Abort()
			return
		}

		// Store workspace in context for handlers
		c.Set("workspace", &workspace)
		c.Next()
	}
}

// RequireOrgAccess middleware validates that the org belongs to the current workspace
// Must be used after RequireWorkspaceContext
func RequireOrgAccess(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		workspace := GetCurrentWorkspace(c)
		if workspace == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Workspace context not found",
			})
			c.Abort()
			return
		}

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

		// Verify org belongs to workspace
		var org models.NuonOrg
		err := db.Where("id = ? AND workspace_id = ? AND deleted_at IS NULL", orgID, workspace.ID).First(&org).Error
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
					"error": "Failed to load organization",
				})
			}
			c.Abort()
			return
		}

		// Store org in context for handlers
		c.Set("org", &org)
		c.Next()
	}
}

// GetCurrentWorkspace retrieves the workspace from context
func GetCurrentWorkspace(c *gin.Context) *models.Workspace {
	if workspace, exists := c.Get("workspace"); exists {
		if ws, ok := workspace.(*models.Workspace); ok {
			return ws
		}
	}
	return nil
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

// SetWorkspaceCookie sets the workspace_id cookie
func SetWorkspaceCookie(c *gin.Context, workspaceID string) {
	c.SetCookie(
		"workspace_id", // name
		workspaceID,    // value
		60*60*24*30,    // maxAge (30 days)
		"/",            // path
		"",             // domain (empty = current domain)
		false,          // secure (set to true in production with HTTPS)
		true,           // httpOnly
	)
}

// ClearWorkspaceCookie removes the workspace_id cookie
func ClearWorkspaceCookie(c *gin.Context) {
	c.SetCookie(
		"workspace_id", // name
		"",             // value (empty)
		-1,             // maxAge (negative = delete)
		"/",            // path
		"",             // domain
		false,          // secure
		true,           // httpOnly
	)
}
