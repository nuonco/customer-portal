package overrides

import (
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// TemplateContext contains all data passed to override templates.
// This provides a unified data structure that both Templ and html/template can use.
type TemplateContext struct {
	// Layout data (available on all pages)
	OrgID         string     `json:"org_id,omitempty"` // Organization ID for template rendering
	Title         string     `json:"title"`
	User          *UserData  `json:"user,omitempty"`
	BasePath      string     `json:"base_path"`
	Theme         *ThemeData `json:"theme"`
	CSSPath       string     `json:"css_path"`
	CustomCSSPath string     `json:"custom_css_path,omitempty"`

	// Page-specific data (varies by page)
	PageData interface{} `json:"page_data,omitempty"`
}

// UserData contains user information for templates.
type UserData struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// ThemeData contains theme settings for templates.
type ThemeData struct {
	PrimaryColor       string `json:"primary_color"`
	PrimaryColorDark   string `json:"primary_color_dark"`
	SecondaryColor     string `json:"secondary_color"`
	SecondaryColorDark string `json:"secondary_color_dark"`
	WhiteColor         string `json:"white_color,omitempty"`       // Background color for light mode
	BlackColor         string `json:"black_color,omitempty"`       // Background color for dark mode
	WhiteColorDark     string `json:"white_color_dark,omitempty"`  // Optional: --theme-white override for dark mode
	BlackColorLight    string `json:"black_color_light,omitempty"` // Optional: --theme-black override for light mode
	LogoBase64         string `json:"logo_base64,omitempty"`       // Backward compatibility (alias for LogoLightBase64)
	LogoLightBase64    string `json:"logo_light_base64,omitempty"` // Light mode logo
	LogoDarkBase64     string `json:"logo_dark_base64,omitempty"`  // Dark mode logo
	HeadingFont        string `json:"heading_font,omitempty"`
	BodyFont           string `json:"body_font,omitempty"`
	HeadingFontBase64  string `json:"heading_font_base64,omitempty"`
	BodyFontBase64     string `json:"body_font_base64,omitempty"`
	BorderRadius       string `json:"border_radius"`
	LoginTitle         string `json:"login_title"`
	LoginSubtitle      string `json:"login_subtitle"`
	RadiusClass        string `json:"radius_class"`
}

// Page-specific data types

// LoginPageData contains data for the login page.
type LoginPageData struct {
	AuthURL string `json:"auth_url"`
	Error   string `json:"error,omitempty"`
}

// RegisterPageData contains data for the register page.
type RegisterPageData struct {
	AuthURL string `json:"auth_url"`
	Error   string `json:"error,omitempty"`
}

// InstallsPageData contains data for the installs list page.
type InstallsPageData struct {
	Installs            []InstallData `json:"installs"`
	CurrentTab          string        `json:"current_tab"`
	TotalCount          int64         `json:"total_count"`
	NeedsAttentionCount int64         `json:"needs_attention_count"`
	UpdatingCount       int64         `json:"updating_count"`
	HealthyCount        int64         `json:"healthy_count"`
	CurrentPage         int           `json:"current_page"`
	TotalPages          int           `json:"total_pages"`
	HasPrevious         bool          `json:"has_previous"`
	HasNext             bool          `json:"has_next"`
	PreviousPage        int           `json:"previous_page"`
	NextPage            int           `json:"next_page"`
	ShowingFrom         int           `json:"showing_from"`
	ShowingTo           int           `json:"showing_to"`
}

// InstallData contains data for a single install.
type InstallData struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Status              string `json:"status"`
	Region              string `json:"region"`
	Platform            string `json:"platform"`
	AppName             string `json:"app_name"`
	CreatedAt           string `json:"created_at"`
	HasHealthChecks     bool   `json:"has_health_checks"`
	HealthChecksPassed  int    `json:"health_checks_passed"`
	HealthChecksPending int    `json:"health_checks_pending"`
	HealthChecksFailed  int    `json:"health_checks_failed"`
	HasPendingApproval  bool   `json:"has_pending_approval"`
}

// InstallDetailPageData contains data for the install detail page.
type InstallDetailPageData struct {
	Install         InstallData       `json:"install"`
	Workflows       []WorkflowData    `json:"workflows"`
	RecentWorkflows []WorkflowData    `json:"recent_workflows"`
	HealthChecks    []HealthCheckData `json:"health_checks"`
	HasHealthChecks bool              `json:"has_health_checks"`
	PendingApproval *WorkflowData     `json:"pending_approval,omitempty"`
}

// WorkflowData contains data for a workflow.
type WorkflowData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	FinishedAt  string `json:"finished_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

// HealthCheckData contains data for a health check.
type HealthCheckData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

// InstallLinkPageData contains data for the install link acceptance page.
type InstallLinkPageData struct {
	SHA          string                 `json:"sha"`
	AppName      string                 `json:"app_name"`
	Regions      []RegionData           `json:"regions"`
	Inputs       []InputData            `json:"inputs,omitempty"`
	VendorInputs map[string]interface{} `json:"vendor_inputs,omitempty"`
	Error        string                 `json:"error,omitempty"`
	SubmitURL    string                 `json:"submit_url"`
}

// RegionData contains data for a region option.
type RegionData struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Cloud string `json:"cloud"`
}

// InputData contains data for a customer input field.
type InputData struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Group       string `json:"group,omitempty"`
}

// WorkflowsPageData contains data for the workflows page.
type WorkflowsPageData struct {
	Install   InstallData    `json:"install"`
	Workflows []WorkflowData `json:"workflows"`
}

// ErrorPageData contains data for the error page.
type ErrorPageData struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// NewUserData creates a UserData from a models.User.
func NewUserData(user *models.User) *UserData {
	if user == nil {
		return nil
	}
	return &UserData{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
		Role:  string(user.Role),
	}
}

// NewThemeData creates a ThemeData from a models.AppTheme.
func NewThemeData(theme *models.AppTheme) *ThemeData {
	if theme == nil {
		return &ThemeData{
			PrimaryColor:   models.DefaultPrimaryColor,
			SecondaryColor: models.DefaultPrimaryColor,
			BorderRadius:   models.DefaultBorderRadius,
			LoginTitle:     models.DefaultLoginTitle,
			LoginSubtitle:  models.DefaultLoginSubtitle,
			RadiusClass:    "radius-" + models.DefaultBorderRadius,
		}
	}

	primaryColor := theme.PrimaryColor
	if primaryColor == "" {
		primaryColor = models.DefaultPrimaryColor
	}

	secondaryColor := theme.SecondaryColor
	if secondaryColor == "" {
		secondaryColor = primaryColor
	}

	return &ThemeData{
		PrimaryColor:       primaryColor,
		PrimaryColorDark:   darkenColor(primaryColor),
		SecondaryColor:     secondaryColor,
		SecondaryColorDark: darkenColor(secondaryColor),
		WhiteColor:         theme.WhiteColor,
		BlackColor:         theme.BlackColor,
		WhiteColorDark:     theme.WhiteColorDark,
		BlackColorLight:    theme.BlackColorLight,
		LogoBase64:         theme.LogoLightBase64, // Backward compatibility
		LogoLightBase64:    theme.LogoLightBase64,
		LogoDarkBase64:     theme.LogoDarkBase64,
		HeadingFont:        theme.HeadingFont,
		BodyFont:           theme.BodyFont,
		HeadingFontBase64:  theme.HeadingFontBase64,
		BodyFontBase64:     theme.BodyFontBase64,
		BorderRadius:       theme.BorderRadius,
		LoginTitle:         theme.GetLoginTitle(),
		LoginSubtitle:      theme.GetLoginSubtitle(),
		RadiusClass:        theme.GetRadiusClass(),
	}
}

// darkenColor darkens a hex color by 20%.
func darkenColor(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}

	// Parse RGB values
	r := hexToInt(hex[1:3])
	g := hexToInt(hex[3:5])
	b := hexToInt(hex[5:7])

	// Darken by 20%
	r = int(float64(r) * 0.8)
	g = int(float64(g) * 0.8)
	b = int(float64(b) * 0.8)

	return "#" + intToHex(r) + intToHex(g) + intToHex(b)
}

func hexToInt(s string) int {
	var result int
	for _, c := range s {
		result *= 16
		if c >= '0' && c <= '9' {
			result += int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			result += int(c - 'a' + 10)
		} else if c >= 'A' && c <= 'F' {
			result += int(c - 'A' + 10)
		}
	}
	return result
}

func intToHex(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	chars := "0123456789ABCDEF"
	return string(chars[n/16]) + string(chars[n%16])
}
