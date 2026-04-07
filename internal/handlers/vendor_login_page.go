package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
)

func (h *Handler) VendorLoginPageTempl(c *gin.Context) {
	// Check for error message in query params (from failed login/register)
	errorMsg := c.Query("error")

	props := vendorpages.VendorLoginPageProps{
		Title:    "Customer Portal",
		BasePath: h.basePath,
		Error:    errorMsg,
		CSSPath:  assets.CustomerCSSPath(),
	}

	// Check if we have an auth provider configured
	if h.authProvider == nil {
		props.Error = "Authentication not configured"
		h.RenderTempl(c, http.StatusInternalServerError, vendorpages.VendorLoginPage(props))
		return
	}

	// Local auth is no longer supported - require OIDC
	if h.authProvider.Name() == "local" {
		props.Error = "OIDC authentication required. Please configure AUTH_* environment variables."
		h.RenderTempl(c, http.StatusInternalServerError, vendorpages.VendorLoginPage(props))
		return
	}

	// Capture redirect destination from query param (relative paths only, to prevent open redirect)
	redirectTo := c.Query("redirect")
	if !strings.HasPrefix(redirectTo, "/") {
		redirectTo = ""
	}

	// Generate state for CSRF protection, embedding the redirect URL so it survives the IdP round-trip
	state, err := auth.GenerateStateWithRedirect("", redirectTo)
	if err != nil {
		props.Error = "Failed to generate security token"
		h.RenderTempl(c, http.StatusInternalServerError, vendorpages.VendorLoginPage(props))
		return
	}

	// Store state in cookie for validation on callback
	c.SetCookie("auth_state", state, 600, "/", "", false, true)

	authURL, err := h.authProvider.GetAuthorizationURL(state)
	if err != nil {
		props.Error = "Failed to generate login URL"
		h.RenderTempl(c, http.StatusInternalServerError, vendorpages.VendorLoginPage(props))
		return
	}

	props.AuthURL = authURL
	h.RenderTempl(c, http.StatusOK, vendorpages.VendorLoginPage(props))
}

// AuthCallback handles the authentication callback from OIDC or SAML providers
