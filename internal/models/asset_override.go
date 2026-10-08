package models

import (
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"gorm.io/gorm"
)

// AssetOverride stores a custom CSS file or image asset for an org.
// These assets are synced from the vendor's GitHub repository.
type AssetOverride struct {
	ID         string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID      string    `gorm:"index:idx_asset_org_path,unique" json:"org_id"`
	AssetType  string    `json:"asset_type"`                                        // "css", "image"
	AssetPath  string    `gorm:"index:idx_asset_org_path,unique" json:"asset_path"` // e.g., "custom.css", "images/logo.png"
	Content    []byte    `gorm:"type:bytea" json:"-"`                               // File content (not exposed in JSON)
	MimeType   string    `json:"mime_type"`
	FileSize   int64     `json:"file_size"`
	SourcePath string    `json:"source_path"` // Original file path in repo
	SourceSHA  string    `json:"source_sha"`  // Git commit SHA when synced
	IsEnabled  bool      `gorm:"default:true" json:"is_enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relationships
	Org NuonOrg `gorm:"foreignKey:OrgID" json:"org,omitempty"`
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

// GetAssetOverride returns the asset override for an org and path, or nil if not found.
func GetAssetOverride(db *gorm.DB, orgID, assetPath string) (*AssetOverride, error) {
	var asset AssetOverride
	if err := db.Where("org_id = ? AND asset_path = ?", orgID, assetPath).First(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

// GetEnabledAssetOverride returns the asset override if it exists and is enabled.
func GetEnabledAssetOverride(db *gorm.DB, orgID, assetPath string) (*AssetOverride, error) {
	var asset AssetOverride
	if err := db.Where("org_id = ? AND asset_path = ? AND is_enabled = ?", orgID, assetPath, true).First(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

// GetAllAssetOverrides returns all asset overrides for an org.
func GetAllAssetOverrides(db *gorm.DB, orgID string) ([]AssetOverride, error) {
	var assets []AssetOverride
	if err := db.Where("org_id = ?", orgID).Order("asset_path ASC").Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}

// GetCSSOverrides returns all CSS asset overrides for an org.
func GetCSSOverrides(db *gorm.DB, orgID string) ([]AssetOverride, error) {
	var assets []AssetOverride
	if err := db.Where("org_id = ? AND asset_type = ? AND is_enabled = ?", orgID, AssetTypeCSS, true).Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}
