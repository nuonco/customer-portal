// Package testutil provides testing utilities for the customer-dashboard service.
// It includes helpers for creating mock Gin contexts, generating test data,
// and setting up JWT claims for authentication testing.
package testutil

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"github.com/nuonco/customer-portal/internal/models"
)

func init() {
	// Set Gin to test mode to suppress debug output
	gin.SetMode(gin.TestMode)
}

// NewTestContext creates a new Gin context for testing with default request/response.
// The context is set up with an empty GET request and a response recorder.
func NewTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

// NewTestContextWithRequest creates a Gin context with a custom HTTP request.
// This is useful for testing handlers that need specific request methods or bodies.
func NewTestContextWithRequest(method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
		c.Request, _ = http.NewRequestWithContext(c.Request.Context(), method, path, bodyReader)
	} else {
		c.Request, _ = http.NewRequest(method, path, nil)
	}

	return c, w
}

// NewTestContextWithQuery creates a Gin context with query parameters.
func NewTestContextWithQuery(queryParams map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	q := url.Values{}
	for k, v := range queryParams {
		q.Add(k, v)
	}

	c.Request, _ = http.NewRequest(http.MethodGet, "/?"+q.Encode(), nil)
	return c, w
}

// NewTestContextWithParams creates a Gin context with URL parameters.
// This is essential for testing handlers that extract parameters like :install_id.
func NewTestContextWithParams(params map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	// Set up URL params
	ginParams := make([]gin.Param, 0, len(params))
	for k, v := range params {
		ginParams = append(ginParams, gin.Param{Key: k, Value: v})
	}
	c.Params = ginParams

	return c, w
}

// NewTestContextWithCookie creates a Gin context with cookies set.
func NewTestContextWithCookie(cookies map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	for name, value := range cookies {
		c.Request.AddCookie(&http.Cookie{
			Name:  name,
			Value: value,
		})
	}

	return c, w
}

// NewTestContextWithHeader creates a Gin context with custom headers.
func NewTestContextWithHeader(headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	for name, value := range headers {
		c.Request.Header.Set(name, value)
	}

	return c, w
}

// SetJWTClaims sets JWT claims on a Gin context for testing authenticated handlers.
// This simulates what the JWT middleware does after validating a token.
func SetJWTClaims(c *gin.Context, user *models.User) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"name":    user.Name,
		"email":   user.Email,
		"role":    string(user.Role),
	}
	c.Set("JWT_PAYLOAD", claims)
}

// SetJWTClaimsMap sets JWT claims from a map for testing edge cases.
func SetJWTClaimsMap(c *gin.Context, claims map[string]interface{}) {
	c.Set("JWT_PAYLOAD", jwt.MapClaims(claims))
}

// RandomString generates a random hex string of the specified length.
// Useful for generating unique test identifiers.
func RandomString(length int) string {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)[:length]
}

// RandomEmail generates a random email address for testing.
func RandomEmail() string {
	return fmt.Sprintf("test-%s@example.com", RandomString(8))
}

// RandomID generates a random 26-character ID matching the shortid format.
func RandomID(prefix string) string {
	// shortid format: 3-char prefix + 23-char nanoid (base36)
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 23)
	for i := range result {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		result[i] = alphabet[int(b[0])%len(alphabet)]
	}
	return prefix + string(result)
}

// RandomUserID generates a random user ID with the correct prefix.
func RandomUserID() string {
	return RandomID("iur")
}

// RandomOrgID generates a random organization ID with the correct prefix.
func RandomOrgID() string {
	return RandomID("ino")
}

// RandomInstallID generates a random install ID with the correct prefix.
func RandomInstallID() string {
	return RandomID("isi")
}

// RandomInstallLinkID generates a random install link ID with the correct prefix.
func RandomInstallLinkID() string {
	return RandomID("ilk")
}

// NewTestRouter creates a new Gin router configured for testing.
// This provides a clean router instance that can be used to register test routes.
func NewTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// HTTPRequest is a helper struct for making HTTP requests in tests.
type HTTPRequest struct {
	Method  string
	Path    string
	Body    []byte
	Headers map[string]string
	Cookies map[string]string
}

// PerformRequest performs an HTTP request against a router and returns the response.
func PerformRequest(router *gin.Engine, req HTTPRequest) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader
	if req.Body != nil {
		bodyReader = bytes.NewReader(req.Body)
	}

	var httpReq *http.Request
	if bodyReader != nil {
		httpReq, _ = http.NewRequest(req.Method, req.Path, bodyReader)
	} else {
		httpReq, _ = http.NewRequest(req.Method, req.Path, nil)
	}

	for name, value := range req.Headers {
		httpReq.Header.Set(name, value)
	}

	for name, value := range req.Cookies {
		httpReq.AddCookie(&http.Cookie{
			Name:  name,
			Value: value,
		})
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httpReq)
	return w
}

// SetContextValue sets a value in the Gin context, useful for middleware testing.
func SetContextValue(c *gin.Context, key string, value interface{}) {
	c.Set(key, value)
}

// GetResponseBody returns the response body as a string from a response recorder.
func GetResponseBody(w *httptest.ResponseRecorder) string {
	return w.Body.String()
}

// AssertContains checks if the haystack contains the needle (case-insensitive).
func AssertContains(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// TimePtr returns a pointer to a time.Time value.
func TimePtr(t time.Time) *time.Time {
	return &t
}

// StringPtr returns a pointer to a string value.
func StringPtr(s string) *string {
	return &s
}
