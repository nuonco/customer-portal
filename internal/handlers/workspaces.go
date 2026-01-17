package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// CreateWorkspace creates a new workspace with a connected Nuon organization.
// Each workspace requires exactly one Nuon org to be connected at creation time.
func (h *Handler) CreateWorkspace(c *gin.Context) {
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

	// Check if this Nuon org is already connected to another workspace
	var existingOrg models.NuonOrg
	if err := h.db.Where("org_id = ? AND deleted_at IS NULL", req.OrgID).First(&existingOrg).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "This Nuon organization is already connected to another workspace",
		})
		return
	}

	// Use org name for workspace name if not provided
	workspaceName := req.Name
	if workspaceName == "" {
		workspaceName = orgName
	}

	// Generate unique subdomain from org name
	subdomain, err := models.GenerateUniqueSubdomain(h.db, orgName, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate subdomain",
		})
		return
	}

	// Create workspace and org in a transaction
	var workspace models.Workspace
	var org models.NuonOrg

	err = h.db.Transaction(func(tx *gorm.DB) error {
		// Create workspace
		workspace = models.Workspace{
			Name:       workspaceName,
			Subdomain:  subdomain,
			IsPersonal: false,
		}
		if err := tx.Create(&workspace).Error; err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		// Create connected org
		org = models.NuonOrg{
			WorkspaceID: workspace.ID,
			UserID:      user.ID,
			NuonOrgID:   req.OrgID,
			APIToken:    req.APIToken,
			Name:        orgName,
		}
		if err := tx.Create(&org).Error; err != nil {
			return fmt.Errorf("failed to connect organization: %w", err)
		}

		// Add user as first member
		now := time.Now()
		member := models.WorkspaceMember{
			WorkspaceID: workspace.ID,
			UserID:      user.ID,
			Status:      models.MemberStatusActive,
			JoinedAt:    &now,
		}
		if err := tx.Create(&member).Error; err != nil {
			return fmt.Errorf("failed to add user to workspace: %w", err)
		}

		// Create default theme for this workspace
		theme := models.AppTheme{
			WorkspaceID:    workspace.ID,
			PrimaryColor:   models.DefaultPrimaryColor,
			SecondaryColor: models.DefaultPrimaryColor,
			BorderRadius:   models.DefaultBorderRadius,
			SpacingDensity: models.DefaultSpacingDensity,
		}
		if err := tx.Create(&theme).Error; err != nil {
			return fmt.Errorf("failed to create theme: %w", err)
		}

		// Create default auth config for this workspace
		authConfig := models.CustomerAuthConfig{
			WorkspaceID: workspace.ID,
			Enabled:     false,
			Scopes:      models.DefaultScopes,
		}
		if err := tx.Create(&authConfig).Error; err != nil {
			return fmt.Errorf("failed to create auth config: %w", err)
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create workspace: " + err.Error(),
		})
		return
	}

	// Set workspace cookie to the new workspace
	middleware.SetWorkspaceCookie(c, workspace.ID)

	// Return success with redirect URL to the org's install links page
	c.JSON(http.StatusOK, gin.H{
		"id":          workspace.ID,
		"name":        workspace.Name,
		"org_id":      org.ID,
		"redirect_to": fmt.Sprintf("/admin/orgs/%s/install-links", org.ID),
	})
}

// UpdateWorkspace updates workspace details
func (h *Handler) UpdateWorkspace(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Workspace context not found",
		})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Update workspace
	if err := h.db.Model(workspace).Updates(map[string]interface{}{
		"name": req.Name,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update workspace",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workspace updated successfully",
	})
}

// SwitchWorkspace changes the active workspace for the user
func (h *Handler) SwitchWorkspace(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		WorkspaceID string `json:"workspace_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate workspace_id format
	if !shortid.IsValid(req.WorkspaceID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid workspace ID format",
		})
		return
	}

	// Verify user is a member of the workspace
	var member models.WorkspaceMember
	err := h.db.Where("workspace_id = ? AND user_id = ? AND status = ?",
		req.WorkspaceID, user.ID, models.MemberStatusActive).First(&member).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied to workspace",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to verify workspace membership",
		})
		return
	}

	// Set workspace cookie
	middleware.SetWorkspaceCookie(c, req.WorkspaceID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Workspace switched successfully",
		"redirect_to": "/admin/orgs",
	})
}

// WorkspaceSelectorPage renders the workspace selection page (for users with multiple workspaces)
func (h *Handler) WorkspaceSelectorPage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Get user's workspaces
	var memberships []models.WorkspaceMember
	if err := h.db.Where("user_id = ? AND status = ?", user.ID, models.MemberStatusActive).
		Preload("Workspace").
		Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load workspaces",
		})
		return
	}

	// Extract workspaces
	workspaces := make([]models.Workspace, 0, len(memberships))
	for _, m := range memberships {
		workspaces = append(workspaces, m.Workspace)
	}

	// If user has no workspaces, create personal workspace
	if len(workspaces) == 0 {
		workspace := models.Workspace{
			Name:       user.Name + "'s Personal Workspace",
			IsPersonal: true,
		}
		if err := h.db.Create(&workspace).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create personal workspace",
			})
			return
		}

		// Add user as member
		now := time.Now()
		member := models.WorkspaceMember{
			WorkspaceID: workspace.ID,
			UserID:      user.ID,
			Status:      models.MemberStatusActive,
			JoinedAt:    &now,
		}
		if err := h.db.Create(&member).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to add user to workspace",
			})
			return
		}

		// Set workspace cookie and redirect
		middleware.SetWorkspaceCookie(c, workspace.ID)
		c.Redirect(http.StatusFound, "/admin/orgs")
		return
	}

	// If user has exactly one workspace, auto-select it
	if len(workspaces) == 1 {
		middleware.SetWorkspaceCookie(c, workspaces[0].ID)
		c.Redirect(http.StatusFound, "/admin/orgs")
		return
	}

	// Render workspace selector for multiple workspaces
	props := pages.WorkspaceSelectProps{
		Title:      "Select Workspace",
		User:       user,
		Workspaces: workspaces,
		BasePath:   h.basePath,
		CSSPath:    assets.VendorCSSPath(),
	}

	// Load theme if available (use first workspace's theme as fallback)
	if len(workspaces) > 0 {
		var theme models.AppTheme
		if err := h.db.Where("workspace_id = ?", workspaces[0].ID).First(&theme).Error; err == nil {
			primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
			secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)
			props.PrimaryColor = primaryColor
			props.PrimaryColorDark = primaryColorDark
			props.SecondaryColor = secondaryColor
			props.SecondaryColorDark = secondaryColorDark
			props.HeadingFont = theme.HeadingFont
			props.BodyFont = theme.BodyFont
			props.HeadingFontBase64 = theme.HeadingFontBase64
			props.BodyFontBase64 = theme.BodyFontBase64
			props.LogoBase64 = theme.LogoLightBase64
		}
	}

	h.RenderTempl(c, http.StatusOK, pages.WorkspaceSelectPage(props))
}

// WorkspaceSettingsPanel renders the workspace settings panel (HTMX)
func (h *Handler) WorkspaceSettingsPanel(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Workspace context not found",
		})
		return
	}

	// Load workspace members
	var members []models.WorkspaceMember
	if err := h.db.Where("workspace_id = ? AND status = ?", workspace.ID, models.MemberStatusActive).
		Preload("User").
		Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load workspace members",
		})
		return
	}

	// Load active invitations
	var invitations []models.WorkspaceInvitation
	if err := h.db.Where("workspace_id = ? AND (expires_at IS NULL OR expires_at > ?) AND (max_uses = 0 OR used_count < max_uses)",
		workspace.ID, time.Now()).
		Find(&invitations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load invitations",
		})
		return
	}

	// Get current user
	user := middleware.GetCurrentUser(c)

	// Render workspace settings panel template
	props := partials.WorkspacePanelProps{
		Workspace:       workspace,
		Members:         members,
		Invitations:     invitations,
		BasePath:        h.basePath,
		CurrentUser:     user,
		CustomerBaseURL: h.customerBaseURL,
	}

	h.RenderTempl(c, http.StatusOK, partials.WorkspacePanel(props))
}

// GenerateInvitation creates a new invitation link for the workspace
func (h *Handler) GenerateInvitation(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Workspace context not found",
		})
		return
	}

	user := middleware.GetCurrentUser(c)

	var req struct {
		Email     string     `json:"email"`
		ExpiresAt *time.Time `json:"expires_at"`
		MaxUses   int        `json:"max_uses"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Create invitation
	invitation := models.WorkspaceInvitation{
		WorkspaceID: workspace.ID,
		Email:       req.Email,
		InvitedBy:   user.ID,
		ExpiresAt:   req.ExpiresAt,
		MaxUses:     req.MaxUses,
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

// DeleteInvitation revokes an invitation link
func (h *Handler) DeleteInvitation(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Workspace context not found",
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

	// Delete invitation (verify it belongs to the workspace)
	result := h.db.Where("id = ? AND workspace_id = ?", invitationID, workspace.ID).
		Delete(&models.WorkspaceInvitation{})

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

// RemoveMember removes a user from the workspace
func (h *Handler) RemoveMember(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Workspace context not found",
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
			"error": "Cannot remove yourself from workspace",
		})
		return
	}

	// Delete membership (verify it belongs to the workspace)
	result := h.db.Where("workspace_id = ? AND user_id = ?", workspace.ID, userID).
		Delete(&models.WorkspaceMember{})

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

// AcceptInvitation is a public endpoint for accepting workspace invitations
func (h *Handler) AcceptInvitation(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invitation token is required",
		})
		return
	}

	// Find invitation
	var invitation models.WorkspaceInvitation
	if err := h.db.Where("token = ?", token).Preload("Workspace").First(&invitation).Error; err != nil {
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
	var existingMember models.WorkspaceMember
	err := h.db.Where("workspace_id = ? AND user_id = ?", invitation.WorkspaceID, user.ID).
		First(&existingMember).Error
	if err == nil {
		// Already a member - just switch to this workspace
		middleware.SetWorkspaceCookie(c, invitation.WorkspaceID)
		c.JSON(http.StatusOK, gin.H{
			"message":     "Already a member of this workspace",
			"redirect_to": "/admin/orgs",
		})
		return
	}

	// Add user as member
	now := time.Now()
	member := models.WorkspaceMember{
		WorkspaceID: invitation.WorkspaceID,
		UserID:      user.ID,
		InvitedBy:   &invitation.InvitedBy,
		Status:      models.MemberStatusActive,
		JoinedAt:    &now,
	}

	if err := h.db.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to add member to workspace",
		})
		return
	}

	// Increment invitation used count
	h.db.Model(&invitation).UpdateColumn("used_count", gorm.Expr("used_count + ?", 1))

	// Mark invitation as accepted
	acceptedAt := time.Now()
	h.db.Model(&invitation).Update("accepted_at", &acceptedAt)

	// Set workspace cookie
	middleware.SetWorkspaceCookie(c, invitation.WorkspaceID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Successfully joined workspace",
		"redirect_to": "/admin/orgs",
	})
}

// AcceptInvitationPage handles invitation acceptance via browser navigation
// This is a public endpoint that redirects to login if not authenticated
func (h *Handler) AcceptInvitationPage(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invitation token is required")
		return
	}

	// Find invitation
	var invitation models.WorkspaceInvitation
	if err := h.db.Where("token = ?", token).Preload("Workspace").First(&invitation).Error; err != nil {
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
		fmt.Printf("AcceptInvitationPage: user not authenticated, setting return_url=%s and redirecting to login\n", returnURL)
		c.SetCookie("return_url", returnURL, 3600, "/", "", false, true)
		c.Redirect(http.StatusFound, h.basePath+"/login/")
		return
	}

	// Check if user is already a member
	var existingMember models.WorkspaceMember
	err := h.db.Where("workspace_id = ? AND user_id = ?", invitation.WorkspaceID, user.ID).
		First(&existingMember).Error
	if err == nil {
		// Already a member - just switch to this workspace
		middleware.SetWorkspaceCookie(c, invitation.WorkspaceID)
		c.Redirect(http.StatusFound, h.basePath+"/orgs")
		return
	}

	// Add user as member
	now := time.Now()
	member := models.WorkspaceMember{
		WorkspaceID: invitation.WorkspaceID,
		UserID:      user.ID,
		InvitedBy:   &invitation.InvitedBy,
		Status:      models.MemberStatusActive,
		JoinedAt:    &now,
	}

	if err := h.db.Create(&member).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to join workspace")
		return
	}

	// Increment invitation used count
	h.db.Model(&invitation).UpdateColumn("used_count", gorm.Expr("used_count + ?", 1))

	// Mark invitation as accepted
	acceptedAt := time.Now()
	h.db.Model(&invitation).Update("accepted_at", &acceptedAt)

	// Set workspace cookie
	middleware.SetWorkspaceCookie(c, invitation.WorkspaceID)

	// Redirect to the workspace
	c.Redirect(http.StatusFound, h.basePath+"/orgs")
}

// tryGetCurrentUser attempts to get the current user from the JWT cookie without requiring auth middleware
func (h *Handler) tryGetCurrentUser(c *gin.Context) *models.User {
	// Get the JWT token from cookie
	tokenCookie, err := c.Cookie("jwt")
	if err != nil || tokenCookie == "" {
		fmt.Printf("tryGetCurrentUser: no JWT cookie found (err=%v, empty=%v)\n", err, tokenCookie == "")
		return nil
	}

	fmt.Printf("tryGetCurrentUser: found JWT cookie, attempting to parse (len=%d)\n", len(tokenCookie))

	// Use the JWT middleware to parse and validate the token
	token, err := h.auth.ParseTokenString(tokenCookie)
	if err != nil {
		fmt.Printf("tryGetCurrentUser: failed to parse JWT: %v\n", err)
		return nil
	}

	// Extract claims from the token
	claims, ok := token.Claims.(gojwt.MapClaims)
	if !ok || !token.Valid {
		fmt.Printf("tryGetCurrentUser: invalid token claims (ok=%v, valid=%v)\n", ok, token.Valid)
		return nil
	}

	// Extract user ID from claims
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		fmt.Printf("tryGetCurrentUser: no user_id in claims (ok=%v, userID=%q)\n", ok, userID)
		return nil
	}

	fmt.Printf("tryGetCurrentUser: found user_id=%s, loading from DB\n", userID)

	// Load user from database
	var user models.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		fmt.Printf("tryGetCurrentUser: failed to load user from DB: %v\n", err)
		return nil
	}

	fmt.Printf("tryGetCurrentUser: successfully loaded user %s (%s)\n", user.ID, user.Email)
	return &user
}
