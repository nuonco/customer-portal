package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SubdomainContext extracts subdomain from the request host and adds it to the context.
// This middleware should be added early in the middleware chain to make subdomain
// information available to all downstream handlers.
//
// The extracted subdomain and a boolean flag indicating if the request is to the
// base domain are stored in the Gin context with keys "subdomain" and "is_base_domain".
func SubdomainContext(baseDomain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host

		// Extract subdomain from host
		subdomain := extractSubdomain(host, baseDomain)

		// Store in context for handlers to use
		c.Set("subdomain", subdomain)
		c.Set("is_base_domain", subdomain == "")

		c.Next()
	}
}

// AllowBaseDomainOnly restricts route access to requests made to the base domain only.
// This is used for authentication routes that must not be accessed from subdomains
// to avoid cookie scoping issues.
//
// Returns 403 Forbidden if accessed from a subdomain.
func AllowBaseDomainOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isBase, exists := c.Get("is_base_domain")
		if !exists || !isBase.(bool) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "This endpoint is only accessible from the base domain",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireSubdomain restricts route access to requests made from subdomains only.
// Returns 400 Bad Request if accessed from the base domain.
func RequireSubdomain() gin.HandlerFunc {
	return func(c *gin.Context) {
		subdomain, exists := c.Get("subdomain")
		if !exists || subdomain.(string) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "This endpoint requires a subdomain",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// extractSubdomain extracts the subdomain portion from a host string.
// Returns empty string if the request is to the base domain.
//
// Examples:
//   - extractSubdomain("localhost:8080", "localhost:8080") -> ""
//   - extractSubdomain("acme.localhost:8080", "localhost:8080") -> "acme"
//   - extractSubdomain("acme.portal.nuon.co", "portal.nuon.co") -> "acme"
func extractSubdomain(host, baseDomain string) string {
	// Remove port from host if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Remove port from baseDomain if present (for comparison)
	baseDomainWithoutPort := baseDomain
	if idx := strings.Index(baseDomain, ":"); idx != -1 {
		baseDomainWithoutPort = baseDomain[:idx]
	}

	// Check if host matches base domain exactly
	if host == baseDomainWithoutPort {
		return ""
	}

	// Extract subdomain by removing the base domain suffix
	suffix := "." + baseDomainWithoutPort
	if strings.HasSuffix(host, suffix) {
		return strings.TrimSuffix(host, suffix)
	}

	// Host doesn't match base domain pattern
	return ""
}
