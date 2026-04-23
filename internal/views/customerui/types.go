package customerui

import (
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

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

	// Install switcher
	Installs         []models.SidebarInstall // All installs visible in sidebar switcher
	CurrentInstallID string                  // ID of the currently viewed install (empty if not on detail page)
	ActiveTab        string                  // Active install detail tab ("overview", "stack", etc.)

	// Account switching
	ActiveAccount *models.CustomerAccount  // Currently active account (nil if no account)
	OtherAccounts []models.CustomerAccount // Other accounts the user can switch to

	// Sidebar
	SidebarMinimized bool // When true, sidebar renders in collapsed icon-only mode

	// API error state
	NuonAPIError string // Error message shown in banner at top of content area (empty = no error)

	// Vendor admin bar context
	OrgName      string // Human-readable org name shown in the vendor admin bar
	PortalDomain string // Portal domain shown in the vendor admin bar (e.g. "acme.customers.nuon.co")
	AdminURL     string // Absolute URL for the "Go to admin" link (e.g. "https://customers.nuon.co/admin/orgs")
}
