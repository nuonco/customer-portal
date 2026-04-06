package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
)

// getCustomerAccountsFromContext returns the active account and all other accounts the user
// can switch to. It checks middleware context first, then falls back to a DB lookup.
func (h *Handler) getCustomerAccountsFromContext(c *gin.Context) (activeAccount *models.CustomerAccount, otherAccounts []models.CustomerAccount) {
	user := h.tryGetLoggedInUser(c)
	if user == nil {
		return nil, nil
	}
	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		return nil, nil
	}

	// Load all memberships for this user+org
	var members []models.CustomerAccountMember
	if allMembers := middleware.GetCustomerAccounts(c); len(allMembers) > 0 {
		members = allMembers
	} else {
		h.db.Preload("Account").Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", user.ID, org.ID).Find(&members)
	}
	if len(members) == 0 {
		return nil, nil
	}

	// Determine which account is active (cookie or first)
	activeAccountID, _ := c.Cookie("active_account_id")
	var active *models.CustomerAccount
	for i := range members {
		if activeAccountID != "" && members[i].AccountID == activeAccountID {
			active = &members[i].Account
			break
		}
	}
	if active == nil {
		active = &members[0].Account
	}

	// Build "other" list
	for i := range members {
		if members[i].AccountID != active.ID {
			otherAccounts = append(otherAccounts, members[i].Account)
		}
	}
	return active, otherAccounts
}

// getCustomerAccountFromContext returns the customer account from middleware context,
// falling back to a DB lookup when the middleware didn't populate it (e.g. /account/* routes).
func (h *Handler) getCustomerAccountFromContext(c *gin.Context) *models.CustomerAccount {
	if account := middleware.GetCustomerAccount(c); account != nil {
		return account
	}
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return nil
	}
	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		return nil
	}

	// Check active_account_id cookie for multi-account support
	activeAccountID, _ := c.Cookie("active_account_id")
	if activeAccountID != "" {
		var member models.CustomerAccountMember
		if err := h.db.Preload("Account").Where("user_id = ? AND org_id = ? AND account_id = ? AND deleted_at IS NULL", user.ID, org.ID, activeAccountID).First(&member).Error; err == nil {
			return &member.Account
		}
	}

	// Fall back to first membership
	var member models.CustomerAccountMember
	if err := h.db.Preload("Account").Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", user.ID, org.ID).First(&member).Error; err != nil {
		return nil
	}
	return &member.Account
}

// AccountSetupPage redirects to /installs since accounts are now auto-created during auth.
func (h *Handler) AccountSetupPage(c *gin.Context) {
	c.Redirect(http.StatusFound, "/installs")
}

// createAccountForUser creates a CustomerAccount and makes the given user the owner.
// It associates any orphaned installs with the new account.
func (h *Handler) createAccountForUser(userID, orgID, accountName string) (*models.CustomerAccount, error) {
	tx := h.db.Begin()

	account := &models.CustomerAccount{
		OrgID:           orgID,
		Name:            accountName,
		CreatedByUserID: userID,
	}
	if err := tx.Create(account).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	member := &models.CustomerAccountMember{
		AccountID: account.ID,
		UserID:    userID,
		OrgID:     orgID,
		Role:      models.CustomerAccountRoleOwner,
	}
	if err := tx.Create(member).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Associate existing installs with the new account
	tx.Model(&models.Install{}).
		Where("user_id = ? AND org_id = ? AND customer_account_id IS NULL AND deleted_at IS NULL", userID, orgID).
		Updates(map[string]interface{}{
			"customer_account_id": account.ID,
			"visibility":          models.VisibilityAccount,
		})

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return account, nil
}

// CreateAccount creates a new CustomerAccount and makes the current user the owner.
func (h *Handler) CreateAccount(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		Name string `json:"name" form:"name" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		if isHTMXRequest(c) {
			c.String(http.StatusOK, `<div class="p-3 text-sm text-red-600 dark:text-red-400">Group name is required.</div>`)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group name is required"})
		return
	}

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		if isHTMXRequest(c) {
			c.String(http.StatusOK, `<div class="p-3 text-sm text-red-600 dark:text-red-400">Organization not found.</div>`)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	account, err := h.createAccountForUser(user.ID, org.ID, req.Name)
	if err != nil {
		if isHTMXRequest(c) {
			c.String(http.StatusOK, `<div class="p-3 text-sm text-red-600 dark:text-red-400">Failed to create group.</div>`)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}

	// Set the new account as active
	c.SetCookie("active_account_id", account.ID, 60*60*24*365, "/", "", false, true)

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/installs")
		c.Status(200)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Account created", "account": account})
}

// SwitchAccount sets the active account for the current user.
func (h *Handler) SwitchAccount(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		AccountID string `json:"account_id" form:"account_id" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account ID is required"})
		return
	}

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Validate user is a member of this account in the current org
	var member models.CustomerAccountMember
	if err := h.db.Where("user_id = ? AND org_id = ? AND account_id = ? AND deleted_at IS NULL", user.ID, org.ID, req.AccountID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this account"})
		return
	}

	c.SetCookie("active_account_id", req.AccountID, 60*60*24*365, "/", "", false, true)

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/installs")
		c.Status(200)
		return
	}
	c.Redirect(302, "/installs")
}

// AccountMembersRedirect redirects /account/members to /account.
func (h *Handler) AccountMembersRedirect(c *gin.Context) {
	c.Redirect(http.StatusFound, "/account")
}

// UpdateAccount allows the account owner to rename the account.
func (h *Handler) UpdateAccount(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	account := h.getCustomerAccountFromContext(c)
	if account == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No account found"})
		return
	}

	// Verify user is owner
	var member models.CustomerAccountMember
	if err := h.db.Where("user_id = ? AND account_id = ? AND deleted_at IS NULL", user.ID, account.ID).First(&member).Error; err != nil || member.Role != models.CustomerAccountRoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the account owner can update settings"})
		return
	}

	var req struct {
		Name string `json:"name" form:"name" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		if isHTMXRequest(c) {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<div class="p-3 text-sm text-red-600 dark:text-red-400">Group name is required.</div>`)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group name is required"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		if isHTMXRequest(c) {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<div class="p-3 text-sm text-red-600 dark:text-red-400">Group name is required.</div>`)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group name is required"})
		return
	}

	if err := h.db.Model(account).Update("name", name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update account"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<div class="p-3 text-sm text-green-600 dark:text-green-400">Group name updated.</div>`)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Account updated", "account": account})
}

// AccountPage renders the account settings page with members and invite functionality.
func (h *Handler) AccountPage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	account := h.getCustomerAccountFromContext(c)
	if account == nil {
		c.Redirect(302, "/installs")
		return
	}

	// Get members
	var members []models.CustomerAccountMember
	h.db.Preload("User").Where("account_id = ? AND deleted_at IS NULL", account.ID).Order("joined_at ASC").Find(&members)

	// Get pending invites
	var invites []models.CustomerAccountInvite
	h.db.Preload("CreatedBy").Where("account_id = ? AND used_by_user_id IS NULL AND deleted_at IS NULL", account.ID).Order("created_at DESC").Find(&invites)

	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)
	layoutProps := h.buildCustomerLayoutProps("My Group", user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.ActiveNav = "account"
	layoutProps.HasPublishedApps = h.orgHasPublishedApps(orgID)

	// Determine if current user is owner
	currentMember := middleware.GetCustomerAccountMember(c)
	if currentMember == nil && user != nil && account != nil {
		var m models.CustomerAccountMember
		if err := h.db.Where("user_id = ? AND account_id = ? AND deleted_at IS NULL", user.ID, account.ID).First(&m).Error; err == nil {
			currentMember = &m
		}
	}
	isOwner := currentMember != nil && currentMember.Role == models.CustomerAccountRoleOwner

	h.RenderTempl(c, http.StatusOK, customerpages.AccountPage(customerpages.AccountPageProps{
		LayoutProps: layoutProps,
		Account:     *account,
		Members:     members,
		Invites:     invites,
		IsOwner:     isOwner,
	}))
}

// CreateAccountInvite invites a user by email. If the email matches an existing user,
// they are added as a member immediately. Otherwise a pending invite is created.
func (h *Handler) CreateAccountInvite(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	account := h.getCustomerAccountFromContext(c)
	if account == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No account found"})
		return
	}

	var req struct {
		Email string `json:"email" form:"email"`
	}
	if err := c.ShouldBind(&req); err != nil || strings.TrimSpace(req.Email) == "" {
		if isHTMXRequest(c) {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<div class="p-3 text-sm text-red-600 dark:text-red-400">Please enter an email address.</div>`)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))

	// Check if already a member
	var existingMember models.CustomerAccountMember
	if err := h.db.Joins("JOIN users ON users.id = customer_account_members.user_id").
		Where("customer_account_members.account_id = ? AND users.email = ? AND customer_account_members.deleted_at IS NULL", account.ID, email).
		First(&existingMember).Error; err == nil {
		if isHTMXRequest(c) {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<div class="p-3 text-sm text-amber-600 dark:text-amber-400">That user is already a member of this group.</div>`)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member"})
		return
	}

	// Check if there's already a pending invite for this email
	var existingInvite models.CustomerAccountInvite
	if err := h.db.Where("account_id = ? AND email = ? AND used_by_user_id IS NULL AND deleted_at IS NULL", account.ID, email).
		First(&existingInvite).Error; err == nil {
		if isHTMXRequest(c) {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<div class="p-3 text-sm text-amber-600 dark:text-amber-400">An invite for that email is already pending.</div>`)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "Invite already pending for this email"})
		return
	}

	// Check if a user with this email already exists
	var existingUser models.User
	if err := h.db.Where("email = ? AND deleted_at IS NULL", email).First(&existingUser).Error; err == nil {
		// User exists — add them as a member immediately
		member := &models.CustomerAccountMember{
			AccountID: account.ID,
			UserID:    existingUser.ID,
			OrgID:     account.OrgID,
			Role:      models.CustomerAccountRoleMember,
		}
		if err := h.db.Create(member).Error; err != nil {
			if isHTMXRequest(c) {
				c.Header("Content-Type", "text/html")
				c.String(http.StatusOK, `<div class="p-3 text-sm text-red-600 dark:text-red-400">Failed to add member.</div>`)
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member"})
			return
		}

		if isHTMXRequest(c) {
			c.Header("HX-Redirect", "/account")
			c.Status(200)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"message": "Member added", "member": member})
		return
	}

	// User doesn't exist — create a pending invite
	invite := &models.CustomerAccountInvite{
		AccountID:       account.ID,
		OrgID:           account.OrgID,
		Email:           email,
		CreatedByUserID: user.ID,
	}
	if err := h.db.Create(invite).Error; err != nil {
		if isHTMXRequest(c) {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<div class="p-3 text-sm text-red-600 dark:text-red-400">Failed to create invite.</div>`)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invite"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/account")
		c.Status(200)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Invite sent", "invite": invite})
}

// DeleteAccountMember removes a member from the account. Owners cannot remove themselves.
func (h *Handler) DeleteAccountMember(c *gin.Context) {
	memberID := c.Param("member_id")
	user := middleware.GetCurrentUser(c)
	account := h.getCustomerAccountFromContext(c)
	if account == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No account found"})
		return
	}

	// Look up the member to remove
	var member models.CustomerAccountMember
	if err := h.db.Where("id = ? AND account_id = ? AND deleted_at IS NULL", memberID, account.ID).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	}

	// Prevent owner from removing themselves
	if member.UserID == user.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You cannot remove yourself"})
		return
	}

	if err := h.db.Delete(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	if isHTMXRequest(c) {
		c.Status(200)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Member removed"})
}

// TransferAccountOwnership transfers ownership from the current user to another member.
func (h *Handler) TransferAccountOwnership(c *gin.Context) {
	memberID := c.Param("member_id")
	user := middleware.GetCurrentUser(c)
	account := h.getCustomerAccountFromContext(c)
	if account == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No account found"})
		return
	}

	// Verify current user is owner
	var currentMember models.CustomerAccountMember
	if err := h.db.Where("user_id = ? AND account_id = ? AND deleted_at IS NULL", user.ID, account.ID).First(&currentMember).Error; err != nil || currentMember.Role != models.CustomerAccountRoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the account owner can transfer ownership"})
		return
	}

	// Look up target member
	var targetMember models.CustomerAccountMember
	if err := h.db.Where("id = ? AND account_id = ? AND deleted_at IS NULL", memberID, account.ID).First(&targetMember).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	}

	// Prevent transferring to yourself
	if targetMember.UserID == user.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You are already the owner"})
		return
	}

	// Swap roles in a transaction
	tx := h.db.Begin()
	if err := tx.Model(&targetMember).Update("role", models.CustomerAccountRoleOwner).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transfer ownership"})
		return
	}
	if err := tx.Model(&currentMember).Update("role", models.CustomerAccountRoleMember).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transfer ownership"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transfer ownership"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/account")
		c.Status(200)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Ownership transferred"})
}

// DeleteAccountInvite revokes an invite.
func (h *Handler) DeleteAccountInvite(c *gin.Context) {
	inviteID := c.Param("invite_id")
	account := h.getCustomerAccountFromContext(c)
	if account == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No account found"})
		return
	}

	result := h.db.Where("id = ? AND account_id = ?", inviteID, account.ID).Delete(&models.CustomerAccountInvite{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete invite"})
		return
	}

	if isHTMXRequest(c) {
		c.Status(200)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Invite revoked"})
}
