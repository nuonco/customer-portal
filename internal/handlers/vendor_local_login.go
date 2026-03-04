package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
)

func (h *Handler) LocalLogin(c *gin.Context) {
	localProvider, ok := h.authProvider.(*auth.LocalProvider)
	if !ok {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Local authentication not configured")
		return
	}

	email := c.PostForm("email")
	password := c.PostForm("password")

	authResult, err := localProvider.Login(email, password)
	if err != nil {
		// Redirect back to login with error
		c.Redirect(http.StatusFound, h.basePath+"/login/?error="+err.Error())
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

	// Handle post-login workspace selection
	h.HandlePostLoginRedirect(c, authResult.User)
}

// VendorRegisterPageTempl renders the vendor registration page using Templ
