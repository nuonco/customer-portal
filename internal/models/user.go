package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

type UserRole string

const (
	RoleVendor   UserRole = "vendor"
	RoleCustomer UserRole = "customer"
)

type User struct {
	ID        string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	Name      string         `gorm:"type:varchar(255)" json:"name"`
	Email     string         `gorm:"uniqueIndex" json:"email"`
	Role      UserRole       `gorm:"type:varchar(20)" json:"role"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	NuonOrgs     []NuonOrg     `gorm:"foreignKey:UserID" json:"nuon_orgs,omitempty"`
	InstallLinks []InstallLink `gorm:"foreignKey:UserID" json:"install_links,omitempty"`
	Installs     []Install     `gorm:"foreignKey:UserID" json:"installs,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = shortid.NewUserID()
	}
	return nil
}

func (u *User) IsVendor() bool {
	return u.Role == RoleVendor
}

func (u *User) IsCustomer() bool {
	return u.Role == RoleCustomer
}
