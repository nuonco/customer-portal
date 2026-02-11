package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// CreateOrg creates a new organization with a connected Nuon platform org.
// Each org requires a Nuon API token and org ID to be connected at creation time.
func (h *Handler) CreateOrg(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		Name     string `json:"name"`                         // Optional - defaults to org name
		OrgID    string `json:"org_id" binding:"required"`    // Nuon platform org ID
		APIToken string `json:"api_token" binding:"required"` // Nuon API token
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate API token with Nuon API
	nuonClient, err := nuon.NewClientWithURL(req.APIToken, req.OrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to initialize Nuon client",
		})
		return
	}

	if err := nuonClient.ValidateOrgAccess(c.Request.Context()); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid API token or organization access",
		})
		return
	}

	// Fetch org name from Nuon API
	nuonOrg, err := nuonClient.GetOrg(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch organization details from Nuon API",
		})
		return
	}

	orgName := nuonOrg.Name
	if orgName == "" {
		orgName = req.OrgID // Fallback to org ID if name is empty
	}

	// Check if this Nuon org is already connected
	var existingOrg models.NuonOrg
	if err := h.db.Where("org_id = ? AND deleted_at IS NULL", req.OrgID).First(&existingOrg).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "This Nuon organization is already connected",
		})
		return
	}

	// Use org name for display if not provided
	displayName := req.Name
	if displayName == "" {
		displayName = orgName
	}

	// Generate unique subdomain from org name
	subdomain, err := models.GenerateUniqueSubdomain(h.db, orgName, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate subdomain",
		})
		return
	}

	// Create org and related entities in a transaction
	var org models.NuonOrg

	err = h.db.Transaction(func(tx *gorm.DB) error {
		// Create the org
		org = models.NuonOrg{
			UserID:    user.ID,
			NuonOrgID: req.OrgID,
			APIToken:  req.APIToken,
			Name:      displayName,
			Subdomain: subdomain,
		}
		if err := tx.Create(&org).Error; err != nil {
			return fmt.Errorf("failed to create organization: %w", err)
		}

		// Add user as first member
		now := time.Now()
		member := models.OrgMember{
			OrgID:    org.ID,
			UserID:   user.ID,
			Status:   models.MemberStatusActive,
			JoinedAt: &now,
		}
		if err := tx.Create(&member).Error; err != nil {
			return fmt.Errorf("failed to add user to organization: %w", err)
		}

		// Create default theme for this org
		theme := models.AppTheme{
			OrgID:          org.ID,
			PrimaryColor:   models.DefaultPrimaryColor,
			SecondaryColor: models.DefaultPrimaryColor,
			BorderRadius:   models.DefaultBorderRadius,
		}
		if err := tx.Create(&theme).Error; err != nil {
			return fmt.Errorf("failed to create theme: %w", err)
		}

		// Create default auth config for this org
		authConfig := models.CustomerAuthConfig{
			OrgID:   org.ID,
			Enabled: false,
			Scopes:  models.DefaultScopes,
		}
		if err := tx.Create(&authConfig).Error; err != nil {
			return fmt.Errorf("failed to create auth config: %w", err)
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create organization: " + err.Error(),
		})
		return
	}

	// Set org cookie to the new org
	middleware.SetOrgCookie(c, org.ID)

	// Return success with redirect URL to the org's install links page
	c.JSON(http.StatusOK, gin.H{
		"id":          org.ID,
		"name":        org.Name,
		"redirect_to": fmt.Sprintf("/admin/orgs/%s/install-links", org.ID),
	})
}

// UpdateOrg updates organization details (name and/or API token)
func (h *Handler) UpdateOrg(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Organization context not found",
		})
		return
	}

	var req struct {
		Name     string `json:"name"`
		APIToken string `json:"api_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.APIToken != "" {
		updates["api_token"] = req.APIToken
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No fields to update",
		})
		return
	}

	// Update org
	if err := h.db.Model(org).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update organization",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Organization updated successfully",
		"org":     *org,
	})
}

// SwitchOrg changes the active organization for the user
func (h *Handler) SwitchOrg(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		OrgID string `json:"org_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate org_id format
	if !shortid.IsValid(req.OrgID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid organization ID format",
		})
		return
	}

	// Verify user is a member of the org
	var member models.OrgMember
	err := h.db.Where("org_id = ? AND user_id = ? AND status = ?",
		req.OrgID, user.ID, models.MemberStatusActive).First(&member).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied to organization",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to verify organization membership",
		})
		return
	}

	// Set org cookie
	middleware.SetOrgCookie(c, req.OrgID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Organization switched successfully",
		"redirect_to": "/admin/orgs",
	})
}

// GenerateOrgInvitation creates a new invitation link for the organization
func (h *Handler) GenerateOrgInvitation(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Organization context not found",
		})
		return
	}

	user := middleware.GetCurrentUser(c)

	var req struct {
		Email     string     `json:"email"`
		ExpiresAt *time.Time `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate email is provided
	if req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Email is required",
		})
		return
	}

	// Set default expiration of 7 days if not provided
	expiresAt := req.ExpiresAt
	if expiresAt == nil {
		defaultExpiry := time.Now().Add(7 * 24 * time.Hour)
		expiresAt = &defaultExpiry
	}

	// Create invitation - always single-use (max_uses = 1)
	invitation := models.OrgInvitation{
		OrgID:     org.ID,
		Email:     req.Email,
		InvitedBy: user.ID,
		ExpiresAt: expiresAt,
		MaxUses:   1,
	}

	// BeforeCreate hook will generate the token
	if err := h.db.Create(&invitation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create invitation",
		})
		return
	}

	// Generate invitation URL
	invitationURL := invitation.GetInvitationURL(c.Request.Host)

	c.JSON(http.StatusOK, gin.H{
		"id":         invitation.ID,
		"token":      invitation.Token,
		"url":        invitationURL,
		"expires_at": invitation.ExpiresAt,
		"max_uses":   invitation.MaxUses,
	})
}

// DeleteOrgInvitation revokes an invitation link
func (h *Handler) DeleteOrgInvitation(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Organization context not found",
		})
		return
	}

	invitationID := c.Param("id")
	if !shortid.IsValid(invitationID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid invitation ID format",
		})
		return
	}

	// Delete invitation (verify it belongs to the org)
	result := h.db.Where("id = ? AND org_id = ?", invitationID, org.ID).
		Delete(&models.OrgInvitation{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete invitation",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Invitation not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invitation deleted successfully",
	})
}

// RemoveOrgMember removes a user from the organization
func (h *Handler) RemoveOrgMember(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Organization context not found",
		})
		return
	}

	userID := c.Param("user_id")
	if !shortid.IsValid(userID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	// Don't allow removing yourself
	currentUser := middleware.GetCurrentUser(c)
	if userID == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cannot remove yourself from organization",
		})
		return
	}

	// Delete membership (verify it belongs to the org)
	result := h.db.Where("org_id = ? AND user_id = ?", org.ID, userID).
		Delete(&models.OrgMember{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to remove member",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Member not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Member removed successfully",
	})
}

// AcceptOrgInvitation is a public endpoint for accepting organization invitations
func (h *Handler) AcceptOrgInvitation(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invitation token is required",
		})
		return
	}

	// Find invitation
	var invitation models.OrgInvitation
	if err := h.db.Where("token = ?", token).Preload("Org").First(&invitation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Invitation not found or expired",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load invitation",
		})
		return
	}

	// Validate invitation
	if !invitation.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invitation is invalid or expired",
		})
		return
	}

	// Get current user (must be authenticated)
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	// Check if user is already a member
	var existingMember models.OrgMember
	err := h.db.Where("org_id = ? AND user_id = ?", invitation.OrgID, user.ID).
		First(&existingMember).Error
	if err == nil {
		// Already a member - just switch to this org
		middleware.SetOrgCookie(c, invitation.OrgID)
		c.JSON(http.StatusOK, gin.H{
			"message":     "Already a member of this organization",
			"redirect_to": "/admin/orgs",
		})
		return
	}

	// Add user as member
	now := time.Now()
	member := models.OrgMember{
		OrgID:     invitation.OrgID,
		UserID:    user.ID,
		InvitedBy: &invitation.InvitedBy,
		Status:    models.MemberStatusActive,
		JoinedAt:  &now,
	}

	if err := h.db.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to add member to organization",
		})
		return
	}

	// Increment invitation used count
	h.db.Model(&invitation).UpdateColumn("used_count", gorm.Expr("used_count + ?", 1))

	// Mark invitation as accepted
	acceptedAt := time.Now()
	h.db.Model(&invitation).Update("accepted_at", &acceptedAt)

	// Set org cookie
	middleware.SetOrgCookie(c, invitation.OrgID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Successfully joined organization",
		"redirect_to": "/admin/orgs",
	})
}

// AcceptOrgInvitationPage handles invitation acceptance via browser navigation
// This is a public endpoint that redirects to login if not authenticated
func (h *Handler) AcceptOrgInvitationPage(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invitation token is required")
		return
	}

	// Find invitation
	var invitation models.OrgInvitation
	if err := h.db.Where("token = ?", token).Preload("Org").First(&invitation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			h.RenderErrorPage(c, http.StatusNotFound, "Invitation not found or expired")
			return
		}
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load invitation")
		return
	}

	// Validate invitation
	if !invitation.IsValid() {
		h.RenderErrorPage(c, http.StatusBadRequest, "This invitation has expired or reached its maximum uses")
		return
	}

	// Try to get current user from JWT cookie
	user := h.tryGetCurrentUser(c)
	if user == nil {
		// Not authenticated - store return URL in cookie and redirect to login
		// URL-encode the token in case it contains special characters
		returnURL := h.basePath + "/invite?token=" + url.QueryEscape(token)
		h.logger.Debug("AcceptOrgInvitationPage: user not authenticated, redirecting to login", zap.String("return_url", returnURL))
		c.SetCookie("return_url", returnURL, 3600, "/", "", false, true)
		c.Redirect(http.StatusFound, h.basePath+"/login/")
		return
	}

	// Check if user is already a member
	var existingMember models.OrgMember
	err := h.db.Where("org_id = ? AND user_id = ?", invitation.OrgID, user.ID).
		First(&existingMember).Error
	if err == nil {
		// Already a member - just switch to this org
		middleware.SetOrgCookie(c, invitation.OrgID)
		c.Redirect(http.StatusFound, h.basePath+"/orgs")
		return
	}

	// Add user as member
	now := time.Now()
	member := models.OrgMember{
		OrgID:     invitation.OrgID,
		UserID:    user.ID,
		InvitedBy: &invitation.InvitedBy,
		Status:    models.MemberStatusActive,
		JoinedAt:  &now,
	}

	if err := h.db.Create(&member).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to join organization")
		return
	}

	// Increment invitation used count
	h.db.Model(&invitation).UpdateColumn("used_count", gorm.Expr("used_count + ?", 1))

	// Mark invitation as accepted
	acceptedAt := time.Now()
	h.db.Model(&invitation).Update("accepted_at", &acceptedAt)

	// Set org cookie
	middleware.SetOrgCookie(c, invitation.OrgID)

	// Redirect to the org
	c.Redirect(http.StatusFound, h.basePath+"/orgs")
}

// tryGetCurrentUser attempts to get the current user from the JWT cookie without requiring auth middleware
func (h *Handler) tryGetCurrentUser(c *gin.Context) *models.User {
	// Get the JWT token from cookie
	tokenCookie, err := c.Cookie("jwt")
	if err != nil || tokenCookie == "" {
		h.logger.Debug("tryGetCurrentUser: no JWT cookie found")
		return nil
	}

	// Use the JWT middleware to parse and validate the token
	token, err := h.auth.ParseTokenString(tokenCookie)
	if err != nil {
		h.logger.Warn("tryGetCurrentUser: failed to parse JWT", zap.Error(err))
		return nil
	}

	// Extract claims from the token
	claims, ok := token.Claims.(gojwt.MapClaims)
	if !ok || !token.Valid {
		h.logger.Warn("tryGetCurrentUser: invalid token claims")
		return nil
	}

	// Extract user ID from claims
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		h.logger.Warn("tryGetCurrentUser: no user_id in claims")
		return nil
	}

	h.logger.Debug("tryGetCurrentUser: loading user from DB", zap.String("user_id", userID))

	// Load user from database
	var user models.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		h.logger.Warn("tryGetCurrentUser: failed to load user from DB", zap.Error(err))
		return nil
	}

	h.logger.Debug("tryGetCurrentUser: loaded user", zap.String("user_id", user.ID), zap.String("email", user.Email))
	return &user
}

// Deprecated aliases for backwards compatibility during migration
// These will be removed after all routes are updated

// CreateWorkspace is deprecated - use CreateOrg instead
func (h *Handler) CreateWorkspace(c *gin.Context) {
	h.CreateOrg(c)
}

// UpdateWorkspace is deprecated - use UpdateOrg instead
func (h *Handler) UpdateWorkspace(c *gin.Context) {
	h.UpdateOrg(c)
}

// SwitchWorkspace is deprecated - use SwitchOrg instead
func (h *Handler) SwitchWorkspace(c *gin.Context) {
	h.SwitchOrg(c)
}

// GenerateInvitation is deprecated - use GenerateOrgInvitation instead
func (h *Handler) GenerateInvitation(c *gin.Context) {
	h.GenerateOrgInvitation(c)
}

// DeleteInvitation is deprecated - use DeleteOrgInvitation instead
func (h *Handler) DeleteInvitation(c *gin.Context) {
	h.DeleteOrgInvitation(c)
}

// RemoveMember is deprecated - use RemoveOrgMember instead
func (h *Handler) RemoveMember(c *gin.Context) {
	h.RemoveOrgMember(c)
}

// AcceptInvitation is deprecated - use AcceptOrgInvitation instead
func (h *Handler) AcceptInvitation(c *gin.Context) {
	h.AcceptOrgInvitation(c)
}

// AcceptInvitationPage is deprecated - use AcceptOrgInvitationPage instead
func (h *Handler) AcceptInvitationPage(c *gin.Context) {
	h.AcceptOrgInvitationPage(c)
}
