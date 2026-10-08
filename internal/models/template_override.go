package models

import (
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"gorm.io/gorm"
)

// TemplateOverride stores a custom HTML template that overrides a default customer page.
// Each org can have one override per page (e.g., login, installs, install_detail).
type TemplateOverride struct {
	ID         string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID      string    `gorm:"index:idx_template_org_page,unique" json:"org_id"`
	PageName   string    `gorm:"index:idx_template_org_page,unique" json:"page_name"` // e.g., "login", "installs"
	Content    string    `gorm:"type:text" json:"content"`                            // HTML template content
	SourcePath string    `json:"source_path"`                                         // Original file path in repo
	SourceSHA  string    `json:"source_sha"`                                          // Git commit SHA when synced
	IsEnabled  bool      `gorm:"default:true" json:"is_enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relationships
	Org NuonOrg `gorm:"foreignKey:OrgID" json:"org,omitempty"`
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

// ValidPartialNames are the customer partials that can be overridden.
var ValidPartialNames = []string{
	"header",
	"footer",
	"sidebar",
	"modal",
	"theme_styles",
	"toast",
}

// IsValidPartialName checks if a partial name is valid for override.
func IsValidPartialName(partialName string) bool {
	for _, valid := range ValidPartialNames {
		if valid == partialName {
			return true
		}
	}
	return false
}

// GetTemplateOverride returns the template override for an org and page, or nil if not found.
func GetTemplateOverride(db *gorm.DB, orgID, pageName string) (*TemplateOverride, error) {
	var override TemplateOverride
	if err := db.Where("org_id = ? AND page_name = ?", orgID, pageName).First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

// GetEnabledTemplateOverride returns the template override if it exists and is enabled.
// Returns (nil, nil) when no override is found, avoiding GORM's "record not found" log noise.
func GetEnabledTemplateOverride(db *gorm.DB, orgID, pageName string) (*TemplateOverride, error) {
	var override TemplateOverride
	if err := db.Where("org_id = ? AND page_name = ? AND is_enabled = ?", orgID, pageName, true).Limit(1).Find(&override).Error; err != nil {
		return nil, err
	}
	if override.ID == "" {
		return nil, nil
	}
	return &override, nil
}

// GetAllTemplateOverrides returns all template overrides for an org.
func GetAllTemplateOverrides(db *gorm.DB, orgID string) ([]TemplateOverride, error) {
	var overrides []TemplateOverride
	if err := db.Where("org_id = ?", orgID).Order("page_name ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}

// GetEnabledPartials returns all enabled partial templates for an org.
// Partials are templates whose page_name matches ValidPartialNames (e.g., "modal", "header").
func GetEnabledPartials(db *gorm.DB, orgID string) ([]TemplateOverride, error) {
	var partials []TemplateOverride
	if err := db.Where("org_id = ? AND is_enabled = ? AND page_name IN ?",
		orgID, true, ValidPartialNames).Find(&partials).Error; err != nil {
		return nil, err
	}
	return partials, nil
}
