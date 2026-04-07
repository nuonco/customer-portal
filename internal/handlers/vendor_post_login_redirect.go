package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"go.uber.org/zap"
)

func (h *Handler) HandlePostLoginRedirect(c *gin.Context, user *models.User, redirectURL string) {
	h.logger.Debug("HandlePostLoginRedirect",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
		zap.String("role", string(user.Role)),
	)

	// Use state-embedded redirect URL (relative paths only, to prevent open redirect)
	if strings.HasPrefix(redirectURL, "/") {
		h.logger.Debug("HandlePostLoginRedirect: found redirect URL in state, redirecting", zap.String("redirect_url", redirectURL))
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Check for return_url cookie (set by invitation flow)
	if returnURL, err := c.Cookie("return_url"); err == nil && returnURL != "" {
		h.logger.Debug("HandlePostLoginRedirect: found return_url cookie, redirecting", zap.String("return_url", returnURL))
		// Clear the cookie
		c.SetCookie("return_url", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, returnURL)
		return
	}
	h.logger.Debug("HandlePostLoginRedirect: no return_url cookie found")

	// Only apply org logic for vendor users
	if user.Role != models.RoleVendor {
		c.Redirect(http.StatusFound, h.basePath+"/orgs")
		return
	}

	// Get user's active orgs (filter out soft-deleted orgs)
	var memberships []models.OrgMember
	if err := h.db.Where("user_id = ? AND status = ?", user.ID, models.MemberStatusActive).
		Preload("Org", "deleted_at IS NULL").
		Find(&memberships).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to load organizations")
		return
	}

	// Extract orgs (only those with valid non-deleted orgs)
	orgs := make([]models.NuonOrg, 0, len(memberships))
	for _, m := range memberships {
		if m.Org.ID != "" {
			orgs = append(orgs, m.Org)
		}
	}

	// Case 1: No orgs - redirect to org creation page
	// (Personal orgs are no longer auto-created; users must connect a Nuon org)
	if len(orgs) == 0 {
		c.Redirect(http.StatusFound, h.basePath+"/orgs")
		return
	}

	// Case 2: One or more orgs - redirect to org list page
	// The /orgs page will redirect to the first org
	c.Redirect(http.StatusFound, h.basePath+"/orgs")
}

// RenderErrorPage renders a templ error page (for vendor pages)
