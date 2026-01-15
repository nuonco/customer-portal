package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// Workspace represents a collaboration namespace for vendor users
type Workspace struct {
	ID         string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	Name       string         `gorm:"not null;type:varchar(255)" json:"name"`
	IsPersonal bool           `gorm:"default:false" json:"is_personal"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Members         []WorkspaceMember     `gorm:"foreignKey:WorkspaceID" json:"members,omitempty"`
	NuonOrgs        []NuonOrg             `gorm:"foreignKey:WorkspaceID" json:"nuon_orgs,omitempty"`
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
