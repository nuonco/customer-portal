package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/auth"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
)

func (h *Handler) CustomerLoginPageTempl(c *gin.Context) {
	// Check for error message in query params
	errorMsg := c.Query("error")

	// Get vendor theme for customer-facing pages
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)

	// Look up org name for default login title/subtitle
	var orgName string
	if orgID != "" {
		var org models.NuonOrg
		if err := h.db.Where("id = ?", orgID).First(&org).Error; err == nil {
			orgName = org.Name
		}
	}

	// Check if we're on a subdomain
	subdomain, _ := c.Get("subdomain")
	var authURL string

	// Get redirect URL from query params (e.g., from install link page when user is not logged in)
	redirectURL := c.Query("redirect")

	if subdomain != nil && subdomain.(string) != "" {
		// On subdomain: buttons should link to base domain auth endpoint
		// This initiates the base domain auth flow to avoid cookie scoping issues
		authURL = fmt.Sprintf("%s/auth/login?return_to=%s",
			h.customerBaseURL, subdomain.(string))

		// Preserve redirect URL through the auth flow
		if redirectURL != "" {
			authURL = fmt.Sprintf("%s&redirect=%s", authURL, url.QueryEscape(redirectURL))
		}
	} else {
		// On base domain: show error or fallback behavior
		if errorMsg == "" {
			errorMsg = "Please access login from your workspace subdomain"
		}

		// Generate fallback OIDC URL for base domain (legacy behavior)
		state, err := auth.GenerateState()
		if err != nil {
			errorMsg = "Failed to generate security token"
		} else {
			// Store state in cookie for validation on callback
			c.SetCookie("auth_state", state, 600, "/", "", false, true)

			fallbackAuthURL, err := h.customerAuthFactory.GetAuthURL(state)
			if err != nil {
				errorMsg = "Failed to generate login URL"
			} else {
				authURL = fallbackAuthURL
			}
		}
	}

	props := customerpages.CustomerLoginPageProps{
		BasePath:      h.basePath,
		Error:         errorMsg,
		AuthURL:       authURL,
		Theme:         theme,
		CSSPath:       assets.CustomerCSSPath(),
		CustomCSSPath: h.getCustomCSSPath(orgID),
		OrgName:       orgName,
	}

	// Try template override first
	pageData := overrides.LoginPageData{
		AuthURL: authURL,
		Error:   errorMsg,
	}
	ctx := h.buildTemplateContext(orgID, theme.GetLoginTitle(), nil, theme, pageData)
	if h.tryRenderOverride(c, orgID, "login", ctx) {
		return
	}

	// Fall back to default Templ template
	h.RenderTempl(c, http.StatusOK, customerpages.CustomerLoginPage(props))
}

// CustomerLocalLogin is disabled - local email/password login is not supported for customers.
// Customers must use OIDC authentication.

func (h *Handler) CustomerLocalLogin(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{"message": "Local login is not supported. Please use SSO."})
}

// CustomerRegisterPage redirects to login - registration is not available for customers.
// Customer accounts are created via OIDC authentication.

func (h *Handler) CustomerRegisterPage(c *gin.Context) {
	redirect := c.Query("redirect")
	if redirect != "" {
		c.Redirect(http.StatusFound, "/login?redirect="+redirect)
	} else {
		c.Redirect(http.StatusFound, "/login")
	}
}

// CustomerRegister is disabled - registration is not available for customers.
// Customer accounts are created via OIDC authentication.

func (h *Handler) CustomerRegister(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"message": "Registration is not available. Please use SSO."})
}

// VendorLoginPageTempl renders the vendor login page
