package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) VendorLogout(c *gin.Context) {
	// Clear the JWT cookie
	c.SetCookie("jwt", "", -1, "/", "", false, true)

	// Get session ID from cookie
	sessionID, _ := c.Cookie("auth_session")

	// Clear the session cookie
	c.SetCookie("auth_session", "", -1, "/", "", false, true)

	// Clear vendor OIDC state cookie
	c.SetCookie("auth_state", "", -1, "/", "", false, true)

	// If provider supports logout, redirect to IdP logout endpoint
	if h.authProvider != nil && h.authProvider.SupportsLogout() {
		logoutURL, err := h.authProvider.GetLogoutURL(sessionID)
		if err == nil && logoutURL != "" {
			c.Redirect(http.StatusFound, logoutURL)
			return
		}
	}

	// Fallback: redirect to login page
	c.Redirect(http.StatusFound, h.basePath+"/login/")
}

// CustomerLogout handles logout for customer users
