package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// NuonOrg represents a connected Nuon organization.
// Theme settings have been moved to the global AppTheme model.
// API URL is now a global env var (NUON_API_URL), not stored per-org.
type NuonOrg struct {
	ID          string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	WorkspaceID string         `gorm:"index" json:"workspace_id"`            // NEW: Workspace ownership (nullable for migration)
	UserID      string         `gorm:"not null" json:"user_id"`              // Kept for audit trail (who created it)
	NuonOrgID   string         `gorm:"column:org_id;not null" json:"org_id"` // Nuon API org ID (renamed from OrgID to avoid GORM FK confusion)
	APIToken    string         `gorm:"not null" json:"-"`                    // Hidden from JSON
	Name        string         `gorm:"not null" json:"name"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"` // NEW
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	// NOTE: InstallLinks relationship removed to avoid GORM FK collision with NuonOrgID column
	// Query install_links separately using: db.Where("org_id = ?", nuonOrg.ID).Find(&installLinks)
}

func (n *NuonOrg) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = shortid.NewNuonOrgID()
	}
	return nil
}
