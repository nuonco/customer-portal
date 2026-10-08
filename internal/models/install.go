package models

import (
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"gorm.io/gorm"
)

type InstallVisibility string

const (
	VisibilityAccount InstallVisibility = "account"
	VisibilityPrivate InstallVisibility = "private"
)

type InstallStatus string

const (
	StatusPendingCustomer InstallStatus = "pending_customer"
	StatusPending         InstallStatus = "pending"
	StatusProvisioning    InstallStatus = "provisioning"
	StatusActive          InstallStatus = "active"
	StatusFailed          InstallStatus = "failed"
	StatusDeprovisioning  InstallStatus = "deprovisioning"
)

type Install struct {
	ID                string            `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID             string            `gorm:"index" json:"org_id"`                     // Vendor org that owns this install
	UserID            string            `gorm:"not null" json:"user_id"`                 // Current owner (vendor initially, then customer)
	CreatedByVendorID *string           `json:"created_by_vendor_id"`                    // Original vendor who created the install (nil for published-app installs)
	InstallLinkID     *string           `json:"install_link_id"`                         // Nullable: nil for published-app installs
	NuonAppID         string            `gorm:"default:''" json:"nuon_app_id,omitempty"` // Set for published-app installs; empty for install-link installs
	NuonInstallID     string            `gorm:"not null" json:"nuon_install_id"`
	Name              string            `gorm:"default:''" json:"name"`     // Human-readable install name
	AppID             string            `gorm:"default:''" json:"app_id"`   // Nuon app ID (set for imported installs)
	AppName           string            `gorm:"default:''" json:"app_name"` // App display name (set for imported installs)
	Visibility        InstallVisibility `gorm:"type:varchar(20);default:'account'" json:"visibility"`
	CustomerAccountID *string           `gorm:"index" json:"customer_account_id,omitempty"`
	Status            InstallStatus     `gorm:"type:varchar(30);default:'pending_customer'" json:"status"`
	APIDeleted        bool              `gorm:"default:false" json:"api_deleted"`
	Region            string            `json:"region,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	DeletedAt         gorm.DeletedAt    `gorm:"index" json:"-"`

	// Relationships
	Org             NuonOrg     `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	User            User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CreatedByVendor User        `gorm:"foreignKey:CreatedByVendorID" json:"created_by_vendor,omitempty"`
	InstallLink     InstallLink `gorm:"foreignKey:InstallLinkID" json:"install_link,omitempty"`
}

func (i *Install) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = shortid.NewInstallID()
	}
	return nil
}

// GetAppID returns the Nuon app ID for this install.
// For install-link installs, returns InstallLink.AppID.
// For published-app installs (InstallLinkID == nil), returns NuonAppID.
// Falls back to AppID for legacy imported installs.
func (i *Install) GetAppID() string {
	if i.InstallLinkID != nil {
		return i.InstallLink.AppID
	}
	if i.NuonAppID != "" {
		return i.NuonAppID
	}
	return i.AppID
}

// SidebarInstall holds minimal install data for the sidebar install switcher.
type SidebarInstall struct {
	ID           string
	Name         string
	AppName      string
	AppLogoLight string // base64 data URI (from PublishedApp)
	AppLogoDark  string // base64 data URI (from PublishedApp)
}

// GetNuonOrg returns the NuonOrg for this install.
// For install-link based installs, returns the org from the install link.
// For published-app installs (no install link), returns the org directly from OrgID relationship.
func (i *Install) GetNuonOrg() *NuonOrg {
	if i.InstallLinkID != nil && i.InstallLink.NuonOrg.ID != "" {
		return &i.InstallLink.NuonOrg
	}
	if i.Org.ID != "" {
		return &i.Org
	}
	return nil
}
