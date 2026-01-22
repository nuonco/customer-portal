package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// AppTheme stores theme settings for an org's customer-facing installer app.
// Each org has its own theme configuration.
type AppTheme struct {
	ID                        string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID                     string    `gorm:"uniqueIndex" json:"org_id"` // One theme per org
	PrimaryColor              string    `json:"primary_color"`             // Hex color for navigation and primary buttons
	SecondaryColor            string    `json:"secondary_color"`           // Hex color for links and accents
	LogoLightBase64           string    `json:"logo_light_base64,omitempty"`
	LogoDarkBase64            string    `json:"logo_dark_base64,omitempty"`
	SupportContact            string    `json:"support_contact"`
	HeadingFont               string    `json:"heading_font"`
	BodyFont                  string    `json:"body_font"`
	HeadingFontBase64         string    `json:"heading_font_base64"`
	BodyFontBase64            string    `json:"body_font_base64"`
	BorderRadius              string    `json:"border_radius"`
	SpacingDensity            string    `json:"spacing_density"`
	LoginTitle                string    `json:"login_title"`
	LoginSubtitle             string    `json:"login_subtitle"`
	LoginRightSideImageBase64 string    `json:"login_right_side_image_base64"`
	LoginRightSideGradient    string    `json:"login_right_side_gradient"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`

	// Relationships
	Org NuonOrg `gorm:"foreignKey:OrgID" json:"org,omitempty"`
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

// GetLoginRightSideImage returns the customer login page right side image (base64 data URI)
func (t *AppTheme) GetLoginRightSideImage() string {
	return t.LoginRightSideImageBase64
}

// GetLoginRightSideGradient returns the customer login page right side gradient CSS
func (t *AppTheme) GetLoginRightSideGradient() string {
	return t.LoginRightSideGradient
}

// GetLogoForMode returns the appropriate logo based on the color scheme.
// If dark mode is requested and no dark logo is set, falls back to light logo.
func (t *AppTheme) GetLogoForMode(isDark bool) string {
	if isDark && t.LogoDarkBase64 != "" {
		return t.LogoDarkBase64
	}
	return t.LogoLightBase64
}

// LogoBase64 provides backward compatibility for any code still referencing the old field name.
// Returns the light mode logo.
func (t *AppTheme) LogoBase64() string {
	return t.LogoLightBase64
}

// GetOrCreateAppTheme returns the AppTheme record for an org, creating it with defaults if it doesn't exist.
// If orgID is empty, returns a default theme without saving to database (for login/register pages).
func GetOrCreateAppTheme(db *gorm.DB, orgID string) (*AppTheme, error) {
	// If no org ID provided (pre-login pages), return default theme
	if orgID == "" {
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

	// Try to get the existing theme for this org
	if err := db.Where("org_id = ?", orgID).First(&theme).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create default theme for this org
			theme = AppTheme{
				OrgID:          orgID,
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
