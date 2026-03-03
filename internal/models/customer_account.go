package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// CustomerAccount represents a company account that groups customers together within a vendor org.
type CustomerAccount struct {
	ID              string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID           string         `gorm:"not null;index" json:"org_id"`
	Name            string         `gorm:"not null" json:"name"`
	CreatedByUserID string         `gorm:"not null" json:"created_by_user_id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Org       NuonOrg `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	CreatedBy User    `gorm:"foreignKey:CreatedByUserID" json:"created_by,omitempty"`
}

func (ca *CustomerAccount) BeforeCreate(tx *gorm.DB) error {
	if ca.ID == "" {
		ca.ID = shortid.NewCustomerAccountID()
	}
	return nil
}
