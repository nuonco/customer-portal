package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

type WorkspaceMemberStatus string

const (
	MemberStatusActive  WorkspaceMemberStatus = "active"
	MemberStatusPending WorkspaceMemberStatus = "pending"
)

// WorkspaceMember represents a user's membership in a workspace
type WorkspaceMember struct {
	ID          string                `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	WorkspaceID string                `gorm:"not null;uniqueIndex:idx_workspace_user" json:"workspace_id"`
	UserID      string                `gorm:"not null;uniqueIndex:idx_workspace_user" json:"user_id"`
	InvitedBy   *string               `json:"invited_by,omitempty"` // UserID of who invited (null for owner/auto-created)
	Status      WorkspaceMemberStatus `gorm:"type:varchar(20);default:'active'" json:"status"`
	InvitedAt   *time.Time            `json:"invited_at,omitempty"` // When invitation was created
	JoinedAt    *time.Time            `json:"joined_at,omitempty"`  // When user accepted invitation
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   gorm.DeletedAt        `gorm:"index" json:"-"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Inviter   *User     `gorm:"foreignKey:InvitedBy" json:"inviter,omitempty"`
}

func (wm *WorkspaceMember) BeforeCreate(tx *gorm.DB) error {
	if wm.ID == "" {
		wm.ID = shortid.NewWorkspaceMemberID()
	}
	return nil
}

// IsActive returns true if the member has accepted the invitation
func (wm *WorkspaceMember) IsActive() bool {
	return wm.Status == MemberStatusActive
}

// IsPending returns true if the member hasn't accepted yet
func (wm *WorkspaceMember) IsPending() bool {
	return wm.Status == MemberStatusPending
}

// Accept marks the invitation as accepted
func (wm *WorkspaceMember) Accept(tx *gorm.DB) error {
	now := time.Now()
	wm.Status = MemberStatusActive
	wm.JoinedAt = &now
	return tx.Save(wm).Error
}
