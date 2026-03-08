package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// RequireCustomerAccount middleware resolves CustomerAccount membership for any authenticated user on a subdomain.
// If no membership is found, customers are redirected to /installs; vendors pass through for portal preview.
// When a user has multiple memberships, the active_account_id cookie selects which one to use.
func RequireCustomerAccount(db *gorm.DB) gin.HandlerFunc {
	exemptPrefixes := []string{
		"/account/",
		"/login",
		"/logout",
		"/auth/",
		"/install-link",
		"/custom/",
		"/static/",
		"/livez",
		"/readyz",
	}

	return func(c *gin.Context) {
		// Check if path is exempt
		path := c.Request.URL.Path
		for _, prefix := range exemptPrefixes {
			if strings.HasPrefix(path, prefix) || path == prefix {
				c.Next()
				return
			}
		}

		user := GetCurrentUser(c)
		if user == nil {
			c.Next()
			return
		}

		// Look up org by subdomain
		subdomain, _ := c.Get("subdomain")
		subdomainStr, _ := subdomain.(string)
		if subdomainStr == "" {
			c.Next()
			return
		}

		var org models.NuonOrg
		if err := db.Where("subdomain = ? AND deleted_at IS NULL", subdomainStr).First(&org).Error; err != nil {
			c.Next()
			return
		}

		// Query all memberships for this user in this org
		var members []models.CustomerAccountMember
		if err := db.Preload("Account").Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", user.ID, org.ID).Find(&members).Error; err != nil {
			c.Next()
			return
		}

		// Check for pending invites (even if user already has memberships)
		var invite models.CustomerAccountInvite
		if err := db.Where("email = ? AND org_id = ? AND used_by_user_id IS NULL AND deleted_at IS NULL", user.Email, org.ID).First(&invite).Error; err == nil {
			txErr := db.Transaction(func(tx *gorm.DB) error {
				newMember := &models.CustomerAccountMember{
					AccountID: invite.AccountID,
					UserID:    user.ID,
					OrgID:     invite.OrgID,
					Role:      models.CustomerAccountRoleMember,
				}
				if err := tx.Create(newMember).Error; err != nil {
					return err
				}
				now := time.Now()
				invite.UsedByUserID = &user.ID
				invite.UsedAt = &now
				if err := tx.Save(&invite).Error; err != nil {
					return err
				}
				tx.Model(&models.Install{}).
					Where("user_id = ? AND org_id = ? AND customer_account_id IS NULL AND deleted_at IS NULL", user.ID, invite.OrgID).
					Updates(map[string]interface{}{
						"customer_account_id": invite.AccountID,
						"visibility":          models.VisibilityAccount,
					})
				return nil
			})
			if txErr != nil {
				zap.L().Error("failed to auto-join via invite", zap.Error(txErr))
			} else {
				// Reload memberships to include the newly joined account
				db.Preload("Account").Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", user.ID, org.ID).Find(&members)
			}
		}

		if len(members) > 0 {
			selected := SelectActiveMember(c, members)
			c.Set("customer_account", &selected.Account)
			c.Set("customer_account_member", selected)
			c.Set("customer_accounts", members)
			c.Next()
			return
		}

		// No membership and no invite found - only redirect customers (vendors pass through for preview)
		if user.Role == models.RoleCustomer {
			if isHTMXRequest(c) {
				c.Header("HX-Redirect", "/installs")
				c.AbortWithStatus(200)
				return
			}
			c.Redirect(302, "/installs")
			c.Abort()
			return
		}

		c.Next()
	}
}

// SelectActiveMember picks the active membership based on the active_account_id cookie.
// Falls back to the first membership if no cookie or no match.
func SelectActiveMember(c *gin.Context, members []models.CustomerAccountMember) *models.CustomerAccountMember {
	activeAccountID, _ := c.Cookie("active_account_id")
	if activeAccountID != "" {
		for i := range members {
			if members[i].AccountID == activeAccountID {
				return &members[i]
			}
		}
	}
	return &members[0]
}

// GetCustomerAccount retrieves the customer account from context.
func GetCustomerAccount(c *gin.Context) *models.CustomerAccount {
	if account, exists := c.Get("customer_account"); exists {
		if a, ok := account.(*models.CustomerAccount); ok {
			return a
		}
	}
	return nil
}

// GetCustomerAccountMember retrieves the customer account membership from context.
func GetCustomerAccountMember(c *gin.Context) *models.CustomerAccountMember {
	if member, exists := c.Get("customer_account_member"); exists {
		if m, ok := member.(*models.CustomerAccountMember); ok {
			return m
		}
	}
	return nil
}

// GetCustomerAccounts retrieves all customer account memberships from context.
func GetCustomerAccounts(c *gin.Context) []models.CustomerAccountMember {
	if members, exists := c.Get("customer_accounts"); exists {
		if m, ok := members.([]models.CustomerAccountMember); ok {
			return m
		}
	}
	return nil
}
