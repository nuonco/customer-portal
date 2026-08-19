package jsonhandlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

type CustomerAuthHandler struct {
	customerBaseURL string
}

func NewCustomerAuthHandler(customerBaseURL string) *CustomerAuthHandler {
	return &CustomerAuthHandler{customerBaseURL: customerBaseURL}
}

// LoginURL returns the base-domain OIDC login URL for the current subdomain.
func (h *CustomerAuthHandler) LoginURL(c *gin.Context) {
	subdomain := strings.TrimSpace(c.Query("subdomain"))
	if subdomain == "" {
		subdomainValue, exists := c.Get("subdomain")
		if exists {
			if val, ok := subdomainValue.(string); ok {
				subdomain = strings.TrimSpace(val)
			}
		}
	}

	if subdomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Login requires an organization subdomain"})
		return
	}

	if err := models.ValidateSubdomain(subdomain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subdomain: " + err.Error()})
		return
	}

	redirect := c.DefaultQuery("redirect", "/")
	if !isSafeRelativeRedirect(redirect) {
		redirect = "/"
	}

	q := url.Values{}
	q.Set("return_to", subdomain)
	q.Set("redirect", redirect)

	authURL := fmt.Sprintf("%s/auth/login?%s", h.customerBaseURL, q.Encode())
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// Session returns the current authenticated customer session.
func (h *CustomerAuthHandler) Session(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
			"role":  user.Role,
		},
	})
}

// Logout clears customer auth cookies and returns JSON success.
func (h *CustomerAuthHandler) Logout(c *gin.Context) {
	c.SetCookie("jwt", "", -1, "/", "", false, true)
	c.SetCookie("auth_session", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func isSafeRelativeRedirect(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.Contains(path, "//")
}

func requestScheme(c *gin.Context) string {
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		return proto
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}
