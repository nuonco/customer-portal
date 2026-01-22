package auth

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// OIDCProvider implements AuthProvider for OIDC/OAuth 2.0 identity providers
type OIDCProvider struct {
	config   ProviderConfig
	db       *gorm.DB
	provider *oidc.Provider
	oauth2   oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewOIDCProvider creates a new OIDC authentication provider
func NewOIDCProvider(cfg ProviderConfig, db *gorm.DB) (*OIDCProvider, error) {
	ctx := context.Background()

	// Create OIDC provider (performs discovery)
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Configure OAuth2
	oauth2Config := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
	}

	// Create ID token verifier
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return &OIDCProvider{
		config:   cfg,
		db:       db,
		provider: provider,
		oauth2:   oauth2Config,
		verifier: verifier,
	}, nil
}

// Name returns the provider identifier
func (p *OIDCProvider) Name() string {
	return "oidc"
}

// GetAuthorizationURL returns the URL to redirect users for login
func (p *OIDCProvider) GetAuthorizationURL(state string) (string, error) {
	// Use prompt=select_account to always show the account selection screen,
	// even if the user has an existing session with the identity provider
	return p.oauth2.AuthCodeURL(state, oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

// HandleCallback processes the OIDC authorization code callback
func (p *OIDCProvider) HandleCallback(ctx context.Context, req CallbackRequest) (*AuthResult, error) {
	// Exchange authorization code for tokens
	token, err := p.oauth2.Exchange(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Extract and verify ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Sub           string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	// Determine email
	email := claims.Email
	if email == "" {
		return nil, fmt.Errorf("no email claim in ID token")
	}

	// Determine name (prefer Name, fall back to GivenName + FamilyName)
	name := claims.Name
	if name == "" {
		name = claims.GivenName
		if claims.FamilyName != "" {
			if name != "" {
				name += " "
			}
			name += claims.FamilyName
		}
	}

	// Find or create user
	user, err := p.findOrCreateUser(email, name)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:      user,
		SessionID: idToken.Subject, // Use subject as session identifier
	}, nil
}

// GetLogoutURL returns the URL for OIDC RP-initiated logout
func (p *OIDCProvider) GetLogoutURL(sessionID string) (string, error) {
	if p.config.PostLogoutRedirectURI == "" {
		return "", nil
	}

	// Build the logout URL based on the issuer
	// Auth0 uses: https://{domain}/v2/logout?client_id={client_id}&returnTo={return_url}
	// Standard OIDC uses end_session_endpoint from discovery
	issuerURL := strings.TrimSuffix(p.config.IssuerURL, "/")

	// Build Auth0-style logout URL (works for Auth0 and similar providers)
	logoutURL, err := url.Parse(issuerURL + "/v2/logout")
	if err != nil {
		return "", fmt.Errorf("failed to parse logout URL: %w", err)
	}

	query := logoutURL.Query()
	query.Set("client_id", p.config.ClientID)
	query.Set("returnTo", p.config.PostLogoutRedirectURI)
	logoutURL.RawQuery = query.Encode()

	return logoutURL.String(), nil
}

// SupportsLogout returns whether this provider supports logout URLs
func (p *OIDCProvider) SupportsLogout() bool {
	return p.config.PostLogoutRedirectURI != ""
}

// findOrCreateUser finds an existing user by email or creates a new one with RoleVendor (admin portal default)
func (p *OIDCProvider) findOrCreateUser(email, name string) (*models.User, error) {
	return p.findOrCreateUserWithRole(email, name, models.RoleVendor)
}

// findOrCreateUserWithRole finds an existing user by email or creates a new one with the specified role
func (p *OIDCProvider) findOrCreateUserWithRole(email, name string, defaultRole models.UserRole) (*models.User, error) {
	var user models.User
	result := p.db.Where("email = ?", email).First(&user)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create new user with the specified role
			user = models.User{
				Email: email,
				Name:  name,
				Role:  defaultRole,
			}
			if err := p.db.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to query user: %w", result.Error)
		}
	} else {
		// Update existing user's name if it was empty
		if user.Name == "" && name != "" {
			user.Name = name
			p.db.Save(&user)
		}
	}

	return &user, nil
}

// HandleCallbackWithRole processes the OIDC callback and creates NEW users with the specified role.
// Existing users keep their current database role.
func (p *OIDCProvider) HandleCallbackWithRole(ctx context.Context, req CallbackRequest, defaultRole models.UserRole) (*AuthResult, error) {
	// Exchange authorization code for tokens
	token, err := p.oauth2.Exchange(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Extract and verify ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Sub           string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	// Determine email
	email := claims.Email
	if email == "" {
		return nil, fmt.Errorf("no email claim in ID token")
	}

	// Determine name (prefer Name, fall back to GivenName + FamilyName)
	name := claims.Name
	if name == "" {
		name = claims.GivenName
		if claims.FamilyName != "" {
			if name != "" {
				name += " "
			}
			name += claims.FamilyName
		}
	}

	// Find or create user with specified role for NEW users
	user, err := p.findOrCreateUserWithRole(email, name, defaultRole)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:      user,
		SessionID: idToken.Subject, // Use subject as session identifier
	}, nil
}
