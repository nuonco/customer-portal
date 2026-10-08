package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
)

// requireVendorRole checks that the logged-in user is a vendor and aborts with 403 if not.
func (h *Handler) requireVendorRole(c *gin.Context) bool {
	user := h.tryGetLoggedInUser(c)
	if user == nil || user.Role != models.RoleVendor {
		c.String(http.StatusForbidden, "Forbidden")
		return false
	}
	return true
}

// DebugRedirect redirects /installs/:install_id/debug to the workflows tab.
func (h *Handler) DebugRedirect(c *gin.Context) {
	if !h.requireVendorRole(c) {
		return
	}
	installID := c.Param("install_id")
	c.Redirect(http.StatusFound, h.basePath+"/installs/"+installID+"/debug/workflows")
}
