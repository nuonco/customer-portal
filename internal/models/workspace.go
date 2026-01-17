package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// Workspace represents a collaboration namespace for vendor users.
// Each workspace is bound to exactly one Nuon organization (one-to-one relationship).
type Workspace struct {
	ID         string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	Name       string         `gorm:"not null;type:varchar(255)" json:"name"`
	Subdomain  string         `gorm:"type:varchar(63);uniqueIndex:idx_unique_subdomain,where:deleted_at IS NULL" json:"subdomain"`
	IsPersonal bool           `gorm:"default:false" json:"is_personal"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Members         []WorkspaceMember     `gorm:"foreignKey:WorkspaceID" json:"members,omitempty"`
	NuonOrg         *NuonOrg              `gorm:"foreignKey:WorkspaceID" json:"nuon_org,omitempty"` // One-to-one: exactly one org per workspace
	InstallLinks    []InstallLink         `gorm:"foreignKey:WorkspaceID" json:"install_links,omitempty"`
	AppInputConfigs []AppInputConfig      `gorm:"foreignKey:WorkspaceID" json:"app_input_configs,omitempty"`
	Installs        []Install             `gorm:"foreignKey:WorkspaceID" json:"installs,omitempty"`
	Theme           *AppTheme             `gorm:"foreignKey:WorkspaceID" json:"theme,omitempty"`
	AuthConfig      *CustomerAuthConfig   `gorm:"foreignKey:WorkspaceID" json:"auth_config,omitempty"`
	Invitations     []WorkspaceInvitation `gorm:"foreignKey:WorkspaceID" json:"invitations,omitempty"`
}

func (w *Workspace) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = shortid.NewWorkspaceID()
	}
	return nil
}

// IsVendorWorkspace checks if this workspace has any vendor members
func (w *Workspace) IsVendorWorkspace(db *gorm.DB) (bool, error) {
	var count int64
	err := db.Model(&WorkspaceMember{}).
		Joins("INNER JOIN users ON users.id = workspace_members.user_id").
		Where("workspace_members.workspace_id = ? AND users.role = ?", w.ID, RoleVendor).
		Count(&count).Error
	return count > 0, err
}

// HasOrg returns true if this workspace has a connected Nuon organization
func (w *Workspace) HasOrg() bool {
	return w.NuonOrg != nil
}

// GetOrgID returns the connected org's ID, or empty string if none
func (w *Workspace) GetOrgID() string {
	if w.NuonOrg != nil {
		return w.NuonOrg.ID
	}
	return ""
}

// GetNuonOrgID returns the Nuon platform org ID, or empty string if none
func (w *Workspace) GetNuonOrgID() string {
	if w.NuonOrg != nil {
		return w.NuonOrg.NuonOrgID
	}
	return ""
}
