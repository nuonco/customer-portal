package middleware

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v4"
	"github.com/workos/workos-go/v4/pkg/usermanagement"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// AuthResult contains the user and session info from WorkOS authentication
type AuthResult struct {
	User      *models.User
	SessionID string // WorkOS session ID for logout
}

// WorkOSAuth handles authentication with WorkOS AuthKit
type WorkOSAuth struct {
	clientID    string
	redirectURI string
	db          *gorm.DB
}

// NewWorkOSAuth creates a new WorkOS authentication handler
func NewWorkOSAuth(apiKey, clientID, redirectURI string, db *gorm.DB) *WorkOSAuth {
	usermanagement.SetAPIKey(apiKey)
	return &WorkOSAuth{
		clientID:    clientID,
		redirectURI: redirectURI,
		db:          db,
	}
}

// GetAuthorizationURL returns the URL to redirect users to for authentication
func (w *WorkOSAuth) GetAuthorizationURL() (string, error) {
	url, err := usermanagement.GetAuthorizationURL(
		usermanagement.GetAuthorizationURLOpts{
			ClientID:    w.clientID,
			RedirectURI: w.redirectURI,
			Provider:    "authkit",
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get authorization URL: %w", err)
	}
	return url.String(), nil
}

// AuthenticateWithCode exchanges an authorization code for user data and creates/updates the local user
func (w *WorkOSAuth) AuthenticateWithCode(ctx context.Context, code string) (*AuthResult, error) {
	resp, err := usermanagement.AuthenticateWithCode(
		ctx,
		usermanagement.AuthenticateWithCodeOpts{
			ClientID: w.clientID,
			Code:     code,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with code: %w", err)
	}

	// Extract session ID from access token for logout
	sessionID := extractSessionID(resp.AccessToken)

	// Build name from WorkOS user
	name := resp.User.FirstName
	if resp.User.LastName != "" {
		if name != "" {
			name += " "
		}
		name += resp.User.LastName
	}

	// Find or create user by email
	var user models.User
	result := w.db.Where("email = ?", resp.User.Email).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create new vendor user
			user = models.User{
				Email: resp.User.Email,
				Name:  name,
				Role:  models.RoleVendor,
			}
			if err := w.db.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to query user: %w", result.Error)
		}
	} else {
		// Update existing user's name if it was empty
		if user.Name == "" && name != "" {
			user.Name = name
			w.db.Save(&user)
		}
	}

	return &AuthResult{
		User:      &user,
		SessionID: sessionID,
	}, nil
}

// extractSessionID extracts the session ID (sid claim) from a WorkOS access token
func extractSessionID(accessToken string) string {
	// Parse the JWT without validation (we trust WorkOS)
	token, _, err := jwt.NewParser().ParseUnverified(accessToken, jwt.MapClaims{})
	if err != nil {
		return ""
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}

	sid, ok := claims["sid"].(string)
	if !ok {
		return ""
	}

	return sid
}

// GetLogoutURL returns the URL to redirect users to for logging out of WorkOS
func (w *WorkOSAuth) GetLogoutURL(sessionID string) (string, error) {
	url, err := usermanagement.GetLogoutURL(
		usermanagement.GetLogoutURLOpts{
			SessionID: sessionID,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get logout URL: %w", err)
	}
	return url.String(), nil
}
