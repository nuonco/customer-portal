package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
)

// BaseDomainLogin initiates OIDC login from the base domain.
// This is the first step in the base domain authentication flow:
// 1. Customer visits subdomain.portal.nuon.co/login
// 2. Redirects to portal.nuon.co/auth/login?return_to=subdomain
// 3. This handler generates state, stores cookie, redirects to OIDC provider
//
// The state parameter embeds the subdomain so we know where to redirect after auth.
func (h *Handler) BaseDomainLogin(c *gin.Context) {
	// Get return subdomain from query parameter
	returnSubdomain := c.Query("return_to")
	if returnSubdomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing return_to parameter",
		})
		return
	}

	// Validate subdomain format
	if err := validateSubdomain(returnSubdomain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid subdomain: " + err.Error(),
		})
		return
	}

	// Look up workspace by subdomain to get workspace-specific OIDC config
	var workspace models.Workspace
	if err := h.db.Where("subdomain = ?", returnSubdomain).First(&workspace).Error; err != nil {
		c.Redirect(http.StatusFound, "/auth/error?message=Workspace+not+found")
		return
	}

	// Generate state with embedded subdomain info
	state, err := auth.GenerateStateWithSubdomain(returnSubdomain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate security token",
		})
		return
	}

	// Store state in cookie on BASE DOMAIN (with explicit domain parameter)
	// This is critical: the cookie must be accessible when the OIDC callback
	// returns to the base domain (portal.nuon.co/auth/callback)
	domain := extractBaseDomain(c.Request.Host)
	c.SetCookie(
		"auth_state", // name
		state,        // value
		600,          // maxAge (10 minutes)
		"/",          // path
		domain,       // domain (base domain for cookie access)
		true,         // secure (HTTPS only in production)
		true,         // httpOnly
	)

	// Get OIDC authorization URL using workspace-specific config
	authURL, err := h.customerAuthFactory.GetAuthURLForWorkspace(state, workspace.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate login URL",
		})
		return
	}

	// Redirect to OIDC provider
	c.Redirect(http.StatusFound, authURL)
}

// BaseDomainCallback handles OIDC callback on the base domain.
// This is the second step in the base domain authentication flow:
// 1. OIDC provider redirects to portal.nuon.co/auth/callback?code=XXX&state=YYY
// 2. This handler validates state, exchanges code for tokens
// 3. Generates JWT for the authenticated user
// 4. Redirects to subdomain.portal.nuon.co/auth/complete?token=JWT
func (h *Handler) BaseDomainCallback(c *gin.Context) {
	// Get authorization code from OIDC provider
	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusFound, "/auth/error?message=Missing+authorization+code")
		return
	}

	// Get state from query parameter and cookie
	state := c.Query("state")
	storedState, err := c.Cookie("auth_state")
	if err != nil || state != storedState {
		c.Redirect(http.StatusFound, "/auth/error?message=Invalid+state+parameter")
		return
	}

	// Clear state cookie immediately after validation
	domain := extractBaseDomain(c.Request.Host)
	c.SetCookie("auth_state", "", -1, "/", domain, true, true)

	// Extract subdomain from state parameter
	returnSubdomain, err := auth.ExtractSubdomainFromState(state)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/error?message=Invalid+state+format")
		return
	}

	// Exchange authorization code for tokens
	result, err := h.customerAuthFactory.HandleCallback(c.Request.Context(), auth.CallbackRequest{
		Code:  code,
		State: state,
	})
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/error?message=Authentication+failed")
		return
	}

	// Generate JWT token for the user
	// The JWT middleware's TokenGenerator creates a token with user claims
	tokenString, _, err := h.auth.TokenGenerator(result.User)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/error?message=Failed+to+generate+token")
		return
	}

	// Redirect to subdomain with JWT in query parameter (single-use)
	// The subdomain completion handler will set the JWT cookie on the subdomain
	protocol := getRequestProtocol(c)
	subdomainURL := fmt.Sprintf("%s://%s.%s/auth/complete?token=%s",
		protocol, returnSubdomain, h.subdomainBaseDomain, tokenString)

	c.Redirect(http.StatusFound, subdomainURL)
}

// CompleteSubdomainAuth completes authentication on the subdomain.
// This is the final step in the base domain authentication flow:
// 1. User is redirected from base domain to subdomain.portal.nuon.co/auth/complete?token=JWT
// 2. This handler validates the JWT token
// 3. Sets JWT cookie on the subdomain (scoped to that subdomain only)
// 4. Redirects to /installs
func (h *Handler) CompleteSubdomainAuth(c *gin.Context) {
	// Get token from query parameter (single-use)
	token := c.Query("token")
	if token == "" {
		c.Redirect(http.StatusFound, "/login?error=Missing+authentication+token")
		return
	}

	// Validate JWT token by attempting to parse it
	// This ensures the token is valid and not tampered with
	parsedToken, err := h.auth.ParseTokenString(token)
	if err != nil || !parsedToken.Valid {
		c.Redirect(http.StatusFound, "/login?error=Invalid+authentication+token")
		return
	}

	// Verify we're on a subdomain (not base domain)
	subdomain, exists := c.Get("subdomain")
	if !exists || subdomain.(string) == "" {
		c.Redirect(http.StatusFound, "/login?error=Must+access+from+subdomain")
		return
	}

	// Note: We could add additional validation here to ensure the user
	// has access to this specific subdomain/workspace, but for now we trust
	// that the JWT was just generated by our callback handler.

	// Set JWT cookie on subdomain (empty domain = current host only)
	// This cookie will ONLY be accessible from this specific subdomain
	c.SetCookie(
		"jwt", // name
		token, // value
		86400, // maxAge (24 hours)
		"/",   // path
		"",    // domain (empty = current host only)
		true,  // secure (HTTPS only in production)
		true,  // httpOnly
	)

	// Redirect to installs page
	c.Redirect(http.StatusFound, "/installs")
}

// AuthErrorPage displays an error page for authentication failures.
// This provides user-friendly error messages when authentication fails.
func (h *Handler) AuthErrorPage(c *gin.Context) {
	message := c.Query("message")
	if message == "" {
		message = "An authentication error occurred. Please try again."
	}

	// Get theme for error page (no user context since auth failed)
	workspaceID := h.getWorkspaceIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, workspaceID)

	props := pages.AuthErrorPageProps{
		LayoutProps: h.buildCustomerLayoutProps("Authentication Error", nil, theme),
		Message:     message,
	}

	h.RenderTempl(c, http.StatusBadRequest, pages.AuthErrorPage(props))
}

// Helper functions

// getRequestProtocol determines the protocol (http or https) for the request.
// Checks X-Forwarded-Proto header first (for proxies/load balancers),
// then falls back to TLS detection.
func getRequestProtocol(c *gin.Context) string {
	// Check X-Forwarded-Proto header (set by reverse proxies/load balancers)
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return proto
	}

	// Check if TLS is enabled on the request
	if c.Request.TLS != nil {
		return "https"
	}

	// Default to http (local development)
	return "http"
}

// extractBaseDomain extracts the base domain from a host string, removing the port.
// Examples:
//   - "localhost:8080" -> "localhost"
//   - "portal.nuon.co" -> "portal.nuon.co"
func extractBaseDomain(host string) string {
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

// validateSubdomain validates that a subdomain string is properly formatted.
// Returns an error if the subdomain is invalid.
//
// Rules:
// - Length: 2-63 characters
// - Must start with a letter
// - Can contain lowercase letters, numbers, and hyphens
// - Must end with a letter or number (not hyphen)
func validateSubdomain(subdomain string) error {
	if len(subdomain) < 2 || len(subdomain) > 63 {
		return fmt.Errorf("subdomain must be 2-63 characters")
	}

	// DNS label rules: start with letter, contain only lowercase letters/numbers/hyphens, end with letter/number
	matched, err := regexp.MatchString(`^[a-z][a-z0-9-]*[a-z0-9]$`, subdomain)
	if err != nil {
		return fmt.Errorf("failed to validate subdomain: %w", err)
	}
	if !matched {
		return fmt.Errorf("subdomain must start with letter, contain only lowercase letters, numbers, and hyphens")
	}

	return nil
}
