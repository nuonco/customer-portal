package auth

import (
	"context"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// AuthResult contains the normalized user info from any IdP
type AuthResult struct {
	User      *models.User
	SessionID string // Provider-specific session ID for logout
}

// CallbackRequest contains provider-agnostic callback data
type CallbackRequest struct {
	// OIDC fields
	Code  string
	State string

	// SAML fields
	SAMLResponse string
	RelayState   string
}

// AuthProvider defines the interface for authentication providers
type AuthProvider interface {
	// Name returns the provider identifier (e.g., "oidc", "saml")
	Name() string

	// GetAuthorizationURL returns the URL to redirect users for login
	// For OIDC: authorization endpoint with state
	// For SAML: IdP SSO URL with SAMLRequest
	GetAuthorizationURL(state string) (string, error)

	// HandleCallback processes the authentication callback
	// For OIDC: exchanges code for tokens, validates ID token
	// For SAML: validates SAMLResponse, extracts assertions
	HandleCallback(ctx context.Context, req CallbackRequest) (*AuthResult, error)

	// GetLogoutURL returns the URL for single logout (optional)
	// Returns empty string if provider doesn't support SLO
	GetLogoutURL(sessionID string) (string, error)

	// SupportsLogout returns whether this provider supports logout URLs
	SupportsLogout() bool
}
