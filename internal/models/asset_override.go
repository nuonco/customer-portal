package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// AssetOverride stores a custom CSS file or image asset for a workspace.
// These assets are synced from the vendor's GitHub repository.
type AssetOverride struct {
	ID          string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	WorkspaceID string    `gorm:"index:idx_asset_workspace_path,unique" json:"workspace_id"`
	AssetType   string    `json:"asset_type"`                                              // "css", "image"
	AssetPath   string    `gorm:"index:idx_asset_workspace_path,unique" json:"asset_path"` // e.g., "custom.css", "images/logo.png"
	Content     []byte    `gorm:"type:bytea" json:"-"`                                     // File content (not exposed in JSON)
	MimeType    string    `json:"mime_type"`
	FileSize    int64     `json:"file_size"`
	SourcePath  string    `json:"source_path"` // Original file path in repo
	SourceSHA   string    `json:"source_sha"`  // Git commit SHA when synced
	IsEnabled   bool      `gorm:"default:true" json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}

func (a *AssetOverride) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = shortid.NewAssetOverrideID()
	}
	return nil
}

// Asset type constants
const (
	AssetTypeCSS   = "css"
	AssetTypeImage = "image"
)

// Allowed MIME types for asset overrides
var AllowedAssetMimeTypes = map[string]string{
	".css":  "text/css",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".svg":  "image/svg+xml",
	".gif":  "image/gif",
	".ico":  "image/x-icon",
	".webp": "image/webp",
}

// GetMimeTypeForExtension returns the MIME type for a file extension.
func GetMimeTypeForExtension(ext string) (string, bool) {
	mimeType, ok := AllowedAssetMimeTypes[ext]
	return mimeType, ok
}

// IsAllowedAssetExtension checks if a file extension is allowed for assets.
func IsAllowedAssetExtension(ext string) bool {
	_, ok := AllowedAssetMimeTypes[ext]
	return ok
}

// GetAssetOverride returns the asset override for a workspace and path, or nil if not found.
func GetAssetOverride(db *gorm.DB, workspaceID, assetPath string) (*AssetOverride, error) {
	var asset AssetOverride
	if err := db.Where("workspace_id = ? AND asset_path = ?", workspaceID, assetPath).First(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

// GetEnabledAssetOverride returns the asset override if it exists and is enabled.
func GetEnabledAssetOverride(db *gorm.DB, workspaceID, assetPath string) (*AssetOverride, error) {
	var asset AssetOverride
	if err := db.Where("workspace_id = ? AND asset_path = ? AND is_enabled = ?", workspaceID, assetPath, true).First(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

// GetAllAssetOverrides returns all asset overrides for a workspace.
func GetAllAssetOverrides(db *gorm.DB, workspaceID string) ([]AssetOverride, error) {
	var assets []AssetOverride
	if err := db.Where("workspace_id = ?", workspaceID).Order("asset_path ASC").Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}

// GetCSSOverrides returns all CSS asset overrides for a workspace.
func GetCSSOverrides(db *gorm.DB, workspaceID string) ([]AssetOverride, error) {
	var assets []AssetOverride
	if err := db.Where("workspace_id = ? AND asset_type = ? AND is_enabled = ?", workspaceID, AssetTypeCSS, true).Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}
