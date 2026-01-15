package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// AppTheme stores theme settings for a workspace's customer-facing installer app.
// Each workspace has its own theme configuration.
type AppTheme struct {
	ID                string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	WorkspaceID       string    `gorm:"uniqueIndex" json:"workspace_id"` // NEW: One theme per workspace (nullable for migration)
	PrimaryColor      string    `json:"primary_color"`                   // Hex color for navigation and primary buttons
	SecondaryColor    string    `json:"secondary_color"`                 // Hex color for links and accents
	LogoBase64        string    `json:"logo_base64,omitempty"`           // Base64-encoded logo image (data URI format)
	SupportContact    string    `json:"support_contact"`                 // Email or URL for customer support
	HeadingFont       string    `json:"heading_font"`                    // Google Font name (or display name if custom)
	BodyFont          string    `json:"body_font"`                       // Google Font name (or display name if custom)
	HeadingFontBase64 string    `json:"heading_font_base64"`             // Custom font data URI (empty = use Google Font)
	BodyFontBase64    string    `json:"body_font_base64"`                // Custom font data URI (empty = use Google Font)
	BorderRadius      string    `json:"border_radius"`                   // Customer page corner style: sharp, subtle, rounded, very-rounded
	SpacingDensity    string    `json:"spacing_density"`                 // Customer page spacing: compact, comfortable, spacious
	LoginTitle        string    `json:"login_title"`                     // Customer login page title (default: "Customer Dashboard")
	LoginSubtitle     string    `json:"login_subtitle"`                  // Customer login page subtitle (default: "Manage your customer's install experience.")
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"` // NEW
}

func (t *AppTheme) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = shortid.NewThemeID()
	}
	return nil
}

// DefaultPrimaryColor is the default blue color (Tailwind blue-600)
const DefaultPrimaryColor = "#2563EB"

// Default customer page styling
const (
	DefaultBorderRadius   = "rounded"     // rounded corners (8px)
	DefaultSpacingDensity = "comfortable" // balanced spacing
)

// Default customer login page text
const (
	DefaultLoginTitle    = "Customer Dashboard"
	DefaultLoginSubtitle = "Manage your customer's install experience."
)

// ValidBorderRadiusValues are the allowed values for BorderRadius
var ValidBorderRadiusValues = []string{"sharp", "subtle", "rounded", "very-rounded"}

// ValidSpacingDensityValues are the allowed values for SpacingDensity
var ValidSpacingDensityValues = []string{"compact", "comfortable", "spacious"}

// IsValidBorderRadius checks if a value is a valid border radius option
func IsValidBorderRadius(value string) bool {
	for _, v := range ValidBorderRadiusValues {
		if v == value {
			return true
		}
	}
	return false
}

// IsValidSpacingDensity checks if a value is a valid spacing density option
func IsValidSpacingDensity(value string) bool {
	for _, v := range ValidSpacingDensityValues {
		if v == value {
			return true
		}
	}
	return false
}

// GetRadiusClass returns the CSS class name for the border radius setting
func (t *AppTheme) GetRadiusClass() string {
	if t.BorderRadius == "" {
		return "radius-" + DefaultBorderRadius
	}
	return "radius-" + t.BorderRadius
}

// GetDensityClass returns the CSS class name for the spacing density setting
func (t *AppTheme) GetDensityClass() string {
	if t.SpacingDensity == "" {
		return "density-" + DefaultSpacingDensity
	}
	return "density-" + t.SpacingDensity
}

// GetLoginTitle returns the customer login page title, or the default if not set
func (t *AppTheme) GetLoginTitle() string {
	if t.LoginTitle == "" {
		return DefaultLoginTitle
	}
	return t.LoginTitle
}

// GetLoginSubtitle returns the customer login page subtitle, or the default if not set
func (t *AppTheme) GetLoginSubtitle() string {
	if t.LoginSubtitle == "" {
		return DefaultLoginSubtitle
	}
	return t.LoginSubtitle
}

// GetOrCreateAppTheme returns the AppTheme record for a workspace, creating it with defaults if it doesn't exist.
// If workspaceID is empty, returns a default theme without saving to database (for login/register pages).
func GetOrCreateAppTheme(db *gorm.DB, workspaceID string) (*AppTheme, error) {
	// If no workspace ID provided (pre-login pages), return default theme
	if workspaceID == "" {
		return &AppTheme{
			PrimaryColor:   DefaultPrimaryColor,
			SecondaryColor: DefaultPrimaryColor,
			BorderRadius:   DefaultBorderRadius,
			SpacingDensity: DefaultSpacingDensity,
			LoginTitle:     DefaultLoginTitle,
			LoginSubtitle:  DefaultLoginSubtitle,
		}, nil
	}

	var theme AppTheme

	// Try to get the existing theme for this workspace
	if err := db.Where("workspace_id = ?", workspaceID).First(&theme).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create default theme for this workspace
			theme = AppTheme{
				WorkspaceID:    workspaceID,
				PrimaryColor:   DefaultPrimaryColor,
				SecondaryColor: DefaultPrimaryColor,
				BorderRadius:   DefaultBorderRadius,
				SpacingDensity: DefaultSpacingDensity,
			}
			if err := db.Create(&theme).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &theme, nil
}
