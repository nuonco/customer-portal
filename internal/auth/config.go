package auth

import (
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"
)

// ProviderType represents the type of authentication provider
type ProviderType string

const (
	ProviderTypeOIDC ProviderType = "oidc"
	ProviderTypeSAML ProviderType = "saml"
)

// ProviderConfig contains configuration for authentication providers
type ProviderConfig struct {
	// Type selects the provider: "oidc" or "saml"
	Type ProviderType

	// RedirectURI is the OAuth/SAML callback URL
	RedirectURI string

	// OIDC-specific configuration
	ClientID              string
	ClientSecret          string
	IssuerURL             string
	Scopes                []string
	PostLogoutRedirectURI string // Where to redirect after IdP logout

	// SAML-specific configuration
	IDPMetadataURL string // Preferred: auto-fetch IdP metadata
	IDPEntityID    string // Manual config: IdP entity ID
	IDPSSOURL      string // Manual config: IdP SSO URL
	IDPCertificate string // Manual config: IdP signing certificate (PEM)
	SPEntityID     string // Service Provider entity ID
}

// LoadConfigFromEnv builds ProviderConfig from environment variables
func LoadConfigFromEnv() ProviderConfig {
	cfg := ProviderConfig{
		Type:        ProviderType(os.Getenv("AUTH_PROVIDER")),
		RedirectURI: os.Getenv("AUTH_REDIRECT_URI"),

		// OIDC
		ClientID:              os.Getenv("AUTH_CLIENT_ID"),
		ClientSecret:          os.Getenv("AUTH_CLIENT_SECRET"),
		IssuerURL:             os.Getenv("AUTH_OIDC_ISSUER_URL"),
		PostLogoutRedirectURI: os.Getenv("AUTH_POST_LOGOUT_REDIRECT_URI"),

		// SAML
		IDPMetadataURL: os.Getenv("AUTH_SAML_IDP_METADATA_URL"),
		IDPEntityID:    os.Getenv("AUTH_SAML_IDP_ENTITY_ID"),
		IDPSSOURL:      os.Getenv("AUTH_SAML_IDP_SSO_URL"),
		IDPCertificate: os.Getenv("AUTH_SAML_IDP_CERTIFICATE"),
		SPEntityID:     os.Getenv("AUTH_SAML_SP_ENTITY_ID"),
	}

	// Parse scopes (comma-separated)
	if scopesStr := os.Getenv("AUTH_OIDC_SCOPES"); scopesStr != "" {
		cfg.Scopes = strings.Split(scopesStr, ",")
		for i := range cfg.Scopes {
			cfg.Scopes[i] = strings.TrimSpace(cfg.Scopes[i])
		}
	} else {
		// Default OIDC scopes
		cfg.Scopes = []string{"openid", "profile", "email"}
	}

	return cfg
}

// IsConfigured returns true if enough configuration is present to initialize a provider
func (c *ProviderConfig) IsConfigured() bool {
	if c.Type == "" {
		return false
	}

	switch c.Type {
	case ProviderTypeOIDC:
		return c.ClientID != "" && c.IssuerURL != ""
	case ProviderTypeSAML:
		return c.SPEntityID != "" && (c.IDPMetadataURL != "" || c.IDPSSOURL != "")
	default:
		return false
	}
}

// Validate checks if the configuration is valid for the selected provider type
func (c *ProviderConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("AUTH_PROVIDER is required (options: oidc, saml)")
	}

	if c.RedirectURI == "" {
		return fmt.Errorf("AUTH_REDIRECT_URI is required")
	}

	switch c.Type {
	case ProviderTypeOIDC:
		if c.ClientID == "" {
			return fmt.Errorf("AUTH_CLIENT_ID is required for OIDC")
		}
		if c.ClientSecret == "" {
			return fmt.Errorf("AUTH_CLIENT_SECRET is required for OIDC")
		}
		if c.IssuerURL == "" {
			return fmt.Errorf("AUTH_OIDC_ISSUER_URL is required for OIDC")
		}

	case ProviderTypeSAML:
		if c.SPEntityID == "" {
			return fmt.Errorf("AUTH_SAML_SP_ENTITY_ID is required for SAML")
		}
		if c.IDPMetadataURL == "" && c.IDPSSOURL == "" {
			return fmt.Errorf("AUTH_SAML_IDP_METADATA_URL or AUTH_SAML_IDP_SSO_URL is required for SAML")
		}
		if c.IDPMetadataURL == "" && c.IDPCertificate == "" {
			return fmt.Errorf("AUTH_SAML_IDP_CERTIFICATE is required when not using metadata URL")
		}

	default:
		return fmt.Errorf("unsupported AUTH_PROVIDER: %s (options: oidc, saml)", c.Type)
	}

	return nil
}

// NewProvider creates the appropriate auth provider based on configuration
func NewProvider(cfg ProviderConfig, db *gorm.DB) (AuthProvider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	switch cfg.Type {
	case ProviderTypeOIDC:
		return NewOIDCProvider(cfg, db)
	case ProviderTypeSAML:
		return NewSAMLProvider(cfg, db)
	default:
		return nil, fmt.Errorf("unsupported auth provider type: %s", cfg.Type)
	}
}

// NewFallbackLocalProvider creates a local password auth provider for use when no IdP is configured
func NewFallbackLocalProvider(db *gorm.DB) AuthProvider {
	return NewLocalProvider(db)
}
