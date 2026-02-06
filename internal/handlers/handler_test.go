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

func TestDarkenColor(t *testing.T) {
	tests := []struct {
		name     string
		hexColor string
		factor   float64
		expected string
	}{
		{
			name:     "darken blue",
			hexColor: "#2563EB",
			factor:   0.8,
			expected: "#1D4FBC",
		},
		{
			name:     "darken with hash prefix",
			hexColor: "#FF0000",
			factor:   0.5,
			expected: "#7F0000",
		},
		{
			name:     "darken without hash prefix",
			hexColor: "00FF00",
			factor:   0.5,
			expected: "#007F00",
		},
		{
			name:     "invalid hex returns default",
			hexColor: "#ABC",
			factor:   0.8,
			expected: "#1D4ED8",
		},
		{
			name:     "too short returns default",
			hexColor: "#12",
			factor:   0.8,
			expected: "#1D4ED8",
		},
		{
			name:     "empty string returns default",
			hexColor: "",
			factor:   0.8,
			expected: "#1D4ED8",
		},
		{
			name:     "black stays black",
			hexColor: "#000000",
			factor:   0.8,
			expected: "#000000",
		},
		{
			name:     "white darkened",
			hexColor: "#FFFFFF",
			factor:   0.8,
			expected: "#CCCCCC",
		},
		{
			name:     "factor 1.0 keeps same color",
			hexColor: "#AABBCC",
			factor:   1.0,
			expected: "#AABBCC",
		},
		{
			name:     "factor 0 produces black",
			hexColor: "#FFFFFF",
			factor:   0,
			expected: "#000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DarkenColor(tt.hexColor, tt.factor)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPrimaryColors(t *testing.T) {
	tests := []struct {
		name         string
		primaryColor string
		expectedPri  string
		expectedDark string
	}{
		{
			name:         "custom color",
			primaryColor: "#FF0000",
			expectedPri:  "#FF0000",
			expectedDark: "#CC0000",
		},
		{
			name:         "empty uses default",
			primaryColor: "",
			expectedPri:  DefaultPrimaryColor,
			expectedDark: DarkenColor(DefaultPrimaryColor, 0.8),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary, dark := GetPrimaryColors(tt.primaryColor)
			assert.Equal(t, tt.expectedPri, primary)
			assert.Equal(t, tt.expectedDark, dark)
		})
	}
}

func TestGetPageFromQuery(t *testing.T) {
	tests := []struct {
		name         string
		queryParams  map[string]string
		expectedPage int
	}{
		{
			name:         "no page param uses default",
			queryParams:  map[string]string{},
			expectedPage: 1,
		},
		{
			name:         "valid page 1",
			queryParams:  map[string]string{"page": "1"},
			expectedPage: 1,
		},
		{
			name:         "valid page 5",
			queryParams:  map[string]string{"page": "5"},
			expectedPage: 5,
		},
		{
			name:         "page 0 defaults to 1",
			queryParams:  map[string]string{"page": "0"},
			expectedPage: 1,
		},
		{
			name:         "negative page defaults to 1",
			queryParams:  map[string]string{"page": "-1"},
			expectedPage: 1,
		},
		{
			name:         "non-numeric page defaults to 1",
			queryParams:  map[string]string{"page": "abc"},
			expectedPage: 1,
		},
		{
			name:         "empty page param defaults to 1",
			queryParams:  map[string]string{"page": ""},
			expectedPage: 1,
		},
		{
			name:         "large page number",
			queryParams:  map[string]string{"page": "999"},
			expectedPage: 999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Build query string
			query := ""
			for k, v := range tt.queryParams {
				if query != "" {
					query += "&"
				}
				query += k + "=" + v
			}

			c.Request, _ = http.NewRequest(http.MethodGet, "/?"+query, nil)

			result := getPageFromQuery(c)
			assert.Equal(t, tt.expectedPage, result)
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

func TestDefaultPrimaryColor(t *testing.T) {
	// Verify the default primary color is Tailwind blue-600
	assert.Equal(t, "#2563EB", DefaultPrimaryColor)
}

func TestVendorLoginConfig(t *testing.T) {
	config := VendorLoginConfig("/admin")

	assert.Equal(t, "Vendor Login", config.Title)
	assert.Equal(t, "Login / Sign Up", config.ButtonText)
	assert.Equal(t, "/admin/orgs", config.RedirectURL)
	assert.Equal(t, "vendor/login.html", config.Template)
}

func TestCustomerLoginConfig(t *testing.T) {
	config := CustomerLoginConfig("")

	assert.Equal(t, "Customer Login", config.Title)
	assert.Equal(t, "Login", config.ButtonText)
	assert.Equal(t, "/installs", config.RedirectURL)
	assert.Equal(t, "customer/login.html", config.Template)
	assert.Contains(t, config.HelpText, "Don't have an account")
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
