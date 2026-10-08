package models

import (
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserRole string

const (
	RoleVendor   UserRole = "vendor"
	RoleCustomer UserRole = "customer"
)

type User struct {
	ID           string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	Name         string         `gorm:"type:varchar(255)" json:"name"`
	Email        string         `gorm:"uniqueIndex" json:"email"`
	PasswordHash string         `gorm:"type:varchar(255)" json:"-"`
	Role         UserRole       `gorm:"type:varchar(20)" json:"role"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

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

// SetPassword hashes the password and stores it
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword verifies the password against the stored hash
func (u *User) CheckPassword(password string) bool {
	if u.PasswordHash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// HasPassword returns true if the user has a password set
func (u *User) HasPassword() bool {
	return u.PasswordHash != ""
}
