package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// WorkspaceInvitation represents an invitation link for joining a workspace
type WorkspaceInvitation struct {
	ID          string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	WorkspaceID string         `gorm:"not null;index" json:"workspace_id"`
	Email       string         `gorm:"type:varchar(255)" json:"email"`    // Optional - can be empty for link-based invites
	InvitedBy   string         `gorm:"not null" json:"invited_by"`        // UserID
	Token       string         `gorm:"uniqueIndex;not null" json:"token"` // Secure random token for invitation link
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`              // Optional expiration
	AcceptedAt  *time.Time     `json:"accepted_at,omitempty"`             // When user accepted
	UsedCount   int            `gorm:"default:0" json:"used_count"`       // Track how many times link was used
	MaxUses     int            `gorm:"default:0" json:"max_uses"`         // 0 = unlimited
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	Inviter   User      `gorm:"foreignKey:InvitedBy" json:"inviter,omitempty"`
}

func (wi *WorkspaceInvitation) BeforeCreate(tx *gorm.DB) error {
	if wi.ID == "" {
		wi.ID = shortid.NewWorkspaceInvitationID()
	}
	if wi.Token == "" {
		// Generate secure random token (32 bytes = 64 hex chars)
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return err
		}
		wi.Token = hex.EncodeToString(bytes)
	}
	return nil
}

// IsExpired returns true if the invitation has expired
func (wi *WorkspaceInvitation) IsExpired() bool {
	if wi.ExpiresAt == nil {
		return false // No expiration set
	}
	return time.Now().After(*wi.ExpiresAt)
}

// IsMaxUsesReached returns true if the invitation has reached its maximum uses
func (wi *WorkspaceInvitation) IsMaxUsesReached() bool {
	if wi.MaxUses == 0 {
		return false // Unlimited uses
	}
	return wi.UsedCount >= wi.MaxUses
}

// IsValid returns true if the invitation can still be used
func (wi *WorkspaceInvitation) IsValid() bool {
	return !wi.IsExpired() && !wi.IsMaxUsesReached()
}

// GetInvitationURL returns the full invitation URL
func (wi *WorkspaceInvitation) GetInvitationURL(baseURL string) string {
	return baseURL + "/invite/" + wi.Token
}

// IncrementUsage increments the used count and optionally marks as accepted
func (wi *WorkspaceInvitation) IncrementUsage(tx *gorm.DB) error {
	wi.UsedCount++
	if wi.AcceptedAt == nil {
		now := time.Now()
		wi.AcceptedAt = &now
	}
	return tx.Save(wi).Error
}
