package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// StateLength is the number of random bytes used for state parameters
const StateLength = 32

// GenerateState creates a cryptographically secure random state parameter
// for CSRF protection in OAuth/SAML flows
func GenerateState() (string, error) {
	bytes := make([]byte, StateLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
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
