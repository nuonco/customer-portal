package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// TemplateOverride stores a custom HTML template that overrides a default customer page.
// Each workspace can have one override per page (e.g., login, installs, install_detail).
type TemplateOverride struct {
	ID          string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	WorkspaceID string    `gorm:"index:idx_template_workspace_page,unique" json:"workspace_id"`
	PageName    string    `gorm:"index:idx_template_workspace_page,unique" json:"page_name"` // e.g., "login", "installs"
	Content     string    `gorm:"type:text" json:"content"`                                  // HTML template content
	SourcePath  string    `json:"source_path"`                                               // Original file path in repo
	SourceSHA   string    `json:"source_sha"`                                                // Git commit SHA when synced
	IsEnabled   bool      `gorm:"default:true" json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}

func (t *TemplateOverride) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = shortid.NewTemplateOverrideID()
	}
	return nil
}

// ValidPageNames are the customer pages that can be overridden.
var ValidPageNames = []string{
	"login",
	"register",
	"installs",
	"install_detail",
	"install_link",
	"workflows",
	"error",
}

// IsValidPageName checks if a page name is valid for override.
func IsValidPageName(pageName string) bool {
	for _, valid := range ValidPageNames {
		if valid == pageName {
			return true
		}
	}
	return false
}

// GetTemplateOverride returns the template override for a workspace and page, or nil if not found.
func GetTemplateOverride(db *gorm.DB, workspaceID, pageName string) (*TemplateOverride, error) {
	var override TemplateOverride
	if err := db.Where("workspace_id = ? AND page_name = ?", workspaceID, pageName).First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

// GetEnabledTemplateOverride returns the template override if it exists and is enabled.
func GetEnabledTemplateOverride(db *gorm.DB, workspaceID, pageName string) (*TemplateOverride, error) {
	var override TemplateOverride
	if err := db.Where("workspace_id = ? AND page_name = ? AND is_enabled = ?", workspaceID, pageName, true).First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

// GetAllTemplateOverrides returns all template overrides for a workspace.
func GetAllTemplateOverrides(db *gorm.DB, workspaceID string) ([]TemplateOverride, error) {
	var overrides []TemplateOverride
	if err := db.Where("workspace_id = ?", workspaceID).Order("page_name ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}
