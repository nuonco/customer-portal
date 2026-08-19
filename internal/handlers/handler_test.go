package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestIsHTMXRequest(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected bool
	}{
		{
			name:     "HTMX request",
			header:   "true",
			expected: true,
		},
		{
			name:     "non-HTMX request (no header)",
			header:   "",
			expected: false,
		},
		{
			name:     "non-HTMX request (false header)",
			header:   "false",
			expected: false,
		},
		{
			name:     "non-HTMX request (other value)",
			header:   "yes",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

			if tt.header != "" {
				c.Request.Header.Set("HX-Request", tt.header)
			}

			result := isHTMXRequest(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstructCustomerDashboardInstallURL(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		baseDomain string
		subdomain  string
		installID  string
		expected   string
	}{
		{
			name:       "with subdomain (https)",
			baseURL:    "https://portal.nuon.co",
			baseDomain: "portal.nuon.co",
			subdomain:  "acme",
			installID:  "inst_123",
			expected:   "https://acme.portal.nuon.co/installs/inst_123",
		},
		{
			name:       "with subdomain (http)",
			baseURL:    "http://localhost:8080",
			baseDomain: "localhost:8080",
			subdomain:  "acme",
			installID:  "inst_456",
			expected:   "http://acme.localhost:8080/installs/inst_456",
		},
		{
			name:       "without subdomain falls back to base URL",
			baseURL:    "https://portal.nuon.co",
			baseDomain: "portal.nuon.co",
			subdomain:  "",
			installID:  "inst_789",
			expected:   "https://portal.nuon.co/installs/inst_789",
		},
		{
			name:       "empty base domain falls back to base URL",
			baseURL:    "https://portal.nuon.co",
			baseDomain: "",
			subdomain:  "acme",
			installID:  "inst_abc",
			expected:   "https://portal.nuon.co/installs/inst_abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constructCustomerDashboardInstallURL(tt.baseURL, tt.baseDomain, tt.subdomain, tt.installID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVendorLoginConfig(t *testing.T) {
	config := VendorLoginConfig("/admin")

	assert.Equal(t, "Vendor Login", config.Title)
	assert.Equal(t, "Login / Sign Up", config.ButtonText)
	assert.Equal(t, "/admin/orgs", config.RedirectURL)
	assert.Equal(t, "vendor/login.html", config.Template)
}

// BenchmarkDarkenColor measures color darkening performance
func BenchmarkDarkenColor(b *testing.B) {
	color := "#2563EB"
	for i := 0; i < b.N; i++ {
		DarkenColor(color, 0.8)
	}
}

// BenchmarkIsHTMXRequest measures HTMX detection performance
func BenchmarkIsHTMXRequest(b *testing.B) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("HX-Request", "true")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isHTMXRequest(c)
	}
}
