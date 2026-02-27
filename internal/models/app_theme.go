package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// AppTheme stores theme settings for an org's customer-facing installer app.
// Each org has its own theme configuration.
type AppTheme struct {
	ID                            string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID                         string    `gorm:"uniqueIndex" json:"org_id"` // One theme per org
	PrimaryColor                  string    `json:"primary_color"`             // Hex color for navigation and primary buttons (light mode)
	SecondaryColor                string    `json:"secondary_color"`           // Hex color for links and accents (light mode)
	PrimaryColorDark              string    `json:"primary_color_dark"`        // Hex color for navigation and primary buttons (dark mode)
	SecondaryColorDark            string    `json:"secondary_color_dark"`      // Hex color for links and accents (dark mode)
	LogoLightBase64               string    `json:"logo_light_base64,omitempty"`
	LogoDarkBase64                string    `json:"logo_dark_base64,omitempty"`
	FaviconBase64                 string    `json:"favicon_base64,omitempty"`
	SupportContact                string    `json:"support_contact"`
	HeadingFont                   string    `json:"heading_font"`
	BodyFont                      string    `json:"body_font"`
	HeadingFontBase64             string    `json:"heading_font_base64"`
	BodyFontBase64                string    `json:"body_font_base64"`
	HeadingFontName               string    `json:"heading_font_name"` // Original filename of uploaded heading font
	BodyFontName                  string    `json:"body_font_name"`    // Original filename of uploaded body font
	WhiteColor                    string    `json:"white_color"`       // Background color for light mode (pages, panels, cards)
	BlackColor                    string    `json:"black_color"`       // Background color for dark mode (pages, panels, cards)
	WhiteColorDark                string    `json:"white_color_dark"`  // Optional: --theme-white override for dark mode
	BlackColorLight               string    `json:"black_color_light"` // Optional: --theme-black override for light mode
	BorderRadius                  string    `json:"border_radius"`
	ThemeMode                     string    `json:"theme_mode"` // "auto" (default), "light", "dark"
	LoginTitle                    string    `json:"login_title"`
	LoginSubtitle                 string    `json:"login_subtitle"`
	LoginRightSideImageBase64     string    `json:"login_right_side_image_base64"`
	LoginRightSideGradient        string    `json:"login_right_side_gradient"`
	LoginRightSideImageBase64Dark string    `json:"login_right_side_image_base64_dark"`
	LoginRightSideGradientDark    string    `json:"login_right_side_gradient_dark"`
	CustomCSS                     string    `json:"custom_css"`          // Vendor-injected CSS for the customer portal
	HeaderTitle                   string    `json:"header_title"`        // Custom header title text (empty = use default)
	HeaderTitleHidden             bool      `json:"header_title_hidden"` // Hide the header title entirely
	CreatedAt                     time.Time `json:"created_at"`
	UpdatedAt                     time.Time `json:"updated_at"`

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
	DefaultBorderRadius = "rounded" // rounded corners (8px)
)

// Default customer login page text
const (
	DefaultLoginTitle    = "Customer Portal"
	DefaultLoginSubtitle = "Manage your installs."
)

// DefaultHeaderTitleSuffix is appended to the org name for the default header title
const DefaultHeaderTitleSuffix = "BYOC"

// ValidBorderRadiusValues are the allowed values for BorderRadius
var ValidBorderRadiusValues = []string{"sharp", "subtle", "rounded", "very-rounded"}

// ValidThemeModeValues are the allowed values for ThemeMode
var ValidThemeModeValues = []string{"auto", "light", "dark"}

// IsValidThemeMode checks if a value is a valid theme mode option
func IsValidThemeMode(value string) bool {
	for _, v := range ValidThemeModeValues {
		if v == value {
			return true
		}
	}
	return false
}

// GetThemeMode returns the theme mode, defaulting to "auto" if empty.
func (t *AppTheme) GetThemeMode() string {
	if t.ThemeMode == "" {
		return "auto"
	}
	return t.ThemeMode
}

// IsValidBorderRadius checks if a value is a valid border radius option
func IsValidBorderRadius(value string) bool {
	for _, v := range ValidBorderRadiusValues {
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

// GetHeaderTitle returns the header title for the customer portal.
// If a custom title is set, returns that. Otherwise returns "<orgName> BYOC".
// If orgName is empty, returns just "BYOC".
func (t *AppTheme) GetHeaderTitle(orgName string) string {
	if t.HeaderTitle != "" {
		return t.HeaderTitle
	}
	if orgName != "" {
		return orgName + " " + DefaultHeaderTitleSuffix
	}
	return DefaultHeaderTitleSuffix
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

// GetLoginRightSideImageDark returns the customer login page right side image for dark mode (base64 data URI)
func (t *AppTheme) GetLoginRightSideImageDark() string {
	return t.LoginRightSideImageBase64Dark
}

// GetLoginRightSideGradientDark returns the customer login page right side gradient CSS for dark mode
func (t *AppTheme) GetLoginRightSideGradientDark() string {
	return t.LoginRightSideGradientDark
}

// GetLogoForMode returns the appropriate logo based on the color scheme.
// If dark mode is requested and no dark logo is set, falls back to light logo.
func (t *AppTheme) GetLogoForMode(isDark bool) string {
	if isDark && t.LogoDarkBase64 != "" {
		return t.LogoDarkBase64
	}
	return t.LogoLightBase64
}

// GetColorsForMode returns the primary and secondary colors for the given mode.
// If dark mode is requested and no dark colors are set, falls back to light mode colors.
// If no light mode colors are set, falls back to defaults.
func (t *AppTheme) GetColorsForMode(isDark bool) (primary, secondary string) {
	if isDark {
		primary = t.PrimaryColorDark
		secondary = t.SecondaryColorDark
		// Fall back to light mode colors if dark not set
		if primary == "" {
			primary = t.PrimaryColor
		}
		if secondary == "" {
			secondary = t.SecondaryColor
		}
	} else {
		primary = t.PrimaryColor
		secondary = t.SecondaryColor
	}
	// Apply defaults
	if primary == "" {
		primary = DefaultPrimaryColor
	}
	if secondary == "" {
		secondary = primary
	}
	return
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
