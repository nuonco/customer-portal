package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

type OrgMemberStatus string

const (
	MemberStatusActive  OrgMemberStatus = "active"
	MemberStatusPending OrgMemberStatus = "pending"
)

// OrgMember represents a user's membership in an organization
type OrgMember struct {
	ID        string          `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID     string          `gorm:"not null;uniqueIndex:idx_org_user" json:"org_id"`
	UserID    string          `gorm:"not null;uniqueIndex:idx_org_user" json:"user_id"`
	InvitedBy *string         `json:"invited_by,omitempty"` // UserID of who invited (null for owner/auto-created)
	Status    OrgMemberStatus `gorm:"type:varchar(20);default:'active'" json:"status"`
	InvitedAt *time.Time      `json:"invited_at,omitempty"` // When invitation was created
	JoinedAt  *time.Time      `json:"joined_at,omitempty"`  // When user accepted invitation
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relationships
	Org     NuonOrg `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Inviter *User   `gorm:"foreignKey:InvitedBy" json:"inviter,omitempty"`
}

func (om *OrgMember) BeforeCreate(tx *gorm.DB) error {
	if om.ID == "" {
		om.ID = shortid.NewOrgMemberID()
	}
	return nil
}

// IsActive returns true if the member has accepted the invitation
func (om *OrgMember) IsActive() bool {
	return om.Status == MemberStatusActive
}

// IsPending returns true if the member hasn't accepted yet
func (om *OrgMember) IsPending() bool {
	return om.Status == MemberStatusPending
}

// Accept marks the invitation as accepted
func (om *OrgMember) Accept(tx *gorm.DB) error {
	now := time.Now()
	om.Status = MemberStatusActive
	om.JoinedAt = &now
	return tx.Save(om).Error
}
