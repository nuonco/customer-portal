package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
)

type LoginCredentials struct {
	Email string `json:"email" binding:"required"`
}

// isHTMXRequest checks if the request is from HTMX
func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
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
			accept := c.GetHeader("Accept")

			// Check if this is an HTMX request
			if isHTMXRequest(c) {
				// For HTMX polling requests, return empty HTML that triggers re-authentication
				// HTMX will swap this empty content, and we can add an event listener to handle it
				c.Header("HX-Trigger", "auth-error")
				c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(""))
				c.Abort()
				return
			}

			// For HTML requests (browser navigation), redirect to login
			if strings.Contains(accept, "text/html") {
				redirectURL := opts.BasePath + "/login"
				currentURL := c.Request.URL.String()
				if currentURL != "/" && currentURL != "/login" {
					redirectURL += "?redirect=" + url.QueryEscape(currentURL)
				}
				c.Redirect(http.StatusFound, redirectURL)
				c.Abort()
				return
			}

			// For all other requests (API, JSON), return JSON error
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
			c.Abort()
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

		// Check access based on active account from middleware context
		var install models.Install
		activeMember := GetCustomerAccountMember(c)

		var err error
		if activeMember != nil {
			err = db.Where("id = ? AND ((user_id = ? AND (customer_account_id IS NULL OR customer_account_id = ?)) OR (customer_account_id = ? AND visibility = ?))",
				installID, user.ID, activeMember.AccountID, activeMember.AccountID, models.VisibilityAccount).First(&install).Error
		} else {
			err = db.Where("id = ? AND user_id = ? AND customer_account_id IS NULL",
				installID, user.ID).First(&install).Error
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

// RequireOrgAccessByParam middleware validates access to a specific org by URL parameter
// Validates the org_id param and that the user has membership to the requested org
func RequireOrgAccessByParam(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)

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

		// Verify user has membership to the requested org
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

// ProcessPendingOrgInvites checks for pending org invitations matching the
// current user's email and auto-joins them to those orgs. Non-blocking:
// errors are logged but don't affect the request.
func ProcessPendingOrgInvites(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)

		var invites []models.OrgInvitation
		if err := db.Where("email = ? AND accepted_at IS NULL AND deleted_at IS NULL", user.Email).
			Find(&invites).Error; err != nil {
			zap.L().Error("failed to query pending org invites", zap.Error(err))
			c.Next()
			return
		}

		for _, invite := range invites {
			// Check if already a member
			var count int64
			db.Model(&models.OrgMember{}).Where("org_id = ? AND user_id = ? AND deleted_at IS NULL",
				invite.OrgID, user.ID).Count(&count)
			if count > 0 {
				// Already a member, just mark invite as accepted
				now := time.Now()
				db.Model(&invite).Updates(map[string]interface{}{
					"accepted_at": &now,
					"used_count":  1,
				})
				continue
			}

			now := time.Now()
			txErr := db.Transaction(func(tx *gorm.DB) error {
				// Check for soft-deleted member and restore instead of creating duplicate
				var existingMember models.OrgMember
				if err := tx.Unscoped().Where("org_id = ? AND user_id = ? AND deleted_at IS NOT NULL", invite.OrgID, user.ID).First(&existingMember).Error; err == nil {
					if err := tx.Unscoped().Model(&existingMember).Updates(map[string]interface{}{
						"deleted_at": nil,
						"status":     models.MemberStatusActive,
						"invited_by": &invite.InvitedBy,
						"joined_at":  &now,
					}).Error; err != nil {
						return err
					}
				} else {
					member := models.OrgMember{
						OrgID:     invite.OrgID,
						UserID:    user.ID,
						InvitedBy: &invite.InvitedBy,
						Status:    models.MemberStatusActive,
						JoinedAt:  &now,
					}
					if err := tx.Create(&member).Error; err != nil {
						return err
					}
				}

				// Upgrade customer to vendor role when auto-joining a vendor org
				var dbUser models.User
				if err := tx.First(&dbUser, "id = ?", user.ID).Error; err == nil {
					if dbUser.Role == models.RoleCustomer {
						tx.Model(&dbUser).Update("role", models.RoleVendor)
					}
				}

				return tx.Model(&invite).Updates(map[string]interface{}{
					"accepted_at": &now,
					"used_count":  1,
				}).Error
			})
			if txErr != nil {
				zap.L().Error("failed to auto-join org via invite",
					zap.String("org_id", invite.OrgID),
					zap.String("email", user.Email),
					zap.Error(txErr))
			}
		}

		// If user was a customer and got upgraded to vendor, update JWT claims
		// so downstream middleware (RequireRole) sees the new role.
		if user.Role == models.RoleCustomer {
			var dbUser models.User
			if err := db.First(&dbUser, "id = ?", user.ID).Error; err == nil {
				if dbUser.Role == models.RoleVendor {
					if claims, ok := c.Get("JWT_PAYLOAD"); ok {
						if mc, ok := claims.(jwt.MapClaims); ok {
							mc["role"] = string(models.RoleVendor)
						}
					}
				}
			}
		}

		c.Next()
	}
}
