package workflows

import (
	"fmt"
	"strings"
	"time"

	nuonmodels "github.com/nuonco/nuon-go/models"
)

// WorkflowDataPanel represents workflow data for the panel (avoids import cycle)
type WorkflowDataPanel struct {
	ID                       string
	Name                     string
	Status                   string
	StatusClass              string
	CreatedAt                time.Time
	FinishedAt               time.Time
	CanApprove               bool
	CanApproveAll            bool
	CanCancel                bool
	HasApprovalSteps         bool
	ApproveDisabledReason    string
	ApproveAllDisabledReason string
	CancelDisabledReason     string
	ApprovalStep             *ApprovalStepDataPanel
	CurrentStepName          string            // Name of the currently executing step
	CurrentStepStatus        string            // Status of the current step (in-progress, approval-awaiting)
	CurrentStepType          string            // Type of current step: "pending", "in-progress", "approval-awaiting", or "initializing"
	CurrentStepNumber        int               // 1-indexed position of current step (1, 2, 3...)
	TotalSteps               int               // Total number of steps in workflow
	FailedStepID             string            // ID of the failed step (for retry)
	FailedStepName           string            // Name of the failed step (for display)
	FailedStepRetryable      bool              // Whether the failed step can be retried
	CurrentStepRole          string            // IAM role used by the current step target
	IsReprovision            bool              // Whether this is a reprovision workflow
	DenyViolations           []PolicyViolation // Policy deny violations for current step
	WarnViolations           []PolicyViolation // Policy warn violations for current step
	HasPolicyData            bool              // Whether the current step has any policy evaluation data
	DisablePolling           bool              // When true, disables HTMX polling (e.g. secondary panel views)
}

// PolicyViolation represents a single policy evaluation violation.
type PolicyViolation struct {
	PolicyID string
	Message  string
	Severity string // "deny" or "warn"
}

// ApprovalStepDataPanel for workflow approval steps
type ApprovalStepDataPanel struct {
	StepID     string
	ApprovalID string
}

// StackSetupData carries platform-specific data for the "await install stack" step.
type StackSetupData struct {
	Platform           string // "aws", "gcp", "azure", etc.
	CloudFormationLink string // AWS only
	TfvarsContent      string // GCP only
	NuonInstallID      string // GCP + Azure (for backend snippet / resource naming)
	AzureTemplateURL   string // Azure only — ARM template URL
	AzureLocation      string // Azure only — deployment location (e.g. "eastus")
}

// WorkflowData holds processed workflow information for display in WorkflowCard.
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

// IsTerminalWorkflowStatus returns true for terminal workflow statuses.
func IsTerminalWorkflowStatus(status string) bool {
	return status == "completed" || status == "success" || status == "error" || status == "cancelled"
}

// FormatConfigDate formats a date string for display
func FormatConfigDate(dateStr string) string {
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t.Format("Jan 2, 2006")
		}
	}
	return dateStr
}

// GcpBackendSnippet returns the GCS backend config snippet
func GcpBackendSnippet(installID string) string {
	return fmt.Sprintf(`terraform {
  backend "gcs" {
    bucket = "<your-state-bucket>"
    prefix = "nuon/%s"
  }
}`, installID)
}

// GcpApplyCmd returns the terraform apply command
func GcpApplyCmd() string {
	return `terraform init && terraform apply -var-file=install.tfvars`
}

// StepGroupType identifies the kind of step group for rendering purposes.
type StepGroupType string

const (
	StepGroupStack     StepGroupType = "stack"
	StepGroupSandbox   StepGroupType = "sandbox"
	StepGroupComponent StepGroupType = "component"
	StepGroupAction    StepGroupType = "action"
	StepGroupOther     StepGroupType = "other"
)

// StepGroup represents a logical group of workflow steps sharing the same GroupIdx.
type StepGroup struct {
	GroupIdx int64
	Type     StepGroupType
	Name     string
	Status   string // not_started, in-progress, completed, error, approval-awaiting
	Steps    []*nuonmodels.AppWorkflowStep
}

// ActiveStep returns the first non-completed step in the group, or nil.
func (g StepGroup) ActiveStep() *nuonmodels.AppWorkflowStep {
	for _, s := range g.Steps {
		if s.Status == nil {
			return s
		}
		status := string(s.Status.Status)
		if status != "completed" && status != "success" && status != "approved" {
			return s
		}
	}
	return nil
}

// CanApprove returns true if the active step is awaiting approval.
func (g StepGroup) CanApprove() bool {
	s := g.ActiveStep()
	return s != nil && s.Status != nil && string(s.Status.Status) == "approval-awaiting" && s.Approval != nil
}

// CanRetry returns true if the group has a retryable error step.
func (g StepGroup) CanRetry() bool {
	return g.RetryableStep() != nil
}

// RetryableStep returns the retryable error step in the group, or nil.
func (g StepGroup) RetryableStep() *nuonmodels.AppWorkflowStep {
	for _, s := range g.Steps {
		if s.Retryable && !s.Finished && s.StartedAt != "" && s.Status != nil && string(s.Status.Status) == "error" {
			return s
		}
	}
	return nil
}

// HasApprovalStep returns true if any step in the group has execution_type "approval".
func (g StepGroup) HasApprovalStep() bool {
	for _, s := range g.Steps {
		if s.ExecutionType == "approval" {
			return true
		}
	}
	return false
}

// HasRetryableStep returns true if any step in the group is marked retryable.
func (g StepGroup) HasRetryableStep() bool {
	for _, s := range g.Steps {
		if s.Retryable {
			return true
		}
	}
	return false
}

// RetryableStepID returns the ID of the retryable error step, or empty string.
func (g StepGroup) RetryableStepID() string {
	s := g.RetryableStep()
	if s != nil {
		return s.ID
	}
	return ""
}

// WorkflowOverviewProps holds all data needed to render the WorkflowOverview component.
type WorkflowOverviewProps struct {
	StepGroups       []StepGroup
	SelectedGroup    int
	InstallID        string
	BasePath         string
	WorkflowID       string
	WorkflowFinished bool
	ShowApproveAll   bool
	Platform         string // "aws", "gcp", "azure"
	StackSetup       StackSetupData
	PageBaseURL      string // Full page URL up to workflow ID (for hx-push-url)
	PanelPartialURL  string // URL for partial=panel requests (swaps entire panel wrapper)
	Expanded         bool   // Panel is expanded (propagated into step group links)
}

// GroupPageURL returns the full page URL for a step group selection, preserving expanded state.
func (p WorkflowOverviewProps) GroupPageURL(groupIdx int) string {
	url := fmt.Sprintf("%s?group=%d", p.PageBaseURL, groupIdx)
	if p.Expanded {
		url += "&expanded=true"
	}
	return url
}

// GroupPanelURL returns the partial=panel URL for a step group selection (swaps entire panel).
func (p WorkflowOverviewProps) GroupPanelURL(groupIdx int) string {
	return fmt.Sprintf("%s&group=%d", p.PanelPartialURL, groupIdx)
}

// BuildStepGroups groups workflow steps by GroupIdx and derives type, name, and status.
func BuildStepGroups(steps []*nuonmodels.AppWorkflowStep) []StepGroup {
	if len(steps) == 0 {
		return nil
	}

	var groups []StepGroup
	var current *StepGroup
	for _, s := range steps {
		if current == nil || s.GroupIdx != current.GroupIdx {
			groups = append(groups, StepGroup{GroupIdx: s.GroupIdx})
			current = &groups[len(groups)-1]
		}
		current.Steps = append(current.Steps, s)
		groups[len(groups)-1] = *current
	}

	var visible []StepGroup
	for i := range groups {
		groups[i].Type = deriveGroupType(groups[i].Steps)
		groups[i].Name = deriveGroupName(groups[i].Steps)
		groups[i].Status = deriveGroupStatus(groups[i].Steps)
		if !isHiddenGroup(groups[i]) {
			visible = append(visible, groups[i])
		}
	}

	return visible
}

func deriveGroupType(steps []*nuonmodels.AppWorkflowStep) StepGroupType {
	for _, s := range steps {
		switch s.StepTargetType {
		case "install_stack_versions":
			return StepGroupStack
		case "install_sandbox_runs":
			return StepGroupSandbox
		case "install_deploys":
			return StepGroupComponent
		case "install_action_workflow_runs":
			return StepGroupAction
		}
	}
	if len(steps) > 0 {
		name := strings.ToLower(steps[0].Name)
		switch {
		case strings.Contains(name, "stack") || strings.Contains(name, "runner"):
			return StepGroupStack
		case strings.Contains(name, "sandbox"):
			return StepGroupSandbox
		case strings.Contains(name, "deploy") || strings.Contains(name, "sync_and_plan") || strings.Contains(name, "apply"):
			return StepGroupComponent
		}
	}
	return StepGroupOther
}

func deriveGroupName(steps []*nuonmodels.AppWorkflowStep) string {
	if len(steps) == 0 {
		return ""
	}
	name := strings.ToLower(strings.ReplaceAll(steps[0].Name, " ", "_"))

	// Stack steps
	if strings.Contains(name, "install_stack") || strings.Contains(name, "generate_install") {
		return "Deploy stack"
	}

	// Sandbox steps
	if strings.Contains(name, "sandbox_plan") || strings.Contains(name, "sandbox_apply") {
		return "Deploy sandbox"
	}

	// Component steps: "sync_and_plan_<name>" → "Deploy <name>"
	if strings.HasPrefix(name, "sync_and_plan_") {
		comp := strings.TrimPrefix(name, "sync_and_plan_")
		comp = strings.ReplaceAll(comp, "_", " ")
		if len(comp) > 0 {
			comp = strings.ToUpper(comp[:1]) + comp[1:]
		}
		return "Deploy " + comp
	}

	// Fallback: title-case the raw name
	display := strings.ReplaceAll(steps[0].Name, "_", " ")
	if len(display) > 0 {
		return strings.ToUpper(display[:1]) + display[1:]
	}
	return display
}

func deriveGroupStatus(steps []*nuonmodels.AppWorkflowStep) string {
	allCompleted := true
	for _, s := range steps {
		status := ""
		if s.Status != nil {
			status = string(s.Status.Status)
		}
		switch status {
		case "error":
			return "error"
		case "approval-awaiting":
			return "approval-awaiting"
		case "in-progress", "active":
			return "in-progress"
		case "completed", "success", "approved":
			continue
		default:
			allCompleted = false
		}
	}
	if allCompleted {
		return "completed"
	}
	return "not_started"
}

func isHiddenGroup(group StepGroup) bool {
	for _, s := range group.Steps {
		if s.ExecutionType == "user" || s.ExecutionType == "approval" {
			return false
		}
	}
	return true
}
