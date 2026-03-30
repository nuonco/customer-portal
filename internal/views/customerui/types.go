package customerui

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// InstallWithApprovalStatus extends Install with approval status info
type InstallWithApprovalStatus struct {
	models.Install
	AppName  string `json:"app_name"` // Fetched from Nuon API at runtime
	Platform string `json:"platform"` // Cloud platform (e.g. "aws", "azure", "gcp") from Nuon API
}

// InstallPaginationData holds pagination metadata for customer installs
type InstallPaginationData struct {
	Installs     []InstallWithApprovalStatus `json:"installs"`
	CurrentPage  int                         `json:"current_page"`
	TotalPages   int                         `json:"total_pages"`
	HasPrevious  bool                        `json:"has_previous"`
	HasNext      bool                        `json:"has_next"`
	PreviousPage int                         `json:"previous_page"`
	NextPage     int                         `json:"next_page"`
	TotalCount   int64                       `json:"total_count"`
	PerPage      int                         `json:"per_page"`
	ShowingFrom  int                         `json:"showing_from"`
	ShowingTo    int                         `json:"showing_to"`
	PageNumbers  []int                       `json:"page_numbers"`
}

// WorkflowData holds processed workflow information for display
type WorkflowData struct {
	ID                       string
	Name                     string
	Status                   string
	StatusClass              string
	CreatedAt                time.Time
	FinishedAt               time.Time
	CanApprove               bool
	CanApproveAll            bool
	CanCancel                bool
	ApprovalStep             *ApprovalStepData
	ApproveDisabledReason    string
	ApproveAllDisabledReason string
	CancelDisabledReason     string
}

// ApprovalStepData holds approval step info for workflow actions
type ApprovalStepData struct {
	StepID     string
	ApprovalID string
}

// AuditLogEntry holds audit log entry data for display
type AuditLogEntry struct {
	LogLine   string
	TimeStamp time.Time
	Type      string
}

// Partial Component Props (for theme/partials/ components)

// HeaderProps contains props for the customer header component
type HeaderProps struct {
	Theme    *models.AppTheme
	User     *models.User
	BasePath string
}

// FooterProps contains props for the customer footer component
type FooterProps struct {
	Theme *models.AppTheme
}

// ThemeStylesProps contains props for theme CSS injection
type ThemeStylesProps struct {
	Theme *models.AppTheme
}

// LayoutProps contains all data needed for the customer layout
type LayoutProps struct {
	Title    string
	User     *models.User
	BasePath string

	// Theme settings (computed from vendor settings)
	PrimaryColor              string
	PrimaryColorDark          string
	PrimaryColorDarkMode      string // vendor-set dark mode primary (overrides --theme-primary in .dark{})
	PrimaryColorDarkModeHover string // darkened variant of PrimaryColorDarkMode for hover states
	WhiteColor                string
	BlackColor                string
	WhiteColorDark            string
	BlackColorLight           string
	HeadingFont               string
	BodyFont                  string
	HeadingFontBase64         string
	BodyFontBase64            string
	LogoBase64                string
	LogoDarkBase64            string
	FaviconBase64             string

	// Style variants
	RadiusClass string // "radius-sharp", "radius-subtle", "radius-rounded", "radius-very-rounded"
	ThemeMode   string // "auto", "light", or "dark"

	// Asset paths (cache-busted)
	CSSPath       string // main customer stylesheet (cache-busted)
	CustomCSSPath string // org-specific custom CSS (empty when not configured)

	// Header
	HeaderTitle string // Resolved header title text (empty = hidden)

	// Navigation
	HasPublishedApps bool   // Whether the org has published apps (shows nav links when true)
	ActiveNav        string // "apps" or "installs" — highlights the current nav item

	// Account switching
	ActiveAccount *models.CustomerAccount  // Currently active account (nil if no account)
	OtherAccounts []models.CustomerAccount // Other accounts the user can switch to

	// Vendor admin bar context
	OrgName      string // Human-readable org name shown in the vendor admin bar
	PortalDomain string // Portal domain shown in the vendor admin bar (e.g. "acme.customers.nuon.co")
	AdminURL     string // Absolute URL for the "Go to admin" link (e.g. "https://customers.nuon.co/admin/orgs")
}
