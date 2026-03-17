package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// NuonOrg represents a connected Nuon organization.
// This is the primary entity for multi-tenant vendor access control.
// Theme settings are stored in AppTheme (one per org).
// API URL defaults to the global env var (NUON_API_URL) but can be overridden per-org.
//
// Each NuonOrgID (the Nuon platform org ID) can only be connected once (unique constraint).
type NuonOrg struct {
	ID        string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	UserID    string         `gorm:"not null" json:"user_id"`              // Audit trail: who created it
	NuonOrgID string         `gorm:"column:org_id;not null" json:"org_id"` // Nuon API org ID (unique constraint via migration)
	APIToken  string         `gorm:"not null" json:"-"`                    // Hidden from JSON
	APIURL    string         `gorm:"column:api_url" json:"api_url"`        // Optional per-org API URL override
	Name      string         `gorm:"not null" json:"name"`
	Subdomain string         `gorm:"type:varchar(63);uniqueIndex:idx_unique_subdomain,where:deleted_at IS NULL" json:"subdomain"` // Customer portal subdomain
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User            User                `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Members         []OrgMember         `gorm:"foreignKey:OrgID" json:"members,omitempty"`
	Invitations     []OrgInvitation     `gorm:"foreignKey:OrgID" json:"invitations,omitempty"`
	InstallLinks    []InstallLink       `gorm:"foreignKey:OrgID" json:"install_links,omitempty"`
	AppInputConfigs []AppInputConfig    `gorm:"foreignKey:OrgID" json:"app_input_configs,omitempty"`
	Installs        []Install           `gorm:"foreignKey:OrgID" json:"installs,omitempty"`
	Theme           *AppTheme           `gorm:"foreignKey:OrgID" json:"theme,omitempty"`
	AuthConfig      *CustomerAuthConfig `gorm:"foreignKey:OrgID" json:"auth_config,omitempty"`
}

func (n *NuonOrg) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = shortid.NewNuonOrgID()
	}
	return nil
}

// HasMembers returns true if members have been loaded
func (n *NuonOrg) HasMembers() bool {
	return len(n.Members) > 0
}
