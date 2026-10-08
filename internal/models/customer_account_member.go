package models

import (
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"gorm.io/gorm"
)

type CustomerAccountRole string

const (
	CustomerAccountRoleOwner  CustomerAccountRole = "owner"
	CustomerAccountRoleMember CustomerAccountRole = "member"
)

// CustomerAccountMember links a user to a customer account within an org.
type CustomerAccountMember struct {
	ID        string              `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	AccountID string              `gorm:"not null;index" json:"account_id"`
	UserID    string              `gorm:"not null" json:"user_id"`
	OrgID     string              `gorm:"not null" json:"org_id"` // Denormalized for queries
	Role      CustomerAccountRole `gorm:"type:varchar(20);not null;default:'member'" json:"role"`
	JoinedAt  time.Time           `json:"joined_at"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	DeletedAt gorm.DeletedAt      `gorm:"index" json:"-"`

	// Relationships
	Account CustomerAccount `gorm:"foreignKey:AccountID" json:"account,omitempty"`
	User    User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (cam *CustomerAccountMember) BeforeCreate(tx *gorm.DB) error {
	if cam.ID == "" {
		cam.ID = shortid.NewCustomerAccountMemberID()
	}
	if cam.JoinedAt.IsZero() {
		cam.JoinedAt = time.Now()
	}
	return nil
}
