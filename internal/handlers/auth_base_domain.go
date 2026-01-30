package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
// 2. Redirects to portal.nuon.co/auth/login?return_to=subdomain&redirect=/some/path
// 3. This handler generates state, stores cookie, redirects to OIDC provider
//
// The state parameter embeds the subdomain, redirect URL, and attribution data
// so we know where to redirect after auth completes and can track marketing source.
func (h *Handler) BaseDomainLogin(c *gin.Context) {
	// Get return subdomain from query parameter
	returnSubdomain := c.Query("return_to")
	if returnSubdomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing return_to parameter",
		})
		return
	}

	// Get redirect URL from query parameter (optional, e.g., /install-link?sha=XYZ)
	redirectURL := c.Query("redirect")

	// Validate subdomain format
	if err := validateSubdomain(returnSubdomain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid subdomain: " + err.Error(),
		})
		return
	}

	// Look up org by subdomain to get org-specific OIDC config
	var org models.NuonOrg
	if err := h.db.Where("subdomain = ?", returnSubdomain).First(&org).Error; err != nil {
		c.Redirect(http.StatusFound, "/auth/error?message=Organization+not+found")
		return
	}

	// Extract attribution data from query parameters (passed from marketing site)
	attribution := auth.Attribution{
		UTMSource:   c.Query("utm_source"),
		UTMMedium:   c.Query("utm_medium"),
		UTMCampaign: c.Query("utm_campaign"),
		UTMTerm:     c.Query("utm_term"),
		UTMContent:  c.Query("utm_content"),
		GCLID:       c.Query("gclid"),
		Referrer:    c.Query("referrer"),
		LandingPage: c.Query("landing_page"),
	}

	// Generate state with embedded subdomain, redirect URL, and attribution
	state, err := auth.GenerateStateWithAttribution(returnSubdomain, redirectURL, attribution)
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

	// Get OIDC authorization URL using org-specific config
	authURL, err := h.customerAuthFactory.GetAuthURLForOrg(state, org.ID)
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

	// Extract redirect URL from state (if present)
	redirectURL, _ := auth.ExtractRedirectFromState(state)

	// Extract attribution from state and set as cookie for ctl-api to read
	// The cookie is set on the base domain so it's accessible during API calls
	attribution, _ := auth.ExtractAttributionFromState(state)
	if !attribution.IsEmpty() {
		attrJSON, err := json.Marshal(attribution.ToMap())
		if err == nil {
			// Set attribution cookie on base domain (accessible to ctl-api)
			// Use a short expiry since it's only needed during account creation
			c.SetCookie(
				"nuon_attribution",        // name
				string(attrJSON),          // value (JSON-encoded attribution)
				3600,                      // maxAge (1 hour - enough time to complete signup)
				"/",                       // path
				"."+h.subdomainBaseDomain, // domain (base domain with leading dot for subdomain access)
				true,                      // secure
				false,                     // httpOnly (false so JS can read it if needed, but ctl-api reads from header)
			)
		}
	}

	// Redirect to subdomain with JWT in query parameter (single-use)
	// The subdomain completion handler will set the JWT cookie on the subdomain
	protocol := getRequestProtocol(c)
	subdomainURL := fmt.Sprintf("%s://%s.%s/auth/complete?token=%s",
		protocol, returnSubdomain, h.subdomainBaseDomain, tokenString)

	// Pass redirect URL to subdomain completion handler if present
	if redirectURL != "" {
		subdomainURL = fmt.Sprintf("%s&redirect=%s", subdomainURL, url.QueryEscape(redirectURL))
	}

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

	// Get redirect URL from query params, default to /installs
	redirectURL := c.Query("redirect")
	if redirectURL == "" {
		redirectURL = "/installs"
	}

	// Security: Validate redirect URL is a relative path to prevent open redirect attacks
	// Only allow paths that start with "/" and don't contain "//" (which could be a protocol-relative URL)
	if !strings.HasPrefix(redirectURL, "/") || strings.Contains(redirectURL, "//") {
		redirectURL = "/installs"
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// AuthErrorPage displays an error page for authentication failures.
// This provides user-friendly error messages when authentication fails.
func (h *Handler) AuthErrorPage(c *gin.Context) {
	message := c.Query("message")
	if message == "" {
		message = "An authentication error occurred. Please try again."
	}

	// Get theme for error page (no user context since auth failed)
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)

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
