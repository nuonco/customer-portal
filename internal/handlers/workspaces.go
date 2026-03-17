package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
		APIURL   string `json:"api_url"`                      // Optional per-org API URL override
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate API token with Nuon API
	apiURL := h.nuonAPIURL
	if req.APIURL != "" {
		apiURL = req.APIURL
	}
	nuonClient, err := nuon.NewClientWithURL(req.APIToken, req.OrgID, apiURL)
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
			APIURL:    req.APIURL,
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
			OrgID:        org.ID,
			PrimaryColor: models.DefaultPrimaryColor,
			BorderRadius: models.DefaultBorderRadius,
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

// GenerateOrgInvitation invites a user to the organization by email.
// If the user already exists, they are added as a member immediately.
// If not, a pending invitation is created and they auto-join on next login.
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
		Email string `json:"email"`
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

	// Check if already a member
	var existingMember models.OrgMember
	if err := h.db.Where("org_id = ? AND user_id IN (SELECT id FROM users WHERE email = ?) AND deleted_at IS NULL",
		org.ID, req.Email).First(&existingMember).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "User is already a member of this organization",
		})
		return
	}

	// Check if there's already a pending invite for this email
	var existingInvite models.OrgInvitation
	if err := h.db.Where("org_id = ? AND email = ? AND accepted_at IS NULL AND deleted_at IS NULL",
		org.ID, req.Email).First(&existingInvite).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "An invitation for this email is already pending",
		})
		return
	}

	// Check if the user already exists
	var existingUser models.User
	userExists := h.db.Where("email = ?", req.Email).First(&existingUser).Error == nil

	now := time.Now()

	// Create the invitation record
	invitation := models.OrgInvitation{
		OrgID:     org.ID,
		Email:     req.Email,
		InvitedBy: user.ID,
		MaxUses:   1,
	}

	if userExists {
		// User exists — add as member immediately and mark invite as accepted
		err := h.db.Transaction(func(tx *gorm.DB) error {
			invitation.AcceptedAt = &now
			invitation.UsedCount = 1
			if err := tx.Create(&invitation).Error; err != nil {
				return err
			}

			// Check for soft-deleted member and restore instead of creating duplicate
			var existingMember models.OrgMember
			if err := tx.Unscoped().Where("org_id = ? AND user_id = ? AND deleted_at IS NOT NULL", org.ID, existingUser.ID).First(&existingMember).Error; err == nil {
				// Restore soft-deleted member
				if err := tx.Unscoped().Model(&existingMember).Updates(map[string]interface{}{
					"deleted_at": nil,
					"status":     models.MemberStatusActive,
					"invited_by": &user.ID,
					"joined_at":  &now,
				}).Error; err != nil {
					return err
				}
			} else {
				member := models.OrgMember{
					OrgID:     org.ID,
					UserID:    existingUser.ID,
					InvitedBy: &user.ID,
					Status:    models.MemberStatusActive,
					JoinedAt:  &now,
				}
				if err := tx.Create(&member).Error; err != nil {
					return err
				}
			}

			// Upgrade customer to vendor role when invited to a vendor org
			if existingUser.Role == models.RoleCustomer {
				if err := tx.Model(&existingUser).Update("role", models.RoleVendor).Error; err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to add member",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":      invitation.ID,
			"status":  "joined",
			"message": "Member added to organization",
		})
		return
	}

	// User doesn't exist — create pending invitation
	if err := h.db.Create(&invitation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create invitation",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      invitation.ID,
		"status":  "pending",
		"message": "Invitation sent",
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
