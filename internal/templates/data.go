package templates

import (
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// ThemeData contains theme customization data for templates
type ThemeData struct {
	PrimaryColor       string
	PrimaryColorDark   string
	SecondaryColor     string
	SecondaryColorDark string
	HeadingFont        string
	BodyFont           string
	HeadingFontBase64  string
	BodyFontBase64     string
	LogoBase64         string
	SupportContact     string
	RadiusClass        string
	DensityClass       string
}

// NewThemeData creates ThemeData from an AppTheme model
func NewThemeData(theme *models.AppTheme) *ThemeData {
	if theme == nil {
		return &ThemeData{
			PrimaryColor:     models.DefaultPrimaryColor,
			PrimaryColorDark: DarkenColor(models.DefaultPrimaryColor, 0.8),
			RadiusClass:      "radius-" + models.DefaultBorderRadius,
			DensityClass:     "density-" + models.DefaultSpacingDensity,
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
		PrimaryColorDark:   DarkenColor(primaryColor, 0.8),
		SecondaryColor:     secondaryColor,
		SecondaryColorDark: DarkenColor(secondaryColor, 0.8),
		HeadingFont:        theme.HeadingFont,
		BodyFont:           theme.BodyFont,
		HeadingFontBase64:  theme.HeadingFontBase64,
		BodyFontBase64:     theme.BodyFontBase64,
		LogoBase64:         theme.LogoBase64,
		SupportContact:     theme.SupportContact,
		RadiusClass:        theme.GetRadiusClass(),
		DensityClass:       theme.GetDensityClass(),
	}
}

// Breadcrumb represents a navigation breadcrumb
type Breadcrumb struct {
	Text   string
	Path   string
	Active bool
}

// EntityAction represents an action button in the title bar
type EntityAction struct {
	Label   string
	Href    string
	Variant string // "primary", "secondary", "danger"
	Icon    string
	OnClick string
	HxPost  string
	HxGet   string
}

// BaseData contains data common to all templates
type BaseData struct {
	BasePath string
	User     *models.User
	Theme    *ThemeData
	CSSPath  string
}

// CustomerLayoutData contains data for the customer layout
type CustomerLayoutData struct {
	BaseData
	Title string
}

// VendorLayoutData contains data for the vendor layout
type VendorLayoutData struct {
	BaseData
	Title         string
	ActivePage    string // "orgs", "links", "apps"
	CurrentOrg    *models.NuonOrg
	Orgs          []models.NuonOrg
	Breadcrumbs   []Breadcrumb
	EntityTitle   string
	EntityID      string
	EntityActions []EntityAction
}

// LoginPageData for login pages
type LoginPageData struct {
	Title        string
	ButtonText   string
	HelpText     string
	BasePath     string
	RedirectURL  string
	Theme        *ThemeData
	UseOIDC      bool
	AuthURL      string
	ProviderName string
	Error        string
	CSSPath      string
	UseLocalAuth bool
}

// RegisterPageData for registration pages
type RegisterPageData struct {
	Title       string
	BasePath    string
	Theme       *ThemeData
	Error       string
	CSSPath     string
	RedirectURL string
	LinkSHA     string
}

// ErrorPageData for error pages
type ErrorPageData struct {
	Title        string
	Error        string
	BasePath     string
	PrimaryColor string
	Theme        *ThemeData
	CSSPath      string
}

// PaginationData holds pagination metadata
type PaginationData struct {
	CurrentPage  int
	TotalPages   int
	HasPrevious  bool
	HasNext      bool
	PreviousPage int
	NextPage     int
	TotalCount   int64
	PerPage      int
	ShowingFrom  int
	ShowingTo    int
}

// InstallPaginationData holds pagination data for installs page
type InstallPaginationData struct {
	PaginationData
	CurrentTab          string
	NeedsAttentionCount int
	UpdatingCount       int
	HealthyCount        int
	Installs            []InstallWithStatus
}

// InstallWithStatus represents an install with its current status
type InstallWithStatus struct {
	Install       *models.Install
	Status        string // "healthy", "updating", "needs_attention"
	ApprovalCount int
	HealthStatus  string
	AppName       string
}

// InstallsPageData for the installs list page
type InstallsPageData struct {
	CustomerLayoutData
	Pagination InstallPaginationData
}

// InstallDetailPageData for install detail page
type InstallDetailPageData struct {
	CustomerLayoutData
	Install       *models.Install
	AppName       string
	Inputs        []InputField
	HealthChecks  []HealthCheckStatus
	RecentActions []WorkflowSummary
}

// InstallLinkPageData for install link acceptance page
type InstallLinkPageData struct {
	CustomerLayoutData
	Link       *models.InstallLink
	InstallURL string
	AppName    string
	Inputs     []InputField
}

// WorkflowsPageData for workflows list page
type WorkflowsPageData struct {
	CustomerLayoutData
	Workflows  []WorkflowSummary
	Pagination PaginationData
}

// OrgsPageData for orgs list page
type OrgsPageData struct {
	VendorLayoutData
	Orgs []models.NuonOrg
}

// OrgDetailPageData for org detail page (install links)
type OrgDetailPageData struct {
	VendorLayoutData
	Org        models.NuonOrg
	Links      []models.InstallLink
	Pagination LinkPaginationData
}

// LinkPaginationData holds pagination data for links
type LinkPaginationData struct {
	PaginationData
	Links          []models.InstallLink
	CurrentTab     string
	AvailableCount int64
	UsedCount      int64
}

// LinkDetailPageData for install link detail page
type LinkDetailPageData struct {
	VendorLayoutData
	Link       *models.InstallLink
	InstallURL string
	Inputs     []InputFieldValue
}

// AppsPageData for apps list page
type AppsPageData struct {
	VendorLayoutData
	Org  models.NuonOrg
	Apps []AppWithHealthCheckStatus
}

// AppWithHealthCheckStatus represents an app with health check info
type AppWithHealthCheckStatus struct {
	ID               string
	Name             string
	Platform         string
	HasHealthChecks  bool
	HealthCheckCount int
}

// AppInputsPageData for app inputs configuration page
type AppInputsPageData struct {
	VendorLayoutData
	Org                    models.NuonOrg
	App                    AppInfo
	AllInputs              []InputField
	VendorInputs           []InputField
	CustomerInputs         []InputField
	HasVendorInputs        bool
	HasCustomerInputs      bool
	ShowCustomerAuthPrompt bool
}

// AppHealthChecksPageData for app health checks configuration page
type AppHealthChecksPageData struct {
	VendorLayoutData
	Org                models.NuonOrg
	App                AppInfo
	HealthChecks       []HealthCheckConfig
	SelectedActionIDs  []string
	HasNoHealthChecks  bool
	ShowHealthCheckTip bool
}

// ThemeSettingsPageData for theme settings page
type ThemeSettingsPageData struct {
	VendorLayoutData
	Theme           *models.AppTheme
	PreviewURL      string
	UploadEndpoints UploadEndpoints
}

// UploadEndpoints contains endpoints for file uploads
type UploadEndpoints struct {
	Logo        string
	HeadingFont string
	BodyFont    string
}

// CustomerAuthSettingsPageData for customer auth settings page
type CustomerAuthSettingsPageData struct {
	VendorLayoutData
	Config          *models.CustomerAuthConfig
	UseLocalAuth    bool
	ConnectionValid bool
	TestEndpoint    string
}

// OrgSettingsPageData for org settings page
type OrgSettingsPageData struct {
	VendorLayoutData
	Org models.NuonOrg
}

// InputField represents a form input field
type InputField struct {
	Name         string
	Label        string
	Description  string
	Type         string // "text", "number", "boolean", "select"
	Required     bool
	Default      string
	Options      []string // For select type
	Source       string   // "vendor", "customer", or empty for both
	DisplayOrder int
}

// InputFieldValue represents an input field with its current value
type InputFieldValue struct {
	InputField
	Value string
}

// HealthCheckConfig represents a health check configuration
type HealthCheckConfig struct {
	ID          string
	Name        string
	Description string
	Type        string
	Selected    bool
	WorkflowID  string
}

// HealthCheckStatus represents the status of a health check
type HealthCheckStatus struct {
	ID        string
	Name      string
	Status    string // "healthy", "unhealthy", "unknown", "running"
	LastRun   string
	Message   string
	CanRerun  bool
	Workflows []WorkflowSummary
}

// WorkflowSummary represents a workflow execution summary
type WorkflowSummary struct {
	ID            string
	Name          string
	Status        string
	StartedAt     string
	CompletedAt   string
	Duration      string
	NeedsApproval bool
	Steps         []WorkflowStep
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	ID          string
	Name        string
	Status      string
	StartedAt   string
	CompletedAt string
	Output      string
	CanApprove  bool
}

// AppInfo represents basic app information
type AppInfo struct {
	ID       string
	Name     string
	Platform string
}

// AlertData for alert component
type AlertData struct {
	Type    string // "info", "success", "warning", "error"
	Title   string
	Message string
	Class   string
}

// EmptyStateData for empty state component
type EmptyStateData struct {
	Icon    string
	Title   string
	Message string
	Action  *ButtonData
}

// ButtonData for button component
type ButtonData struct {
	Text         string
	Type         string // "button", "submit"
	Variant      string // "primary", "secondary", "danger", "ghost", "link"
	Size         string // "xs", "sm", "md", "lg", "xl"
	Href         string
	Disabled     bool
	Class        string
	OnClick      string
	PrimaryColor string
	// HTMX attributes
	HxPost      string
	HxGet       string
	HxDelete    string
	HxPut       string
	HxTarget    string
	HxSwap      string
	HxConfirm   string
	HxVals      string
	HxIndicator string
	ShowSpinner bool
}

// StatusBadgeData for status badge component
type StatusBadgeData struct {
	Status string
	Size   string // "xs", "sm", "md"
	Class  string
}

// ToastData for toast notification
type ToastData struct {
	ID      string
	Type    string // "success", "error", "warning", "info"
	Title   string
	Message string
}

// ConfirmModalData for confirmation modal
type ConfirmModalData struct {
	ID           string
	Title        string
	Message      string
	ConfirmText  string
	CancelText   string
	ConfirmClass string
}

// LinkStatusData for link status partial (HTMX polling)
type LinkStatusData struct {
	Link       *models.InstallLink
	InstallURL string
	BasePath   string
	OrgID      string
	Theme      *ThemeData
}

// ThemePanelData for theme settings panel (HTMX lazy load)
type ThemePanelData struct {
	Theme           *models.AppTheme
	BasePath        string
	UploadEndpoints UploadEndpoints
}

// ProfilePanelData for profile settings panel (HTMX lazy load)
type ProfilePanelData struct {
	User     *models.User
	BasePath string
}

// CustomerAuthPanelData for customer auth settings panel (HTMX lazy load)
type CustomerAuthPanelData struct {
	Config       *models.CustomerAuthConfig
	BasePath     string
	UseLocalAuth bool
	TestEndpoint string
}

// SidebarData for sidebar partial
type SidebarData struct {
	ActivePage string
	CurrentOrg *models.NuonOrg
	Orgs       []models.NuonOrg
	BasePath   string
	LogoBase64 string
}

// TopbarData for topbar partial
type TopbarData struct {
	Breadcrumbs []Breadcrumb
	User        *models.User
	BasePath    string
}

// TitleBarData for entity title bar partial
type TitleBarData struct {
	EntityTitle   string
	EntityID      string
	EntityActions []EntityAction
}

// AppSubnavData for app subnav partial
type AppSubnavData struct {
	ActiveTab string // "inputs", "health-checks"
	OrgID     string
	AppID     string
	BasePath  string
}

// WorkflowCardData for workflow card component
type WorkflowCardData struct {
	Workflow       WorkflowSummary
	InstallID      string
	BasePath       string
	SecondaryColor string
	ShowActions    bool
}

// HealthStatusData for health status component
type HealthStatusData struct {
	InstallID    string
	HealthChecks []HealthCheckStatus
	BasePath     string
	CanRunChecks bool
}
