package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

func (h *Handler) ProfilePanelContent(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Load full user from DB to get current name
	var dbUser models.User
	if err := h.db.First(&dbUser, "id = ?", user.ID).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to load user profile")
		return
	}

	// Query archived orgs for this user (soft-deleted, via creator or membership)
	var archivedOrgs []models.NuonOrg
	h.db.Unscoped().
		Where("user_id = ? AND deleted_at IS NOT NULL", user.ID).
		Order("deleted_at DESC").
		Find(&archivedOrgs)

	// Also include orgs where user is a member but not the creator
	var memberOrgIDs []string
	h.db.Model(&models.OrgMember{}).
		Where("user_id = ?", user.ID).
		Pluck("org_id", &memberOrgIDs)
	if len(memberOrgIDs) > 0 {
		var memberArchivedOrgs []models.NuonOrg
		h.db.Unscoped().
			Where("id IN ? AND user_id != ? AND deleted_at IS NOT NULL", memberOrgIDs, user.ID).
			Order("deleted_at DESC").
			Find(&memberArchivedOrgs)
		archivedOrgs = append(archivedOrgs, memberArchivedOrgs...)
	}

	props := partials.ProfilePanelProps{
		User:         &dbUser,
		BasePath:     h.basePath,
		ArchivedOrgs: archivedOrgs,
	}

	h.RenderTempl(c, http.StatusOK, partials.ProfilePanel(props))
}

// UpdateProfile handles PUT request to update user profile (name)
