package models

import (
	"strings"
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// CustomerAuthConfig stores customer authentication settings for an org.
// Each org can configure its own OIDC provider for customer authentication,
// or fall back to email/password authentication if not configured.
type CustomerAuthConfig struct {
	ID           string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID        string    `gorm:"uniqueIndex" json:"org_id"` // One config per org
	Enabled      bool      `gorm:"default:false" json:"enabled"`
	ProviderName string    `gorm:"type:varchar(100)" json:"provider_name"` // e.g., "Google", "Okta", "Auth0"
	ClientID     string    `gorm:"type:varchar(255)" json:"client_id"`
	ClientSecret string    `gorm:"type:varchar(500)" json:"-"` // Never expose in JSON
	IssuerURL    string    `gorm:"type:varchar(500)" json:"issuer_url"`
	Scopes       string    `gorm:"type:varchar(255)" json:"scopes"` // Comma-separated, defaults to "openid,profile,email"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships
	Org NuonOrg `gorm:"foreignKey:OrgID" json:"org,omitempty"`
}

// DefaultScopes are the OIDC scopes used when none are specified
const DefaultScopes = "openid,profile,email"

func (c *CustomerAuthConfig) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = shortid.NewCustomerAuthConfigID()
	}
	return nil
}

// GetOrCreateCustomerAuthConfig returns the CustomerAuthConfig record for an org,
// creating it with defaults if it doesn't exist.
// If orgID is empty, returns a default config without saving to database.
func GetOrCreateCustomerAuthConfig(db *gorm.DB, orgID string) (*CustomerAuthConfig, error) {
	// If no org ID provided, return default config
	if orgID == "" {
		return &CustomerAuthConfig{
			Enabled: false,
			Scopes:  DefaultScopes,
		}, nil
	}

	var config CustomerAuthConfig

	// Try to get the existing config for this org
	if err := db.Where("org_id = ?", orgID).First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create default config for this org
			config = CustomerAuthConfig{
				OrgID:   orgID,
				Enabled: false,
				Scopes:  DefaultScopes,
			}
			if err := db.Create(&config).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &config, nil
}

// IsConfigured returns true if OIDC settings are fully configured
func (c *CustomerAuthConfig) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.IssuerURL != ""
}

// IsActive returns true if OIDC is both configured AND enabled
func (c *CustomerAuthConfig) IsActive() bool {
	return c.Enabled && c.IsConfigured()
}

// GetScopes returns scopes as a slice
func (c *CustomerAuthConfig) GetScopes() []string {
	scopes := c.Scopes
	if scopes == "" {
		scopes = DefaultScopes
	}

	parts := strings.Split(scopes, ",")
	result := make([]string, 0, len(parts))
	for _, s := range parts {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// HasClientSecret returns true if a client secret is set (for UI display)
func (c *CustomerAuthConfig) HasClientSecret() bool {
	return c.ClientSecret != ""
}
