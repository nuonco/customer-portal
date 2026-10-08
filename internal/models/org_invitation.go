package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"gorm.io/gorm"
)

// OrgInvitation represents an invitation link for joining an organization
type OrgInvitation struct {
	ID         string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID      string         `gorm:"not null;index" json:"org_id"`
	Email      string         `gorm:"type:varchar(255)" json:"email"`    // Optional - can be empty for link-based invites
	InvitedBy  string         `gorm:"not null" json:"invited_by"`        // UserID
	Token      string         `gorm:"uniqueIndex;not null" json:"token"` // Secure random token for invitation link
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`              // Optional expiration
	AcceptedAt *time.Time     `json:"accepted_at,omitempty"`             // When user accepted
	UsedCount  int            `gorm:"default:0" json:"used_count"`       // Track how many times link was used
	MaxUses    int            `gorm:"default:0" json:"max_uses"`         // 0 = unlimited
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Org     NuonOrg `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	Inviter User    `gorm:"foreignKey:InvitedBy" json:"inviter,omitempty"`
}

func (oi *OrgInvitation) BeforeCreate(tx *gorm.DB) error {
	if oi.ID == "" {
		oi.ID = shortid.NewOrgInvitationID()
	}
	if oi.Token == "" {
		// Generate secure random token (32 bytes = 64 hex chars)
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return err
		}
		oi.Token = hex.EncodeToString(bytes)
	}
	return nil
}

// IsExpired returns true if the invitation has expired
func (oi *OrgInvitation) IsExpired() bool {
	if oi.ExpiresAt == nil {
		return false // No expiration set
	}
	return time.Now().After(*oi.ExpiresAt)
}

// IsMaxUsesReached returns true if the invitation has reached its maximum uses
func (oi *OrgInvitation) IsMaxUsesReached() bool {
	if oi.MaxUses == 0 {
		return false // Unlimited uses
	}
	return oi.UsedCount >= oi.MaxUses
}

// IsValid returns true if the invitation can still be used
func (oi *OrgInvitation) IsValid() bool {
	return !oi.IsExpired() && !oi.IsMaxUsesReached()
}

// GetInvitationURL returns the full invitation URL
func (oi *OrgInvitation) GetInvitationURL(baseURL string) string {
	return baseURL + "/invite/" + oi.Token
}

// IncrementUsage increments the used count and optionally marks as accepted
func (oi *OrgInvitation) IncrementUsage(tx *gorm.DB) error {
	oi.UsedCount++
	if oi.AcceptedAt == nil {
		now := time.Now()
		oi.AcceptedAt = &now
	}
	return tx.Save(oi).Error
}
