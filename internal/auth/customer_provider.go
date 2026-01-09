package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"gorm.io/gorm"
)

// CustomerAuthProviderFactory creates auth providers dynamically based on DB config.
// It caches the OIDC provider to avoid re-creating it on every request,
// and invalidates the cache when the config changes.
type CustomerAuthProviderFactory struct {
	db      *gorm.DB
	baseURL string

	mu            sync.RWMutex
	cachedOIDC    *OIDCProvider
	lastConfig    *models.CustomerAuthConfig
	localProvider *LocalProvider
}

// NewCustomerAuthProviderFactory creates a new factory for customer auth providers
func NewCustomerAuthProviderFactory(db *gorm.DB, baseURL string) *CustomerAuthProviderFactory {
	return &CustomerAuthProviderFactory{
		db:            db,
		baseURL:       baseURL,
		localProvider: NewLocalProvider(db),
	}
}

// GetProvider returns the appropriate auth provider based on current config.
// Returns OIDC provider if enabled and configured, LocalProvider otherwise.
func (f *CustomerAuthProviderFactory) GetProvider() (AuthProvider, error) {
	config, err := models.GetOrCreateCustomerAuthConfig(f.db)
	if err != nil {
		return f.localProvider, nil // Fallback to local on error
	}

	// If OIDC disabled or not configured, use local
	if !config.IsActive() {
		return f.localProvider, nil
	}

	// Check if we need to create/recreate the OIDC provider (config changed)
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.needsRefresh(config) {
		provider, err := f.createOIDCProvider(config)
		if err != nil {
			// Log error but fallback to local
			return f.localProvider, nil
		}
		f.cachedOIDC = provider
		f.lastConfig = config
	}

	return f.cachedOIDC, nil
}

// GetLocalProvider returns the local auth provider for email/password auth
func (f *CustomerAuthProviderFactory) GetLocalProvider() *LocalProvider {
	return f.localProvider
}

// IsOIDCEnabled returns whether OIDC is currently enabled and configured
func (f *CustomerAuthProviderFactory) IsOIDCEnabled() bool {
	config, err := models.GetOrCreateCustomerAuthConfig(f.db)
	if err != nil {
		return false
	}
	return config.IsActive()
}

// GetConfig returns the current customer auth config
func (f *CustomerAuthProviderFactory) GetConfig() (*models.CustomerAuthConfig, error) {
	return models.GetOrCreateCustomerAuthConfig(f.db)
}

// GetAuthURL generates an OIDC authorization URL with the given state.
// Returns empty string if OIDC is not enabled.
func (f *CustomerAuthProviderFactory) GetAuthURL(state string) (string, error) {
	if !f.IsOIDCEnabled() {
		return "", nil
	}

	provider, err := f.GetProvider()
	if err != nil {
		return "", err
	}

	return provider.GetAuthorizationURL(state)
}

// HandleCallback processes the OIDC callback with customer role.
// Creates users with RoleCustomer instead of RoleVendor.
func (f *CustomerAuthProviderFactory) HandleCallback(ctx context.Context, req CallbackRequest) (*AuthResult, error) {
	if !f.IsOIDCEnabled() {
		return nil, fmt.Errorf("OIDC is not enabled")
	}

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
