package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/auth"
)

func (h *Handler) AuthCallback(c *gin.Context) {
	if h.authProvider == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Authentication not configured")
		return
	}

	// Validate state parameter (CSRF protection)
	expectedState, _ := c.Cookie("auth_state")
	receivedState := c.Query("state")
	if receivedState == "" {
		// SAML uses RelayState
		receivedState = c.PostForm("RelayState")
	}

	if expectedState != "" && !auth.ValidateState(expectedState, receivedState) {
		h.RenderErrorPage(c, http.StatusBadRequest, "Invalid state parameter")
		return
	}

	// Clear state cookie
	c.SetCookie("auth_state", "", -1, "/", "", false, true)

	// Build callback request (supports both OIDC and SAML)
	req := auth.CallbackRequest{
		Code:         c.Query("code"),
		State:        receivedState,
		SAMLResponse: c.PostForm("SAMLResponse"),
		RelayState:   c.PostForm("RelayState"),
	}

	// Validate we have either code (OIDC) or SAMLResponse (SAML)
	if req.Code == "" && req.SAMLResponse == "" {
		h.RenderErrorPage(c, http.StatusBadRequest, "Missing authentication response")
		return
	}

	// Handle the callback
	authResult, err := h.authProvider.HandleCallback(c.Request.Context(), req)
	if err != nil {
		h.RenderErrorPage(c, http.StatusUnauthorized, fmt.Sprintf("Authentication failed: %v", err))
		return
	}

	// Generate JWT token using the existing auth middleware
	token, _, err := h.auth.TokenGenerator(authResult.User)
	if err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Failed to generate session token")
		return
	}

	// Set the JWT cookie
	maxAge := int(h.auth.Timeout.Seconds())
	c.SetCookie("jwt", token, maxAge, "/", "", false, true)

	// Store session ID for logout (if available)
	if authResult.SessionID != "" {
		c.SetCookie("auth_session", authResult.SessionID, maxAge, "/", "", false, true)
	}

	// Extract redirect URL from state (embedded by login page handler, may be empty)
	redirectURL, _ := auth.ExtractRedirectFromState(receivedState)

	// Handle post-login workspace selection
	h.HandlePostLoginRedirect(c, authResult.User, redirectURL)
}

// VendorLogout handles logout for vendor users
