package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CustomerLogout(c *gin.Context) {
	// Clear the JWT cookie server-side (same params as when it was set)
	c.SetCookie("jwt", "", -1, "/", "", false, true)

	// Clear the session cookie if present
	c.SetCookie("auth_session", "", -1, "/", "", false, true)

	// Redirect to customer login
	c.Redirect(http.StatusFound, "/login")
}

// LocalLogin handles POST /admin/login/ for email/password authentication
