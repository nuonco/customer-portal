package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
)

func (h *Handler) LocalRegister(c *gin.Context) {
	localProvider, ok := h.authProvider.(*auth.LocalProvider)
	if !ok {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Local authentication not configured")
		return
	}

	name := c.PostForm("name")
	email := c.PostForm("email")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")

	// Validate password confirmation
	if password != confirmPassword {
		c.Redirect(http.StatusFound, h.basePath+"/register?error=Passwords do not match")
		return
	}

	authResult, err := localProvider.Register(email, password, name)
	if err != nil {
		// Redirect back to register with error
		c.Redirect(http.StatusFound, h.basePath+"/register?error="+err.Error())
		return
	}

	// Generate JWT token
	token, _, err := h.auth.TokenGenerator(authResult.User)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to generate session token")
		return
	}

	// Set the JWT cookie
	maxAge := int(h.auth.Timeout.Seconds())
	c.SetCookie("jwt", token, maxAge, "/", "", false, true)

	// Store session ID for logout
	if authResult.SessionID != "" {
		c.SetCookie("auth_session", authResult.SessionID, maxAge, "/", "", false, true)
	}

	// Redirect to the orgs page
	c.Redirect(http.StatusFound, h.basePath+"/orgs")
}

// OrgsPage renders the organizations list page for a org.
// Each workspace has exactly one connected org (one-to-one relationship).
// New users may have a workspace but no org yet - they need to see the empty state
// with the "Connect Your First Org" button rather than an error page.
