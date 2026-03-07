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

	// Redirect back to the current page if provided, otherwise to login
	redirect := c.Query("redirect")
	if redirect == "" || redirect[0] != '/' {
		redirect = "/login"
	}
	c.Redirect(http.StatusFound, redirect)
}

// LocalLogin handles POST /admin/login/ for email/password authentication
