package models

import (
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"gorm.io/gorm"
)

// CustomerAccountInvite represents an email-based invite for joining a customer account.
// When a user with a matching email logs in, they are automatically added as a member.
type CustomerAccountInvite struct {
	ID              string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	AccountID       string         `gorm:"not null;index" json:"account_id"`
	OrgID           string         `gorm:"not null;index" json:"org_id"`
	Email           string         `gorm:"not null;index" json:"email"`
	CreatedByUserID string         `gorm:"not null" json:"created_by_user_id"`
	UsedByUserID    *string        `json:"used_by_user_id,omitempty"`
	UsedAt          *time.Time     `json:"used_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Account   CustomerAccount `gorm:"foreignKey:AccountID" json:"account,omitempty"`
	CreatedBy User            `gorm:"foreignKey:CreatedByUserID" json:"created_by,omitempty"`
	UsedBy    User            `gorm:"foreignKey:UsedByUserID" json:"used_by,omitempty"`
}

func (cai *CustomerAccountInvite) BeforeCreate(tx *gorm.DB) error {
	if cai.ID == "" {
		cai.ID = shortid.NewCustomerAccountInviteID()
	}
	return nil
}

// IsUsed returns true if the invite has been used.
func (cai *CustomerAccountInvite) IsUsed() bool {
	return cai.UsedByUserID != nil
}
