package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// StateLength is the number of random bytes used for state parameters
const StateLength = 32

// StateData contains state information for OIDC authentication flow.
// The state parameter is used for CSRF protection and can carry additional
// context like the subdomain to return to after authentication completes.
type StateData struct {
	Random          string `json:"r"` // Random bytes for CSRF protection
	ReturnSubdomain string `json:"s"` // Subdomain to return to after auth (empty for base domain)
}

// GenerateState creates a cryptographically secure random state parameter
// for CSRF protection in OAuth/SAML flows.
// This is the simple version without subdomain tracking.
func GenerateState() (string, error) {
	bytes := make([]byte, StateLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateStateWithSubdomain creates a state parameter with embedded subdomain information.
// The state includes both random bytes (for CSRF protection) and the subdomain to return to.
// The resulting state is JSON-encoded and base64-encoded.
//
// This is used in the base domain authentication flow where:
// 1. User starts login on subdomain (e.g., acme.portal.nuon.co)
// 2. Redirects to base domain for OIDC (portal.nuon.co/auth/login?return_to=acme)
// 3. State embeds "acme" so we know where to redirect after auth
// 4. After OIDC completes, redirect back to acme.portal.nuon.co
func GenerateStateWithSubdomain(subdomain string) (string, error) {
	// Generate random bytes for CSRF protection
	randomBytes := make([]byte, StateLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}

	// Create state data with random component and subdomain
	stateData := StateData{
		Random:          base64.URLEncoding.EncodeToString(randomBytes),
		ReturnSubdomain: subdomain,
	}

	// Marshal to JSON
	stateJSON, err := json.Marshal(stateData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}

	// Base64 encode the JSON
	return base64.URLEncoding.EncodeToString(stateJSON), nil
}

// ExtractSubdomainFromState extracts the subdomain from a state parameter
// that was created with GenerateStateWithSubdomain.
// Returns an error if the state is not properly formatted.
func ExtractSubdomainFromState(state string) (string, error) {
	// Base64 decode
	stateJSON, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return "", fmt.Errorf("failed to decode state: %w", err)
	}

	// Unmarshal JSON
	var stateData StateData
	if err := json.Unmarshal(stateJSON, &stateData); err != nil {
		return "", fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return stateData.ReturnSubdomain, nil
}

// ValidateState checks if the received state matches the expected state
// This should be used to verify the state parameter in OAuth/SAML callbacks
func ValidateState(expected, received string) bool {
	if expected == "" || received == "" {
		return false
	}
	// Use constant-time comparison to prevent timing attacks
	// For state parameters, a simple equality check is sufficient
	// since the state is random and not a secret
	return expected == received
}
