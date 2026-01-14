package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"gorm.io/gorm"
)

// OIDCSource indicates where the OIDC configuration came from
type OIDCSource string

const (
	OIDCSourceDatabase    OIDCSource = "database"
	OIDCSourceEnvironment OIDCSource = "environment"
)

// CustomerAuthProviderFactory creates auth providers dynamically based on DB config.
// It caches the OIDC provider to avoid re-creating it on every request,
// and invalidates the cache when the config changes.
// If no DB config is active, it falls back to OIDC configured via environment variables.
type CustomerAuthProviderFactory struct {
	db      *gorm.DB
	baseURL string

	mu              sync.RWMutex
	cachedOIDC      *OIDCProvider
	lastConfig      *models.CustomerAuthConfig
	envOIDCConfig   *ProviderConfig // OIDC config from environment variables (fallback)
	envOIDCProvider *OIDCProvider   // Cached env var OIDC provider
}

// NewCustomerAuthProviderFactory creates a new factory for customer auth providers.
// envConfig provides fallback OIDC configuration from environment variables.
// Returns an error if no OIDC source is available (neither DB config nor env vars).
func NewCustomerAuthProviderFactory(db *gorm.DB, baseURL string, envConfig *ProviderConfig) (*CustomerAuthProviderFactory, error) {
	factory := &CustomerAuthProviderFactory{
		db:      db,
		baseURL: baseURL,
	}

	// Check if env var OIDC is configured as fallback
	if envConfig != nil && envConfig.Type == ProviderTypeOIDC && envConfig.IsConfigured() {
		// Create a copy with the customer callback URL
		customerEnvConfig := *envConfig
		customerEnvConfig.RedirectURI = baseURL + "/callback"
		factory.envOIDCConfig = &customerEnvConfig
	}

	// Validate that at least one OIDC source is available
	// We check DB config at startup to see if it's configured
	dbConfig, _ := models.GetOrCreateCustomerAuthConfig(db)
	if !dbConfig.IsActive() && factory.envOIDCConfig == nil {
		return nil, fmt.Errorf("customer authentication requires OIDC: configure customer OIDC in settings or set AUTH_* environment variables")
	}

	return factory, nil
}

// GetProvider returns the appropriate auth provider based on current config.
// Returns DB-configured OIDC provider if active, otherwise falls back to env var OIDC.
func (f *CustomerAuthProviderFactory) GetProvider() (AuthProvider, error) {
	config, err := models.GetOrCreateCustomerAuthConfig(f.db)
	if err != nil {
		// On DB error, try env var fallback
		return f.getEnvOIDCProvider()
	}

	// If DB OIDC is active, use it
	if config.IsActive() {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.needsRefresh(config) {
			provider, err := f.createOIDCProvider(config)
			if err != nil {
				// On error, try env var fallback
				f.mu.Unlock()
				envProvider, envErr := f.getEnvOIDCProvider()
				f.mu.Lock()
				if envErr != nil {
					return nil, fmt.Errorf("failed to create DB OIDC provider: %w, and no env fallback available", err)
				}
				return envProvider, nil
			}
			f.cachedOIDC = provider
			f.lastConfig = config
		}
		return f.cachedOIDC, nil
	}

	// DB OIDC not active, use env var fallback
	return f.getEnvOIDCProvider()
}

// getEnvOIDCProvider returns the cached env var OIDC provider, creating it if necessary.
func (f *CustomerAuthProviderFactory) getEnvOIDCProvider() (*OIDCProvider, error) {
	if f.envOIDCConfig == nil {
		return nil, fmt.Errorf("no OIDC configuration available")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.envOIDCProvider == nil {
		provider, err := NewOIDCProvider(*f.envOIDCConfig, f.db)
		if err != nil {
			return nil, fmt.Errorf("failed to create env OIDC provider: %w", err)
		}
		f.envOIDCProvider = provider
	}

	return f.envOIDCProvider, nil
}

// IsOIDCEnabled always returns true since OIDC is required for customer auth.
// The factory validates at construction time that at least one OIDC source is available.
func (f *CustomerAuthProviderFactory) IsOIDCEnabled() bool {
	return true
}

// GetOIDCSource returns which OIDC source is currently being used.
func (f *CustomerAuthProviderFactory) GetOIDCSource() OIDCSource {
	config, err := models.GetOrCreateCustomerAuthConfig(f.db)
	if err == nil && config.IsActive() {
		return OIDCSourceDatabase
	}
	return OIDCSourceEnvironment
}

// GetConfig returns the current customer auth config from the database.
// Note: This may return an inactive config; use GetOIDCSource to determine the actual source.
func (f *CustomerAuthProviderFactory) GetConfig() (*models.CustomerAuthConfig, error) {
	return models.GetOrCreateCustomerAuthConfig(f.db)
}

// GetAuthURL generates an OIDC authorization URL with the given state.
func (f *CustomerAuthProviderFactory) GetAuthURL(state string) (string, error) {
	provider, err := f.GetProvider()
	if err != nil {
		return "", err
	}

	return provider.GetAuthorizationURL(state)
}

// HandleCallback processes the OIDC callback with customer role.
// Creates users with RoleCustomer instead of RoleVendor.
func (f *CustomerAuthProviderFactory) HandleCallback(ctx context.Context, req CallbackRequest) (*AuthResult, error) {
	provider, err := f.GetProvider()
	if err != nil {
		return nil, err
	}

	oidcProvider, ok := provider.(*OIDCProvider)
	if !ok {
		return nil, fmt.Errorf("expected OIDC provider")
	}

	// Handle the callback - this will find or create a user
	result, err := oidcProvider.HandleCallback(ctx, req)
	if err != nil {
		return nil, err
	}

	// Ensure the user has customer role
	// This handles the case where a user might have been created as a vendor
	// and is now logging in as a customer
	if result.User.Role != models.RoleCustomer {
		// If user exists as vendor, we need to decide what to do
		// For now, we'll allow them to log in but not change their role
		// They can use the same account for both
	}

	return result, nil
}

// TestConnection tests if the OIDC provider can be initialized with the current config.
// Returns nil if successful, or an error describing what went wrong.
func (f *CustomerAuthProviderFactory) TestConnection(ctx context.Context) error {
	config, err := models.GetOrCreateCustomerAuthConfig(f.db)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !config.IsConfigured() {
		return fmt.Errorf("OIDC is not fully configured: missing client ID, client secret, or issuer URL")
	}

	// Try to create the OIDC provider - this performs discovery
	_, err = f.createOIDCProvider(config)
	if err != nil {
		return fmt.Errorf("failed to connect to OIDC provider: %w", err)
	}

	return nil
}

// InvalidateCache clears the cached OIDC provider, forcing re-creation on next request.
// Call this after updating the CustomerAuthConfig.
func (f *CustomerAuthProviderFactory) InvalidateCache() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cachedOIDC = nil
	f.lastConfig = nil
}

// needsRefresh checks if the OIDC provider needs to be recreated due to config changes.
// Caller must hold f.mu lock.
func (f *CustomerAuthProviderFactory) needsRefresh(config *models.CustomerAuthConfig) bool {
	if f.cachedOIDC == nil || f.lastConfig == nil {
		return true
	}

	// Check if any relevant config values have changed
	return f.lastConfig.ClientID != config.ClientID ||
		f.lastConfig.ClientSecret != config.ClientSecret ||
		f.lastConfig.IssuerURL != config.IssuerURL ||
		f.lastConfig.Scopes != config.Scopes
}

// createOIDCProvider creates a new OIDC provider from the given config.
func (f *CustomerAuthProviderFactory) createOIDCProvider(config *models.CustomerAuthConfig) (*OIDCProvider, error) {
	cfg := ProviderConfig{
		Type:         ProviderTypeOIDC,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		IssuerURL:    config.IssuerURL,
		RedirectURI:  f.baseURL + "/callback",
		Scopes:       config.GetScopes(),
	}

	return NewOIDCProvider(cfg, f.db)
}
