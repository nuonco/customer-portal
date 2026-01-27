package customerui

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// InstallWithApprovalStatus extends Install with approval and health check info
type InstallWithApprovalStatus struct {
	models.Install
	HasPendingApprovals bool   `json:"has_pending_approvals"`
	IsUpdating          bool   `json:"is_updating"` // has in-progress workflow without approval steps
	HasHealthChecks     bool   `json:"has_health_checks"`
	HealthChecksPending int    `json:"health_checks_pending"`
	HealthChecksPassed  int    `json:"health_checks_passed"`
	HealthChecksFailed  int    `json:"health_checks_failed"`
	OverallHealthStatus string `json:"overall_health_status"` // "passing", "pending", "failing", ""
}

// InstallPaginationData holds pagination metadata for customer installs
type InstallPaginationData struct {
	Installs            []InstallWithApprovalStatus `json:"installs"`
	CurrentPage         int                         `json:"current_page"`
	TotalPages          int                         `json:"total_pages"`
	HasPrevious         bool                        `json:"has_previous"`
	HasNext             bool                        `json:"has_next"`
	PreviousPage        int                         `json:"previous_page"`
	NextPage            int                         `json:"next_page"`
	TotalCount          int64                       `json:"total_count"`
	PerPage             int                         `json:"per_page"`
	ShowingFrom         int                         `json:"showing_from"`
	ShowingTo           int                         `json:"showing_to"`
	PageNumbers         []int                       `json:"page_numbers"`
	CurrentTab          string                      `json:"current_tab"`
	NeedsAttentionCount int64                       `json:"needs_attention_count"`
	HealthyCount        int64                       `json:"healthy_count"`
	UpdatingCount       int64                       `json:"updating_count"`
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

// HealthCheckStatusData holds health check status for display
type HealthCheckStatusData struct {
	ActionID      string
	ActionName    string
	Status        string
	StatusClass   string
	StatusMessage string
	LastRunAt     time.Time
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
	Theme          *models.AppTheme
	SupportContact string
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

	// Style variants
	RadiusClass string // "radius-sharp", "radius-subtle", "radius-rounded", "radius-very-rounded"

	// Asset paths (cache-busted)
	CSSPath string
}

// GetSupportContact returns the support contact from theme or empty string
func (p *FooterProps) GetSupportContact() string {
	if p.SupportContact != "" {
		return p.SupportContact
	}
	if p.Theme != nil && p.Theme.SupportContact != "" {
		return p.Theme.SupportContact
	}
	return ""
}

// GetSupportHref returns the support href (mailto: or http://)
func (p *FooterProps) GetSupportHref() string {
	contact := p.GetSupportContact()
	if contact == "" {
		return ""
	}
	if len(contact) > 4 && contact[:4] == "http" {
		return contact
	}
	return "mailto:" + contact
}
