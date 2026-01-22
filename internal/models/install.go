package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
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
	ID                string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID             string         `gorm:"index" json:"org_id"`                  // Vendor org that owns this install
	UserID            string         `gorm:"not null" json:"user_id"`              // Current owner (vendor initially, then customer)
	CreatedByVendorID string         `gorm:"not null" json:"created_by_vendor_id"` // Original vendor who created the install
	InstallLinkID     string         `gorm:"not null" json:"install_link_id"`
	NuonInstallID     string         `gorm:"not null" json:"nuon_install_id"`
	Name              string         `gorm:"default:''" json:"name"` // Human-readable install name
	Status            InstallStatus  `gorm:"type:varchar(30);default:'pending_customer'" json:"status"`
	Region            string         `json:"region,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

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
