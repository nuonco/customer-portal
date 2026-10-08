package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
)

// DeleteOrg archives (soft-deletes) a connected organization.
// The API token is cleared before soft-deleting so stale credentials don't persist.
func (h *Handler) DeleteOrg(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Clear the API token before soft-deleting
	if err := h.db.Model(org).Update("api_token", "").Error; err != nil {
		h.logger.Error("failed to clear API token: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to archive organization"})
		return
	}

	// Soft-delete the org record
	if err := h.db.Delete(org).Error; err != nil {
		h.logger.Error("failed to archive org: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to archive organization"})
		return
	}

	// HTMX or JSON response
	if c.GetHeader("HX-Request") != "" {
		c.Header("HX-Redirect", h.basePath+"/orgs")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Organization archived", "redirect": h.basePath + "/orgs"})
}

// RestoreOrg restores a previously archived organization.
// Requires a new API token since the old one was cleared on archive.
func (h *Handler) RestoreOrg(c *gin.Context) {
	orgID := c.Param("org_id")
	user := middleware.GetCurrentUser(c)

	var input struct {
		APIToken string `json:"api_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API token is required"})
		return
	}

	// Find the soft-deleted org belonging to this user
	var org models.NuonOrg
	result := h.db.Unscoped().
		Where("id = ? AND deleted_at IS NOT NULL", orgID).
		First(&org)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Archived organization not found"})
		return
	}

	// Verify the user is the creator or a member of this org
	var isMember bool
	if org.UserID == user.ID {
		isMember = true
	} else {
		var count int64
		h.db.Unscoped().Model(&models.OrgMember{}).
			Where("org_id = ? AND user_id = ?", orgID, user.ID).
			Count(&count)
		isMember = count > 0
	}
	if !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this organization"})
		return
	}

	// Restore: clear deleted_at and set new API token
	if err := h.db.Unscoped().Model(&org).Updates(map[string]interface{}{
		"deleted_at": nil,
		"api_token":  input.APIToken,
	}).Error; err != nil {
		h.logger.Error("failed to restore org: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore organization"})
		return
	}

	redirect := h.basePath + "/orgs/" + orgID + "/connection"
	if c.GetHeader("HX-Request") != "" {
		c.Header("HX-Redirect", redirect)
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Organization restored", "redirect": redirect})
}
