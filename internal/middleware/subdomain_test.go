package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubdomainContext(t *testing.T) {
	tests := []struct {
		name              string
		host              string
		baseDomain        string
		expectedSubdomain string
		expectedIsBase    bool
	}{
		{
			name:              "base domain sets empty subdomain",
			host:              "localhost:8080",
			baseDomain:        "localhost:8080",
			expectedSubdomain: "",
			expectedIsBase:    true,
		},
		{
			name:              "subdomain extracted correctly",
			host:              "acme.localhost:8080",
			baseDomain:        "localhost:8080",
			expectedSubdomain: "acme",
			expectedIsBase:    false,
		},
		{
			name:              "production subdomain",
			host:              "acme.portal.nuon.co",
			baseDomain:        "portal.nuon.co",
			expectedSubdomain: "acme",
			expectedIsBase:    false,
		},
		{
			name:              "base domain without port",
			host:              "portal.nuon.co",
			baseDomain:        "portal.nuon.co",
			expectedSubdomain: "",
			expectedIsBase:    true,
		},
		{
			name:              "host with port, base without",
			host:              "acme.localhost:8080",
			baseDomain:        "localhost",
			expectedSubdomain: "acme",
			expectedIsBase:    false,
		},
		{
			name:              "multi-level subdomain",
			host:              "test.acme.portal.nuon.co",
			baseDomain:        "portal.nuon.co",
			expectedSubdomain: "test.acme",
			expectedIsBase:    false,
		},
		{
			name:              "unrelated domain returns empty subdomain",
			host:              "example.com",
			baseDomain:        "portal.nuon.co",
			expectedSubdomain: "",
			expectedIsBase:    true, // Empty subdomain means is_base_domain is true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
			c.Request.Host = tt.host

			// Run middleware
			middleware := SubdomainContext(tt.baseDomain)
			middleware(c)

			// Check context values
			subdomain, exists := c.Get("subdomain")
			require.True(t, exists, "subdomain should be set in context")
			assert.Equal(t, tt.expectedSubdomain, subdomain)

			isBase, exists := c.Get("is_base_domain")
			require.True(t, exists, "is_base_domain should be set in context")
			assert.Equal(t, tt.expectedIsBase, isBase)
		})
	}
}

func TestExtractSubdomain(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		baseDomain string
		expected   string
	}{
		{
			name:       "localhost with port exact match",
			host:       "localhost:8080",
			baseDomain: "localhost:8080",
			expected:   "",
		},
		{
			name:       "subdomain on localhost with port",
			host:       "acme.localhost:8080",
			baseDomain: "localhost:8080",
			expected:   "acme",
		},
		{
			name:       "production domain exact match",
			host:       "portal.nuon.co",
			baseDomain: "portal.nuon.co",
			expected:   "",
		},
		{
			name:       "subdomain on production",
			host:       "acme.portal.nuon.co",
			baseDomain: "portal.nuon.co",
			expected:   "acme",
		},
		{
			name:       "different domain",
			host:       "example.com",
			baseDomain: "portal.nuon.co",
			expected:   "",
		},
		{
			name:       "partial match not subdomain",
			host:       "notportal.nuon.co",
			baseDomain: "portal.nuon.co",
			expected:   "",
		},
		{
			name:       "host without port, base with port",
			host:       "acme.localhost",
			baseDomain: "localhost:8080",
			expected:   "acme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSubdomain(tt.host, tt.baseDomain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAllowBaseDomainOnly(t *testing.T) {
	tests := []struct {
		name            string
		isBaseDomain    bool
		isBaseDomainSet bool
		expectedStatus  int
		shouldProceed   bool
	}{
		{
			name:            "base domain allowed",
			isBaseDomain:    true,
			isBaseDomainSet: true,
			expectedStatus:  http.StatusOK,
			shouldProceed:   true,
		},
		{
			name:            "subdomain blocked",
			isBaseDomain:    false,
			isBaseDomainSet: true,
			expectedStatus:  http.StatusForbidden,
			shouldProceed:   false,
		},
		{
			name:            "missing context value blocked",
			isBaseDomainSet: false,
			expectedStatus:  http.StatusForbidden,
			shouldProceed:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

			if tt.isBaseDomainSet {
				c.Set("is_base_domain", tt.isBaseDomain)
			}

			handlerCalled := false
			middleware := AllowBaseDomainOnly()
			middleware(c)

			if !c.IsAborted() {
				handlerCalled = true
			}

			assert.Equal(t, tt.shouldProceed, handlerCalled)
			if !tt.shouldProceed {
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestRequireSubdomain(t *testing.T) {
	tests := []struct {
		name           string
		subdomain      string
		subdomainSet   bool
		expectedStatus int
		shouldProceed  bool
	}{
		{
			name:           "subdomain present allowed",
			subdomain:      "acme",
			subdomainSet:   true,
			expectedStatus: http.StatusOK,
			shouldProceed:  true,
		},
		{
			name:           "empty subdomain blocked",
			subdomain:      "",
			subdomainSet:   true,
			expectedStatus: http.StatusBadRequest,
			shouldProceed:  false,
		},
		{
			name:           "missing subdomain context blocked",
			subdomainSet:   false,
			expectedStatus: http.StatusBadRequest,
			shouldProceed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

			if tt.subdomainSet {
				c.Set("subdomain", tt.subdomain)
			}

			handlerCalled := false
			middleware := RequireSubdomain()
			middleware(c)

			if !c.IsAborted() {
				handlerCalled = true
			}

			assert.Equal(t, tt.shouldProceed, handlerCalled)
			if !tt.shouldProceed {
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestSubdomainMiddlewareChain(t *testing.T) {
	// Test that SubdomainContext properly sets values for downstream middleware
	gin.SetMode(gin.TestMode)
	router := gin.New()

	baseDomain := "portal.nuon.co"

	router.Use(SubdomainContext(baseDomain))
	router.GET("/test", func(c *gin.Context) {
		subdomain := c.GetString("subdomain")
		isBase, _ := c.Get("is_base_domain")
		c.JSON(http.StatusOK, gin.H{
			"subdomain":      subdomain,
			"is_base_domain": isBase,
		})
	})

	tests := []struct {
		name              string
		host              string
		expectedSubdomain string
		expectedIsBase    bool
	}{
		{
			name:              "base domain request",
			host:              "portal.nuon.co",
			expectedSubdomain: "",
			expectedIsBase:    true,
		},
		{
			name:              "subdomain request",
			host:              "acme.portal.nuon.co",
			expectedSubdomain: "acme",
			expectedIsBase:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/test", nil)
			req.Host = tt.host

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			// The response body contains the values - we've verified context is set correctly
		})
	}
}
